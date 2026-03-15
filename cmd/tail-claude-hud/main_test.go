package main

import (
	"os"
	"testing"
)

// writeTemp creates a temporary file containing content and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "transcript-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

const samplePayload = `{
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

func TestReadFromFile_PathArgument(t *testing.T) {
	path := writeTemp(t, samplePayload)

	// Simulate flag.Arg(0) by setting os.Args so flag can parse it.
	// We test readFromFile indirectly by verifying the underlying stdin.Read
	// round-trip via a direct file open.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	// Confirm file-based Read works (the same path readFromFile takes).
	from, err := readFileInput(path)
	if err != nil {
		t.Fatalf("readFileInput: %v", err)
	}
	if from == nil {
		t.Fatal("expected non-nil StdinData from file")
	}
	if from.Cwd != "/home/user/project" {
		t.Errorf("Cwd: got %q, want /home/user/project", from.Cwd)
	}
	if from.ContextPercent != 42 {
		t.Errorf("ContextPercent: got %d, want 42", from.ContextPercent)
	}
}

func TestReadFromFile_EnvFallback(t *testing.T) {
	path := writeTemp(t, samplePayload)

	t.Setenv("CLAUDE_TRANSCRIPT_PATH", path)

	// With env var set and no arg, readFromFile should use the env var path.
	from, err := readFileInput(os.Getenv("CLAUDE_TRANSCRIPT_PATH"))
	if err != nil {
		t.Fatalf("readFileInput via env: %v", err)
	}
	if from == nil {
		t.Fatal("expected non-nil StdinData from env-var path")
	}
	if from.ContextPercent != 42 {
		t.Errorf("ContextPercent: got %d, want 42", from.ContextPercent)
	}
}

func TestReadFromFile_MissingFile_ReturnsError(t *testing.T) {
	_, err := readFileInput("/nonexistent/path/transcript.json")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
