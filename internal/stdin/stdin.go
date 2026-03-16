// Package stdin reads and parses the JSON blob that Claude Code pipes to stdin
// on every invocation. It handles TTY detection and context-percent computation.
package stdin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// snapshotDir is the directory where the last-stdin snapshot is persisted.
// Same location as transcript state files: ~/.claude/plugins/tail-claude-hud/
var snapshotDir = defaultSnapshotDir()

const snapshotFile = "last-stdin.json"

// Read decodes one JSON object from f and returns the parsed StdinData.
//
// When f is a TTY (not a pipe), Read returns (nil, nil) — the caller should
// print an initialising message and exit gracefully.
//
// The *os.File parameter instead of io.Reader allows TTY detection via Stat().
// Tests create a temporary file and pass that in place of os.Stdin.
func Read(f *os.File) (*model.StdinData, error) {
	if isTTY(f) {
		// No pipe — nothing to parse. Signal this with a nil result, not an error.
		return nil, nil
	}

	return decode(f)
}

// isTTY reports whether f is a character device (i.e. an interactive terminal,
// not a pipe or redirected file).
func isTTY(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		// If we can't stat, assume there is data to read.
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// decode reads exactly one JSON object from r and computes ContextPercent.
func decode(r io.Reader) (*model.StdinData, error) {
	var data model.StdinData
	dec := json.NewDecoder(r)
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("stdin: decode JSON: %w", err)
	}

	data.ContextPercent = computeContextPercent(&data)
	return &data, nil
}

// SaveSnapshot persists data as JSON to snapshotDir/last-stdin.json.
// Called on every successful Read so --dump-current can replay the most
// recent stdin state. Errors are silently ignored — a missing snapshot
// degrades dump output but never blocks the live statusline.
func SaveSnapshot(data *model.StdinData) {
	if snapshotDir == "" {
		return
	}
	_ = os.MkdirAll(snapshotDir, 0o755)

	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(snapshotDir, snapshotFile), b, 0o644)
}

// LoadSnapshot reads the last-stdin snapshot from disk and returns the
// decoded StdinData. Returns nil and an error if the file is missing or
// corrupt — callers should fall back gracefully.
func LoadSnapshot() (*model.StdinData, error) {
	if snapshotDir == "" {
		return nil, fmt.Errorf("stdin: snapshot dir unknown")
	}

	path := filepath.Join(snapshotDir, snapshotFile)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("stdin: open snapshot: %w", err)
	}
	defer f.Close()

	return decode(f)
}

func defaultSnapshotDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "plugins", "tail-claude-hud")
}

// computeContextPercent returns the context usage as a 0–100 integer.
// It prefers the used_percentage field when present, and falls back to
// manual calculation from token counts when it is absent.
func computeContextPercent(d *model.StdinData) int {
	if d.ContextWindow == nil {
		return 0
	}

	// Prefer the explicit percentage provided by Claude Code.
	if d.ContextWindow.UsedPercent != nil {
		return int(*d.ContextWindow.UsedPercent)
	}

	// Fall back: sum tokens and divide by window size.
	size := d.ContextWindow.Size
	if size <= 0 || d.ContextWindow.CurrentUsage == nil {
		return 0
	}

	u := d.ContextWindow.CurrentUsage
	total := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
	return (total * 100) / size
}
