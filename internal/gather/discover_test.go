package gather

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// writeSubagentFile creates a minimal subagent JSONL file with the given
// message content and a controllable timestamp. Returns the path to the created file.
func writeSubagentFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	return writeSubagentFileAt(t, dir, name, content, time.Now())
}

// writeSubagentFileAt creates a minimal subagent JSONL file with the given
// message content and an explicit timestamp in the JSON payload.
func writeSubagentFileAt(t *testing.T, dir, name, content string, ts time.Time) string {
	t.Helper()
	entry := map[string]interface{}{
		"type":        "user",
		"uuid":        "test-uuid",
		"timestamp":   ts.UTC().Format(time.RFC3339),
		"isSidechain": true,
		"agentId":     name,
		"message": map[string]interface{}{
			"role":    "user",
			"content": content,
		},
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal subagent entry: %v", err)
	}
	data = append(data, '\n')

	path := filepath.Join(dir, "agent-"+name+".jsonl")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write subagent file: %v", err)
	}
	return path
}

// setupSubagentsDir creates a session directory with a subagents/ subdirectory
// and returns the transcript path and subagents directory path.
func setupSubagentsDir(t *testing.T) (transcriptPath, subagentsDir string) {
	t.Helper()
	tmp := t.TempDir()
	sessionID := "test-session-id"
	transcriptPath = filepath.Join(tmp, sessionID+".jsonl")
	// Create the transcript file so it exists.
	if err := os.WriteFile(transcriptPath, []byte{}, 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	subagentsDir = filepath.Join(tmp, sessionID, "subagents")
	if err := os.MkdirAll(subagentsDir, 0o755); err != nil {
		t.Fatalf("mkdir subagents: %v", err)
	}
	return transcriptPath, subagentsDir
}

func TestDiscoverSubagents_NoSubagentsDir(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "no-session.jsonl")

	agents := discoverSubagents(path)
	if len(agents) != 0 {
		t.Errorf("expected 0 agents for missing dir, got %d", len(agents))
	}
}

func TestDiscoverSubagents_FiltersWarmupAgents(t *testing.T) {
	transcriptPath, subagentsDir := setupSubagentsDir(t)

	writeSubagentFile(t, subagentsDir, "a1b2c3d", "Warmup")
	writeSubagentFile(t, subagentsDir, "e4f5g6h", "Implement the feature")

	agents := discoverSubagents(transcriptPath)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent (warmup filtered), got %d", len(agents))
	}
	if agents[0].Name != "e4f5g6h" {
		t.Errorf("expected agent name 'e4f5g6h', got %q", agents[0].Name)
	}
}

func TestDiscoverSubagents_FiltersCompactAgents(t *testing.T) {
	transcriptPath, subagentsDir := setupSubagentsDir(t)

	writeSubagentFile(t, subagentsDir, "acompact123", "some content")
	writeSubagentFile(t, subagentsDir, "abc1234", "real task")

	agents := discoverSubagents(transcriptPath)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent (compact filtered), got %d", len(agents))
	}
	if agents[0].Name != "abc1234" {
		t.Errorf("expected agent name 'abc1234', got %q", agents[0].Name)
	}
}

func TestDiscoverSubagents_FiltersEmptyFiles(t *testing.T) {
	transcriptPath, subagentsDir := setupSubagentsDir(t)

	// Create an empty file.
	emptyPath := filepath.Join(subagentsDir, "agent-empty1.jsonl")
	if err := os.WriteFile(emptyPath, []byte{}, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	writeSubagentFile(t, subagentsDir, "real1", "do the thing")

	agents := discoverSubagents(transcriptPath)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent (empty filtered), got %d", len(agents))
	}
}

func TestDiscoverSubagents_RunningVsCompleted(t *testing.T) {
	transcriptPath, subagentsDir := setupSubagentsDir(t)

	// Write a "recently modified" agent.
	writeSubagentFile(t, subagentsDir, "running1", "active task")

	// Write an "old" agent by backdating its modtime.
	oldPath := writeSubagentFile(t, subagentsDir, "done1", "finished task")
	oldTime := time.Now().Add(-5 * time.Minute)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	agents := discoverSubagents(transcriptPath)
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	// Find agents by name.
	byName := make(map[string]model.AgentEntry, len(agents))
	for _, a := range agents {
		byName[a.Name] = a
	}

	running, ok := byName["running1"]
	if !ok {
		t.Fatal("missing agent 'running1'")
	}
	if running.Status != "running" {
		t.Errorf("expected running1 status 'running', got %q", running.Status)
	}

	done, ok := byName["done1"]
	if !ok {
		t.Fatal("missing agent 'done1'")
	}
	if done.Status != "completed" {
		t.Errorf("expected done1 status 'completed', got %q", done.Status)
	}
}

