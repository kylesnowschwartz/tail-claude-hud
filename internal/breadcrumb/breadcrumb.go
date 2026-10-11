// Package breadcrumb manages permission-waiting breadcrumb files.
//
// When Claude Code's PermissionRequest hook fires, the hook handler writes a
// small file to ~/.config/tail-claude-hud/waiting/{session_id}. The statusline
// gather stage scans this directory to detect other sessions blocked on
// permission approval. The Stop hook removes the breadcrumb; PostToolUse and
// PostToolUseFailure remove it unless Claude Code still reports the session
// waiting on a dialog.
//
// Breadcrumbs older than staleTTL are ignored (covers hard-crash scenarios
// where neither PostToolUse nor Stop fires), and the cleanup hook deletes them.
package breadcrumb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// staleTTL is the maximum age of a breadcrumb before it is considered stale
// and ignored. Covers the case where a session is killed without cleanup. The
// breadcrumb is written once, so the limit also bounds how long an unanswered
// dialog keeps its alert.
const staleTTL = 10 * time.Minute

// Breadcrumb represents a permission-waiting marker for a Claude Code session.
type Breadcrumb struct {
	SessionID string `json:"session_id"`
	Project   string `json:"project"` // last path component of CWD
	ToolName  string `json:"tool_name,omitempty"`
}

// WaitingDir returns the directory where breadcrumb files are stored.
// It is a variable so tests can redirect to a temp directory.
var WaitingDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "tail-claude-hud", "waiting")
	}
	return filepath.Join(home, ".config", "tail-claude-hud", "waiting")
}

// Write atomically creates a breadcrumb file for the given session.
// Uses temp-file + rename to avoid partial reads by the scanner.
func Write(b Breadcrumb) error {
	dir := WaitingDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(b)
	if err != nil {
		return err
	}

	target := filepath.Join(dir, b.SessionID)

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

// Exists reports whether a breadcrumb file is present for the session,
// without parsing it.
func Exists(sessionID string) bool {
	_, err := os.Stat(filepath.Join(WaitingDir(), sessionID))
	return err == nil
}

// Remove deletes the breadcrumb for a session. Returns nil if the file
// does not exist (removal is idempotent).
func Remove(sessionID string) error {
	path := filepath.Join(WaitingDir(), sessionID)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// RemoveStale deletes every file in the breadcrumb directory older than
// staleTTL: breadcrumbs of sessions that ended without cleanup, and temp
// files of interrupted writes. FindWaiting already ignores them; removing
// them keeps the directory from growing. A missing directory is not an error.
func RemoveStale() error {
	dir := WaitingDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	now := time.Now()
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		info, err := de.Info()
		if err != nil || now.Sub(info.ModTime()) <= staleTTL {
			continue
		}
		if err := os.Remove(filepath.Join(dir, de.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// FindWaiting scans the breadcrumb directory for a non-stale breadcrumb from
// a session other than ownSessionID. Returns the first match, or nil if none.
func FindWaiting(ownSessionID string) *Breadcrumb {
	dir := WaitingDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	now := time.Now()
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
		if now.Sub(info.ModTime()) > staleTTL {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, de.Name()))
		if err != nil {
			continue
		}

		var b Breadcrumb
		if json.Unmarshal(data, &b) != nil {
			continue
		}
		return &b
	}
	return nil
}
