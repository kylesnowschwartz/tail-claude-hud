package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	// Completed == false means running.
	if tool.Completed {
		t.Errorf("expected Completed=false (running), got true")
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
	if !data.Tools[0].Completed {
		t.Errorf("expected Completed=true, got false")
	}
}

func TestProcessEntry_ToolResult_IsError_StillMarksCompleted(t *testing.T) {
	// is_error tools are still marked as completed. The error distinction is
	// tracked via HasError.
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-2", "Bash", map[string]interface{}{"command": "exit 1"}))
	es.ProcessEntry(makeToolResultEntry("id-2", true))

	data := es.ToTranscriptData()
	if !data.Tools[0].Completed {
		t.Errorf("expected Completed=true for error tool, got false")
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

// ---- Agent type fallback (spec: subagent_type empty → description → tool name) ----

// TestAgentType_FallsBackToDescription checks that when subagent_type is empty
// the description field is used as the display name (truncated if long).
func TestAgentType_FallsBackToDescription(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("agent-1", "Agent", map[string]interface{}{
		"description": "Explore the codebase",
	}))

	data := es.ToTranscriptData()
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(data.Agents))
	}
	if data.Agents[0].Name != "Explore the codebase" {
		t.Errorf("expected Name=%q, got %q", "Explore the codebase", data.Agents[0].Name)
	}
}

// TestAgentType_TruncatesLongDescription checks that descriptions longer than
// 30 runes are truncated with "..." appended.
func TestAgentType_TruncatesLongDescription(t *testing.T) {
	longDesc := "Read every file in the project and summarize its contents"
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("agent-1", "Agent", map[string]interface{}{
		"description": longDesc,
	}))

	data := es.ToTranscriptData()
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(data.Agents))
	}
	name := data.Agents[0].Name
	if len([]rune(name)) > 33 { // 30 chars + "..."
		t.Errorf("expected name length <= 33 runes, got %d: %q", len([]rune(name)), name)
	}
	want := "Read every file in the project..."
	if name != want {
		t.Errorf("expected Name=%q, got %q", want, name)
	}
}

// TestAgentType_FallsBackToToolName checks that when both subagent_type and
// description are empty the tool name ("Agent" or "Task") is used.
func TestAgentType_FallsBackToToolName(t *testing.T) {
	tests := []struct {
		toolName string
	}{
		{"Agent"},
		{"Task"},
	}
	for _, tc := range tests {
		t.Run(tc.toolName, func(t *testing.T) {
			es := NewExtractionState()
			es.ProcessEntry(makeToolUseEntry("agent-1", tc.toolName, map[string]interface{}{}))

			data := es.ToTranscriptData()
			if len(data.Agents) != 1 {
				t.Fatalf("expected 1 agent, got %d", len(data.Agents))
			}
			if data.Agents[0].Name != tc.toolName {
				t.Errorf("expected Name=%q, got %q", tc.toolName, data.Agents[0].Name)
			}
		})
	}
}

// ---- Thinking block detection (specs 5 & 6) --------------------------------

// makeThinkingEntry builds an Entry with a thinking block only (no tool_use, no text).
func makeThinkingEntry() Entry {
	content, _ := json.Marshal([]interface{}{
		map[string]interface{}{"type": "thinking", "thinking": "Let me consider this..."},
	})
	var e Entry
	e.Message.Role = "assistant"
	e.Message.Content = content
	e.Timestamp = "2025-01-15T10:00:00Z"
	return e
}

// makeThinkingThenToolUseEntry builds an Entry with a thinking block followed by a tool_use.
func makeThinkingThenToolUseEntry(toolID, toolName string, input map[string]interface{}) Entry {
	inputJSON, _ := json.Marshal(input)
	content, _ := json.Marshal([]interface{}{
		map[string]interface{}{"type": "thinking", "thinking": "Let me use a tool"},
		map[string]interface{}{
			"type":  "tool_use",
			"id":    toolID,
			"name":  toolName,
			"input": json.RawMessage(inputJSON),
		},
	})
	var e Entry
	e.Message.Role = "assistant"
	e.Message.Content = content
	e.Timestamp = "2025-01-15T10:00:01Z"
	return e
}

// makeThinkingThenTextEntry builds an Entry with a thinking block followed by a text block.
func makeThinkingThenTextEntry() Entry {
	content, _ := json.Marshal([]interface{}{
		map[string]interface{}{"type": "thinking", "thinking": "Let me respond"},
		map[string]interface{}{"type": "text", "text": "Here is my answer."},
	})
	var e Entry
	e.Message.Role = "assistant"
	e.Message.Content = content
	e.Timestamp = "2025-01-15T10:00:02Z"
	return e
}

func TestThinking_ActiveWhenOnlyThinkingBlock(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeThinkingEntry())

	data := es.ToTranscriptData()
	if !data.ThinkingActive {
		t.Error("expected ThinkingActive=true when last message contained only a thinking block")
	}
	if data.ThinkingCount != 1 {
		t.Errorf("expected ThinkingCount=1, got %d", data.ThinkingCount)
	}
}

func TestThinking_CountAccumulates(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeThinkingEntry())
	es.ProcessEntry(makeThinkingEntry())
	es.ProcessEntry(makeThinkingEntry())

	data := es.ToTranscriptData()
	if data.ThinkingCount != 3 {
		t.Errorf("expected ThinkingCount=3, got %d", data.ThinkingCount)
	}
}

func TestThinking_ActiveClearedWhenToolUseFollows(t *testing.T) {
	// A thinking block in the same message as a tool_use should not be active.
	es := NewExtractionState()
	es.ProcessEntry(makeThinkingThenToolUseEntry("id-1", "Read", map[string]interface{}{"file_path": "x.go"}))

	data := es.ToTranscriptData()
	if data.ThinkingActive {
		t.Error("expected ThinkingActive=false when thinking block is followed by tool_use in the same message")
	}
	// Count still increments — thinking happened even if not active.
	if data.ThinkingCount != 1 {
		t.Errorf("expected ThinkingCount=1, got %d", data.ThinkingCount)
	}
}

func TestThinking_ActiveClearedWhenTextFollows(t *testing.T) {
	// A thinking block followed by text in the same message should not be active.
	es := NewExtractionState()
	es.ProcessEntry(makeThinkingThenTextEntry())

	data := es.ToTranscriptData()
	if data.ThinkingActive {
		t.Error("expected ThinkingActive=false when thinking block is followed by text in the same message")
	}
	if data.ThinkingCount != 1 {
		t.Errorf("expected ThinkingCount=1, got %d", data.ThinkingCount)
	}
}