func TestDiscoverSubagents_IgnoresNonAgentFiles(t *testing.T) {
	transcriptPath, subagentsDir := setupSubagentsDir(t)

	// Non-agent files should be ignored.
	if err := os.WriteFile(filepath.Join(subagentsDir, "other.jsonl"), []byte("data\n"), 0o644); err != nil {
		t.Fatalf("write other file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subagentsDir, "agent-abc.txt"), []byte("data\n"), 0o644); err != nil {
		t.Fatalf("write txt file: %v", err)
	}

	writeSubagentFile(t, subagentsDir, "real1", "task content")

	agents := discoverSubagents(transcriptPath)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
}

func TestDiscoverSubagents_ColorIndexWraps(t *testing.T) {
	transcriptPath, subagentsDir := setupSubagentsDir(t)

	// Create 10 agents to verify color index wraps at 8.
	for i := 0; i < 10; i++ {
		name := "agent" + string(rune('a'+i))
		writeSubagentFile(t, subagentsDir, name, "task")
	}

	agents := discoverSubagents(transcriptPath)
	if len(agents) != 10 {
		t.Fatalf("expected 10 agents, got %d", len(agents))
	}

	for i, a := range agents {
		expected := i % 8
		if a.ColorIndex != expected {
			t.Errorf("agent %d: expected ColorIndex %d, got %d", i, expected, a.ColorIndex)
		}
	}
}

func TestParseFirstEntry_ValidTimestamp(t *testing.T) {
	dir := t.TempDir()
	wantTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	path := writeSubagentFileAt(t, dir, "abc123", "Implement the feature", wantTime)

	result := parseFirstEntry(path)

	if result.isWarmup {
		t.Error("expected isWarmup=false for non-warmup content")
	}
	if result.timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
	if !result.timestamp.Equal(wantTime) {
		t.Errorf("timestamp: got %v, want %v", result.timestamp, wantTime)
	}
}

func TestParseFirstEntry_WarmupAgent(t *testing.T) {
	dir := t.TempDir()
	path := writeSubagentFile(t, dir, "warmup", "Warmup")

	result := parseFirstEntry(path)

	if !result.isWarmup {
		t.Error("expected isWarmup=true for 'Warmup' content")
	}
}

func TestParseFirstEntry_MissingFile(t *testing.T) {
	result := parseFirstEntry("/nonexistent/path.jsonl")

	if result.isWarmup {
		t.Error("expected isWarmup=false for missing file")
	}
	if !result.timestamp.IsZero() {
		t.Error("expected zero timestamp for missing file")
	}
}

func TestDiscoverSubagents_ComputesDuration(t *testing.T) {
	transcriptPath, subagentsDir := setupSubagentsDir(t)

	// Use a start time well in the past so the agent is classified as "completed".
	// The modtime is set to startTime + 10s, and both are >30s ago so the
	// subagentStaleThreshold check classifies the agent as completed.
	startTime := time.Now().Add(-60 * time.Second).Truncate(time.Second)
	agentPath := writeSubagentFileAt(t, subagentsDir, "abc123def", "do work", startTime)

	// Set the modtime to 10s after the first-entry timestamp.
	modTime := startTime.Add(10 * time.Second)
	if err := os.Chtimes(agentPath, modTime, modTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	agents := discoverSubagents(transcriptPath)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	a := agents[0]
	if a.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", a.Status)
	}
	// Allow ±200ms tolerance for filesystem timestamp precision.
	if a.DurationMs < 9800 || a.DurationMs > 10200 {
		t.Errorf("expected DurationMs ≈ 10000, got %d", a.DurationMs)
	}
	if a.ID != "abc123def" {
		t.Errorf("expected ID 'abc123def', got %q", a.ID)
	}
}

// TestMergeSubagents_EnrichesFromTranscript verifies that a filesystem agent
// matched by Name overwrites timing fields while preserving transcript metadata.
func TestMergeSubagents_EnrichesFromTranscript(t *testing.T) {
	start := time.Now().Add(-10 * time.Second)
	td := &model.TranscriptData{
		Agents: []model.AgentEntry{
			{Name: "worker", Status: "completed", Model: "claude-haiku-4-5", Description: "do the thing", DurationMs: 5},
		},
	}
	fsAgents := []model.AgentEntry{
		{ID: "abc123", Name: "worker", Status: "completed", StartTime: start, DurationMs: 10000},
	}

	mergeSubagents(td, fsAgents)

	if len(td.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(td.Agents))
	}
	a := td.Agents[0]
	// Filesystem timing is authoritative.
	if a.DurationMs != 10000 {
		t.Errorf("expected DurationMs 10000 from filesystem, got %d", a.DurationMs)
	}
	if !a.StartTime.Equal(start) {
		t.Errorf("expected StartTime from filesystem, got %v", a.StartTime)
	}
	if a.ID != "abc123" {
		t.Errorf("expected ID 'abc123' from filesystem, got %q", a.ID)
	}
	// Transcript metadata is preserved.
	if a.Model != "claude-haiku-4-5" {
		t.Errorf("expected Model 'claude-haiku-4-5' from transcript, got %q", a.Model)
	}
	if a.Description != "do the thing" {
		t.Errorf("expected Description 'do the thing' from transcript, got %q", a.Description)
	}
}

