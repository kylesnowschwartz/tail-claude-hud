// Package transcript — ExtractionState bridges parsed transcript entries into
// model.TranscriptData for statusline rendering.
//
// Call ProcessEntry for each parsed Entry in order. Call ToTranscriptData to
// produce a snapshot suitable for passing to the render layer.
package transcript

import (
	"encoding/json"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// maxTools is the maximum number of ToolEntries kept in the display slice.
const maxTools = 20

// maxAgents is the maximum number of AgentEntries kept in the display slice.
const maxAgents = 10

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
	agentType   string
	model       string
	description string
	status      string // "running" or "completed"
	startTime   time.Time
	durationMs  int // 0 = still running; populated by a separate card
}

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

	// Todos is the authoritative todo list; replaced on TodoWrite, mutated on
	// TaskCreate/TaskUpdate.
	Todos []model.TodoItem

	// taskIDIndex maps TaskCreate-assigned IDs to positions in Todos.
	taskIDIndex map[string]int

	// displayTools holds the ordered list of tools for rendering (newest last).
	displayTools []*internalTool

	// displayAgents holds the ordered list of agents for rendering (newest last).
	displayAgents []*internalAgent

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

	// spinnerFrame is a monotonic counter incremented on each statusline invocation.
	// It is persisted in the snapshot so successive invocations always advance the frame.
	spinnerFrame int

	// dividerOffset is a monotonic counter incremented once per new tool_use.
	// Persisted in the snapshot so the scrolling ticker position survives
	// across statusline invocations.
	dividerOffset int
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
func (es *ExtractionState) ProcessEntry(e Entry) {
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

	blocks := ExtractContentBlocks(e)
	ts := e.ParsedTimestamp()
	if ts.IsZero() {
		ts = time.Now()
	}

	for _, b := range blocks.ToolUse {
		es.processToolUse(b, ts)
	}

	for _, b := range blocks.ToolResult {
		es.processToolResult(b, ts)
	}

	// Update thinking state based on blocks present in this entry.
	// Thinking is active only when a thinking block was seen but no subsequent
	// tool_use or text block appeared in the same message.
	if len(blocks.Thinking) > 0 {
		es.thinkingCount += len(blocks.Thinking)
		es.thinkingActive = len(blocks.ToolUse) == 0 && !blocks.HasText
	} else if len(blocks.ToolUse) > 0 || blocks.HasText {
		// A message with tool_use or text but no thinking clears the active flag.
		es.thinkingActive = false
	}
}

// processToolUse dispatches a tool_use block to the appropriate handler.
func (es *ExtractionState) processToolUse(b ToolUseBlock, ts time.Time) {
	switch b.Name {
	case "Task", "Agent":
		es.handleAgentToolUse(b, ts)
	case "TodoWrite":
		es.handleTodoWrite(b)
	case "TaskCreate":
		es.handleTaskCreate(b)
	case "TaskUpdate":
		es.handleTaskUpdate(b)
	default:
		es.handleRegularToolUse(b, ts)
	}
}

// handleRegularToolUse records a running tool entry and appends it to the
// display slice, pruning the oldest if the limit is exceeded.
func (es *ExtractionState) handleRegularToolUse(b ToolUseBlock, ts time.Time) {
	t := &internalTool{
		id:        b.ID,
		name:      b.Name,
		target:    extractTarget(b.Name, b.Input),
		category:  toolCategory(b.Name),
		startTime: ts,
	}
	es.toolMap[b.ID] = t
	es.displayTools = append(es.displayTools, t)
	es.dividerOffset++

	if len(es.displayTools) > maxTools {
		// Prune the oldest entry from both the display slice and the map.
		oldest := es.displayTools[0]
		es.displayTools = es.displayTools[1:]
		delete(es.toolMap, oldest.id)
	}
}

// handleAgentToolUse records a running agent entry.
func (es *ExtractionState) handleAgentToolUse(b ToolUseBlock, ts time.Time) {
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
		agentType:   agentType,
		model:       input.Model,
		description: input.Description,
		status:      "running",
		startTime:   ts,
	}
	es.agentMap[b.ID] = a
	es.displayAgents = append(es.displayAgents, a)

	if len(es.displayAgents) > maxAgents {
		oldest := es.displayAgents[0]
		es.displayAgents = es.displayAgents[1:]
		delete(es.agentMap, oldest.id)
	}
}

// handleTodoWrite replaces the entire todo list. The input JSON is expected to
// have shape {"todos": [{...}, ...]}.
func (es *ExtractionState) handleTodoWrite(b ToolUseBlock) {
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

	es.Todos = es.Todos[:0]
	es.taskIDIndex = make(map[string]int)

	for _, t := range input.Todos {
		es.Todos = append(es.Todos, model.TodoItem{
			ID:      t.ID,
			Content: t.Content,
			Done:    normalizeStatusDone(t.Status),
		})
		if t.ID != "" {
			es.taskIDIndex[t.ID] = len(es.Todos) - 1
		}
	}
}

// handleTaskCreate appends a new todo item from a TaskCreate tool_use block.
func (es *ExtractionState) handleTaskCreate(b ToolUseBlock) {
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

	es.Todos = append(es.Todos, item)
	if taskID != "" {
		es.taskIDIndex[taskID] = len(es.Todos) - 1
	}
}

