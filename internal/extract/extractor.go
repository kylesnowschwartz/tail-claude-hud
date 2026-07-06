// Package extract bridges parsed transcript entries into
// model.TranscriptData for statusline rendering.
//
// Call ProcessEntry for each parsed transcript.Entry in order. Call
// ToTranscriptData to produce a snapshot suitable for passing to the render
// layer.
package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kylesnowschwartz/agent-ouija/claude/tools"
	"github.com/kylesnowschwartz/agent-ouija/claude/transcript"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// maxTools is the maximum number of ToolEntries kept in the display slice.
const maxTools = 20

// maxAgents is the maximum number of AgentEntries kept in the display slice.
const maxAgents = 10

// maxSkills is the maximum number of skill names kept in the display slice.
const maxSkills = 20

// nativeCommandNames lists Claude Code's built-in slash commands (and their
// documented aliases) that are coded into the CLI rather than being skills.
// Both route through the same <command-name> user-message tag (see
// extractSkillFromUserMessage), so native commands must be filtered out here
// or the skills widget would show things like "/model" or "/clear" as if
// they were invoked skills. Source of truth: https://code.claude.com/docs/en/commands
// (everything NOT tagged [Skill] or [Workflow] in that table).
var nativeCommandNames = map[string]bool{
	"add-dir": true, "advisor": true, "agents": true, "autofix-pr": true,
	"background": true, "bg": true, "branch": true, "btw": true, "cd": true,
	"chrome": true, "clear": true, "reset": true, "new": true, "color": true,
	"compact": true, "config": true, "settings": true, "context": true,
	"copy": true, "cost": true, "design-login": true, "desktop": true,
	"app": true, "diff": true, "doctor": true, "effort": true, "exit": true,
	"quit": true, "export": true, "fast": true, "feedback": true, "bug": true,
	"share": true, "focus": true, "fork": true, "goal": true, "heapdump": true,
	"help": true, "hooks": true, "ide": true, "init": true, "insights": true,
	"install-github-app": true, "install-slack-app": true, "keybindings": true,
	"login": true, "logout": true, "mcp": true, "memory": true, "mobile": true,
	"ios": true, "android": true, "model": true, "passes": true,
	"permissions": true, "allowed-tools": true, "plan": true, "plugin": true,
	"powerup": true, "pr-comments": true, "privacy-settings": true,
	"radio": true, "recap": true, "release-notes": true, "reload-plugins": true,
	"reload-skills": true, "remote-control": true, "rc": true,
	"remote-env": true, "rename": true, "resume": true, "continue": true,
	"review": true, "rewind": true, "checkpoint": true, "undo": true,
	"sandbox": true, "schedule": true, "routines": true, "scroll-speed": true,
	"security-review": true, "setup-bedrock": true, "setup-vertex": true,
	"skills": true, "stats": true, "status": true, "statusline": true,
	"stickers": true, "stop": true, "tasks": true, "bashes": true,
	"team-onboarding": true, "teleport": true, "tp": true,
	"terminal-setup": true, "theme": true, "tui": true, "ultraplan": true,
	"ultrareview": true, "upgrade": true, "usage": true, "usage-credits": true,
	"vim": true, "voice": true, "web-setup": true, "workflows": true,
}

// bashTargetMaxLen is the number of characters kept from a Bash command for
// the target field (matches claude-hud's 40-char truncation convention; the
// TS source uses 30 but the card spec says 40).
const bashTargetMaxLen = 40

// internalTool holds richer per-invocation state than model.ToolEntry.
// It is collapsed into model.ToolEntry by ToTranscriptData.
type internalTool struct {
	id         string
	name       string
	target     string
	completed  bool // false = running, true = completed or error
	hasError   bool
	durationMs int
	category   string
	startTime  time.Time
}

// internalAgent holds richer per-invocation state than model.AgentEntry.
type internalAgent struct {
	id          string
	name        string // display name: subagent_type, truncated description, or tool name
	model       string
	description string
	status      string // "running" or "completed"
	startTime   time.Time
	durationMs  int // 0 = still running; populated by a separate card
}

// maxTokenSamples is the maximum number of TokenSample entries retained.
// At one sample per assistant message, 200 samples covers many sessions
// while keeping the snapshot small.
const maxTokenSamples = 200

