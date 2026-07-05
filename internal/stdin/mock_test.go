package stdin

import (
	"testing"
)

func TestMockStdinData_NonNilFields(t *testing.T) {
	data := MockStdinData("/tmp/test-transcript.jsonl")
	if data == nil {
		t.Fatal("MockStdinData returned nil")
	}
	if data.Model == nil {
		t.Error("Model must be non-nil")
	}
	if data.ContextWindow == nil {
		t.Error("ContextWindow must be non-nil")
	}
	if data.Cost == nil {
		t.Error("Cost must be non-nil")
	}
	if data.OutputStyle == nil {
		t.Error("OutputStyle must be non-nil")
	}
}

func TestMockStdinData_TranscriptPath(t *testing.T) {
	const path = "/home/user/.claude/projects/test/session.jsonl"
	data := MockStdinData(path)
	if data.TranscriptPath != path {
		t.Errorf("TranscriptPath: got %q, want %q", data.TranscriptPath, path)
	}
}

func TestMockStdinData_CwdPopulated(t *testing.T) {
	data := MockStdinData("")
	if data.CWD == "" {
		t.Error("Cwd must not be empty")
	}
}

func TestMockStdinData_ContextPercentInRange(t *testing.T) {
	data := MockStdinData("")
	if data.ContextPercent < 50 || data.ContextPercent > 70 {
		t.Errorf("ContextPercent: got %d, want 50-70", data.ContextPercent)
	}
}

func TestMockStdinData_CostNonZero(t *testing.T) {
	data := MockStdinData("")
	if data.Cost.TotalCostUSD == 0 {
		t.Error("Cost.TotalCostUSD must be non-zero")
	}
	if data.Cost.TotalDurationMs == 0 {
		t.Error("Cost.TotalDurationMs must be non-zero")
	}
	if data.Cost.TotalLinesAdded == 0 {
		t.Error("Cost.TotalLinesAdded must be non-zero")
	}
	if data.Cost.TotalLinesRemoved == 0 {
		t.Error("Cost.TotalLinesRemoved must be non-zero")
	}
}

func TestMockStdinData_ModelValues(t *testing.T) {
	data := MockStdinData("")
	if data.Model.ID != "claude-sonnet-4-20250514" {
		t.Errorf("Model.ID: got %q, want %q", data.Model.ID, "claude-sonnet-4-20250514")
	}
	if data.Model.DisplayName != "Sonnet" {
		t.Errorf("Model.DisplayName: got %q, want %q", data.Model.DisplayName, "Sonnet")
	}
}

func TestMockStdinData_ContextWindowValues(t *testing.T) {
	data := MockStdinData("")
	if data.ContextWindow.Size != 200000 {
		t.Errorf("ContextWindow.Size: got %d, want 200000", data.ContextWindow.Size)
	}
	if data.ContextWindow.CurrentUsage == nil {
		t.Fatal("ContextWindow.CurrentUsage must be non-nil")
	}
	if data.ContextWindow.CurrentUsage.InputTokens != 116000 {
		t.Errorf("InputTokens: got %d, want 116000", data.ContextWindow.CurrentUsage.InputTokens)
	}
	if data.ContextWindow.CurrentUsage.CacheCreationInputTokens != 12000 {
		t.Errorf("CacheCreationInputTokens: got %d, want 12000", data.ContextWindow.CurrentUsage.CacheCreationInputTokens)
	}
	if data.ContextWindow.CurrentUsage.CacheReadInputTokens != 8000 {
		t.Errorf("CacheReadInputTokens: got %d, want 8000", data.ContextWindow.CurrentUsage.CacheReadInputTokens)
	}
}

func TestMockStdinData_OutputStyle(t *testing.T) {
	data := MockStdinData("")
	if data.OutputStyle.Name != "concise" {
		t.Errorf("OutputStyle.Name: got %q, want %q", data.OutputStyle.Name, "concise")
	}
}
