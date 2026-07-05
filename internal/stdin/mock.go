package stdin

import (
	"os"

	"github.com/kylesnowschwartz/agent-ouija/claude/statusline"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// MockStdinData returns a realistic StdinData suitable for preview rendering.
// All top-level fields are populated so every widget has data to display.
//
// The context window is set to ~68% used (116k+12k+8k / 200k * 100),
// which falls in the normal green range and exercises the context widget.
//
// transcriptPath is used verbatim for TranscriptPath; CWD is os.Getwd()
// falling back to "/tmp" on error.
func MockStdinData(transcriptPath string) *model.StdinData {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "/tmp"
	}

	usedPct := float64(68) // (116000+12000+8000) / 200000 * 100 = 68

	data := &model.StdinData{ContextPercent: 68}
	data.SessionID = "mock-preview-session"
	data.TranscriptPath = transcriptPath
	data.CWD = cwd
	data.Model = &statusline.Model{
		ID:          "claude-sonnet-4-20250514",
		DisplayName: "Sonnet",
	}
	data.ContextWindow = &statusline.ContextWindow{
		Size:        200000,
		UsedPercent: &usedPct,
		CurrentUsage: &statusline.Usage{
			InputTokens:              116000,
			CacheCreationInputTokens: 12000,
			CacheReadInputTokens:     8000,
		},
	}
	data.Cost = &statusline.Cost{
		TotalCostUSD:       2.47,
		TotalDurationMs:    425000,
		TotalAPIDurationMs: 38000,
		TotalLinesAdded:    187,
		TotalLinesRemoved:  42,
	}
	data.OutputStyle = &statusline.OutputStyle{
		Name: "concise",
	}
	return data
}
