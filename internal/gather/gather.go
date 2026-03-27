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
	"github.com/kylesnowschwartz/tail-claude-hud/internal/breadcrumb"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/extracmd"
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
		ctx.APIDurationMs = input.Cost.TotalAPIDurationMs
		ctx.LinesAdded = input.Cost.TotalLinesAdded
		ctx.LinesRemoved = input.Cost.TotalLinesRemoved
	}
	if input.OutputStyle != nil {
		ctx.OutputStyle = input.OutputStyle.Name
	}
	if input.Worktree != nil {
		ctx.WorktreeName = input.Worktree.Name
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

	// Extra command goroutine: runs user-configured command when set.
	if cfg.Extra.Command != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx.ExtraOutput = extracmd.Run(cfg.Extra.Command)
		}()
	}

	// Usage: populated from stdin rate_limits (zero-cost, no network).
	if active["usage"] {
		ctx.Usage = usageFromStdin(input)
	}

	// Permission detection goroutine: scans breadcrumb files written by
	// Claude Code hooks. Only runs when the widget is active.
	if active["permission"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b := breadcrumb.FindWaiting(input.SessionID); b != nil {
				ctx.PermissionProject = b.Project
			}
		}()
	}

	wg.Wait()

	// Post-gather: compute derived fields that depend on gathered data.
	ctx.SessionStart = sessionStart(ctx.Transcript, input.TranscriptPath)
	ctx.TerminalWidth = terminalWidth()

	// Claude Code's pseudo-TTY reports 80 columns regardless of actual
	// window width. In pipe mode ioctl fails entirely and COLUMNS is often
	// inherited as 80. Both sources are unreliable. Use 120 as a floor so
	// the statusline uses available space instead of truncating at a fake
	// 80. On a genuinely narrow split pane the line may wrap, but Claude
	// Code already hides wrapped output — no worse than the existing
	// behavior when COLUMNS is unset.
	const minWidth = 120
	if ctx.TerminalWidth < minWidth {
		ctx.TerminalWidth = minWidth
	}

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

// terminalWidth returns the current terminal width. Tries each standard
// fd via ioctl, then falls back to the COLUMNS env var. Returns 0 when
// no source provides a positive value.
//
// In Claude Code's statusline mode all three fds are pipes, so ioctl
// fails on all of them. The render stage applies a conservative fallback
// (defaultTerminalWidth) when this returns 0.
func terminalWidth() int {
	for _, fd := range []uintptr{os.Stdin.Fd(), os.Stderr.Fd(), os.Stdout.Fd()} {
		if w, _, err := term.GetSize(fd); err == nil && w > 0 {
			return w
		}
	}

	// Last resort: COLUMNS env var.
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

		// Filter warmup agents and parse first-entry timestamp.
		filePath := filepath.Join(subagentsDir, name)
		first := parseFirstEntry(filePath)
		if first.isWarmup {
			continue
		}

		status := "completed"
		if now.Sub(info.ModTime()) < subagentStaleThreshold {
			status = "running"
		}

		// Compute duration for completed agents from first-entry timestamp to
		// file modtime. This gives the real agent runtime instead of the
		// tool_use→tool_result delta (which is only a few ms for background agents).
		durationMs := 0
		if status == "completed" && !first.timestamp.IsZero() {
			durationMs = int(info.ModTime().Sub(first.timestamp).Milliseconds())
		}

		// Use first-entry timestamp as StartTime so running agents compute
		// elapsed time correctly. Falls back to modtime when unparseable.
		startTime := first.timestamp
		if startTime.IsZero() {
			startTime = info.ModTime()
		}

		// Read .meta.json sidecar to determine the display name.
		// Prefer description (human task name), fall back to agentType ("rb-worker"),
		// then the raw hex UUID as last resort.
		meta := readAgentMeta(filepath.Join(subagentsDir, "agent-"+agentID+".meta.json"))
		displayName := agentID // fallback: raw hex UUID
		if meta.description != "" {
			displayName = meta.description // best: "Add regression tests"
		} else if meta.agentType != "" {
			displayName = meta.agentType // ok: "Explore", "Plan"
		}

		agents = append(agents, model.AgentEntry{
			ID:         agentID,
			Name:       displayName,
			Status:     status,
			StartTime:  startTime,
			DurationMs: durationMs,
			ColorIndex: colorIdx % 8,
		})
		colorIdx++
	}

	return agents
}

// firstEntryInfo holds the parsed results from a subagent JSONL's first line.
type firstEntryInfo struct {
	isWarmup  bool
	timestamp time.Time
}

