// Package transcript handles incremental JSONL transcript reads for the statusline.
// state.go manages byte offset persistence between process invocations so each tick
// reads only the new bytes written since last time (O(delta) vs O(n)).
package transcript

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// stateFileTTL is how long a state file must go unmodified before it is
	// eligible for deletion. 30 days covers any realistic session gap.
	stateFileTTL = 30 * 24 * time.Hour

	// sweepOdds controls how often a tick triggers a stale-file sweep.
	// At 1-in-100, a session ticking at 5/s sweeps roughly every 20 seconds —
	// precise enough given the 30-day TTL.
	sweepOdds = 100
)

// stateSchemaVersion is bumped when extraction semantics change so that
// stale snapshots are discarded and the transcript is re-read from byte 0.
// Bump this whenever the extraction logic changes in a way that would
// produce different results from the same transcript data.
const stateSchemaVersion = 2 // v2: skill detection from <command-name> tags

// stateFile is the JSON structure persisted to disk.
type stateFile struct {
	SchemaVersion      int             `json:"schema_version,omitempty"`
	TranscriptPath     string          `json:"transcript_path"`
	ByteOffset         int64           `json:"byte_offset"`
	SessionStart       string          `json:"session_start"` // RFC3339, informational
	ExtractionSnapshot json.RawMessage `json:"extraction_snapshot,omitempty"`
}

// StateManager handles byte-offset tracking for incremental reads.
type StateManager struct {
	stateDir       string
	offset         int64
	lastPath       string
	snapshot       json.RawMessage // set by SetSnapshot; included in next SaveState
	loadedSnapshot json.RawMessage // loaded from disk by loadState; returned by LoadSnapshot
}

// NewStateManager creates a manager using the given directory for state files.
func NewStateManager(stateDir string) *StateManager {
	return &StateManager{stateDir: stateDir}
}

// pathHash returns the first 12 characters of the SHA-256 hex digest of a path.
// This is the key used in the state file name.
func pathHash(path string) string {
	sum := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", sum)[:12]
}

// stateFilePath returns the full path to the state file for a given transcript path.
func (sm *StateManager) stateFilePath(transcriptPath string) string {
	name := ".ts-" + pathHash(transcriptPath) + ".json"
	return filepath.Join(sm.stateDir, name)
}

// loadState reads and parses the state file. Returns zero-value stateFile if
// the file is missing or contains invalid JSON (spec: start from byte 0).
// As a side-effect it stores the extraction_snapshot in sm.loadedSnapshot so
// callers can retrieve it via LoadSnapshot after calling ReadIncremental.
func (sm *StateManager) loadState(transcriptPath string) stateFile {
	data, err := os.ReadFile(sm.stateFilePath(transcriptPath))
	if err != nil {
		sm.loadedSnapshot = nil
		return stateFile{}
	}
	var sf stateFile
	if json.Unmarshal(data, &sf) != nil {
		sm.loadedSnapshot = nil
		return stateFile{}
	}
	// Schema mismatch: extraction logic has changed since this snapshot was
	// written. Discard it so the transcript is re-read from byte 0.
	if sf.SchemaVersion != stateSchemaVersion {
		sm.loadedSnapshot = nil
		return stateFile{}
	}
	sm.loadedSnapshot = sf.ExtractionSnapshot
	return sf
}

// LoadSnapshot returns the extraction snapshot that was loaded from disk during
// the most recent ReadIncremental call. Returns nil when no snapshot is
// available (e.g., first run, corrupt state, or path mismatch).
func (sm *StateManager) LoadSnapshot() json.RawMessage {
	return sm.loadedSnapshot
}

// SetSnapshot stores data so it will be included in the next SaveState call.
func (sm *StateManager) SetSnapshot(data json.RawMessage) {
	sm.snapshot = data
}

