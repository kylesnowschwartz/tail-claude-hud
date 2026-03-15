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