// parseFirstEntry reads only the first line of a subagent JSONL file and
// returns the warmup status and RFC3339 timestamp. Returns a zero-value
// firstEntryInfo on any read or parse error.
func parseFirstEntry(path string) firstEntryInfo {
	line := readFirstLine(path)
	if line == nil {
		return firstEntryInfo{}
	}

	var entry struct {
		Timestamp string `json:"timestamp"`
		Message   struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return firstEntryInfo{}
	}

	var content string
	isWarmup := false
	if err := json.Unmarshal(entry.Message.Content, &content); err == nil {
		isWarmup = content == "Warmup"
	}

	ts, _ := time.Parse(time.RFC3339, entry.Timestamp)
	return firstEntryInfo{
		isWarmup:  isWarmup,
		timestamp: ts,
	}
}

// agentMeta holds the parsed fields from a .meta.json sidecar file.
type agentMeta struct {
	agentType   string
	description string
}

// readAgentMeta reads a .meta.json sidecar file and returns both the agentType
// and description fields. Returns a zero agentMeta when the file is missing,
// empty, or unparseable.
func readAgentMeta(path string) agentMeta {
	data, err := os.ReadFile(path)
	if err != nil {
		return agentMeta{}
	}
	var meta struct {
		AgentType   string `json:"agentType"`
		Description string `json:"description"`
	}
	if json.Unmarshal(data, &meta) != nil {
		return agentMeta{}
	}
	return agentMeta{agentType: meta.AgentType, description: meta.Description}
}

// usageFromStdin converts stdin rate_limits into a UsageInfo when present.
// Returns nil when the field is absent (older Claude Code or API users),
// letting the caller fall back to the OAuth API path.
func usageFromStdin(input *model.StdinData) *model.UsageInfo {
	if input.RateLimits == nil {
		return nil
	}
	rl := input.RateLimits

	info := &model.UsageInfo{
		FiveHourPercent: -1,
		SevenDayPercent: -1,
	}

	if rl.FiveHour != nil {
		info.FiveHourPercent = parseStdinPercent(rl.FiveHour.UsedPercentage)
		info.FiveHourResetAt = parseStdinTime(rl.FiveHour.ResetsAt)
	}
	if rl.SevenDay != nil {
		info.SevenDayPercent = parseStdinPercent(rl.SevenDay.UsedPercentage)
		info.SevenDayResetAt = parseStdinTime(rl.SevenDay.ResetsAt)
	}

	return info
}

// parseStdinPercent clamps a float percentage to 0-100 and rounds.
// Returns -1 when nil.
func parseStdinPercent(v *float64) int {
	if v == nil {
		return -1
	}
	pct := *v
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return int(pct + 0.5)
}

// parseStdinTime converts a Unix epoch timestamp (seconds) to time.Time.
// Returns zero time when nil.
func parseStdinTime(epoch *float64) time.Time {
	if epoch == nil {
		return time.Time{}
	}
	sec := int64(*epoch)
	nsec := int64((*epoch - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}

// mergeSubagents is a union merge that keeps transcript agents as the base.
// For each filesystem agent:
//   - If a matching transcript agent exists (by Name), overwrite StartTime,
//     DurationMs, Status, and ID from the filesystem entry while preserving
//     Model and Description from the transcript entry.
//   - If no match exists, append the filesystem agent.
//
// Transcript-only agents (cleaned up files, warmup-filtered, compact-filtered)
// are never removed. When multiple transcript agents share the same Name, the
// filesystem agent matches the first unmatched one.
func mergeSubagents(td *model.TranscriptData, fsAgents []model.AgentEntry) {
	if len(fsAgents) == 0 {
		return
	}

	// Build a name → list-of-indices map so same-name agents can be matched
	// one-for-one without a second matching stealing the same slot.
	//
	// Also build a description → list-of-indices map as a fallback. The
	// transcript extractor names agents by subagent_type (e.g. "claude-code-guide")
	// while filesystem discovery names them by description (e.g. "Research Claude
	// Code SDK/headless"). Without the fallback, the same agent appears twice.
	byName := make(map[string][]int, len(td.Agents))
	byDesc := make(map[string][]int, len(td.Agents))
	for i, a := range td.Agents {
		byName[a.Name] = append(byName[a.Name], i)
		if a.Description != "" {
			byDesc[a.Description] = append(byDesc[a.Description], i)
		}
	}

	matched := make(map[int]bool)
	for _, fa := range fsAgents {
		bestIdx := -1
		// Primary: match by Name.
		for _, idx := range byName[fa.Name] {
			if !matched[idx] {
				bestIdx = idx
				break
			}
		}
		// Fallback: the filesystem agent's Name is the description string,
		// so try matching it against transcript agents' Description field.
		if bestIdx < 0 {
			for _, idx := range byDesc[fa.Name] {
				if !matched[idx] {
					bestIdx = idx
					break
				}
			}
		}
		if bestIdx >= 0 {
			matched[bestIdx] = true
			td.Agents[bestIdx].StartTime = fa.StartTime
			td.Agents[bestIdx].DurationMs = fa.DurationMs
			td.Agents[bestIdx].Status = fa.Status
			td.Agents[bestIdx].ID = fa.ID
		} else {
			td.Agents = append(td.Agents, fa)
		}
	}
}