// handleTaskUpdate mutates an existing todo item. Unknown task IDs are ignored.
func (es *ExtractionState) handleTaskUpdate(b ToolUseBlock) {
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
		es.Todos[idx].Done = normalizeStatusDone(input.Status)
	}

	newContent := input.Subject
	if newContent == "" {
		newContent = input.Description
	}
	if newContent != "" {
		es.Todos[idx].Content = newContent
	}
}

// processToolResult marks the matching tool or agent as completed/error and
// computes duration from the delta between result timestamp and start time.
func (es *ExtractionState) processToolResult(b ToolResultBlock, ts time.Time) {
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
		tools = append(tools, model.ToolEntry{
			Name:       t.name,
			Completed:  t.completed,
			DurationMs: t.durationMs,
			HasError:   t.hasError,
			Category:   t.category,
			Target:     t.target,
		})
	}

	agents := make([]model.AgentEntry, 0, len(es.displayAgents))
	for i, a := range es.displayAgents {
		agents = append(agents, model.AgentEntry{
			Name:        a.agentType,
			Status:      a.status,
			Model:       a.model,
			Description: a.description,
			ColorIndex:  i % 8,
			StartTime:   a.startTime,
			DurationMs:  a.durationMs,
		})
	}

	todos := make([]model.TodoItem, len(es.Todos))
	copy(todos, es.Todos)

	return &model.TranscriptData{
		SessionName:    es.sessionName,
		Tools:          tools,
		Agents:         agents,
		Todos:          todos,
		ThinkingActive: es.thinkingActive,
		ThinkingCount:  es.thinkingCount,
		SpinnerFrame:   es.spinnerFrame,
		DividerOffset:  es.dividerOffset,
	}
}

// resolveTaskIndex looks up a task ID in the index. If the ID is numeric, it
// also tries a one-based positional lookup as a fallback (matching claude-hud's
// resolveTaskIndex behaviour). Returns -1 when no match is found.
func (es *ExtractionState) resolveTaskIndex(taskID string) int {
	if taskID == "" {
		return -1
	}

	if idx, ok := es.taskIDIndex[taskID]; ok && idx < len(es.Todos) {
		return idx
	}

	// Numeric one-based fallback: "1" => index 0.
	if isNumericString(taskID) {
		n := parseInt(taskID)
		if n >= 1 && n <= len(es.Todos) {
			return n - 1
		}
	}

	return -1
}

// toolCategory returns the display category for a tool name.
// Categories: file, shell, search, web, agent, internal.
func toolCategory(name string) string {
	switch name {
	case "Read", "Write", "Edit":
		return "file"
	case "Bash":
		return "shell"
	case "Grep", "Glob":
		return "search"
	case "WebFetch", "WebSearch":
		return "web"
	case "Agent", "Task":
		return "agent"
	default:
		return "internal"
	}
}

// extractTarget returns a short contextual string describing what a tool is
// operating on. Ported from claude-hud transcript.ts:173-190.
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

// getStrField returns the string value of the first key found in fields.
// Returns empty string when no key matches or the value is not a JSON string.
func getStrField(fields map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		raw, ok := fields[k]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
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

// extractionSnapshot is the serializable form of ExtractionState for persistence.
// StartTime is intentionally excluded — it is only meaningful within a single
// invocation for elapsed-time computation. Restored entries use DurationMs directly.
type extractionSnapshot struct {
	Tools          []snapshotTool   `json:"tools"`
	Agents         []snapshotAgent  `json:"agents"`
	Todos          []model.TodoItem `json:"todos"`
	SessionName    string           `json:"session_name"`
	ThinkingActive bool             `json:"thinking_active"`
	ThinkingCount  int              `json:"thinking_count"`
	SpinnerFrame   int              `json:"spinner_frame"`
	DividerOffset  int              `json:"divider_offset"`
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
			AgentType:   a.agentType,
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

	todos := make([]model.TodoItem, len(es.Todos))
	copy(todos, es.Todos)

	snap := extractionSnapshot{
		Tools:          tools,
		Agents:         agents,
		Todos:          todos,
		SessionName:    es.sessionName,
		ThinkingActive: es.thinkingActive,
		ThinkingCount:  es.thinkingCount,
		SpinnerFrame:   es.spinnerFrame,
		DividerOffset:  es.dividerOffset,
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
			agentType:   sa.AgentType,
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
		es.Todos = snap.Todos
		es.taskIDIndex = make(map[string]int)
		for i, item := range es.Todos {
			if item.ID != "" {
				es.taskIDIndex[item.ID] = i
			}
		}
	}

	es.sessionName = snap.SessionName
	es.thinkingActive = snap.ThinkingActive
	es.thinkingCount = snap.ThinkingCount
	es.spinnerFrame = snap.SpinnerFrame
	es.dividerOffset = snap.DividerOffset
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

// isNumericString reports whether s is a non-empty string of ASCII digits.
func isNumericString(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseInt parses a decimal integer string. Panics are not possible because
// callers gate with isNumericString; overflow is theoretically possible but
// irrelevant at todo-list scales.
func parseInt(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
