package gather

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// minimalInput returns a StdinData with required scalar fields set.
func minimalInput() *model.StdinData {
	return &model.StdinData{
		Cwd:            "/tmp/test-project",
		ContextPercent: 42,
		Model: &struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		}{
			ID:          "claude-opus-4",
			DisplayName: "Claude Opus 4",
		},
		ContextWindow: &struct {
			Size         int      `json:"context_window_size"`
			UsedPercent  *float64 `json:"used_percentage"`
			CurrentUsage *struct {
				InputTokens              int `json:"input_tokens"`
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
				CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			} `json:"current_usage"`
		}{
			Size: 200000,
		},
	}
}

// cfgWithWidgets builds a minimal Config that activates the given widget names.
func cfgWithWidgets(widgets ...string) *config.Config {
	cfg := &config.Config{}
	cfg.Lines = []config.Line{
		{Widgets: widgets},
	}
	return cfg
}

func TestGather_StdinScalarsCopied(t *testing.T) {
	input := minimalInput()
	cfg := cfgWithWidgets("model", "context", "directory")

	ctx := Gather(input, cfg)

	if ctx.Cwd != input.Cwd {
		t.Errorf("Cwd: got %q, want %q", ctx.Cwd, input.Cwd)
	}
	if ctx.ContextPercent != input.ContextPercent {
		t.Errorf("ContextPercent: got %d, want %d", ctx.ContextPercent, input.ContextPercent)
	}
	if ctx.ModelID != input.Model.ID {
		t.Errorf("ModelID: got %q, want %q", ctx.ModelID, input.Model.ID)
	}
	if ctx.ModelDisplayName != input.Model.DisplayName {
		t.Errorf("ModelDisplayName: got %q, want %q", ctx.ModelDisplayName, input.Model.DisplayName)
	}
	if ctx.ContextWindowSize != input.ContextWindow.Size {
		t.Errorf("ContextWindowSize: got %d, want %d", ctx.ContextWindowSize, input.ContextWindow.Size)
	}
}

func TestGather_NoTranscriptGoroutineWhenWidgetsAbsent(t *testing.T) {
	// Widgets that do NOT need transcript — goroutine must not be spawned,
	// so Transcript stays nil even when a path is present.
	input := minimalInput()
	input.TranscriptPath = "/nonexistent/transcript.jsonl"
	cfg := cfgWithWidgets("model", "context", "directory", "env", "git")

	ctx := Gather(input, cfg)

	if ctx.Transcript != nil {
		t.Errorf("expected Transcript to be nil when no transcript widgets configured, got non-nil")
	}
}

func TestGather_NoEnvGoroutineWhenWidgetAbsent(t *testing.T) {
	input := minimalInput()
	cfg := cfgWithWidgets("model", "context")

	ctx := Gather(input, cfg)

	if ctx.EnvCounts != nil {
		t.Errorf("expected EnvCounts to be nil when env widget not configured, got non-nil")
	}
}

func TestGather_EnvCountsPopulatedWhenWidgetActive(t *testing.T) {
	input := minimalInput()
	cfg := cfgWithWidgets("env")

	ctx := Gather(input, cfg)

	// CountEnv never returns nil, so the field should be populated.
	if ctx.EnvCounts == nil {
		t.Errorf("expected EnvCounts to be non-nil when env widget is active")
	}
}

func TestGather_GitNilWhenWidgetAbsent(t *testing.T) {
	input := minimalInput()
	cfg := cfgWithWidgets("model", "context")

	ctx := Gather(input, cfg)

	// Git goroutine not spawned — field stays nil.
	if ctx.Git != nil {
		t.Errorf("expected Git to be nil when git widget not configured, got non-nil")
	}
}

func TestGather_NoTranscriptWhenPathEmpty(t *testing.T) {
	input := minimalInput()
	input.TranscriptPath = "" // no path
	cfg := cfgWithWidgets("tools", "agents", "todos")

	ctx := Gather(input, cfg)

	if ctx.Transcript != nil {
		t.Errorf("expected Transcript nil when TranscriptPath is empty, got non-nil")
	}
}

func TestGather_TranscriptPopulatedFromFile(t *testing.T) {
	// Write a minimal transcript JSONL with one assistant message.
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "session.jsonl")

	entry := map[string]interface{}{
		"type":      "assistant",
		"uuid":      "test-uuid-1",
		"timestamp": "2024-01-15T10:00:00Z",
		"message": map[string]interface{}{
			"role":    "assistant",
			"content": "hello",
		},
	}
	line, _ := json.Marshal(entry)
	if err := os.WriteFile(transcriptPath, append(line, '\n'), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	input := minimalInput()
	input.TranscriptPath = transcriptPath
	cfg := cfgWithWidgets("tools")

	ctx := Gather(input, cfg)

	if ctx.Transcript == nil {
		t.Fatalf("expected Transcript non-nil when tools widget active and path set")
	}
	if ctx.Transcript.Path != transcriptPath {
		t.Errorf("Transcript.Path: got %q, want %q", ctx.Transcript.Path, transcriptPath)
	}
}

