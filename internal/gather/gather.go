// Package gather coordinates parallel data collection for the statusline.
// It inspects which widgets are configured, spawns goroutines only for the
// data sources those widgets need, waits for all goroutines to finish, and
// returns a fully-populated RenderContext.
package gather

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

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
	"session":  true,
	"messages": true,
	"skills":   true,
	"speed":    true,
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
	if input.Cost != nil {
		ctx.SessionCostUSD = input.Cost.TotalCostUSD
		ctx.TotalDurationMs = input.Cost.TotalDurationMs
		ctx.ApiDurationMs = input.Cost.TotalAPIDurationMs
		ctx.LinesAdded = input.Cost.TotalLinesAdded
		ctx.LinesRemoved = input.Cost.TotalLinesRemoved
	}
	if input.OutputStyle != nil {
		ctx.OutputStyle = input.OutputStyle.Name
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
	ctx.SessionStart = sessionStart(ctx.Transcript, input.TranscriptPath)
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
	sm := transcript.NewStateManager(model.PluginDir())
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

	// Advance the spinner frame once per invocation so successive renders always
	// show a different frame, independent of wall-clock time granularity.
	es.IncrementSpinnerFrame()

	td := es.ToTranscriptData()
	td.Path = path

	// Merge filesystem-discovered subagents that the transcript extractor
	// missed (background agents complete in ~6ms, so the extractor marks them
	// "completed" before they actually finish).
	mergeSubagents(td, discoverSubagents(path))

	// Persist the updated extraction state alongside the byte offset.
	if snap, err := es.MarshalSnapshot(); err == nil {
		sm.SetSnapshot(snap)
	}
	_ = sm.SaveState(path)
	return td
}

// readFirstLine opens a file and returns its first newline-terminated line.
// Returns nil when the file is missing, empty, or unreadable.
func readFirstLine(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	if n == 0 {
		return nil
	}

	line := buf[:n]
	for i, b := range line {
		if b == '\n' {
			line = line[:i]
			break
		}
	}
	return line
}

// sessionStart returns the RFC3339 timestamp of the first entry in the
// transcript file, which the duration widget uses as the session start time.
// Falls back to "" when the transcript path is empty, unreadable, or has no
// parseable entries.
func sessionStart(td *model.TranscriptData, path string) string {
	if path == "" {
		return ""
	}

	line := readFirstLine(path)
	if line == nil {
		return ""
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

// subagentStaleThreshold is the duration after which a subagent file's modtime
// indicates the agent has completed. Files modified within this window are
// considered still running.
const subagentStaleThreshold = 30 * time.Second

// discoverSubagents scans the filesystem for subagent JSONL files associated
// with the given transcript path. It derives the subagents directory from the
// transcript path ({dir}/{session-uuid}/subagents/) and returns lightweight
// AgentEntry values based on file metadata alone.
//
// The approach mirrors .cloned-sources/tail-claude/parser/subagent.go:DiscoverSubagents
// but avoids full file parsing. Status is inferred from modtime: files modified
// within subagentStaleThreshold are "running", others are "completed".
func discoverSubagents(transcriptPath string) []model.AgentEntry {
	dir := filepath.Dir(transcriptPath)
	base := strings.TrimSuffix(filepath.Base(transcriptPath), ".jsonl")
	subagentsDir := filepath.Join(dir, base, "subagents")

	entries, err := os.ReadDir(subagentsDir)
	if err != nil {
		return nil
	}

	now := time.Now()
	var agents []model.AgentEntry
	colorIdx := 0

	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		agentID := strings.TrimPrefix(name, "agent-")
		agentID = strings.TrimSuffix(agentID, ".jsonl")

		// Filter compact agents (context compaction artifacts).
		if strings.HasPrefix(agentID, "acompact") {
			continue
		}

		info, err := de.Info()
		if err != nil || info.Size() == 0 {
			continue
		}

		// Filter warmup agents by checking the first user message.
		filePath := filepath.Join(subagentsDir, name)
		if isWarmupAgent(filePath) {
			continue
		}

		status := "completed"
		if now.Sub(info.ModTime()) < subagentStaleThreshold {
			status = "running"
		}

		// Read .meta.json sidecar for the agentType display name.
		// Falls back to the raw agentID when the sidecar is missing.
		displayName := agentID
		if at := readAgentType(filepath.Join(subagentsDir, "agent-"+agentID+".meta.json")); at != "" {
			displayName = at
		}

		agents = append(agents, model.AgentEntry{
			Name:       displayName,
			Status:     status,
			StartTime:  info.ModTime(),
			ColorIndex: colorIdx % 8,
		})
		colorIdx++
	}

	return agents
}

// isWarmupAgent reads only the first line of a subagent JSONL file to check
// whether the agent is a warmup probe (content == "Warmup"). Returns false
// on any read or parse error.
func isWarmupAgent(path string) bool {
	line := readFirstLine(path)
	if line == nil {
		return false
	}

	var entry struct {
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return false
	}

	var content string
	if err := json.Unmarshal(entry.Message.Content, &content); err != nil {
		return false
	}
	return content == "Warmup"
}

// readAgentType reads a .meta.json sidecar file and returns the agentType
// field. Returns "" when the file is missing, empty, or lacks the field.
func readAgentType(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var meta struct {
		AgentType string `json:"agentType"`
	}
	if json.Unmarshal(data, &meta) != nil {
		return ""
	}
	return meta.AgentType
}

// mergeSubagents folds filesystem-discovered agents into the transcript data.
// For each filesystem agent: if the transcript already has a matching agent
// (by name), the filesystem status takes precedence when it says "running"
// but the transcript says "completed". If no match exists, the agent is appended.
func mergeSubagents(td *model.TranscriptData, fsAgents []model.AgentEntry) {
	if len(fsAgents) == 0 {
		return
	}

	// Index existing transcript agents by name for quick lookup.
	byName := make(map[string]int, len(td.Agents))
	for i, a := range td.Agents {
		byName[a.Name] = i
	}

	for _, fa := range fsAgents {
		if idx, ok := byName[fa.Name]; ok {
			// Filesystem says running but transcript says completed: override.
			if fa.Status == "running" && td.Agents[idx].Status == "completed" {
				td.Agents[idx].Status = "running"
				td.Agents[idx].DurationMs = 0
			}
		} else {
			td.Agents = append(td.Agents, fa)
		}
	}
}