// TestMergeSubagents_PreservesTranscriptOnlyAgents verifies that transcript
// agents with no filesystem match are not removed from td.Agents.
func TestMergeSubagents_PreservesTranscriptOnlyAgents(t *testing.T) {
	td := &model.TranscriptData{
		Agents: []model.AgentEntry{
			{Name: "transcript-only", Status: "completed", Model: "claude-3-5"},
			{Name: "matched", Status: "completed"},
		},
	}
	fsAgents := []model.AgentEntry{
		{ID: "fs1", Name: "matched", Status: "completed", DurationMs: 5000},
	}

	mergeSubagents(td, fsAgents)

	if len(td.Agents) != 2 {
		t.Fatalf("expected 2 agents (transcript-only preserved), got %d", len(td.Agents))
	}
	// First agent unchanged.
	if td.Agents[0].Name != "transcript-only" {
		t.Errorf("expected first agent 'transcript-only', got %q", td.Agents[0].Name)
	}
	if td.Agents[0].Model != "claude-3-5" {
		t.Errorf("expected Model preserved on transcript-only agent, got %q", td.Agents[0].Model)
	}
}

// TestMergeSubagents_AppendsUnmatchedFsAgents verifies that a filesystem agent
// with no transcript match is appended to td.Agents.
func TestMergeSubagents_AppendsUnmatchedFsAgents(t *testing.T) {
	td := &model.TranscriptData{
		Agents: []model.AgentEntry{
			{Name: "existing", Status: "running"},
		},
	}
	fsAgents := []model.AgentEntry{
		{ID: "new1", Name: "brand-new", Status: "completed", DurationMs: 3000},
	}

	mergeSubagents(td, fsAgents)

	if len(td.Agents) != 2 {
		t.Fatalf("expected 2 agents after append, got %d", len(td.Agents))
	}
	appended := td.Agents[1]
	if appended.Name != "brand-new" {
		t.Errorf("expected appended agent 'brand-new', got %q", appended.Name)
	}
	if appended.ID != "new1" {
		t.Errorf("expected appended ID 'new1', got %q", appended.ID)
	}
}

// TestMergeSubagents_SameNameMatchesFirstUnmatched verifies that when multiple
// transcript agents share a Name, each filesystem agent matches a distinct one.
func TestMergeSubagents_SameNameMatchesFirstUnmatched(t *testing.T) {
	td := &model.TranscriptData{
		Agents: []model.AgentEntry{
			{Name: "rb-worker", Status: "completed", Model: "haiku", DurationMs: 1},
			{Name: "rb-worker", Status: "completed", Model: "sonnet", DurationMs: 2},
		},
	}
	fsAgents := []model.AgentEntry{
		{ID: "fs-a", Name: "rb-worker", Status: "completed", DurationMs: 1000},
		{ID: "fs-b", Name: "rb-worker", Status: "completed", DurationMs: 2000},
	}

	mergeSubagents(td, fsAgents)

	// Both transcript agents updated; no duplicates appended.
	if len(td.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(td.Agents))
	}
	// First agent matched fs-a.
	if td.Agents[0].ID != "fs-a" {
		t.Errorf("agent[0]: expected ID 'fs-a', got %q", td.Agents[0].ID)
	}
	if td.Agents[0].DurationMs != 1000 {
		t.Errorf("agent[0]: expected DurationMs 1000, got %d", td.Agents[0].DurationMs)
	}
	if td.Agents[0].Model != "haiku" {
		t.Errorf("agent[0]: expected Model 'haiku' preserved, got %q", td.Agents[0].Model)
	}
	// Second agent matched fs-b.
	if td.Agents[1].ID != "fs-b" {
		t.Errorf("agent[1]: expected ID 'fs-b', got %q", td.Agents[1].ID)
	}
	if td.Agents[1].DurationMs != 2000 {
		t.Errorf("agent[1]: expected DurationMs 2000, got %d", td.Agents[1].DurationMs)
	}
	if td.Agents[1].Model != "sonnet" {
		t.Errorf("agent[1]: expected Model 'sonnet' preserved, got %q", td.Agents[1].Model)
	}
}

