package transcript

import (
	"encoding/json"
	"testing"
	"time"
)

// makeToolUseEntry builds a minimal Entry containing a single tool_use block.
func makeToolUseEntry(id, name string, input map[string]interface{}) Entry {
	inputJSON, _ := json.Marshal(input)
	contentItem := map[string]interface{}{
		"type":  "tool_use",
		"id":    id,
		"name":  name,
		"input": json.RawMessage(inputJSON),
	}
	content, _ := json.Marshal([]interface{}{contentItem})
	var e Entry
	e.Message.Content = content
	e.Message.Role = "assistant"
	e.Timestamp = time.Now().Format(time.RFC3339Nano)
	return e
}

// makeToolResultEntry builds a minimal Entry containing a single tool_result block.
func makeToolResultEntry(toolUseID string, isError bool) Entry {
	return makeToolResultEntryAt(toolUseID, isError, time.Now())
}

// makeToolResultEntryAt builds a tool_result Entry with an explicit timestamp.
func makeToolResultEntryAt(toolUseID string, isError bool, ts time.Time) Entry {
	contentItem := map[string]interface{}{
		"type":        "tool_result",
		"tool_use_id": toolUseID,
		"is_error":    isError,
		"content":     "ok",
	}
	content, _ := json.Marshal([]interface{}{contentItem})
	var e Entry
	e.Message.Content = content
	e.Message.Role = "user"
	e.Timestamp = ts.Format(time.RFC3339Nano)
	return e
}

// makeToolUseEntryAt builds a tool_use Entry with an explicit timestamp.
func makeToolUseEntryAt(id, name string, input map[string]interface{}, ts time.Time) Entry {
	e := makeToolUseEntry(id, name, input)
	e.Timestamp = ts.Format(time.RFC3339Nano)
	return e
}

// ---- Spec 1: tool_use records a running ToolEntry --------------------------

func TestProcessEntry_ToolUse_RecordsRunning(t *testing.T) {
	es := NewExtractionState()
	e := makeToolUseEntry("id-1", "Read", map[string]interface{}{
		"file_path": "main.go",
	})
	es.ProcessEntry(e)

	data := es.ToTranscriptData()
	if len(data.Tools) != 1 {
		t.Fatalf("expected 1 tool entry, got %d", len(data.Tools))
	}
	tool := data.Tools[0]
	if tool.Name != "Read" {
		t.Errorf("expected Name=Read, got %q", tool.Name)
	}
	// Count == 0 means running (per convention).
	if tool.Count != 0 {
		t.Errorf("expected Count=0 (running), got %d", tool.Count)
	}
}

func TestProcessEntry_ToolUse_RecordsID(t *testing.T) {
	es := NewExtractionState()
	e := makeToolUseEntry("toolu-abc", "Bash", map[string]interface{}{
		"command": "ls -la",
	})
	es.ProcessEntry(e)

	// Internal map should have the entry.
	if _, ok := es.toolMap["toolu-abc"]; !ok {
		t.Error("expected toolMap to contain entry for 'toolu-abc'")
	}
}

// ---- Spec 2: tool_result marks the matching entry completed/error ----------

func TestProcessEntry_ToolResult_MarksCompleted(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-1", "Read", map[string]interface{}{"file_path": "x.go"}))
	es.ProcessEntry(makeToolResultEntry("id-1", false))

	data := es.ToTranscriptData()
	if len(data.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(data.Tools))
	}
	if data.Tools[0].Count != 1 {
		t.Errorf("expected Count=1 (completed), got %d", data.Tools[0].Count)
	}
}

func TestProcessEntry_ToolResult_IsError_StillMarksCompleted(t *testing.T) {
	// is_error tools are still marked as "completed" from the model perspective
	// (Count > 0 = not running). The error distinction is tracked internally.
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-2", "Bash", map[string]interface{}{"command": "exit 1"}))
	es.ProcessEntry(makeToolResultEntry("id-2", true))

	data := es.ToTranscriptData()
	if data.Tools[0].Count != 1 {
		t.Errorf("expected Count=1 for error tool, got %d", data.Tools[0].Count)
	}
}