func TestThinking_ActiveSetThenClearedBySubsequentToolUse(t *testing.T) {
	// First entry: thinking only (active=true).
	// Second entry: tool_use only (active=false).
	es := NewExtractionState()
	es.ProcessEntry(makeThinkingEntry())

	data := es.ToTranscriptData()
	if !data.ThinkingActive {
		t.Fatal("expected ThinkingActive=true after first entry")
	}

	es.ProcessEntry(makeToolUseEntry("id-1", "Read", map[string]interface{}{"file_path": "x.go"}))
	data = es.ToTranscriptData()
	if data.ThinkingActive {
		t.Error("expected ThinkingActive=false after a subsequent tool_use entry")
	}
	// Count should still be 1 (only the first entry had a thinking block).
	if data.ThinkingCount != 1 {
		t.Errorf("expected ThinkingCount=1, got %d", data.ThinkingCount)
	}
}

func TestThinking_NoThinkingBlocks_ActiveFalse(t *testing.T) {
	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntry("id-1", "Bash", map[string]interface{}{"command": "ls"}))

	data := es.ToTranscriptData()
	if data.ThinkingActive {
		t.Error("expected ThinkingActive=false when no thinking blocks present")
	}
	if data.ThinkingCount != 0 {
		t.Errorf("expected ThinkingCount=0, got %d", data.ThinkingCount)
	}
}

// ---- Snapshot round-trip (spec 6: tools from prior invocation appear after restore) ----

func TestMarshalUnmarshalSnapshot_ToolsRoundTrip(t *testing.T) {
	// First invocation: process a completed tool.
	es1 := NewExtractionState()
	t0 := makeToolUseEntry("id-1", "Read", map[string]interface{}{"file_path": "main.go"})
	es1.ProcessEntry(t0)
	es1.ProcessEntry(makeToolResultEntry("id-1", false))

	snap, err := es1.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	// Second invocation: fresh state, restore snapshot, process new line.
	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}
	// Process a new tool in the second invocation.
	es2.ProcessEntry(makeToolUseEntry("id-2", "Bash", map[string]interface{}{"command": "ls"}))

	data := es2.ToTranscriptData()
	if len(data.Tools) != 2 {
		t.Fatalf("expected 2 tools after restore+new entry, got %d", len(data.Tools))
	}
	// The first tool (from snapshot) should be completed.
	if data.Tools[0].Name != "Read" {
		t.Errorf("expected restored tool Name=Read, got %q", data.Tools[0].Name)
	}
	if !data.Tools[0].Completed {
		t.Errorf("expected restored tool Completed=true, got false")
	}
	// The second tool (new) should be running.
	if data.Tools[1].Name != "Bash" {
		t.Errorf("expected new tool Name=Bash, got %q", data.Tools[1].Name)
	}
	if data.Tools[1].Completed {
		t.Errorf("expected new tool Completed=false (running), got true")
	}
}

// TestMarshalUnmarshalSnapshot_DurationMsPreserved verifies that a completed
// tool's DurationMs value survives a full marshal/unmarshal snapshot cycle.
// This is the core requirement for spec 2: after snapshot restore, the display
// layer must see the original elapsed duration, not a misleading 0.
func TestMarshalUnmarshalSnapshot_DurationMsPreserved(t *testing.T) {
	t0 := time.Date(2024, 11, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(2500 * time.Millisecond) // 2.5 seconds

	es1 := NewExtractionState()
	es1.ProcessEntry(makeToolUseEntryAt("dur-1", "Read",
		map[string]interface{}{"file_path": "foo.go"}, t0))
	es1.ProcessEntry(makeToolResultEntryAt("dur-1", false, t1))

	data1 := es1.ToTranscriptData()
	if data1.Tools[0].DurationMs != 2500 {
		t.Fatalf("pre-snapshot DurationMs=%d, want 2500", data1.Tools[0].DurationMs)
	}

	snap, err := es1.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}

	data2 := es2.ToTranscriptData()
	if len(data2.Tools) != 1 {
		t.Fatalf("expected 1 tool after restore, got %d", len(data2.Tools))
	}
	if data2.Tools[0].DurationMs != 2500 {
		t.Errorf("DurationMs not preserved across snapshot: got %d, want 2500", data2.Tools[0].DurationMs)
	}
}

func TestMarshalUnmarshalSnapshot_AgentsRoundTrip(t *testing.T) {
	es1 := NewExtractionState()
	es1.ProcessEntry(makeToolUseEntry("agent-1", "Task", map[string]interface{}{
		"subagent_type": "coding",
		"model":         "claude-haiku",
		"description":   "Implement feature",
	}))
	es1.ProcessEntry(makeToolResultEntry("agent-1", false))

	snap, err := es1.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}

	data := es2.ToTranscriptData()
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent after restore, got %d", len(data.Agents))
	}
	a := data.Agents[0]
	if a.Name != "coding" {
		t.Errorf("expected Name=coding, got %q", a.Name)
	}
	if a.Model != "claude-haiku" {
		t.Errorf("expected Model=claude-haiku, got %q", a.Model)
	}
	if a.Status != "completed" {
		t.Errorf("expected Status=completed, got %q", a.Status)
	}
}

func TestMarshalUnmarshalSnapshot_TodosRoundTrip(t *testing.T) {
	es1 := NewExtractionState()
	todos := []map[string]interface{}{
		{"id": "t1", "content": "Buy milk", "status": "pending"},
		{"id": "t2", "content": "Write tests", "status": "completed"},
	}
	es1.ProcessEntry(makeToolUseEntry("tw-1", "TodoWrite", map[string]interface{}{
		"todos": todos,
	}))

	snap, err := es1.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}

	data := es2.ToTranscriptData()
	if len(data.Todos) != 2 {
		t.Fatalf("expected 2 todos after restore, got %d", len(data.Todos))
	}
	if data.Todos[0].Content != "Buy milk" {
		t.Errorf("expected 'Buy milk', got %q", data.Todos[0].Content)
	}
	if data.Todos[1].Done != true {
		t.Error("expected second todo Done=true after restore")
	}
}