// TestMergeSubagents_MatchesByDescription verifies that a filesystem agent whose
// Name is the description string merges into a transcript agent that has a
// different Name (subagent_type) but matching Description. This is the case
// when e.g. the transcript names an agent "claude-code-guide" but the FS
// names it "Research Claude Code SDK/headless".
func TestMergeSubagents_MatchesByDescription(t *testing.T) {
	td := &model.TranscriptData{
		Agents: []model.AgentEntry{
			{
				Name:        "claude-code-guide",
				Description: "Research Claude Code SDK/headless",
				Status:      "running",
				Model:       "sonnet",
			},
		},
	}
	fsAgents := []model.AgentEntry{
		{
			ID:         "addcbaae399bfe4b8",
			Name:       "Research Claude Code SDK/headless",
			Status:     "completed",
			DurationMs: 5000,
		},
	}

	mergeSubagents(td, fsAgents)

	if len(td.Agents) != 1 {
		t.Fatalf("expected 1 agent (merged), got %d", len(td.Agents))
	}
	a := td.Agents[0]
	if a.Name != "claude-code-guide" {
		t.Errorf("expected Name preserved as 'claude-code-guide', got %q", a.Name)
	}
	if a.Model != "sonnet" {
		t.Errorf("expected Model 'sonnet' preserved, got %q", a.Model)
	}
	if a.Status != "completed" {
		t.Errorf("expected Status 'completed' from FS, got %q", a.Status)
	}
	if a.DurationMs != 5000 {
		t.Errorf("expected DurationMs 5000 from FS, got %d", a.DurationMs)
	}
	if a.ID != "addcbaae399bfe4b8" {
		t.Errorf("expected ID from FS, got %q", a.ID)
	}
}

func TestMergeSubagents_Empty(t *testing.T) {
	td := &model.TranscriptData{
		Agents: []model.AgentEntry{
			{Name: "existing", Status: "running"},
		},
	}

	mergeSubagents(td, nil)

	if len(td.Agents) != 1 {
		t.Fatalf("expected 1 agent unchanged, got %d", len(td.Agents))
	}
}

// writeMetaJSON creates a .meta.json sidecar file in dir with the given fields.
func writeMetaJSON(t *testing.T, dir, agentID, agentType, description string) {
	t.Helper()
	meta := map[string]interface{}{}
	if agentType != "" {
		meta["agentType"] = agentType
	}
	if description != "" {
		meta["description"] = description
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	path := filepath.Join(dir, "agent-"+agentID+".meta.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write meta: %v", err)
	}
}

// TestDiscoverSubagents_PrefersDescription verifies that when a .meta.json has
// a description field, the agent Name is set to that description.
func TestDiscoverSubagents_PrefersDescription(t *testing.T) {
	transcriptPath, subagentsDir := setupSubagentsDir(t)

	agentID := "abc1234567"
	writeSubagentFile(t, subagentsDir, agentID, "do work")
	writeMetaJSON(t, subagentsDir, agentID, "rb-worker", "Add regression tests")

	agents := discoverSubagents(transcriptPath)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "Add regression tests" {
		t.Errorf("expected Name 'Add regression tests' (from description), got %q", agents[0].Name)
	}
}

// TestDiscoverSubagents_FallsBackToAgentType verifies that when a .meta.json
// has agentType but no description, the agent Name is set to agentType.
func TestDiscoverSubagents_FallsBackToAgentType(t *testing.T) {
	transcriptPath, subagentsDir := setupSubagentsDir(t)

	agentID := "def9876543"
	writeSubagentFile(t, subagentsDir, agentID, "explore task")
	writeMetaJSON(t, subagentsDir, agentID, "Explore", "")

	agents := discoverSubagents(transcriptPath)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "Explore" {
		t.Errorf("expected Name 'Explore' (from agentType), got %q", agents[0].Name)
	}
}

