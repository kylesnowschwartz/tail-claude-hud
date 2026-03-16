// Package gather coordinates parallel data collection for the statusline.
// It inspects which widgets are configured, spawns goroutines only for the
// data sources those widgets need, waits for all goroutines to finish, and
// returns a fully-populated RenderContext.
package gather

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/charmbracelet/x/term"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/git"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/transcript"
)

// transcriptWidgets are the widget names that require transcript data.
var transcriptWidgets = map[string]bool{
	"tools":    true,
	"agents":   true,
	"todos":    true,
	"thinking": true,
}

// Gather builds a RenderContext by collecting data in parallel for each data
// source required by the configured widgets. Goroutines are spawned only for
// active sources; a sync.WaitGroup provides the happens-before guarantee so no
// mutex is needed for the distinct field writes.
func Gather(input *model.StdinData, cfg *config.Config) *model.RenderContext {
	ctx := &model.RenderContext{
		Cwd:            input.Cwd,
		ContextPercent: input.ContextPercent,
	}
	if input.Model != nil {
		ctx.ModelID = input.Model.ID
		ctx.ModelDisplayName = input.Model.DisplayName
	}
	if input.ContextWindow != nil {
		ctx.ContextWindowSize = input.ContextWindow.Size
		if input.ContextWindow.CurrentUsage != nil {
			ctx.InputTokens = input.ContextWindow.CurrentUsage.InputTokens
			ctx.CacheCreation = input.ContextWindow.CurrentUsage.CacheCreationInputTokens
			ctx.CacheRead = input.ContextWindow.CurrentUsage.CacheReadInputTokens
		}
	}

	// Determine which widget names are active across all configured lines.
	active := activeWidgets(cfg)

	var wg sync.WaitGroup

	// Transcript goroutine: needed when any of tools/agents/todos are active
	// and a transcript path is available.
	if needsTranscript(active) && input.TranscriptPath != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx.Transcript = gatherTranscript(input.TranscriptPath)
		}()
	}

	// Env goroutine: needed when the "env" widget is active.
	if active["env"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx.EnvCounts = config.CountEnv(input.Cwd)
		}()
	}

	// Git goroutine: needed when the "git" or "project" widget is active.
	// "project" renders the project name with optional ahead/behind counts,
	// so it requires the same git data as the "git" widget.
	if active["git"] || active["project"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx.Git = git.GetStatus(input.Cwd)
		}()
	}

	wg.Wait()

	// Post-gather: compute derived fields that depend on gathered data.
	ctx.SessionDuration = sessionStart(ctx.Transcript, input.TranscriptPath)
	ctx.TerminalWidth = terminalWidth()

	return ctx
}

// activeWidgets flattens all widget names from all config lines into a set.
func activeWidgets(cfg *config.Config) map[string]bool {
	set := make(map[string]bool)
	for _, line := range cfg.Lines {
		for _, w := range line.Widgets {
			set[w] = true
		}
	}
	return set
}

// needsTranscript reports whether any of the transcript-backed widgets are active.
func needsTranscript(active map[string]bool) bool {
	for w := range transcriptWidgets {
		if active[w] {
			return true
		}
	}
	return false
}

// gatherTranscript runs the full transcript pipeline for the given path:
// restore snapshot → incremental read → parse → extract → save snapshot → return TranscriptData.
// Returns nil on any error so callers treat missing data gracefully.
func gatherTranscript(path string) *model.TranscriptData {
	sm := transcript.NewStateManager(transcriptStateDir())
	lines, err := sm.ReadIncremental(path)
	if err != nil {
		return nil
	}

	es := transcript.NewExtractionState()
	// Restore previous display state so completed tools/agents remain visible
	// across invocations. The snapshot is nil on first run or after a reset.
	if snap := sm.LoadSnapshot(); snap != nil {
		_ = es.UnmarshalSnapshot(snap)
	}

	for _, line := range lines {
		e, err := transcript.ParseEntry([]byte(line))
		if err != nil {
			continue
		}
		es.ProcessEntry(e)
	}

	td := es.ToTranscriptData()
	td.Path = path

	// Persist the updated extraction state alongside the byte offset.
	if snap, err := es.MarshalSnapshot(); err == nil {
		sm.SetSnapshot(snap)
	}
	_ = sm.SaveState(path)
	return td
}

// sessionStart returns the RFC3339 timestamp of the first entry in the
// transcript file, which the duration widget uses as the session start time.
// Falls back to "" when the transcript path is empty, unreadable, or has no
// parseable entries.
func sessionStart(td *model.TranscriptData, path string) string {
	if path == "" {
		return ""
	}

	// We need the raw first-line timestamp from the file, independent of the
	// incremental-read offset, so open the file directly from byte 0.
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	// Read just enough bytes to find the first complete line.
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	if n == 0 {
		return ""
	}

	// Find the first newline-terminated line.
	line := buf[:n]
	for i, b := range line {
		if b == '\n' {
			line = line[:i]
			break
		}
	}

	e, err := transcript.ParseEntry(line)
	if err != nil {
		return ""
	}

	t := e.ParsedTimestamp()
	if t.IsZero() {
		return ""
	}

	return t.Format("2006-01-02T15:04:05Z07:00")
}

// terminalWidth returns the current terminal width by querying the TTY via
// ioctl first, falling back to the COLUMNS environment variable. Returns 0
// when neither source provides a positive value, which signals the render
// layer to skip truncation.
//
// Querying the TTY directly (rather than relying on COLUMNS) ensures we get
// the actual width when the terminal is split or resized, because shells only
// update COLUMNS on receipt of a SIGWINCH signal — Claude Code may not
// propagate that signal to the subprocess before invoking the HUD.
func terminalWidth() int {
	// Try stdin fd first — works when the process has a real TTY attached.
	if w, _, err := term.GetSize(os.Stdin.Fd()); err == nil && w > 0 {
		return w
	}

	// Fall back to COLUMNS for contexts where stdin is not a TTY (e.g. pipes,
	// tests, or non-interactive environments).
	s := os.Getenv("COLUMNS")
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// transcriptStateDir returns the directory used for incremental-read state files.
// Follows the same convention as the plugin directory: ~/.claude/plugins/tail-claude-hud/
func transcriptStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(home, ".claude", "plugins", "tail-claude-hud")
}
