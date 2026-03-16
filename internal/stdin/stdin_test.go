package stdin

import (
	"os"
	"testing"
)

// writeTemp creates a temporary file containing s and returns it open for reading.
// The caller is responsible for closing and removing the file.
func writeTemp(t *testing.T, s string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(s); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	// Rewind so Read() can decode from the beginning.
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek temp file: %v", err)
	}
	return f
}

func TestRead_ValidJSON_WithUsedPercentage(t *testing.T) {
	const payload = `{
		"transcript_path": "/tmp/transcript.jsonl",
		"cwd": "/home/user/project",
		"model": {"id": "claude-sonnet-4-6", "display_name": "Claude Sonnet 4.6"},
		"context_window": {
			"context_window_size": 200000,
			"used_percentage": 42.7,
			"current_usage": {
				"input_tokens": 10000,
				"cache_creation_input_tokens": 5000,
				"cache_read_input_tokens": 3000
			}
		}
	}`

	f := writeTemp(t, payload)
	defer f.Close()

	data, err := Read(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil StdinData")
	}
	if data.TranscriptPath != "/tmp/transcript.jsonl" {
		t.Errorf("TranscriptPath: got %q, want %q", data.TranscriptPath, "/tmp/transcript.jsonl")
	}
	// used_percentage (42.7) is preferred; int truncation gives 42.
	if data.ContextPercent != 42 {
		t.Errorf("ContextPercent: got %d, want 42", data.ContextPercent)
	}
}

func TestRead_ValidJSON_WithoutUsedPercentage_FallsBackToManualCalc(t *testing.T) {
	// 90 000 tokens out of 200 000 = 45 %
	const payload = `{
		"context_window": {
			"context_window_size": 200000,
			"current_usage": {
				"input_tokens": 50000,
				"cache_creation_input_tokens": 20000,
				"cache_read_input_tokens": 20000
			}
		}
	}`

	f := writeTemp(t, payload)
	defer f.Close()

	data, err := Read(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil StdinData")
	}
	// (50000 + 20000 + 20000) * 100 / 200000 = 45
	if data.ContextPercent != 45 {
		t.Errorf("ContextPercent: got %d, want 45", data.ContextPercent)
	}
}

func TestRead_InvalidJSON_ReturnsError(t *testing.T) {
	f := writeTemp(t, `{ this is not valid json }`)
	defer f.Close()

	data, err := Read(f)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
	if data != nil {
		t.Errorf("expected nil data on error, got %+v", data)
	}
}

func TestRead_EmptyInput_ReturnsError(t *testing.T) {
	f := writeTemp(t, ``)
	defer f.Close()

	data, err := Read(f)
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
	if data != nil {
		t.Errorf("expected nil data on error, got %+v", data)
	}
}

func TestRead_CostFields_DecodedCorrectly(t *testing.T) {
	const payload = `{
		"transcript_path": "/tmp/t.jsonl",
		"cwd": "/home/user",
		"cost": {
			"total_cost_usd": 0.1234,
			"total_duration_ms": 185000,
			"total_api_duration_ms": 42000,
			"total_lines_added": 87,
			"total_lines_removed": 23
		},
		"output_style": {"name": "auto"}
	}`

	f := writeTemp(t, payload)
	defer f.Close()

	data, err := Read(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil StdinData")
	}
	if data.Cost == nil {
		t.Fatal("expected non-nil Cost")
	}
	if data.Cost.TotalCostUSD != 0.1234 {
		t.Errorf("TotalCostUSD: got %v, want 0.1234", data.Cost.TotalCostUSD)
	}
	if data.Cost.TotalDurationMs != 185000 {
		t.Errorf("TotalDurationMs: got %d, want 185000", data.Cost.TotalDurationMs)
	}
	if data.Cost.TotalAPIDurationMs != 42000 {
		t.Errorf("TotalAPIDurationMs: got %d, want 42000", data.Cost.TotalAPIDurationMs)
	}
	if data.Cost.TotalLinesAdded != 87 {
		t.Errorf("TotalLinesAdded: got %d, want 87", data.Cost.TotalLinesAdded)
	}
	if data.Cost.TotalLinesRemoved != 23 {
		t.Errorf("TotalLinesRemoved: got %d, want 23", data.Cost.TotalLinesRemoved)
	}
	if data.OutputStyle == nil {
		t.Fatal("expected non-nil OutputStyle")
	}
	if data.OutputStyle.Name != "auto" {
		t.Errorf("OutputStyle.Name: got %q, want %q", data.OutputStyle.Name, "auto")
	}
}

func TestRead_MissingCostObject_NilCost(t *testing.T) {
	// When cost is absent from the payload, StdinData.Cost must be nil
	// so callers can distinguish "no cost data" from "zero cost".
	const payload = `{
		"transcript_path": "/tmp/t.jsonl",
		"cwd": "/home/user"
	}`

	f := writeTemp(t, payload)
	defer f.Close()

	data, err := Read(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil StdinData")
	}
	if data.Cost != nil {
		t.Errorf("expected nil Cost when absent from JSON, got %+v", data.Cost)
	}
	if data.OutputStyle != nil {
		t.Errorf("expected nil OutputStyle when absent from JSON, got %+v", data.OutputStyle)
	}
}
