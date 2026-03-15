// Package stdin reads and parses the JSON blob that Claude Code pipes to stdin
// on every invocation. It handles TTY detection and context-percent computation.
package stdin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

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