func TestProcessEntry_ToolResult_UnknownID_IsIgnored(t *testing.T) {
	es := NewExtractionState()
	// Result for a tool_use we never saw — should not panic or create entries.
	es.ProcessEntry(makeToolResultEntry("ghost-id", false))

	data := es.ToTranscriptData()
	if len(data.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(data.Tools))
	}
}

// ---- Spec 3: max 20 ToolEntries and 10 AgentEntries -----------------------

func TestProcessEntry_ToolLimit_KeepsLast20(t *testing.T) {
	es := NewExtractionState()
	for i := 0; i < 25; i++ {
		id := string(rune('a' + i)) // unique IDs: a, b, c, ...
		es.ProcessEntry(makeToolUseEntry(id, "Read", map[string]interface{}{"file_path": "f.go"}))
	}

	data := es.ToTranscriptData()
	if len(data.Tools) != maxTools {
		t.Errorf("expected %d tools, got %d", maxTools, len(data.Tools))
	}
}

func TestProcessEntry_AgentLimit_KeepsLast10(t *testing.T) {
	es := NewExtractionState()
	for i := 0; i < 15; i++ {
		id := string(rune('a' + i))
		es.ProcessEntry(makeToolUseEntry(id, "Task", map[string]interface{}{
			"subagent_type": "research",
		}))
	}

	data := es.ToTranscriptData()
	if len(data.Agents) != maxAgents {
		t.Errorf("expected %d agents, got %d", maxAgents, len(data.Agents))
	}
}

func TestProcessEntry_ToolMap_PrunedWhenLimitExceeded(t *testing.T) {
	es := NewExtractionState()
	// Add 21 tools; the first one should be pruned from the map.
	firstID := "first-tool"
	es.ProcessEntry(makeToolUseEntry(firstID, "Bash", map[string]interface{}{"command": "echo 1"}))
	for i := 0; i < 20; i++ {
		id := string(rune('a' + i))
		es.ProcessEntry(makeToolUseEntry(id, "Read", map[string]interface{}{"file_path": "f.go"}))
	}

	if _, ok := es.toolMap[firstID]; ok {
		t.Error("expected first tool to be pruned from toolMap after exceeding limit")
	}
}

// ---- Spec 4: TodoWrite replaces the full list ------------------------------

func TestProcessEntry_TodoWrite_ReplacesList(t *testing.T) {
	es := NewExtractionState()

	// Seed with an initial todo via TaskCreate.
	es.ProcessEntry(makeToolUseEntry("tc-1", "TaskCreate", map[string]interface{}{
		"subject": "Old task",
	}))

	todos := []map[string]interface{}{
		{"id": "t1", "content": "Buy milk", "status": "pending"},
		{"id": "t2", "content": "Write tests", "status": "completed"},
	}
	es.ProcessEntry(makeToolUseEntry("tw-1", "TodoWrite", map[string]interface{}{
		"todos": todos,
	}))

	data := es.ToTranscriptData()
	if len(data.Todos) != 2 {
		t.Fatalf("expected 2 todos after TodoWrite, got %d", len(data.Todos))
	}
	if data.Todos[0].Content != "Buy milk" {
		t.Errorf("expected 'Buy milk', got %q", data.Todos[0].Content)
	}
	if data.Todos[0].Done {
		t.Error("expected first todo to be not done")
	}
	if !data.Todos[1].Done {
		t.Error("expected second todo to be done")
	}
}

func TestProcessEntry_TaskCreate_AppendsTodo(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("tc-1", "TaskCreate", map[string]interface{}{
		"subject": "Implement feature",
		"status":  "in_progress",
	}))

	data := es.ToTranscriptData()
	if len(data.Todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(data.Todos))
	}
	if data.Todos[0].Content != "Implement feature" {
		t.Errorf("unexpected content: %q", data.Todos[0].Content)
	}
	if data.Todos[0].Done {
		t.Error("in_progress should map to Done=false")
	}
}

