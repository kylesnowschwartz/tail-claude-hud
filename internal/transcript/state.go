// Package transcript handles incremental JSONL transcript reads for the statusline.
// state.go manages byte offset persistence between process invocations so each tick
// reads only the new bytes written since last time (O(delta) vs O(n)).
package transcript

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// stateFile is the JSON structure persisted to disk.
type stateFile struct {
	TranscriptPath string `json:"transcript_path"`
	ByteOffset     int64  `json:"byte_offset"`
	SessionStart   string `json:"session_start"` // RFC3339, informational
}

// StateManager handles byte-offset tracking for incremental reads.
type StateManager struct {
	stateDir   string
	offset     int64
	lastPath   string
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
func (sm *StateManager) loadState(transcriptPath string) stateFile {
	data, err := os.ReadFile(sm.stateFilePath(transcriptPath))
	if err != nil {
		return stateFile{}
	}
	var sf stateFile
	if json.Unmarshal(data, &sf) != nil {
		return stateFile{}
	}
	return sf
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
			// Truncated transcript: reset to beginning.
			startOffset = 0
		} else {
			startOffset = sf.ByteOffset
		}
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
	var segmentOffsets []int // byte offset of start of each segment
	start := 0
	for i, b := range data {
		if b == '\n' {
			segments = append(segments, data[start:i])
			segmentOffsets = append(segmentOffsets, start)
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
	var resultEnd int64 = confirmedEnd // bytes consumed through last valid line
	for i, seg := range segments {
		if len(seg) == 0 {
			continue
		}
		if json.Valid(seg) {
			result = append(result, string(seg))
			// Advance resultEnd to include this segment + its newline.
			// segmentOffsets[i] is the start; segment ends at segmentOffsets[i]+len(seg); newline at +1.
			resultEnd = int64(segmentOffsets[i]) + int64(len(seg)) + 1 // +1 for newline
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

	// Use the furthest valid position within confirmed (newline-terminated) bytes.
	_ = resultEnd // resultEnd is the furthest line we accepted
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
		TranscriptPath: transcriptPath,
		ByteOffset:     sm.offset,
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

	return os.Rename(tmp, target)
}