func TestMarshalUnmarshalSnapshot_SessionNameAndThinking(t *testing.T) {
	es1 := NewExtractionState()
	var titleEntry Entry
	titleEntry.Type = "custom-title"
	titleEntry.CustomTitle = "My Session"
	es1.ProcessEntry(titleEntry)
	es1.ProcessEntry(makeThinkingEntry())
	es1.ProcessEntry(makeThinkingEntry())

	snap, err := es1.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}

	data := es2.ToTranscriptData()
	if data.SessionName != "My Session" {
		t.Errorf("expected SessionName=My Session, got %q", data.SessionName)
	}
	if data.ThinkingCount != 2 {
		t.Errorf("expected ThinkingCount=2, got %d", data.ThinkingCount)
	}
	if !data.ThinkingActive {
		t.Error("expected ThinkingActive=true after restore")
	}
}

func TestUnmarshalSnapshot_NilData_NoError(t *testing.T) {
	es := NewExtractionState()
	if err := es.UnmarshalSnapshot(nil); err != nil {
		t.Errorf("expected no error for nil snapshot, got %v", err)
	}
	// State should remain empty.
	data := es.ToTranscriptData()
	if len(data.Tools) != 0 || len(data.Agents) != 0 || len(data.Todos) != 0 {
		t.Error("expected empty state after nil snapshot restore")
	}
}

func TestUnmarshalSnapshot_MalformedData_ReturnsError(t *testing.T) {
	es := NewExtractionState()
	err := es.UnmarshalSnapshot(json.RawMessage(`not valid json`))
	if err == nil {
		t.Error("expected error for malformed snapshot data")
	}
}

// ---- Snapshot: spec 1 — tool_use and tool_result in the same JSONL entry (zero-delta timing) ----

// TestSnapshotSpec1_ToolUseAndResultSameEntry verifies that a single Entry containing
// both a tool_use and a tool_result block (same-entry round-trip) is handled correctly:
// the tool is recorded as completed with DurationMs=0 (zero delta) and survives a
// marshal/unmarshal snapshot cycle without getting stuck as running.
func TestSnapshotSpec1_ToolUseAndResultSameEntry(t *testing.T) {
	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	// Build an Entry with both tool_use and tool_result blocks at the same timestamp.
	inputJSON, _ := json.Marshal(map[string]string{"file_path": "main.go"})
	toolUseItem := map[string]interface{}{
		"type":  "tool_use",
		"id":    "same-id-1",
		"name":  "Read",
		"input": json.RawMessage(inputJSON),
	}
	toolResultItem := map[string]interface{}{
		"type":        "tool_result",
		"tool_use_id": "same-id-1",
		"is_error":    false,
		"content":     "file contents",
	}
	content, _ := json.Marshal([]interface{}{toolUseItem, toolResultItem})
	var e Entry
	e.Message.Role = "assistant"
	e.Message.Content = content
	e.Timestamp = ts.Format(time.RFC3339Nano)

	es := NewExtractionState()
	es.ProcessEntry(e)

	data := es.ToTranscriptData()
	if len(data.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(data.Tools))
	}
	tool := data.Tools[0]
	if !tool.Completed {
		t.Errorf("expected Completed=true, got false")
	}
	if tool.DurationMs != 0 {
		t.Errorf("expected DurationMs=0 for zero-delta same-entry, got %d", tool.DurationMs)
	}
	if tool.HasError {
		t.Error("expected HasError=false for successful same-entry result")
	}

	// Snapshot round-trip: restored state must also show the tool as completed.
	snap, err := es.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}

	data2 := es2.ToTranscriptData()
	if len(data2.Tools) != 1 {
		t.Fatalf("expected 1 tool after restore, got %d", len(data2.Tools))
	}
	if !data2.Tools[0].Completed {
		t.Errorf("restored tool: expected Completed=true, got false")
	}
	// The tool must NOT be in the toolMap (it is completed — no pending result needed).
	if _, ok := es2.toolMap["same-id-1"]; ok {
		t.Error("completed tool should not be in restored toolMap")
	}
}

// TestSnapshotSpec1_ZeroDeltaIsNotStuck ensures that a tool resolved within the
// same entry does not remain "running" after a snapshot restore, which would
// mean it would permanently show as running in the HUD.
func TestSnapshotSpec1_ZeroDeltaIsNotStuck(t *testing.T) {
	es := NewExtractionState()
	ts := time.Now()

	inputJSON, _ := json.Marshal(map[string]string{"command": "echo hi"})
	content, _ := json.Marshal([]interface{}{
		map[string]interface{}{
			"type":  "tool_use",
			"id":    "zero-delta-1",
			"name":  "Bash",
			"input": json.RawMessage(inputJSON),
		},
		map[string]interface{}{
			"type":        "tool_result",
			"tool_use_id": "zero-delta-1",
			"is_error":    false,
			"content":     "hi",
		},
	})
	var e Entry
	e.Message.Role = "assistant"
	e.Message.Content = content
	e.Timestamp = ts.Format(time.RFC3339Nano)
	es.ProcessEntry(e)

	snap, _ := es.MarshalSnapshot()
	es2 := NewExtractionState()
	_ = es2.UnmarshalSnapshot(snap)

	data := es2.ToTranscriptData()
	for _, tool := range data.Tools {
		if !tool.Completed {
			t.Errorf("tool %q is stuck as running after restore; expected Completed=true", tool.Name)
		}
	}
}

// ---- Snapshot: spec 2 — tool_use in invocation N, tool_result in invocation N+3 ----