// ExtractionState accumulates tool, agent, todo, and thinking data across a
// sequence of parsed transcript entries. It is designed for incremental use:
// call ProcessEntry for each new line, then call ToTranscriptData for the
// latest snapshot.
//
// The state is NOT safe for concurrent use. Callers must synchronise externally
// if ProcessEntry and ToTranscriptData are called from multiple goroutines.
type ExtractionState struct {
	// toolMap correlates tool_use IDs with their in-flight or recently-completed
	// internal tool records. Entries are pruned once the display slice is full.
	toolMap map[string]*internalTool

	// agentMap correlates agent tool_use IDs with their state.
	agentMap map[string]*internalAgent

	// todos is the authoritative todo list; replaced on TodoWrite, mutated on
	// TaskCreate/TaskUpdate.
	todos []model.TodoItem

	// taskIDIndex maps TaskCreate-assigned IDs to positions in Todos.
	taskIDIndex map[string]int

	// displayTools holds the ordered list of tools for rendering (newest last).
	displayTools []*internalTool

	// displayAgents holds the ordered list of agents for rendering (newest last).
	displayAgents []*internalAgent

	// tokenSamples accumulates timestamp+output-token pairs from assistant
	// messages. Used by the speed widget to compute a rolling tokens/sec average.
	tokenSamples []model.TokenSample

	// lastSampleUsage is the usage of the most recently recorded token sample.
	// One API response spans several assistant entries (one per content block),
	// each repeating the same message.usage object; matching this key drops the
	// repeats so a response is counted once.
	lastSampleUsage usageKey

	// sessionName holds the display name for the current session. Set from a
	// custom-title entry when present, otherwise falls back to the slug field.
	sessionName string

	// thinkingActive is true when the most recent assistant message that
	// contained a thinking block did not also contain a tool_use or text block.
	// It is cleared whenever a tool_use or text block is seen in the same entry.
	thinkingActive bool

	// thinkingCount is the total number of thinking blocks observed across all
	// assistant messages in the session.
	thinkingCount int

	// thinkingTool points to the most recent thinking ToolEntry while it is
	// still running (Completed=false). It is set by handleThinkingStart and
	// cleared (to nil) by handleThinkingEnd.
	thinkingTool *internalTool

	// thinkingSeq is a monotonically increasing counter used to generate
	// unique synthetic IDs for thinking ToolEntries.
	thinkingSeq int

	// spinnerFrame is a monotonic counter incremented on each statusline invocation.
	// It is persisted in the snapshot so successive invocations always advance the frame.
	spinnerFrame int

	// dividerOffset is a monotonic counter incremented once per new tool_use.
	// Persisted in the snapshot so the scrolling ticker position survives
	// across statusline invocations.
	dividerOffset int

	// messageCount is the number of user/assistant conversation turns observed
	// in the transcript, excluding tool_result entries (which are infrastructure
	// messages, not conversational turns).
	messageCount int

	// skillNames is the ordered list of skill names invoked in the session
	// (newest last), capped at maxSkills. Detected from two sources:
	// user-typed slash commands (<command-name> tags) and assistant-side
	// Skill tool_use blocks.
	skillNames []string
}

// NewExtractionState returns an initialised, empty ExtractionState.
func NewExtractionState() *ExtractionState {
	return &ExtractionState{
		toolMap:     make(map[string]*internalTool),
		agentMap:    make(map[string]*internalAgent),
		taskIDIndex: make(map[string]int),
	}
}

// IncrementSpinnerFrame advances the monotonic spinner counter by one.
// Call this once per statusline invocation, after restoring the snapshot and
// processing new transcript entries, so the frame always advances between renders.
func (es *ExtractionState) IncrementSpinnerFrame() {
	es.spinnerFrame++
}