// ReadIncremental reads new lines from the transcript since the last read.
// It returns complete, valid-JSON lines only. A partial last line (mid-write)
// is discarded; the offset is not advanced past it so the next tick picks it up.
//
// Reset conditions (start from byte 0):
//   - State file missing or corrupt
//   - Stored path differs (new session)
//   - Stored offset exceeds current file size (truncation)
func (sm *StateManager) ReadIncremental(transcriptPath string) ([]string, error) {
	sf := sm.loadState(transcriptPath)

	f, err := os.Open(transcriptPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Determine start offset.
	var startOffset int64
	if sf.TranscriptPath == transcriptPath && sf.ByteOffset > 0 {
		fi, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if sf.ByteOffset > fi.Size() {
			// Truncated transcript: reset to beginning and discard snapshot.
			startOffset = 0
			sm.loadedSnapshot = nil
		} else {
			startOffset = sf.ByteOffset
		}
	} else if sf.TranscriptPath != "" && sf.TranscriptPath != transcriptPath {
		// Path mismatch (new session): discard snapshot.
		sm.loadedSnapshot = nil
	}

	if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
		return nil, err
	}

	newBytes, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	lines, consumed := splitLines(newBytes)
	sm.offset = startOffset + consumed
	sm.lastPath = transcriptPath

	return lines, nil
}

// splitLines splits raw bytes into complete lines, discarding the last segment
// if it is not valid JSON (partial write protection).
//
// Returns the valid lines and the number of bytes consumed (excluding any
// discarded partial last line).
func splitLines(data []byte) (lines []string, consumed int64) {
	if len(data) == 0 {
		return nil, 0
	}

	// Split on newlines.
	var segments [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			segments = append(segments, data[start:i])
			start = i + 1
		}
	}
	// Any remaining bytes after the last newline form a trailing segment.
	trailing := data[start:]

	// Determine how many bytes are "confirmed complete".
	// If there is a trailing segment (no trailing newline), we need to check
	// whether it is valid JSON before including it.
	confirmedEnd := int64(start) // bytes up to and including the last newline

	// Collect valid JSON lines from newline-terminated segments.
	var result []string
	for _, seg := range segments {
		if len(seg) == 0 {
			continue
		}
		if json.Valid(seg) {
			result = append(result, string(seg))
		}
		// Invalid JSON lines within the file are skipped but we still advance past them
		// (they are complete lines that happen to be non-JSON or malformed entries).
		// The offset advances to confirmedEnd regardless.
	}

	// Handle trailing (no trailing newline): check if valid JSON.
	if len(trailing) > 0 {
		if json.Valid(trailing) {
			// It's a complete line that just happens to lack a newline yet.
			// But per spec, we must not advance past a line that may be a partial write.
			// A line without a trailing newline could be mid-write, so we discard it
			// and do not advance the offset past confirmedEnd.
			_ = trailing // discard: may be partial write
		}
		// Either way, do not advance past confirmedEnd for trailing bytes.
	}

	consumed = confirmedEnd

	return result, consumed
}

// SaveState persists the current offset to disk atomically.
// Writes to a temp file then renames to prevent partial reads from concurrent processes.
func (sm *StateManager) SaveState(transcriptPath string) error {
	if err := os.MkdirAll(sm.stateDir, 0o755); err != nil {
		return err
	}

	sf := stateFile{
		SchemaVersion:      stateSchemaVersion,
		TranscriptPath:     transcriptPath,
		ByteOffset:         sm.offset,
		ExtractionSnapshot: sm.snapshot,
	}

	data, err := json.Marshal(sf)
	if err != nil {
		return err
	}

	target := sm.stateFilePath(transcriptPath)
	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	if err := os.Rename(tmp, target); err != nil {
		return err
	}

	if rand.Intn(sweepOdds) == 0 {
		sm.sweepStaleStateFiles()
	}

	return nil
}

// sweepStaleStateFiles removes state files that have not been modified in
// stateFileTTL. It is best-effort: errors are silently ignored so a failed
// sweep never disrupts the normal write path.
func (sm *StateManager) sweepStaleStateFiles() {
	entries, err := os.ReadDir(sm.stateDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-stateFileTTL)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".ts-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(sm.stateDir, name)) //nolint:errcheck
		}
	}
}