func TestProcessEntry_TaskUpdate_ModifiesTodo(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("tc-1", "TaskCreate", map[string]interface{}{
		"taskId":  "task-1",
		"subject": "Old subject",
		"status":  "pending",
	}))
	es.ProcessEntry(makeToolUseEntry("tu-1", "TaskUpdate", map[string]interface{}{
		"taskId":  "task-1",
		"subject": "New subject",
		"status":  "completed",
	}))

	data := es.ToTranscriptData()
	if len(data.Todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(data.Todos))
	}
	if data.Todos[0].Content != "New subject" {
		t.Errorf("expected 'New subject', got %q", data.Todos[0].Content)
	}
	if !data.Todos[0].Done {
		t.Error("expected Done=true after status=completed update")
	}
}

// ---- Spec 5: extractTarget returns the right field per tool ----------------

func TestExtractTarget_ReadWriteEdit(t *testing.T) {
	for _, name := range []string{"Read", "Write", "Edit"} {
		input, _ := json.Marshal(map[string]string{"file_path": "/tmp/foo.go"})
		got := extractTarget(name, input)
		if got != "/tmp/foo.go" {
			t.Errorf("%s: expected '/tmp/foo.go', got %q", name, got)
		}
	}
}

func TestExtractTarget_EditFallbackToPath(t *testing.T) {
	// Some blocks use "path" instead of "file_path".
	input, _ := json.Marshal(map[string]string{"path": "/tmp/bar.go"})
	got := extractTarget("Edit", input)
	if got != "/tmp/bar.go" {
		t.Errorf("expected '/tmp/bar.go', got %q", got)
	}
}

func TestExtractTarget_GlobGrep(t *testing.T) {
	for _, name := range []string{"Glob", "Grep"} {
		input, _ := json.Marshal(map[string]string{"pattern": "*.go"})
		got := extractTarget(name, input)
		if got != "*.go" {
			t.Errorf("%s: expected '*.go', got %q", name, got)
		}
	}
}

func TestExtractTarget_Bash_Short(t *testing.T) {
	input, _ := json.Marshal(map[string]string{"command": "ls -la"})
	got := extractTarget("Bash", input)
	if got != "ls -la" {
		t.Errorf("expected 'ls -la', got %q", got)
	}
}

func TestExtractTarget_Bash_LongCommandTruncated(t *testing.T) {
	cmd := "echo this is a very long command that exceeds the limit by a lot"
	input, _ := json.Marshal(map[string]string{"command": cmd})
	got := extractTarget("Bash", input)

	expectedPrefix := cmd[:bashTargetMaxLen]
	if len(got) != bashTargetMaxLen+3 { // 3 = len("...")
		t.Errorf("expected length %d, got %d (%q)", bashTargetMaxLen+3, len(got), got)
	}
	if got[:bashTargetMaxLen] != expectedPrefix {
		t.Errorf("prefix mismatch: expected %q, got %q", expectedPrefix, got[:bashTargetMaxLen])
	}
	if got[len(got)-3:] != "..." {
		t.Errorf("expected trailing '...', got %q", got[len(got)-3:])
	}
}

func TestExtractTarget_UnknownTool_Empty(t *testing.T) {
	input, _ := json.Marshal(map[string]string{"anything": "value"})
	got := extractTarget("WebFetch", input)
	if got != "" {
		t.Errorf("expected empty string for unknown tool, got %q", got)
	}
}

func TestExtractTarget_NilInput_Empty(t *testing.T) {
	got := extractTarget("Read", nil)
	if got != "" {
		t.Errorf("expected empty string for nil input, got %q", got)
	}
}

// ---- Agent tracking --------------------------------------------------------

func TestProcessEntry_AgentEntry_Running(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("agent-1", "Task", map[string]interface{}{
		"subagent_type": "research",
		"model":         "claude-haiku",
		"description":   "Find relevant files",
	}))

	data := es.ToTranscriptData()
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(data.Agents))
	}
	a := data.Agents[0]
	if a.Status != "running" {
		t.Errorf("expected status=running, got %q", a.Status)
	}
	if a.Name != "research" {
		t.Errorf("expected Name=research, got %q", a.Name)
	}
}