// TestSnapshotSpec2_MultiHopRestore verifies that a tool started in one invocation
// and resolved three invocations later is matched correctly through snapshot restores.
// This is the core cross-invocation correlation use case for the toolMap rebuild.
func TestSnapshotSpec2_MultiHopToolRestore(t *testing.T) {
	t0 := time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC)

	// Invocation 1: tool_use is seen; no result yet.
	es1 := NewExtractionState()
	es1.ProcessEntry(makeToolUseEntryAt("multi-hop-1", "Bash",
		map[string]interface{}{"command": "long-running-script.sh"}, t0))

	snap1, err := es1.MarshalSnapshot()
	if err != nil {
		t.Fatalf("invocation 1 MarshalSnapshot: %v", err)
	}

	// Invocation 2: no new entries; snapshot passes through unchanged.
	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap1); err != nil {
		t.Fatalf("invocation 2 UnmarshalSnapshot: %v", err)
	}
	// Tool is still running after restore.
	if data := es2.ToTranscriptData(); data.Tools[0].Completed {
		t.Fatalf("invocation 2: expected tool still running (Completed=false), got Completed=true")
	}
	// toolMap must contain the running tool so the result can be matched.
	if _, ok := es2.toolMap["multi-hop-1"]; !ok {
		t.Fatal("invocation 2: running tool not in toolMap after restore")
	}

	snap2, _ := es2.MarshalSnapshot()

	// Invocation 3: still no result; another hop.
	es3 := NewExtractionState()
	if err := es3.UnmarshalSnapshot(snap2); err != nil {
		t.Fatalf("invocation 3 UnmarshalSnapshot: %v", err)
	}
	if _, ok := es3.toolMap["multi-hop-1"]; !ok {
		t.Fatal("invocation 3: running tool not in toolMap after restore")
	}
	snap3, _ := es3.MarshalSnapshot()

	// Invocation 4: tool_result finally arrives (3 hops later).
	t1 := t0.Add(5 * time.Second)
	es4 := NewExtractionState()
	if err := es4.UnmarshalSnapshot(snap3); err != nil {
		t.Fatalf("invocation 4 UnmarshalSnapshot: %v", err)
	}
	es4.ProcessEntry(makeToolResultEntryAt("multi-hop-1", false, t1))

	data := es4.ToTranscriptData()
	if len(data.Tools) != 1 {
		t.Fatalf("invocation 4: expected 1 tool, got %d", len(data.Tools))
	}
	tool := data.Tools[0]
	if !tool.Completed {
		t.Errorf("invocation 4: expected Completed=true, got false")
	}
	if tool.HasError {
		t.Error("invocation 4: expected HasError=false")
	}
	// The display must show completed — the tool must not be stuck as running.
	// (Map cleanup is an implementation detail; the critical invariant is the display value.)
}

// TestSnapshotSpec2_MultiHopAgentRestore verifies that a running agent in invocation N
// receives its tool_result in invocation N+3 and is marked completed correctly.
func TestSnapshotSpec2_MultiHopAgentRestore(t *testing.T) {
	t0 := time.Date(2024, 7, 1, 9, 0, 0, 0, time.UTC)

	es1 := NewExtractionState()
	es1.ProcessEntry(makeToolUseEntryAt("agent-hop-1", "Task",
		map[string]interface{}{"subagent_type": "research", "model": "claude-haiku"}, t0))

	snap1, _ := es1.MarshalSnapshot()

	es2 := NewExtractionState()
	_ = es2.UnmarshalSnapshot(snap1)
	if _, ok := es2.agentMap["agent-hop-1"]; !ok {
		t.Fatal("invocation 2: running agent not in agentMap after restore")
	}
	snap2, _ := es2.MarshalSnapshot()

	es3 := NewExtractionState()
	_ = es3.UnmarshalSnapshot(snap2)
	snap3, _ := es3.MarshalSnapshot()

	// Invocation 4: agent completes.
	t1 := t0.Add(10 * time.Second)
	es4 := NewExtractionState()
	_ = es4.UnmarshalSnapshot(snap3)
	es4.ProcessEntry(makeToolResultEntryAt("agent-hop-1", false, t1))

	data := es4.ToTranscriptData()
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(data.Agents))
	}
	if data.Agents[0].Status != "completed" {
		t.Errorf("expected Status=completed, got %q", data.Agents[0].Status)
	}
	// The display must show completed — the agent must not be stuck as running.
	// (Map cleanup is an implementation detail; the critical invariant is the display value.)
}

// ---- Snapshot: spec 3 — 10 agents completing in rapid succession (display slot eviction) ----

// TestSnapshotSpec3_TenAgentsRapidSuccession verifies that 10 agents completing in rapid
// succession respect the maxAgents display cap and that evicted agents do not leave
// dangling entries in agentMap that could prevent future results from being processed.
func TestSnapshotSpec3_TenAgentsRapidSuccession(t *testing.T) {
	es := NewExtractionState()
	base := time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)

	// Add 10 agents (exactly the limit).
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("agent-%02d", i)
		agentType := fmt.Sprintf("worker-%d", i)
		es.ProcessEntry(makeToolUseEntryAt(id, "Task",
			map[string]interface{}{"subagent_type": agentType},
			base.Add(time.Duration(i)*time.Millisecond),
		))
	}

	if len(es.displayAgents) != maxAgents {
		t.Fatalf("expected %d agents after 10 additions, got %d", maxAgents, len(es.displayAgents))
	}

	// Complete all 10 in rapid succession.
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("agent-%02d", i)
		es.ProcessEntry(makeToolResultEntryAt(id, false,
			base.Add(time.Duration(i)*time.Millisecond+500*time.Millisecond),
		))
	}

	data := es.ToTranscriptData()
	if len(data.Agents) != maxAgents {
		t.Fatalf("expected %d agents, got %d", maxAgents, len(data.Agents))
	}
	for i, a := range data.Agents {
		if a.Status != "completed" {
			t.Errorf("agent[%d] (%s): expected Status=completed, got %q", i, a.Name, a.Status)
		}
	}
}

// TestSnapshotSpec3_ElevenAgentsEvictsOldest verifies that adding an 11th agent evicts
// the oldest from displayAgents and that the evicted agent's ID is removed from agentMap,
// preventing a memory leak from dangling map entries.
func TestSnapshotSpec3_ElevenAgentsEvictsOldest(t *testing.T) {
	es := NewExtractionState()
	base := time.Date(2024, 8, 2, 0, 0, 0, 0, time.UTC)

	// Add exactly maxAgents (10) agents; all remain running.
	for i := 0; i < maxAgents; i++ {
		id := fmt.Sprintf("ag-%d", i)
		es.ProcessEntry(makeToolUseEntryAt(id, "Task",
			map[string]interface{}{"subagent_type": fmt.Sprintf("type-%d", i)},
			base.Add(time.Duration(i)*time.Millisecond),
		))
	}

	// The 11th agent should evict "ag-0".
	es.ProcessEntry(makeToolUseEntryAt("ag-10", "Task",
		map[string]interface{}{"subagent_type": "type-10"},
		base.Add(100*time.Millisecond),
	))

	// ag-0 must no longer appear in displayAgents.
	for _, a := range es.displayAgents {
		if a.id == "ag-0" {
			t.Error("evicted agent 'ag-0' still present in displayAgents")
		}
	}

	// ag-0 must not be in agentMap (it was running when evicted).
	if _, ok := es.agentMap["ag-0"]; ok {
		t.Error("evicted running agent 'ag-0' still present in agentMap; this is a map leak")
	}

	// The display slice should be capped at maxAgents.
	if len(es.displayAgents) != maxAgents {
		t.Errorf("expected %d agents after eviction, got %d", maxAgents, len(es.displayAgents))
	}

	// Snapshot round-trip: eviction state is preserved.
	snap, _ := es.MarshalSnapshot()
	es2 := NewExtractionState()
	_ = es2.UnmarshalSnapshot(snap)

	data := es2.ToTranscriptData()
	if len(data.Agents) != maxAgents {
		t.Errorf("after restore: expected %d agents, got %d", maxAgents, len(data.Agents))
	}
	for _, a := range data.Agents {
		if a.Name == "type-0" {
			t.Error("evicted agent 'type-0' should not appear after snapshot restore")
		}
	}
}