// ProcessEntry classifies the content blocks in e and updates the extraction
// state accordingly. Unknown entry types and malformed blocks are silently
// ignored — the caller is responsible for feeding entries in order.
func (es *ExtractionState) ProcessEntry(e transcript.Entry) {
	// Sidechain entries are internal agent activity — not part of the main
	// conversation thread. Skip them to avoid double-counting tool calls and
	// agent launches from sub-agent subprocesses.
	if e.IsSidechain {
		return
	}

	// custom-title entries take priority over slug for the session name.
	if e.Type == "custom-title" && e.CustomTitle != "" {
		es.sessionName = e.CustomTitle
	} else if e.Slug != "" && es.sessionName == "" {
		// slug is a fallback: only set when no custom-title has been seen yet.
		es.sessionName = e.Slug
	}

	blocks := transcript.ExtractContentBlocks(e)
	ts := e.ParsedTimestamp()

	// Count conversational turns: user and assistant messages that are not
	// pure tool_result responses. Tool results are infrastructure — they carry
	// tool output back to the model but do not represent a human or model turn.
	role := e.Message.Role
	if role == "user" || role == "assistant" {
		isToolResultOnly := len(blocks.ToolResult) > 0 &&
			len(blocks.ToolUse) == 0 &&
			len(blocks.Thinking) == 0 &&
			!blocks.HasText
		if !isToolResultOnly {
			es.messageCount++
		}
	}
	// Detect skill invocations from user messages. Claude Code records
	// slash-command usage as user messages with the skill name wrapped in
	// <command-name>/skill-name</command-name> XML tags in the raw content string.
	if role == "user" {
		es.extractSkillFromUserMessage(e)
	}

	if ts.IsZero() {
		ts = time.Now()
	}

	for _, b := range blocks.ToolUse {
		es.processToolUse(b, ts)
	}

	for _, b := range blocks.ToolResult {
		es.processToolResult(b, ts)
	}

	// Record output tokens from assistant messages for the speed widget.
	// Output only: input/cache tokens are processed, not generated, and would
	// spike the rate. Usage is a value type on the library Entry; absent usage
	// decodes to the zero value, which the OutputTokens > 0 guard filters.
	// The transcript's message.id isn't modeled by the library, so identical
	// usage vs the previous sample is the split-entry dedupe key.
	if e.Message.Role == "assistant" {
		key := usageKeyOf(e.Message.Usage)
		if e.Message.Usage.OutputTokens > 0 && key != es.lastSampleUsage {
			es.lastSampleUsage = key
			es.tokenSamples = append(es.tokenSamples, model.TokenSample{
				Timestamp: ts,
				Tokens:    e.Message.Usage.OutputTokens,
			})
			// Prune oldest entries when the cap is exceeded.
			if len(es.tokenSamples) > maxTokenSamples {
				es.tokenSamples = es.tokenSamples[len(es.tokenSamples)-maxTokenSamples:]
			}
		}
	}

	// Update thinking state based on blocks present in this entry.
	// Thinking is active only when a thinking block was seen but no subsequent
	// tool_use or text block appeared in the same message.
	if len(blocks.Thinking) > 0 {
		es.thinkingCount += len(blocks.Thinking)
		newActive := len(blocks.ToolUse) == 0 && !blocks.HasText
		if !es.thinkingActive {
			// Thinking just started: emit a running ToolEntry.
			es.handleThinkingStart(ts)
		}
		es.thinkingActive = newActive
		if !newActive {
			// Thinking ended in the same entry (tool_use or text also present):
			// mark the entry completed immediately.
			es.handleThinkingEnd(ts)
		}
	} else if len(blocks.ToolUse) > 0 || blocks.HasText {
		// A message with tool_use or text but no thinking clears the active flag.
		if es.thinkingActive {
			es.handleThinkingEnd(ts)
		}
		es.thinkingActive = false
	}
}

// processToolUse dispatches a tool_use block to the appropriate handler.
func (es *ExtractionState) processToolUse(b transcript.ToolUseBlock, ts time.Time) {
	switch b.Name {
	case "Task", "Agent":
		es.handleAgentToolUse(b, ts)
	case "TodoWrite":
		es.handleTodoWrite(b)
	case "TaskCreate":
		es.handleTaskCreate(b)
	case "TaskUpdate":
		es.handleTaskUpdate(b)
	case "Skill":
		es.handleSkillToolUse(b, ts)
	default:
		es.handleRegularToolUse(b, ts)
	}
}

// extractSkillFromUserMessage parses a <command-name>/skill</command-name> tag
// from a user message's raw content string. Claude Code records slash-command
// invocations this way rather than as tool_use blocks.
func (es *ExtractionState) extractSkillFromUserMessage(e transcript.Entry) {
	// User message content is either a JSON string or an array. Skill
	// invocations arrive as plain strings, so skip arrays early.
	if len(e.Message.Content) == 0 || e.Message.Content[0] == '[' {
		return
	}

	// Fast path: skip the unmarshal when the tag isn't in the raw JSON.
	if !bytes.Contains(e.Message.Content, []byte("<command-name>/")) {
		return
	}

	var content string
	if err := json.Unmarshal(e.Message.Content, &content); err != nil {
		return
	}

	// Real skill invocations are short messages that start with
	// <command-message> or <command-name>. Longer messages that happen
	// to contain these tags (e.g. agent results quoting code) are not
	// skill invocations.
	if !strings.HasPrefix(content, "<command-message>") && !strings.HasPrefix(content, "<command-name>") {
		return
	}

	const prefix = "<command-name>/"
	const suffix = "</command-name>"
	start := strings.Index(content, prefix)
	if start < 0 {
		return
	}
	start += len(prefix)
	end := strings.Index(content[start:], suffix)
	if end < 0 {
		return
	}
	name := content[start : start+end]
	if name == "" || nativeCommandNames[name] {
		return
	}

	es.recordSkill(name)
}