func TestProcessEntry_AgentEntry_CompletedOnResult(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("agent-1", "Task", map[string]interface{}{
		"subagent_type": "coding",
	}))
	es.ProcessEntry(makeToolResultEntry("agent-1", false))

	data := es.ToTranscriptData()
	if data.Agents[0].Status != "completed" {
		t.Errorf("expected status=completed, got %q", data.Agents[0].Status)
	}
}

// ---- normalizeStatusDone ---------------------------------------------------

func TestNormalizeStatusDone(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"completed", true},
		{"complete", true},
		{"done", true},
		{"pending", false},
		{"in_progress", false},
		{"running", false},
		{"", false},
		{"unknown", false},
	}
	for _, c := range cases {
		got := normalizeStatusDone(c.status)
		if got != c.want {
			t.Errorf("normalizeStatusDone(%q) = %v, want %v", c.status, got, c.want)
		}
	}
}

// ---- resolveTaskIndex -------------------------------------------------------

func TestResolveTaskIndex_ByID(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("tc-1", "TaskCreate", map[string]interface{}{
		"taskId":  "task-99",
		"subject": "Test task",
	}))

	idx := es.resolveTaskIndex("task-99")
	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}
}

func TestResolveTaskIndex_NumericFallback(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("tc-1", "TaskCreate", map[string]interface{}{
		"subject": "First task",
	}))
	es.ProcessEntry(makeToolUseEntry("tc-2", "TaskCreate", map[string]interface{}{
		"subject": "Second task",
	}))

	// "2" should resolve to index 1 (one-based).
	idx := es.resolveTaskIndex("2")
	if idx != 1 {
		t.Errorf("expected index 1 for numeric ID '2', got %d", idx)
	}
}

func TestResolveTaskIndex_UnknownID(t *testing.T) {
	es := NewExtractionState()
	idx := es.resolveTaskIndex("nonexistent")
	if idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

// ---- ToTranscriptData snapshot is a copy -----------------------------------

func TestToTranscriptData_TodosCopied(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("tc-1", "TaskCreate", map[string]interface{}{
		"subject": "Task A",
	}))

	data := es.ToTranscriptData()
	data.Todos[0].Content = "mutated"

	// The internal state must not be affected.
	if es.Todos[0].Content == "mutated" {
		t.Error("ToTranscriptData should return a copy, not a reference to internal state")
	}
}

// ---- New ToolEntry fields: Category, Target, HasError, DurationMs ---------

func TestToTranscriptData_Category_FileTools(t *testing.T) {
	for _, name := range []string{"Read", "Write", "Edit"} {
		es := NewExtractionState()
		es.ProcessEntry(makeToolUseEntry("id-1", name, map[string]interface{}{"file_path": "x.go"}))
		data := es.ToTranscriptData()
		if data.Tools[0].Category != "file" {
			t.Errorf("%s: expected category=file, got %q", name, data.Tools[0].Category)
		}
	}
}

func TestToTranscriptData_Category_ShellTool(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-1", "Bash", map[string]interface{}{"command": "ls"}))
	data := es.ToTranscriptData()
	if data.Tools[0].Category != "shell" {
		t.Errorf("expected category=shell, got %q", data.Tools[0].Category)
	}
}

func TestToTranscriptData_Category_SearchTools(t *testing.T) {
	for _, name := range []string{"Grep", "Glob"} {
		es := NewExtractionState()
		es.ProcessEntry(makeToolUseEntry("id-1", name, map[string]interface{}{"pattern": "*.go"}))
		data := es.ToTranscriptData()
		if data.Tools[0].Category != "search" {
			t.Errorf("%s: expected category=search, got %q", name, data.Tools[0].Category)
		}
	}
}