// ---- Snapshot: spec 4 — transcript truncation mid-session (offset > file size reset) ----

// TestSnapshotSpec4_TruncationResetsSnapshot verifies the StateManager behaviour
// when the stored byte offset exceeds the current file size: the snapshot must be
// discarded and the read must restart from byte 0. This mirrors a transcript file
// being truncated (new session started in the same file path).
func TestSnapshotSpec4_TruncationResetsSnapshot(t *testing.T) {
	dir := t.TempDir()
	stateDir := t.TempDir()
	transcriptPath := filepath.Join(dir, "session.jsonl")

	sm := NewStateManager(stateDir)

	// Write a line and save state so the offset advances past byte 0.
	toolUseEntry := makeToolUseEntryAt("truncate-tool-1", "Read",
		map[string]interface{}{"file_path": "x.go"},
		time.Now(),
	)
	line1, _ := json.Marshal(toolUseEntry)
	if err := os.WriteFile(transcriptPath, append(line1, '\n'), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	lines, err := sm.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("first ReadIncremental: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	// Save snapshot at the advanced offset.
	es1 := NewExtractionState()
	for _, l := range lines {
		var e Entry
		if err := json.Unmarshal([]byte(l), &e); err == nil {
			es1.ProcessEntry(e)
		}
	}
	snap1, _ := es1.MarshalSnapshot()
	sm.SetSnapshot(snap1)
	if err := sm.SaveState(transcriptPath); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Simulate truncation: write a new (shorter) file, resetting to before the saved offset.
	toolUseEntry2 := makeToolUseEntryAt("truncate-tool-2", "Write",
		map[string]interface{}{"file_path": "new.go"},
		time.Now(),
	)
	line2, _ := json.Marshal(toolUseEntry2)
	// Write only this new entry (shorter than the old file + offset).
	if err := os.WriteFile(transcriptPath, append(line2, '\n'), 0o644); err != nil {
		t.Fatalf("truncate-write: %v", err)
	}

	// The new file is smaller than the saved offset only if line1 was longer than line2.
	// To guarantee the offset exceeds the new size, truncate to an empty file first,
	// then write a very short new entry.
	shortContent := []byte("{\"type\":\"summary\",\"slug\":\"new-session\"}\n")
	if err := os.WriteFile(transcriptPath, shortContent, 0o644); err != nil {
		t.Fatalf("truncate to short: %v", err)
	}

	// Now ReadIncremental should detect offset > file size, reset to 0, and clear snapshot.
	lines2, err := sm.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("ReadIncremental after truncation: %v", err)
	}

	// The snapshot must be discarded when truncation is detected.
	restoredSnap := sm.LoadSnapshot()
	if restoredSnap != nil {
		t.Error("expected nil snapshot after truncation reset, but snapshot was retained")
	}

	// Lines from the new (shorter) file should be returned.
	if len(lines2) != 1 {
		t.Fatalf("expected 1 line from new file, got %d", len(lines2))
	}
}

// TestSnapshotSpec4_OffsetExactlyAtFileSizeIsNotReset verifies the boundary condition:
// an offset equal to the file size is valid (the file hasn't grown yet) and must NOT
// trigger a truncation reset.
func TestSnapshotSpec4_OffsetExactlyAtFileSizeIsNotReset(t *testing.T) {
	dir := t.TempDir()
	stateDir := t.TempDir()
	transcriptPath := filepath.Join(dir, "session.jsonl")

	sm := NewStateManager(stateDir)

	content := []byte("{\"type\":\"summary\",\"slug\":\"my-session\"}\n")
	if err := os.WriteFile(transcriptPath, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// First read: advances offset to len(content).
	_, err := sm.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if err := sm.SaveState(transcriptPath); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Second read: file unchanged, offset == file size. Must return 0 lines (no new data),
	// not reset as if truncated.
	lines, err := sm.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected 0 new lines when offset == file size, got %d", len(lines))
	}
}

// ---- Snapshot: spec 5 — no tools or agents permanently stuck as 'running' ----

// TestSnapshotSpec5_NoToolsStuckRunning verifies that after processing a tool_result,
// the corresponding tool is never stuck as running across snapshot round-trips.
func TestSnapshotSpec5_NoToolsStuckRunning(t *testing.T) {
	es := NewExtractionState()
	t0 := time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)

	// Start several tools.
	ids := []string{"stuck-1", "stuck-2", "stuck-3"}
	for i, id := range ids {
		es.ProcessEntry(makeToolUseEntryAt(id, "Read",
			map[string]interface{}{"file_path": fmt.Sprintf("file%d.go", i)},
			t0.Add(time.Duration(i)*time.Second),
		))
	}

	// Resolve all of them.
	for i, id := range ids {
		es.ProcessEntry(makeToolResultEntryAt(id, false,
			t0.Add(time.Duration(i)*time.Second+500*time.Millisecond),
		))
	}

	// After resolution, no tool should be running.
	data := es.ToTranscriptData()
	for _, tool := range data.Tools {
		if !tool.Completed {
			t.Errorf("tool %q is still running after tool_result was processed", tool.Name)
		}
	}

	// After snapshot round-trip, no tool should be running either.
	snap, _ := es.MarshalSnapshot()
	es2 := NewExtractionState()
	_ = es2.UnmarshalSnapshot(snap)
	data2 := es2.ToTranscriptData()
	for _, tool := range data2.Tools {
		if !tool.Completed {
			t.Errorf("tool %q is stuck as running after snapshot restore", tool.Name)
		}
	}
	// Completed tools must not be in the toolMap (they have no pending result).
	if len(es2.toolMap) != 0 {
		t.Errorf("expected empty toolMap after restoring all-completed tools, got %d entries", len(es2.toolMap))
	}
}