// handleSkillToolUse records a skill invocation from an assistant-side Skill
// tool_use block (input.skill contains the skill name). Also registers as a
// regular tool so the entry appears in the tools activity feed.
func (es *ExtractionState) handleSkillToolUse(b transcript.ToolUseBlock, ts time.Time) {
	var input struct {
		Skill string `json:"skill"`
	}
	_ = json.Unmarshal(b.Input, &input)
	if input.Skill != "" {
		es.recordSkill(input.Skill)
	}
	es.handleRegularToolUse(b, ts)
}

// recordSkill appends a skill name and enforces the cap.
func (es *ExtractionState) recordSkill(name string) {
	es.skillNames = append(es.skillNames, name)
	if len(es.skillNames) > maxSkills {
		es.skillNames = es.skillNames[1:]
	}
}

// handleRegularToolUse records a running tool entry and appends it to the
// display slice, pruning the oldest if the limit is exceeded.
func (es *ExtractionState) handleRegularToolUse(b transcript.ToolUseBlock, ts time.Time) {
	t := &internalTool{
		id:        b.ID,
		name:      b.Name,
		target:    extractTarget(b.Name, b.Input),
		category:  toolCategory(b.Name),
		startTime: ts,
	}
	es.toolMap[b.ID] = t
	es.appendTool(t)
}

// appendTool adds a tool to the display slice, increments the divider offset,
// and prunes the oldest entry when the slice exceeds maxTools.
func (es *ExtractionState) appendTool(t *internalTool) {
	es.displayTools = append(es.displayTools, t)
	es.dividerOffset++
	if len(es.displayTools) > maxTools {
		oldest := es.displayTools[0]
		es.displayTools = es.displayTools[1:]
		delete(es.toolMap, oldest.id)
	}
}

// appendAgent adds an agent to the display slice and prunes the oldest entry
// when the slice exceeds maxAgents.
func (es *ExtractionState) appendAgent(a *internalAgent) {
	es.displayAgents = append(es.displayAgents, a)
	if len(es.displayAgents) > maxAgents {
		oldest := es.displayAgents[0]
		es.displayAgents = es.displayAgents[1:]
		delete(es.agentMap, oldest.id)
	}
}

// handleThinkingStart emits a running ToolEntry for an in-progress thinking block.
// Called when the first thinking block is encountered in an entry where thinking
// was not already active.
func (es *ExtractionState) handleThinkingStart(ts time.Time) {
	es.thinkingSeq++
	id := fmt.Sprintf("thinking-%d", es.thinkingSeq)
	t := &internalTool{
		id:        id,
		name:      "Thinking",
		category:  "Thinking",
		startTime: ts,
	}
	es.toolMap[id] = t
	es.appendTool(t)
	es.thinkingTool = t
}

// handleThinkingEnd marks the current thinking ToolEntry as completed.
// Called when a subsequent entry contains tool_use or text, signalling that
// the model finished thinking and began acting.
func (es *ExtractionState) handleThinkingEnd(ts time.Time) {
	if es.thinkingTool == nil {
		return
	}
	t := es.thinkingTool
	t.completed = true
	if !t.startTime.IsZero() {
		t.durationMs = int(ts.Sub(t.startTime).Milliseconds())
	}
	delete(es.toolMap, t.id)
	es.thinkingTool = nil
}

// handleAgentToolUse records a running agent entry.
func (es *ExtractionState) handleAgentToolUse(b transcript.ToolUseBlock, ts time.Time) {
	var input struct {
		SubagentType string `json:"subagent_type"`
		Model        string `json:"model"`
		Description  string `json:"description"`
	}
	// Intentionally ignore parse errors — partial data is fine.
	_ = json.Unmarshal(b.Input, &input)

	agentType := input.SubagentType
	if agentType == "" {
		agentType = truncateAgentDescription(input.Description)
	}
	if agentType == "" {
		agentType = b.Name // "Agent" or "Task"
	}

	a := &internalAgent{
		id:          b.ID,
		name:        agentType,
		model:       input.Model,
		description: input.Description,
		status:      "running",
		startTime:   ts,
	}
	es.agentMap[b.ID] = a
	es.appendAgent(a)
}