func TestToTranscriptData_Category_WebTools(t *testing.T) {
	for _, name := range []string{"WebFetch", "WebSearch"} {
		es := NewExtractionState()
		es.ProcessEntry(makeToolUseEntry("id-1", name, map[string]interface{}{}))
		data := es.ToTranscriptData()
		if data.Tools[0].Category != "web" {
			t.Errorf("%s: expected category=web, got %q", name, data.Tools[0].Category)
		}
	}
}

func TestToTranscriptData_Category_InternalTool(t *testing.T) {
	// Tools not in any known category fall back to "internal".
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-1", "SomeFutureTool", map[string]interface{}{}))
	data := es.ToTranscriptData()
	if data.Tools[0].Category != "internal" {
		t.Errorf("expected category=internal for unknown tool, got %q", data.Tools[0].Category)
	}
}

func TestToTranscriptData_Target_PassedThrough(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-1", "Read", map[string]interface{}{"file_path": "/src/main.go"}))
	data := es.ToTranscriptData()
	if data.Tools[0].Target != "/src/main.go" {
		t.Errorf("expected Target=/src/main.go, got %q", data.Tools[0].Target)
	}
}

func TestToTranscriptData_HasError_FalseWhenNoError(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-1", "Bash", map[string]interface{}{"command": "true"}))
	es.ProcessEntry(makeToolResultEntry("id-1", false))
	data := es.ToTranscriptData()
	if data.Tools[0].HasError {
		t.Error("expected HasError=false for successful tool result")
	}
}

func TestToTranscriptData_HasError_TrueWhenIsError(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-1", "Bash", map[string]interface{}{"command": "exit 1"}))
	es.ProcessEntry(makeToolResultEntry("id-1", true))
	data := es.ToTranscriptData()
	if !data.Tools[0].HasError {
		t.Error("expected HasError=true when tool_result.is_error is true")
	}
}

func TestToTranscriptData_HasError_FalseWhenStillRunning(t *testing.T) {
	// A running tool (no result yet) must have HasError=false.
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-1", "Bash", map[string]interface{}{"command": "sleep 10"}))
	data := es.ToTranscriptData()
	if data.Tools[0].HasError {
		t.Error("expected HasError=false for still-running tool")
	}
}

func TestToTranscriptData_DurationMs_ZeroWhenRunning(t *testing.T) {
	// A tool with no result yet must have DurationMs=0.
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-1", "Read", map[string]interface{}{"file_path": "x.go"}))
	data := es.ToTranscriptData()
	if data.Tools[0].DurationMs != 0 {
		t.Errorf("expected DurationMs=0 for running tool, got %d", data.Tools[0].DurationMs)
	}
}

// ---- Agent metadata fields flow through ToTranscriptData -------------------

func TestAgentMetadata_ModelAndDescription(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("agent-1", "Task", map[string]interface{}{
		"subagent_type": "coding",
		"model":         "claude-haiku-4-5",
		"description":   "Implement the feature",
	}))

	data := es.ToTranscriptData()
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(data.Agents))
	}
	a := data.Agents[0]
	if a.Model != "claude-haiku-4-5" {
		t.Errorf("expected Model=%q, got %q", "claude-haiku-4-5", a.Model)
	}
	if a.Description != "Implement the feature" {
		t.Errorf("expected Description=%q, got %q", "Implement the feature", a.Description)
	}
}

func TestAgentMetadata_StartTimePopulated(t *testing.T) {
	es := NewExtractionState()
	before := time.Now()
	es.ProcessEntry(makeToolUseEntry("agent-1", "Task", map[string]interface{}{
		"subagent_type": "coding",
	}))
	after := time.Now()

	data := es.ToTranscriptData()
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(data.Agents))
	}
	st := data.Agents[0].StartTime
	if st.IsZero() {
		t.Error("expected StartTime to be set, got zero value")
	}
	if st.Before(before) || st.After(after) {
		t.Errorf("StartTime %v is outside expected range [%v, %v]", st, before, after)
	}
}

func TestAgentMetadata_DurationMsZero(t *testing.T) {
	// DurationMs starts at 0 (still running); a separate card will compute it.
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("agent-1", "Task", map[string]interface{}{
		"subagent_type": "coding",
	}))

	data := es.ToTranscriptData()
	if data.Agents[0].DurationMs != 0 {
		t.Errorf("expected DurationMs=0, got %d", data.Agents[0].DurationMs)
	}
}