// TestSnapshotSpec5_NoAgentsStuckRunning verifies that agents resolved in the same
// invocation are not stuck as running after a snapshot restore.
func TestSnapshotSpec5_NoAgentsStuckRunning(t *testing.T) {
	es := NewExtractionState()
	t0 := time.Date(2024, 9, 2, 0, 0, 0, 0, time.UTC)

	agentIDs := []string{"a-1", "a-2", "a-3"}
	for i, id := range agentIDs {
		es.ProcessEntry(makeToolUseEntryAt(id, "Task",
			map[string]interface{}{"subagent_type": fmt.Sprintf("worker-%d", i)},
			t0.Add(time.Duration(i)*100*time.Millisecond),
		))
	}
	for i, id := range agentIDs {
		es.ProcessEntry(makeToolResultEntryAt(id, false,
			t0.Add(time.Duration(i)*100*time.Millisecond+2*time.Second),
		))
	}

	snap, _ := es.MarshalSnapshot()
	es2 := NewExtractionState()
	_ = es2.UnmarshalSnapshot(snap)

	data := es2.ToTranscriptData()
	for i, a := range data.Agents {
		if a.Status != "completed" {
			t.Errorf("agent[%d] (%s): stuck as %q after restore; expected completed", i, a.Name, a.Status)
		}
	}
	// Completed agents must not be in agentMap.
	if len(es2.agentMap) != 0 {
		t.Errorf("expected empty agentMap after restoring all-completed agents, got %d entries", len(es2.agentMap))
	}
}

// TestSnapshotSpec5_MixedRunningAndCompletedAfterRestore verifies that after a restore,
// completed tools are absent from toolMap while running tools remain present.
func TestSnapshotSpec5_MixedRunningAndCompletedAfterRestore(t *testing.T) {
	t0 := time.Date(2024, 9, 3, 0, 0, 0, 0, time.UTC)

	es := NewExtractionState()
	// Tool 1: completed.
	es.ProcessEntry(makeToolUseEntryAt("done-tool", "Read",
		map[string]interface{}{"file_path": "done.go"}, t0))
	es.ProcessEntry(makeToolResultEntryAt("done-tool", false, t0.Add(time.Second)))
	// Tool 2: still running (no result).
	es.ProcessEntry(makeToolUseEntryAt("running-tool", "Bash",
		map[string]interface{}{"command": "long-op"}, t0.Add(2*time.Second)))

	snap, _ := es.MarshalSnapshot()
	es2 := NewExtractionState()
	_ = es2.UnmarshalSnapshot(snap)

	data := es2.ToTranscriptData()
	if len(data.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(data.Tools))
	}

	// done-tool must be completed and absent from toolMap.
	if !data.Tools[0].Completed {
		t.Errorf("done-tool: expected Completed=true, got false")
	}
	if _, ok := es2.toolMap["done-tool"]; ok {
		t.Error("done-tool should not be in toolMap after restore")
	}

	// running-tool must be running and present in toolMap for future result matching.
	if data.Tools[1].Completed {
		t.Errorf("running-tool: expected Completed=false (running), got true")
	}
	if _, ok := es2.toolMap["running-tool"]; !ok {
		t.Error("running-tool should be in toolMap after restore for future result matching")
	}
}

// ---- Snapshot: spec 6 — duration computation accurate within 100ms across invocation boundaries ----

// TestSnapshotSpec6_DurationAccuracyAcrossInvocations verifies that a tool whose
// tool_use and tool_result span invocation boundaries has its DurationMs computed
// accurately from the persisted StartTime. The tolerance is 100ms per spec.
func TestSnapshotSpec6_DurationAccuracyAcrossInvocations(t *testing.T) {
	// Exact known timestamps: tool starts at t0, result arrives at t0 + 7300ms.
	t0 := time.Date(2024, 10, 1, 8, 0, 0, 0, time.UTC)
	expectedDurationMs := 7300
	t1 := t0.Add(time.Duration(expectedDurationMs) * time.Millisecond)

	// Invocation 1: tool_use only.
	es1 := NewExtractionState()
	es1.ProcessEntry(makeToolUseEntryAt("dur-tool-1", "Bash",
		map[string]interface{}{"command": "expensive-build.sh"}, t0))

	snap1, err := es1.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	// Invocation 2: restore snapshot, then receive tool_result.
	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap1); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}
	es2.ProcessEntry(makeToolResultEntryAt("dur-tool-1", false, t1))

	data := es2.ToTranscriptData()
	if len(data.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(data.Tools))
	}
	tool := data.Tools[0]
	if !tool.Completed {
		t.Errorf("expected Completed=true, got false")
	}

	// Duration must be within 100ms of the expected 7300ms.
	diff := tool.DurationMs - expectedDurationMs
	if diff < 0 {
		diff = -diff
	}
	if diff > 100 {
		t.Errorf("DurationMs=%d, expected ~%d (diff %d > 100ms tolerance)",
			tool.DurationMs, expectedDurationMs, diff)
	}
}

// TestSnapshotSpec6_AgentDurationAccuracyAcrossInvocations verifies the same
// cross-invocation duration accuracy for agents.
func TestSnapshotSpec6_AgentDurationAccuracyAcrossInvocations(t *testing.T) {
	t0 := time.Date(2024, 10, 2, 9, 0, 0, 0, time.UTC)
	expectedDurationMs := 15000 // 15 seconds
	t1 := t0.Add(time.Duration(expectedDurationMs) * time.Millisecond)

	es1 := NewExtractionState()
	es1.ProcessEntry(makeToolUseEntryAt("dur-agent-1", "Task",
		map[string]interface{}{"subagent_type": "coding", "model": "claude-sonnet"},
		t0,
	))

	snap1, _ := es1.MarshalSnapshot()

	es2 := NewExtractionState()
	_ = es2.UnmarshalSnapshot(snap1)
	es2.ProcessEntry(makeToolResultEntryAt("dur-agent-1", false, t1))

	data := es2.ToTranscriptData()
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(data.Agents))
	}
	agent := data.Agents[0]
	if agent.Status != "completed" {
		t.Errorf("expected Status=completed, got %q", agent.Status)
	}

	diff := agent.DurationMs - expectedDurationMs
	if diff < 0 {
		diff = -diff
	}
	if diff > 100 {
		t.Errorf("agent DurationMs=%d, expected ~%d (diff %d > 100ms tolerance)",
			agent.DurationMs, expectedDurationMs, diff)
	}
}