// handleTodoWrite replaces the entire todo list. The input JSON is expected to
// have shape {"todos": [{...}, ...]}.
func (es *ExtractionState) handleTodoWrite(b transcript.ToolUseBlock) {
	var input struct {
		Todos []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
			Status  string `json:"status"`
		} `json:"todos"`
	}
	if err := json.Unmarshal(b.Input, &input); err != nil {
		return
	}

	es.todos = es.todos[:0]
	es.taskIDIndex = make(map[string]int)

	for _, t := range input.Todos {
		es.todos = append(es.todos, model.TodoItem{
			ID:      t.ID,
			Content: t.Content,
			Done:    normalizeStatusDone(t.Status),
		})
		if t.ID != "" {
			es.taskIDIndex[t.ID] = len(es.todos) - 1
		}
	}
}

// handleTaskCreate appends a new todo item from a TaskCreate tool_use block.
func (es *ExtractionState) handleTaskCreate(b transcript.ToolUseBlock) {
	var input struct {
		TaskID      string `json:"taskId"`
		Subject     string `json:"subject"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(b.Input, &input); err != nil {
		return
	}

	content := input.Subject
	if content == "" {
		content = input.Description
	}
	if content == "" {
		content = "Untitled task"
	}

	item := model.TodoItem{
		Content: content,
		Done:    normalizeStatusDone(input.Status),
	}

	// Determine the canonical ID: prefer the explicit taskId field, fall back
	// to the tool_use block ID so the result map stays consistent.
	taskID := input.TaskID
	if taskID == "" {
		taskID = b.ID
	}
	item.ID = taskID

	es.todos = append(es.todos, item)
	if taskID != "" {
		es.taskIDIndex[taskID] = len(es.todos) - 1
	}
}

// handleTaskUpdate mutates an existing todo item. Unknown task IDs are ignored.
func (es *ExtractionState) handleTaskUpdate(b transcript.ToolUseBlock) {
	var input struct {
		TaskID      string `json:"taskId"`
		Subject     string `json:"subject"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(b.Input, &input); err != nil {
		return
	}

	idx := es.resolveTaskIndex(input.TaskID)
	if idx < 0 {
		return
	}

	if input.Status != "" {
		es.todos[idx].Done = normalizeStatusDone(input.Status)
	}

	newContent := input.Subject
	if newContent == "" {
		newContent = input.Description
	}
	if newContent != "" {
		es.todos[idx].Content = newContent
	}
}

// processToolResult marks the matching tool or agent as completed/error and
// computes duration from the delta between result timestamp and start time.
func (es *ExtractionState) processToolResult(b transcript.ToolResultBlock, ts time.Time) {
	if t, ok := es.toolMap[b.ToolUseID]; ok {
		t.completed = true
		t.hasError = b.IsError
		// Only compute duration when startTime is set. Snapshot-restored tools
		// have zero startTime — leave their durationMs at 0 rather than
		// computing a nonsensical delta from the year 0001.
		if !t.startTime.IsZero() {
			t.durationMs = int(ts.Sub(t.startTime).Milliseconds())
		}
	}

	if a, ok := es.agentMap[b.ToolUseID]; ok {
		a.status = "completed"
		if !a.startTime.IsZero() {
			a.durationMs = int(ts.Sub(a.startTime).Milliseconds())
		}
	}
}

// ToTranscriptData collapses the current extraction state into a
// model.TranscriptData snapshot for the render layer.
func (es *ExtractionState) ToTranscriptData() *model.TranscriptData {
	tools := make([]model.ToolEntry, 0, len(es.displayTools))
	for _, t := range es.displayTools {
		entry := model.ToolEntry{
			Name:       t.name,
			Completed:  t.completed,
			DurationMs: t.durationMs,
			HasError:   t.hasError,
			Category:   t.category,
			Target:     t.target,
			StartTime:  t.startTime,
		}
		// If thinking is still "active" at snapshot time, the transcript has
		// been fully read — there's just no subsequent entry to close it yet.
		// Mark it completed in the output so the widget renders it as finished
		// rather than permanently yellow.
		if t == es.thinkingTool && !t.completed {
			entry.Completed = true
			if !t.startTime.IsZero() {
				entry.DurationMs = int(time.Since(t.startTime).Milliseconds())
			}
		}
		tools = append(tools, entry)
	}

	agents := make([]model.AgentEntry, 0, len(es.displayAgents))
	for i, a := range es.displayAgents {
		agents = append(agents, model.AgentEntry{
			ID:          a.id,
			Name:        a.name,
			Status:      a.status,
			Model:       a.model,
			Description: a.description,
			ColorIndex:  i % 8,
			StartTime:   a.startTime,
			DurationMs:  a.durationMs,
		})
	}

	todos := make([]model.TodoItem, len(es.todos))
	copy(todos, es.todos)

	skillNames := make([]string, len(es.skillNames))
	copy(skillNames, es.skillNames)

	tokenSamples := make([]model.TokenSample, len(es.tokenSamples))
	copy(tokenSamples, es.tokenSamples)

	return &model.TranscriptData{
		SessionName:    es.sessionName,
		Tools:          tools,
		Agents:         agents,
		Todos:          todos,
		SkillNames:     skillNames,
		TokenSamples:   tokenSamples,
		ThinkingActive: es.thinkingActive,
		ThinkingCount:  es.thinkingCount,
		SpinnerFrame:   es.spinnerFrame,
		DividerOffset:  es.dividerOffset,
		MessageCount:   es.messageCount,
	}
}

// resolveTaskIndex looks up a task ID in the index. If the ID is numeric, it
// also tries a one-based positional lookup as a fallback (matching claude-hud's
// resolveTaskIndex behaviour). Returns -1 when no match is found.
func (es *ExtractionState) resolveTaskIndex(taskID string) int {
	if taskID == "" {
		return -1
	}

	if idx, ok := es.taskIDIndex[taskID]; ok && idx < len(es.todos) {
		return idx
	}

	// Numeric one-based fallback: "1" => index 0.
	if n, err := strconv.Atoi(taskID); err == nil {
		if n >= 1 && n <= len(es.todos) {
			return n - 1
		}
	}

	return -1
}

// toolCategory returns the display category for a tool name: the library
// taxonomy plus one HUD policy override — Skill keeps its own category
// because the renderer has a dedicated Skill icon (the library folds Skill
// into Task). Everything else delegates so new tool names (Workflow,
// multi-agent aliases) categorize without a HUD release.
func toolCategory(name string) string {
	if name == "Skill" {
		return "Skill"
	}
	return string(tools.CategorizeToolName(name))
}

// extractTarget returns a short contextual string describing what a tool is
// operating on. Ported from claude-hud transcript.ts:173-190.
//
// Deliberately NOT tools.ToolSummary: the summary is a formatted one-liner
// (verb + shortened path), while the HUD renders bare targets next to
// category icons. Different display intent, kept app-side.
func extractTarget(toolName string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil {
		return ""
	}

	switch toolName {
	case "Read", "Write", "Edit":
		return getStrField(fields, "file_path", "path")
	case "Glob", "Grep":
		return getStrField(fields, "pattern")
	case "Bash":
		cmd := getStrField(fields, "command")
		if len(cmd) > bashTargetMaxLen {
			return cmd[:bashTargetMaxLen] + "..."
		}
		return cmd
	}
	return ""
}

// getStrField returns the string value of the first key found in fields
// (a variadic-fallback wrapper over the library's tools.GetString).
func getStrField(fields map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		if s := tools.GetString(fields, k); s != "" {
			return s
		}
	}
	return ""
}