// TestDiscoverSubagents_FallsBackToUUID verifies that when the .meta.json is
// missing entirely, the agent Name falls back to the raw UUID.
func TestDiscoverSubagents_FallsBackToUUID(t *testing.T) {
	transcriptPath, subagentsDir := setupSubagentsDir(t)

	agentID := "deadbeef01"
	writeSubagentFile(t, subagentsDir, agentID, "some task")
	// No .meta.json written.

	agents := discoverSubagents(transcriptPath)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != agentID {
		t.Errorf("expected Name %q (UUID fallback), got %q", agentID, agents[0].Name)
	}
}

// TestReadAgentMeta_BothFields verifies that readAgentMeta returns both fields
// when both are present in the file.
func TestReadAgentMeta_BothFields(t *testing.T) {
	dir := t.TempDir()
	writeMetaJSON(t, dir, "test-id", "rb-worker", "Build the feature")
	path := filepath.Join(dir, "agent-test-id.meta.json")

	meta := readAgentMeta(path)
	if meta.agentType != "rb-worker" {
		t.Errorf("expected agentType 'rb-worker', got %q", meta.agentType)
	}
	if meta.description != "Build the feature" {
		t.Errorf("expected description 'Build the feature', got %q", meta.description)
	}
}

// TestReadAgentMeta_MissingFile verifies that readAgentMeta returns zero value
// when the file does not exist.
func TestReadAgentMeta_MissingFile(t *testing.T) {
	meta := readAgentMeta("/nonexistent/agent-xyz.meta.json")
	if meta.agentType != "" || meta.description != "" {
		t.Errorf("expected zero agentMeta for missing file, got %+v", meta)
	}
}

// TestReadAgentMeta_AgentTypeOnly verifies description is empty when absent.
func TestReadAgentMeta_AgentTypeOnly(t *testing.T) {
	dir := t.TempDir()
	writeMetaJSON(t, dir, "test-id", "Plan", "")
	path := filepath.Join(dir, "agent-test-id.meta.json")

	meta := readAgentMeta(path)
	if meta.agentType != "Plan" {
		t.Errorf("expected agentType 'Plan', got %q", meta.agentType)
	}
	if meta.description != "" {
		t.Errorf("expected empty description, got %q", meta.description)
	}
}

func TestParseFirstEntry_IsWarmup_True(t *testing.T) {
	dir := t.TempDir()
	path := writeSubagentFile(t, dir, "warmup", "Warmup")
	result := parseFirstEntry(path)
	if !result.isWarmup {
		t.Error("expected isWarmup=true for 'Warmup' content")
	}
}

func TestParseFirstEntry_IsWarmup_False(t *testing.T) {
	dir := t.TempDir()
	path := writeSubagentFile(t, dir, "real", "Implement feature X")
	result := parseFirstEntry(path)
	if result.isWarmup {
		t.Error("expected isWarmup=false for non-warmup content")
	}
}

func TestParseFirstEntry_IsWarmup_MissingFile(t *testing.T) {
	result := parseFirstEntry("/nonexistent/path.jsonl")
	if result.isWarmup {
		t.Error("expected isWarmup=false for missing file")
	}
}

func BenchmarkDiscoverSubagents(b *testing.B) {
	b.ReportAllocs()

	tmp := b.TempDir()
	sessionID := "bench-session"
	transcriptPath := filepath.Join(tmp, sessionID+".jsonl")
	if err := os.WriteFile(transcriptPath, []byte{}, 0o644); err != nil {
		b.Fatalf("write transcript: %v", err)
	}
	subagentsDir := filepath.Join(tmp, sessionID, "subagents")
	if err := os.MkdirAll(subagentsDir, 0o755); err != nil {
		b.Fatalf("mkdir: %v", err)
	}

	// Create 5 real agents and 3 warmup agents.
	for i := 0; i < 5; i++ {
		entry := map[string]interface{}{
			"type": "user", "uuid": "u",
			"timestamp": time.Now().Format(time.RFC3339Nano),
			"message":   map[string]interface{}{"role": "user", "content": "real task"},
		}
		data, _ := json.Marshal(entry)
		data = append(data, '\n')
		name := filepath.Join(subagentsDir, "agent-"+string(rune('a'+i))+".jsonl")
		if err := os.WriteFile(name, data, 0o644); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		entry := map[string]interface{}{
			"type": "user", "uuid": "u",
			"timestamp": time.Now().Format(time.RFC3339Nano),
			"message":   map[string]interface{}{"role": "user", "content": "Warmup"},
		}
		data, _ := json.Marshal(entry)
		data = append(data, '\n')
		name := filepath.Join(subagentsDir, "agent-warmup"+string(rune('a'+i))+".jsonl")
		if err := os.WriteFile(name, data, 0o644); err != nil {
			b.Fatalf("write: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		discoverSubagents(transcriptPath)
	}
}