// TestSnapshotSpec6_StartTimePreservedWithNanosecondPrecision verifies that the
// RFC3339Nano serialization preserves StartTime with sufficient precision for the
// 100ms tolerance requirement. Nanosecond timestamps must survive a JSON round-trip.
func TestSnapshotSpec6_StartTimePreservedWithNanosecondPrecision(t *testing.T) {
	// Choose a time with sub-millisecond precision to confirm no truncation occurs.
	t0 := time.Date(2024, 10, 3, 10, 30, 45, 123456789, time.UTC)

	es := NewExtractionState()
	es.ProcessEntry(makeToolUseEntryAt("precision-1", "Read",
		map[string]interface{}{"file_path": "precise.go"}, t0))

	snap, _ := es.MarshalSnapshot()
	es2 := NewExtractionState()
	_ = es2.UnmarshalSnapshot(snap)

	// The restored tool should be in toolMap (still running, no result).
	restored, ok := es2.toolMap["precision-1"]
	if !ok {
		t.Fatal("running tool not in toolMap after restore")
	}

	// StartTime must be restored with at least millisecond precision.
	delta := restored.startTime.Sub(t0)
	if delta < 0 {
		delta = -delta
	}
	if delta > time.Millisecond {
		t.Errorf("StartTime lost precision: original=%v, restored=%v, delta=%v",
			t0, restored.startTime, delta)
	}
}

// ---- Spinner frame counter specs --------------------------------------------

// Spec: IncrementSpinnerFrame advances the counter by 1 each call.
func TestIncrementSpinnerFrame_AdvancesCounter(t *testing.T) {
	es := NewExtractionState()

	if es.spinnerFrame != 0 {
		t.Fatalf("expected initial spinnerFrame=0, got %d", es.spinnerFrame)
	}

	es.IncrementSpinnerFrame()
	if es.spinnerFrame != 1 {
		t.Errorf("after 1 increment: expected spinnerFrame=1, got %d", es.spinnerFrame)
	}

	es.IncrementSpinnerFrame()
	if es.spinnerFrame != 2 {
		t.Errorf("after 2 increments: expected spinnerFrame=2, got %d", es.spinnerFrame)
	}
}

// Spec: ToTranscriptData includes SpinnerFrame so widgets can read it.
func TestToTranscriptData_IncludesSpinnerFrame(t *testing.T) {
	es := NewExtractionState()
	es.IncrementSpinnerFrame()
	es.IncrementSpinnerFrame()
	es.IncrementSpinnerFrame()

	td := es.ToTranscriptData()
	if td.SpinnerFrame != 3 {
		t.Errorf("expected SpinnerFrame=3 in TranscriptData, got %d", td.SpinnerFrame)
	}
}

// Spec: SpinnerFrame is persisted in and restored from the snapshot so the
// counter advances on every successive invocation, not just within one process.
func TestSpinnerFrame_PersistedInSnapshot(t *testing.T) {
	es := NewExtractionState()
	es.IncrementSpinnerFrame()
	es.IncrementSpinnerFrame()
	es.IncrementSpinnerFrame() // spinnerFrame == 3

	snap, err := es.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	// Simulate next invocation: fresh state, restore snapshot.
	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}

	// Frame must be restored to 3 before the next increment.
	if es2.spinnerFrame != 3 {
		t.Errorf("expected spinnerFrame=3 after restore, got %d", es2.spinnerFrame)
	}

	// Simulating the gather-stage increment: must advance to 4.
	es2.IncrementSpinnerFrame()
	if es2.spinnerFrame != 4 {
		t.Errorf("expected spinnerFrame=4 after post-restore increment, got %d", es2.spinnerFrame)
	}
}

// Spec: Successive simulated invocations always produce different spinner frames
// independent of wall-clock time.
func TestSpinnerFrame_SuccessiveInvocationsProduceDifferentFrames(t *testing.T) {
	// Simulate 12 successive invocations and collect their spinner frames.
	var frames []int
	currentFrame := 0

	for i := 0; i < 12; i++ {
		// Simulate: restore snapshot (currentFrame), increment, record.
		es := NewExtractionState()
		es.spinnerFrame = currentFrame
		es.IncrementSpinnerFrame()
		currentFrame = es.spinnerFrame
		frames = append(frames, es.ToTranscriptData().SpinnerFrame)
	}

	// Every consecutive pair must differ.
	for i := 1; i < len(frames); i++ {
		if frames[i] == frames[i-1] {
			t.Errorf("invocation %d and %d produced the same frame %d — spinner did not advance",
				i-1, i, frames[i])
		}
	}
}

// Spec: SpinnerFrame does not depend on wall-clock time — two invocations at the
// same millisecond still produce different frames.
func TestSpinnerFrame_NoWallClockDependency(t *testing.T) {
	// Freeze the concept of "same millisecond" by calling the counter twice
	// back-to-back with no sleep. If the counter were time-based, they might
	// produce the same frame; with a counter they must differ.
	es1 := NewExtractionState()
	es1.IncrementSpinnerFrame()
	frame1 := es1.spinnerFrame

	es2 := NewExtractionState()
	es2.spinnerFrame = frame1
	es2.IncrementSpinnerFrame()
	frame2 := es2.spinnerFrame

	if frame1 == frame2 {
		t.Errorf("back-to-back counter increments produced same frame %d — counter not advancing", frame1)
	}
}