func TestAgentMetadata_ColorIndexSequential(t *testing.T) {
	es := NewExtractionState()
	for i := 0; i < 10; i++ {
		id := string(rune('a' + i))
		es.ProcessEntry(makeToolUseEntry(id, "Task", map[string]interface{}{
			"subagent_type": "coding",
		}))
	}

	data := es.ToTranscriptData()
	// With maxAgents=10 and 10 agents added, all 10 should be present.
	if len(data.Agents) != 10 {
		t.Fatalf("expected 10 agents, got %d", len(data.Agents))
	}
	for i, a := range data.Agents {
		want := i % 8
		if a.ColorIndex != want {
			t.Errorf("agent[%d]: expected ColorIndex=%d, got %d", i, want, a.ColorIndex)
		}
	}
}

func TestAgentMetadata_ColorIndexWrapsAt8(t *testing.T) {
	// Add 9 agents: indices 0-7 then wrap back to 0.
	es := NewExtractionState()
	for i := 0; i < 9; i++ {
		id := string(rune('a' + i))
		es.ProcessEntry(makeToolUseEntry(id, "Task", map[string]interface{}{
			"subagent_type": "coding",
		}))
	}

	data := es.ToTranscriptData()
	if len(data.Agents) != 9 {
		t.Fatalf("expected 9 agents, got %d", len(data.Agents))
	}
	// The 9th agent (index 8) should wrap to ColorIndex 0.
	if data.Agents[8].ColorIndex != 0 {
		t.Errorf("expected ColorIndex=0 for 9th agent (wrap), got %d", data.Agents[8].ColorIndex)
	}
}

// ---- Regular tools are not tracked as agents ------------------------------

func TestProcessEntry_RegularTool_NotInAgents(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-1", "Read", map[string]interface{}{"file_path": "x.go"}))

	data := es.ToTranscriptData()
	if len(data.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(data.Agents))
	}
}

// ---- Task/Agent tool_use is not tracked as a regular tool ------------------

func TestProcessEntry_TaskToolUse_NotInTools(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("agent-1", "Task", map[string]interface{}{
		"subagent_type": "research",
	}))

	data := es.ToTranscriptData()
	if len(data.Tools) != 0 {
		t.Errorf("expected 0 tools (Task should only appear in Agents), got %d", len(data.Tools))
	}
}

// ---- SessionName extraction -------------------------------------------------

func TestProcessEntry_CustomTitle_SetsSessionName(t *testing.T) {
	es := NewExtractionState()
	var e Entry
	e.Type = "custom-title"
	e.CustomTitle = "My Session Title"
	es.ProcessEntry(e)

	data := es.ToTranscriptData()
	if data.SessionName != "My Session Title" {
		t.Errorf("expected SessionName=%q, got %q", "My Session Title", data.SessionName)
	}
}

func TestProcessEntry_CustomTitle_EmptyValue_NoChange(t *testing.T) {
	// A custom-title entry with an empty CustomTitle should not set the session name.
	es := NewExtractionState()
	var e Entry
	e.Type = "custom-title"
	e.CustomTitle = ""
	es.ProcessEntry(e)

	data := es.ToTranscriptData()
	if data.SessionName != "" {
		t.Errorf("expected empty SessionName when CustomTitle is empty, got %q", data.SessionName)
	}
}

func TestProcessEntry_Slug_FallbackWhenNoCustomTitle(t *testing.T) {
	// Slug is used as SessionName only when no custom-title has been seen.
	es := NewExtractionState()
	var e Entry
	e.Type = "summary"
	e.Slug = "slug-based-name"
	es.ProcessEntry(e)

	data := es.ToTranscriptData()
	if data.SessionName != "slug-based-name" {
		t.Errorf("expected SessionName=%q from slug, got %q", "slug-based-name", data.SessionName)
	}
}

