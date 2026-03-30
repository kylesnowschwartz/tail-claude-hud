// Package heartbeat manages session heartbeat files for multi-session awareness.
//
// Each Claude Code session writes a heartbeat file to
// ~/.config/tail-claude-hud/sessions/{session_id} on every hook event
// (PreToolUse, PostToolUse). The statusline gather stage scans this directory
// to discover other running sessions and display them in the sessions widget.
//
// Heartbeat freshness is determined by file modtime:
//   - Running: modtime < 30s ago
//   - Idle: modtime between 30s and 120s ago
//   - Stale: modtime > 120s ago (ignored)
package heartbeat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// runningTTL is the maximum age of a heartbeat before it is considered idle.
// A session whose heartbeat was updated within this window is "running".
const runningTTL = 30 * time.Second

// staleTTL is the maximum age of a heartbeat before it is considered stale
// and ignored. Covers the case where a session exits without cleanup.
const staleTTL = 120 * time.Second

// Heartbeat represents a session marker written by the hook handler.
type Heartbeat struct {
	SessionID string `json:"session_id"`
	Project   string `json:"project"` // last path component of CWD
}

// SessionInfo is a discovered session with its liveness state.
type SessionInfo struct {
	Heartbeat
	Running bool // true = modtime < runningTTL; false = idle (between runningTTL and staleTTL)
}

// SessionsDir returns the directory where heartbeat files are stored.
// It is a variable so tests can redirect to a temp directory.
var SessionsDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "tail-claude-hud", "sessions")
	}
	return filepath.Join(home, ".config", "tail-claude-hud", "sessions")
}

// Write atomically creates a heartbeat file for the given session.
// Uses temp-file + rename to avoid partial reads by the scanner.
func Write(h Heartbeat) error {
	dir := SessionsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(h)
	if err != nil {
		return err
	}

	target := filepath.Join(dir, h.SessionID)

	// Write to a temp file first, then rename for atomicity.
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, target)
}

// Remove deletes the heartbeat for a session. Returns nil if the file
// does not exist (removal is idempotent).
func Remove(sessionID string) error {
	path := filepath.Join(SessionsDir(), sessionID)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// FindOthers scans the heartbeat directory for non-stale heartbeats from
// sessions other than ownSessionID. Returns all matches sorted by project
// name. Returns an empty non-nil slice when none are found.
func FindOthers(ownSessionID string) []SessionInfo {
	dir := SessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []SessionInfo{}
	}

	now := time.Now()
	var sessions []SessionInfo

	for _, de := range entries {
		if de.IsDir() || de.Name() == ownSessionID {
			continue
		}
		// Skip temp files from in-progress writes.
		if len(de.Name()) > 0 && de.Name()[0] == '.' {
			continue
		}

		info, err := de.Info()
		if err != nil {
			continue
		}
		age := now.Sub(info.ModTime())
		if age > staleTTL {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, de.Name()))
		if err != nil {
			continue
		}

		var h Heartbeat
		if json.Unmarshal(data, &h) != nil {
			continue
		}

		sessions = append(sessions, SessionInfo{
			Heartbeat: h,
			Running:   age < runningTTL,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Project < sessions[j].Project
	})

	if sessions == nil {
		sessions = []SessionInfo{}
	}
	return sessions
}
