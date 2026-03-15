package gather

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render"
)

// BenchmarkGather_NoTranscript measures baseline gather overhead when no
// transcript-backed widgets are active and no I/O is performed.
func BenchmarkGather_NoTranscript(b *testing.B) {
	b.ReportAllocs()

	input := &model.StdinData{
		Cwd:            "/tmp/bench-project",
		ContextPercent: 42,
		Model: &struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		}{
			ID:          "claude-sonnet-4-20250514",
			DisplayName: "Claude Sonnet 4",
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

	// Only non-transcript, non-I/O widgets: model, context, directory.
	cfg := &config.Config{}
	cfg.Lines = []config.Line{
		{Widgets: []string{"model", "context", "directory"}},
	}
	cfg.Style.Separator = " | "

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Gather(input, cfg)
	}
}

// BenchmarkGather_WithTranscript measures the cost of transcript parsing
// through the gather pipeline. The JSONL file (1000 lines of alternating
// tool_use / tool_result entries) is created once during setup.
func BenchmarkGather_WithTranscript(b *testing.B) {
	b.ReportAllocs()

	transcriptPath := filepath.Join(b.TempDir(), "bench-session.jsonl")
	if err := writeSyntheticTranscript(transcriptPath, 1000); err != nil {
		b.Fatalf("write synthetic transcript: %v", err)
	}

	input := &model.StdinData{
		Cwd:            "/tmp/bench-project",
		ContextPercent: 42,
		TranscriptPath: transcriptPath,
		Model: &struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		}{
			ID:          "claude-sonnet-4-20250514",
			DisplayName: "Claude Sonnet 4",
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

	cfg := &config.Config{}
	cfg.Lines = []config.Line{
		{Widgets: []string{"tools"}},
	}
	cfg.Style.Separator = " | "

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Gather(input, cfg)
	}
}

// BenchmarkRender_FullContext measures pure render cost with a fully-populated
// RenderContext and default config. No I/O occurs during the benchmark loop.
func BenchmarkRender_FullContext(b *testing.B) {
	b.ReportAllocs()

	ctx := &model.RenderContext{
		ModelID:           "claude-sonnet-4-20250514",
		ModelDisplayName:  "Claude Sonnet 4",
		ContextWindowSize: 200000,
		ContextPercent:    42,
		Cwd:               "/Users/kyle/Code/my-projects/tail-claude-hud",
		InputTokens:       45000,
		CacheCreation:     12000,
		CacheRead:         8000,
		SessionDuration:   "2026-03-15T09:00:00Z",
		TerminalWidth:     200,
		Transcript: &model.TranscriptData{
			Path:        "/tmp/bench-session.jsonl",
			SessionName: "Bench Session",
			Tools: []model.ToolEntry{
				{Name: "Bash", Count: 12},
				{Name: "Read", Count: 8},
				{Name: "Write", Count: 5},
				{Name: "Grep", Count: 4},
				{Name: "Edit", Count: 3},
			},
			Agents: []model.AgentEntry{
				{Name: "worker-1", Status: "done"},
				{Name: "worker-2", Status: "running"},
			},
			Todos: []model.TodoItem{
				{ID: "1", Content: "Implement benchmarks", Done: true},
				{ID: "2", Content: "Write fixture file", Done: true},
				{ID: "3", Content: "Run verification", Done: false},
			},
		},
		EnvCounts: &model.EnvCounts{
			MCPServers:   5,
			ToolsAllowed: 32,
		},
		Git: &model.GitStatus{
			Branch:  "feat/benchmarks",
			Dirty:   true,
			AheadBy: 2,
			BehindBy: 0,
		},
	}

	cfg := config.LoadHud()

	var buf bytes.Buffer

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		render.Render(&buf, ctx, cfg)
	}
}

// writeSyntheticTranscript writes nLines of alternating assistant (tool_use)
// and user (tool_result) JSONL entries to path.
func writeSyntheticTranscript(path string, nLines int) error {
	toolNames := []string{"Bash", "Read", "Write", "Grep", "Edit"}

	var buf bytes.Buffer
	for i := 0; i < nLines; i++ {
		toolName := toolNames[i%len(toolNames)]
		toolID := fmt.Sprintf("tool-id-%04d", i)

		if i%2 == 0 {
			// assistant entry with tool_use content block
			entry := map[string]interface{}{
				"type":      "assistant",
				"uuid":      fmt.Sprintf("uuid-asst-%04d", i),
				"timestamp": "2026-03-15T09:00:00Z",
				"message": map[string]interface{}{
					"role": "assistant",
					"content": []map[string]interface{}{
						{
							"type":  "tool_use",
							"id":    toolID,
							"name":  toolName,
							"input": map[string]string{"command": "echo bench"},
						},
					},
				},
			}
			line, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			buf.Write(line)
			buf.WriteByte('\n')
		} else {
			// user entry with tool_result content block
			entry := map[string]interface{}{
				"type":      "user",
				"uuid":      fmt.Sprintf("uuid-user-%04d", i),
				"timestamp": "2026-03-15T09:00:01Z",
				"message": map[string]interface{}{
					"role": "user",
					"content": []map[string]interface{}{
						{
							"type":       "tool_result",
							"tool_use_id": toolID,
							"content":    "output text",
							"is_error":   false,
						},
					},
				},
			}
			line, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			buf.Write(line)
			buf.WriteByte('\n')
		}
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}