// normalizeStatusDone converts a status string to the Done boolean used by
// model.TodoItem. "completed", "complete", and "done" map to true; everything
// else maps to false.
func normalizeStatusDone(status string) bool {
	switch status {
	case "completed", "complete", "done":
		return true
	default:
		return false
	}
}

// snapshotTokenSample is the serializable form of model.TokenSample.
type snapshotTokenSample struct {
	Timestamp string `json:"ts"` // RFC3339Nano
	Tokens    int    `json:"n"`
}

// usageKey identifies one API response by its token counts, standing in for
// the transcript's unmodeled message.id. Comparable so split entries that
// repeat the same usage dedupe with ==; the zero value means "no sample yet"
// (unreachable for a real sample, which requires OutputTokens > 0).
type usageKey struct {
	Input         int `json:"in"`
	Output        int `json:"out"`
	CacheRead     int `json:"cr"`
	CacheCreation int `json:"cc"`
}

// usageKeyOf extracts the dedupe key from a library usage record.
func usageKeyOf(u transcript.EntryUsage) usageKey {
	return usageKey{
		Input:         u.InputTokens,
		Output:        u.OutputTokens,
		CacheRead:     u.CacheReadInputTokens,
		CacheCreation: u.CacheCreationInputTokens,
	}
}

// extractionSnapshot is the serializable form of ExtractionState for persistence.
// StartTime is intentionally excluded — it is only meaningful within a single
// invocation for elapsed-time computation. Restored entries use DurationMs directly.
type extractionSnapshot struct {
	Tools          []snapshotTool        `json:"tools"`
	Agents         []snapshotAgent       `json:"agents"`
	Todos          []model.TodoItem      `json:"todos"`
	SkillNames     []string              `json:"skill_names,omitempty"`
	TokenSamples   []snapshotTokenSample `json:"token_samples,omitempty"`
	LastSampleKey  usageKey              `json:"last_sample_usage"`
	SessionName    string                `json:"session_name"`
	ThinkingActive bool                  `json:"thinking_active"`
	ThinkingCount  int                   `json:"thinking_count"`
	SpinnerFrame   int                   `json:"spinner_frame"`
	DividerOffset  int                   `json:"divider_offset"`
	MessageCount   int                   `json:"message_count"`
}

