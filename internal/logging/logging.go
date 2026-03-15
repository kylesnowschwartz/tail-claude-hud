// Package logging provides a debug file logger gated behind an environment variable.
//
// Set TAIL_CLAUDE_HUD_DEBUG=1 to enable logging. All output goes to
// ~/.claude/plugins/tail-claude-hud/debug.log. Nothing is ever written to
// stderr — writing to stderr corrupts Claude Code's terminal layout because
// Claude Code uses the terminal directly and does not separate stderr from the
// display output.
package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	once    sync.Once
	logger  *log.Logger
	enabled bool
)

// init resolves the enabled flag and, when debug mode is on, opens the log file.
// Errors opening the file are silently swallowed — the process must never crash
// or write to stderr because of a missing log directory.
func init() {
	if os.Getenv("TAIL_CLAUDE_HUD_DEBUG") != "1" {
		return
	}
	enabled = true
}

// getLogger returns the shared logger, creating the log file on first use.
func getLogger() *log.Logger {
	once.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			// Cannot determine home directory — disable logging silently.
			enabled = false
			return
		}

		dir := filepath.Join(home, ".claude", "plugins", "tail-claude-hud")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			enabled = false
			return
		}

		logPath := filepath.Join(dir, "debug.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			enabled = false
			return
		}

		logger = log.New(f, "", 0)
	})
	return logger
}

// Debug writes a formatted log line when TAIL_CLAUDE_HUD_DEBUG=1.
// It is a no-op when the env var is unset or empty.
func Debug(format string, args ...any) {
	if !enabled {
		return
	}
	l := getLogger()
	if l == nil {
		return
	}
	ts := time.Now().Format("2006-01-02T15:04:05.000")
	l.Print(ts + " " + fmt.Sprintf(format, args...))
}