func TestProcessEntry_CustomTitle_TakesPriorityOverSlug(t *testing.T) {
	// custom-title should override a previously-set slug.
	es := NewExtractionState()

	// First: a slug from an earlier entry.
	var slugEntry Entry
	slugEntry.Type = "summary"
	slugEntry.Slug = "slug-name"
	es.ProcessEntry(slugEntry)

	// Then: a custom-title entry arrives.
	var titleEntry Entry
	titleEntry.Type = "custom-title"
	titleEntry.CustomTitle = "Proper Title"
	es.ProcessEntry(titleEntry)

	data := es.ToTranscriptData()
	if data.SessionName != "Proper Title" {
		t.Errorf("expected custom-title to override slug, got %q", data.SessionName)
	}
}

func TestProcessEntry_Slug_NotOverridenByLaterSlug(t *testing.T) {
	// Once a slug is set, a later entry with a different slug should not change it
	// (slug is first-seen wins when no custom-title is present).
	es := NewExtractionState()

	var e1 Entry
	e1.Type = "summary"
	e1.Slug = "first-slug"
	es.ProcessEntry(e1)

	var e2 Entry
	e2.Type = "summary"
	e2.Slug = "second-slug"
	es.ProcessEntry(e2)

	data := es.ToTranscriptData()
	if data.SessionName != "first-slug" {
		t.Errorf("expected first slug to be retained, got %q", data.SessionName)
	}
}

// ---- Tool duration computation from timestamp deltas -----------------------

func TestToolDuration_ComputedFromTimestampDelta(t *testing.T) {
	// tool_use at T=0, tool_result at T=1.5s => DurationMs=1500.
	es := NewExtractionState()
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(1500 * time.Millisecond)

	es.ProcessEntry(makeToolUseEntryAt("id-1", "Bash", map[string]interface{}{"command": "sleep 1"}, t0))
	es.ProcessEntry(makeToolResultEntryAt("id-1", false, t1))

	data := es.ToTranscriptData()
	if len(data.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(data.Tools))
	}
	if data.Tools[0].DurationMs != 1500 {
		t.Errorf("expected DurationMs=1500, got %d", data.Tools[0].DurationMs)
	}
}

func TestToolDuration_ZeroWhenStillRunning(t *testing.T) {
	// A tool with no result yet must have DurationMs=0.
	es := NewExtractionState()
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	es.ProcessEntry(makeToolUseEntryAt("id-1", "Read", map[string]interface{}{"file_path": "x.go"}, t0))

	data := es.ToTranscriptData()
	if data.Tools[0].DurationMs != 0 {
		t.Errorf("expected DurationMs=0 for running tool, got %d", data.Tools[0].DurationMs)
	}
}

// ---- Agent duration computation from timestamp deltas ----------------------

func TestAgentDuration_ComputedFromTimestampDelta(t *testing.T) {
	// agent tool_use at T=0, tool_result at T=3s => DurationMs=3000.
	es := NewExtractionState()
	t0 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(3000 * time.Millisecond)

	es.ProcessEntry(makeToolUseEntryAt("agent-1", "Task", map[string]interface{}{
		"subagent_type": "coding",
		"model":         "claude-haiku",
	}, t0))
	es.ProcessEntry(makeToolResultEntryAt("agent-1", false, t1))

	data := es.ToTranscriptData()
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(data.Agents))
	}
	if data.Agents[0].DurationMs != 3000 {
		t.Errorf("expected DurationMs=3000, got %d", data.Agents[0].DurationMs)
	}
}

func TestAgentDuration_ZeroWhenStillRunning(t *testing.T) {
	// An agent with no result yet must have DurationMs=0.
	es := NewExtractionState()
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	es.ProcessEntry(makeToolUseEntryAt("agent-1", "Task", map[string]interface{}{
		"subagent_type": "research",
	}, t0))

	data := es.ToTranscriptData()
	if data.Agents[0].DurationMs != 0 {
		t.Errorf("expected DurationMs=0 for running agent, got %d", data.Agents[0].DurationMs)
	}
}