type snapshotTool struct {
	ID         string `json:"id,omitempty"` // tool_use ID; needed to correlate tool_result across invocations
	Name       string `json:"name"`
	Target     string `json:"target"`
	Category   string `json:"category"`
	Completed  bool   `json:"completed"`
	HasError   bool   `json:"has_error"`
	DurationMs int    `json:"duration_ms"`
	StartTime  string `json:"start_time,omitempty"` // RFC3339Nano; enables duration computation after restore
}

type snapshotAgent struct {
	ID          string `json:"id,omitempty"` // tool_use ID; needed to correlate tool_result across invocations
	AgentType   string `json:"agent_type"`
	Model       string `json:"model"`
	Description string `json:"description"`
	Status      string `json:"status"`
	DurationMs  int    `json:"duration_ms"`
	StartTime   string `json:"start_time,omitempty"` // RFC3339Nano; enables duration computation after restore
}

// MarshalSnapshot serializes the display-relevant state to JSON. It omits
// toolMap and agentMap because in-flight tool_use→tool_result correlations
// do not span invocations.
func (es *ExtractionState) MarshalSnapshot() (json.RawMessage, error) {
	tools := make([]snapshotTool, 0, len(es.displayTools))
	for _, t := range es.displayTools {
		st := snapshotTool{
			ID:         t.id,
			Name:       t.name,
			Target:     t.target,
			Category:   t.category,
			Completed:  t.completed,
			HasError:   t.hasError,
			DurationMs: t.durationMs,
		}
		if !t.startTime.IsZero() {
			st.StartTime = t.startTime.Format(time.RFC3339Nano)
		}
		tools = append(tools, st)
	}

	agents := make([]snapshotAgent, 0, len(es.displayAgents))
	for _, a := range es.displayAgents {
		sa := snapshotAgent{
			ID:          a.id,
			AgentType:   a.name,
			Model:       a.model,
			Description: a.description,
			Status:      a.status,
			DurationMs:  a.durationMs,
		}
		if !a.startTime.IsZero() {
			sa.StartTime = a.startTime.Format(time.RFC3339Nano)
		}
		agents = append(agents, sa)
	}

	todos := make([]model.TodoItem, len(es.todos))
	copy(todos, es.todos)

	skillNames := make([]string, len(es.skillNames))
	copy(skillNames, es.skillNames)

	tokenSamples := make([]snapshotTokenSample, 0, len(es.tokenSamples))
	for _, s := range es.tokenSamples {
		tokenSamples = append(tokenSamples, snapshotTokenSample{
			Timestamp: s.Timestamp.Format(time.RFC3339Nano),
			Tokens:    s.Tokens,
		})
	}

	snap := extractionSnapshot{
		Tools:          tools,
		Agents:         agents,
		Todos:          todos,
		SkillNames:     skillNames,
		TokenSamples:   tokenSamples,
		LastSampleKey:  es.lastSampleUsage,
		SessionName:    es.sessionName,
		ThinkingActive: es.thinkingActive,
		ThinkingCount:  es.thinkingCount,
		SpinnerFrame:   es.spinnerFrame,
		DividerOffset:  es.dividerOffset,
		MessageCount:   es.messageCount,
	}
	return json.Marshal(snap)
}