// TestDividerOffset_SnapshotRoundTrip verifies that the divider offset is a
// monotonic counter that increments once per tool_use and survives snapshot
// round-trips. The widget uses offset % numSeparators to highlight one
// separator in a wrapping ticker pattern.
func TestDividerOffset_SnapshotRoundTrip(t *testing.T) {
	// Invocation 1: 2 tool_use events → offset=2.
	es1 := NewExtractionState()
	es1.ProcessEntry(makeToolUseEntry("id-1", "Read", map[string]interface{}{"file_path": "a.go"}))
	es1.ProcessEntry(makeToolResultEntry("id-1", false))
	es1.ProcessEntry(makeToolUseEntry("id-2", "Write", map[string]interface{}{"file_path": "b.go"}))
	es1.ProcessEntry(makeToolResultEntry("id-2", false))

	data1 := es1.ToTranscriptData()
	if data1.DividerOffset != 2 {
		t.Errorf("inv1 DividerOffset = %d, want 2 (2 tool_use events)", data1.DividerOffset)
	}

	snap1, err := es1.MarshalSnapshot()
	if err != nil {
		t.Fatalf("inv1 MarshalSnapshot: %v", err)
	}

	// Invocation 2: restore, add 1 more tool → offset=3.
	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap1); err != nil {
		t.Fatalf("inv2 UnmarshalSnapshot: %v", err)
	}
	es2.ProcessEntry(makeToolUseEntry("id-3", "Bash", map[string]interface{}{"command": "ls"}))

	data2 := es2.ToTranscriptData()
	if data2.DividerOffset != 3 {
		t.Errorf("inv2 DividerOffset = %d, want 3 (restored 2 + 1 new)", data2.DividerOffset)
	}

	// Multiple saves must produce identical offsets.
	snap2a, err := es2.MarshalSnapshot()
	if err != nil {
		t.Fatalf("inv2 MarshalSnapshot (save 1): %v", err)
	}
	snap2b, err := es2.MarshalSnapshot()
	if err != nil {
		t.Fatalf("inv2 MarshalSnapshot (save 2): %v", err)
	}

	for _, snap := range [][]byte{snap2a, snap2b} {
		es3 := NewExtractionState()
		if err := es3.UnmarshalSnapshot(snap); err != nil {
			t.Fatalf("inv3 UnmarshalSnapshot: %v", err)
		}

		// No new tools: offset stays at 3.
		data3 := es3.ToTranscriptData()
		if data3.DividerOffset != 3 {
			t.Errorf("inv3 DividerOffset = %d, want 3 (no new tools, offset stable)", data3.DividerOffset)
		}

		snap3, err := es3.MarshalSnapshot()
		if err != nil {
			t.Fatalf("inv3 MarshalSnapshot: %v", err)
		}

		// Another no-tool invocation: still 3.
		es4 := NewExtractionState()
		if err := es4.UnmarshalSnapshot(snap3); err != nil {
			t.Fatalf("inv4 UnmarshalSnapshot: %v", err)
		}
		data4 := es4.ToTranscriptData()
		if data4.DividerOffset != 3 {
			t.Errorf("inv4 DividerOffset = %d, want 3 (still stable)", data4.DividerOffset)
		}

		snap4, err := es4.MarshalSnapshot()
		if err != nil {
			t.Fatalf("inv4 MarshalSnapshot: %v", err)
		}

		// Invocation 5: 2 new tools → offset=5.
		es5 := NewExtractionState()
		if err := es5.UnmarshalSnapshot(snap4); err != nil {
			t.Fatalf("inv5 UnmarshalSnapshot: %v", err)
		}
		es5.ProcessEntry(makeToolUseEntry("id-4", "Grep", map[string]interface{}{"pattern": "foo"}))
		es5.ProcessEntry(makeToolUseEntry("id-5", "Glob", map[string]interface{}{"pattern": "*.go"}))

		data5 := es5.ToTranscriptData()
		if data5.DividerOffset != 5 {
			t.Errorf("inv5 DividerOffset = %d, want 5 (restored 3 + 2 new)", data5.DividerOffset)
		}
	}
}

// TestDividerOffset_StartsAtZero verifies that DividerOffset begins at 0
// when no prior snapshot exists and increments with each tool_use.
func TestDividerOffset_StartsAtZero(t *testing.T) {
	es := NewExtractionState()
	data0 := es.ToTranscriptData()
	if data0.DividerOffset != 0 {
		t.Errorf("DividerOffset = %d, want 0 (no tools yet)", data0.DividerOffset)
	}

	es.ProcessEntry(makeToolUseEntry("id-1", "Read", map[string]interface{}{"file_path": "a.go"}))
	es.ProcessEntry(makeToolResultEntry("id-1", false))

	data1 := es.ToTranscriptData()
	if data1.DividerOffset != 1 {
		t.Errorf("DividerOffset = %d, want 1 (1 tool_use)", data1.DividerOffset)
	}
}

// ---- Sidechain filtering ---------------------------------------------------

// TestProcessEntry_SidechainEntriesSkipped verifies that entries with
// IsSidechain=true are ignored entirely. Sidechain entries come from agent
// subprocesses and must not contribute tools, agents, or thinking state to
// the main session display.
func TestProcessEntry_SidechainEntriesSkipped(t *testing.T) {
	es := NewExtractionState()
	e := makeToolUseEntry("sidechain-1", "Read", map[string]interface{}{"file_path": "x.go"})
	e.IsSidechain = true
	es.ProcessEntry(e)

	data := es.ToTranscriptData()
	if len(data.Tools) != 0 {
		t.Errorf("expected 0 tools from sidechain entry, got %d", len(data.Tools))
	}
}

// TestProcessEntry_SidechainAgentEntriesSkipped verifies that a sidechain
// Task/Agent tool_use is not added to the agents display list.
func TestProcessEntry_SidechainAgentEntriesSkipped(t *testing.T) {
	es := NewExtractionState()
	e := makeToolUseEntry("sidechain-agent-1", "Task", map[string]interface{}{
		"description": "sub-agent work",
	})
	e.IsSidechain = true
	es.ProcessEntry(e)

	data := es.ToTranscriptData()
	if len(data.Agents) != 0 {
		t.Errorf("expected 0 agents from sidechain entry, got %d", len(data.Agents))
	}
}

// TestProcessEntry_NonSidechainStillProcessed verifies that a normal (non-sidechain)
// entry continues to be processed after the sidechain filter is in place.
func TestProcessEntry_NonSidechainStillProcessed(t *testing.T) {
	es := NewExtractionState()

	sidechain := makeToolUseEntry("sidechain-1", "Read", map[string]interface{}{"file_path": "sidechain.go"})
	sidechain.IsSidechain = true
	es.ProcessEntry(sidechain)

	main := makeToolUseEntry("main-1", "Write", map[string]interface{}{"file_path": "main.go"})
	es.ProcessEntry(main)

	data := es.ToTranscriptData()
	if len(data.Tools) != 1 {
		t.Fatalf("expected 1 tool (non-sidechain only), got %d", len(data.Tools))
	}
	if data.Tools[0].Name != "Write" {
		t.Errorf("expected tool name Write, got %q", data.Tools[0].Name)
	}
}