func TestGather_SessionStartFromTranscriptTimestamp(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "session.jsonl")

	entry := map[string]interface{}{
		"type":      "assistant",
		"uuid":      "test-uuid-1",
		"timestamp": "2024-01-15T10:00:00Z",
		"message": map[string]interface{}{
			"role":    "assistant",
			"content": "hello",
		},
	}
	line, _ := json.Marshal(entry)
	if err := os.WriteFile(transcriptPath, append(line, '\n'), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	input := minimalInput()
	input.TranscriptPath = transcriptPath
	cfg := cfgWithWidgets("duration", "tools")

	ctx := Gather(input, cfg)

	// SessionStart should be the RFC3339 timestamp of the first entry.
	if ctx.SessionStart == "" {
		t.Error("expected SessionStart to be set from transcript timestamp, got empty string")
	}
}

func TestGather_TerminalWidthFromEnv(t *testing.T) {
	t.Setenv("COLUMNS", "120")

	input := minimalInput()
	cfg := cfgWithWidgets("model")

	ctx := Gather(input, cfg)

	if ctx.TerminalWidth != 120 {
		t.Errorf("TerminalWidth: got %d, want 120", ctx.TerminalWidth)
	}
}

func TestGather_TerminalWidthZeroWhenEnvUnset(t *testing.T) {
	t.Setenv("COLUMNS", "")

	input := minimalInput()
	cfg := cfgWithWidgets("model")

	ctx := Gather(input, cfg)

	if ctx.TerminalWidth != 0 {
		t.Errorf("TerminalWidth: got %d, want 0 when COLUMNS unset", ctx.TerminalWidth)
	}
}

func TestActiveWidgets_FlattensAllLines(t *testing.T) {
	cfg := &config.Config{
		Lines: []config.Line{
			{Widgets: []string{"model", "context"}},
			{Widgets: []string{"tools", "git"}},
		},
	}

	active := activeWidgets(cfg)

	expected := []string{"model", "context", "tools", "git"}
	for _, w := range expected {
		if !active[w] {
			t.Errorf("expected widget %q in active set, but not found", w)
		}
	}
	if len(active) != len(expected) {
		t.Errorf("active widget count: got %d, want %d", len(active), len(expected))
	}
}

func TestNeedsTranscript(t *testing.T) {
	cases := []struct {
		name    string
		widgets []string
		want    bool
	}{
		{"tools active", []string{"tools"}, true},
		{"agents active", []string{"agents"}, true},
		{"todos active", []string{"todos"}, true},
		{"thinking active", []string{"thinking"}, true},
		{"none active", []string{"model", "git", "env"}, false},
		{"empty", []string{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			active := make(map[string]bool)
			for _, w := range tc.widgets {
				active[w] = true
			}
			if got := needsTranscript(active); got != tc.want {
				t.Errorf("needsTranscript(%v): got %v, want %v", tc.widgets, got, tc.want)
			}
		})
	}
}

func TestNeedsTranscript_ThinkingWidget(t *testing.T) {
	// "thinking" must be recognised as a transcript-backed widget.
	active := map[string]bool{"thinking": true}
	if !needsTranscript(active) {
		t.Error("needsTranscript: want true when 'thinking' widget is active")
	}
}

func TestGather_TranscriptSpawnedForThinkingWidget(t *testing.T) {
	// Ensure the transcript goroutine runs when the "thinking" widget is active.
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "session.jsonl")

	entry := map[string]interface{}{
		"type":      "assistant",
		"uuid":      "test-uuid-thinking",
		"timestamp": "2024-01-15T10:00:00Z",
		"message": map[string]interface{}{
			"role":    "assistant",
			"content": "hello",
		},
	}
	line, _ := json.Marshal(entry)
	if err := os.WriteFile(transcriptPath, append(line, '\n'), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	input := minimalInput()
	input.TranscriptPath = transcriptPath
	cfg := cfgWithWidgets("thinking")

	ctx := Gather(input, cfg)

	if ctx.Transcript == nil {
		t.Fatal("expected Transcript non-nil when thinking widget active and path set")
	}
}

func TestGather_GitSpawnedForProjectWidget(t *testing.T) {
	// When "project" is active but "git" is not, the git goroutine should
	// still be spawned so the project widget has ahead/behind data available.
	// We can only observe this indirectly: Git field must be non-nil when
	// the cwd is inside a real git repository.
	input := minimalInput()
	// Use a real directory that is inside a git repo so git.GetStatus returns data.
	input.Cwd = "/Users/kyle/Code/my-projects/tail-claude-hud"
	cfg := cfgWithWidgets("project") // "git" widget NOT listed

	ctx := Gather(input, cfg)

	// git.GetStatus was called — ctx.Git must be non-nil.
	if ctx.Git == nil {
		t.Error("expected Git non-nil when project widget is active, got nil")
	}
}

func TestGather_NilModelAndContextWindow(t *testing.T) {
	// Ensure Gather doesn't panic when optional StdinData fields are nil.
	input := &model.StdinData{
		Cwd:            "/tmp",
		ContextPercent: 0,
		Model:          nil,
		ContextWindow:  nil,
	}
	cfg := cfgWithWidgets("model")

	ctx := Gather(input, cfg)

	if ctx.ModelID != "" {
		t.Errorf("expected empty ModelID when input.Model is nil, got %q", ctx.ModelID)
	}
	if ctx.ContextWindowSize != 0 {
		t.Errorf("expected zero ContextWindowSize when input.ContextWindow is nil, got %d", ctx.ContextWindowSize)
	}
}