// UnmarshalSnapshot restores display state from a previously serialized snapshot.
// The toolMap and agentMap are not restored (in-flight correlations don't survive
// across invocations). Restored tools have no startTime, so duration won't be
// recomputed — DurationMs retains its final value from the snapshot.
func (es *ExtractionState) UnmarshalSnapshot(data json.RawMessage) error {
	if len(data) == 0 {
		return nil
	}
	var snap extractionSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	es.displayTools = make([]*internalTool, 0, len(snap.Tools))
	for _, st := range snap.Tools {
		t := &internalTool{
			id:         st.ID,
			name:       st.Name,
			target:     st.Target,
			category:   st.Category,
			completed:  st.Completed,
			hasError:   st.HasError,
			durationMs: st.DurationMs,
		}
		if st.StartTime != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, st.StartTime); err == nil {
				t.startTime = parsed
			}
		}
		es.displayTools = append(es.displayTools, t)
		// Rebuild toolMap for non-completed tools so their tool_result can be
		// matched in the next incremental read.
		if !t.completed && t.id != "" {
			es.toolMap[t.id] = t
		}
	}

	es.displayAgents = make([]*internalAgent, 0, len(snap.Agents))
	for _, sa := range snap.Agents {
		a := &internalAgent{
			id:          sa.ID,
			name:        sa.AgentType,
			model:       sa.Model,
			description: sa.Description,
			status:      sa.Status,
			durationMs:  sa.DurationMs,
		}
		if sa.StartTime != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, sa.StartTime); err == nil {
				a.startTime = parsed
			}
		}
		es.displayAgents = append(es.displayAgents, a)
		// Rebuild agentMap for running agents so their tool_result can complete them.
		if a.status == "running" && a.id != "" {
			es.agentMap[a.id] = a
		}
	}

	if snap.Todos != nil {
		es.todos = snap.Todos
		es.taskIDIndex = make(map[string]int)
		for i, item := range es.todos {
			if item.ID != "" {
				es.taskIDIndex[item.ID] = i
			}
		}
	}

	if snap.SkillNames != nil {
		es.skillNames = snap.SkillNames
	}

	es.tokenSamples = make([]model.TokenSample, 0, len(snap.TokenSamples))
	for _, s := range snap.TokenSamples {
		if s.Timestamp == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, s.Timestamp)
		if err != nil {
			continue
		}
		es.tokenSamples = append(es.tokenSamples, model.TokenSample{
			Timestamp: parsed,
			Tokens:    s.Tokens,
		})
	}

	es.lastSampleUsage = snap.LastSampleKey
	es.sessionName = snap.SessionName
	es.thinkingActive = snap.ThinkingActive
	es.thinkingCount = snap.ThinkingCount
	es.spinnerFrame = snap.SpinnerFrame
	es.dividerOffset = snap.DividerOffset
	es.messageCount = snap.MessageCount
	return nil
}

// agentDescriptionMaxLen is the maximum number of runes kept from a description
// field when it is used as the agent display name.
const agentDescriptionMaxLen = 30

// truncateAgentDescription returns s truncated to agentDescriptionMaxLen runes
// with "..." appended when truncation occurs. Returns an empty string unchanged.
func truncateAgentDescription(s string) string {
	runes := []rune(s)
	if len(runes) <= agentDescriptionMaxLen {
		return s
	}
	return string(runes[:agentDescriptionMaxLen]) + "..."
}

// SchemaVersion is the offset-store schema version for extraction snapshots.
// Bump it whenever extraction semantics change in a way that would produce
// different results from the same transcript data, so stale snapshots are
// discarded and the transcript is re-read from byte 0.
//
// v2: skill detection from <command-name> tags.
// v3: extraction re-typed over the agent-ouija library Entry.
// v4: tool categories delegate to tools.CategorizeToolName (Skill override
// kept app-side); Workflow and multi-agent tool names now categorize as
// Task instead of Other, so stored categories change.
// v5: native CLI commands (e.g. /model, /clear) are excluded from skill
// detection, so SkillNames no longer contains built-in command names.
// v6: token samples record output tokens only (input/cache excluded) and
// dedupe split assistant entries that repeat one API response's usage, so
// stored samples change meaning and count.
const SchemaVersion = 6
