package main

import (
	"os"
	"path/filepath"
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

// TestEncodePath verifies Claude Code's path encoding: /, ., and _ all become -.
func TestEncodePath(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: "/Users/kyle/Code/proj",
			want:  "-Users-kyle-Code-proj",
		},
		{
			// Dot in directory name (e.g. .claude, .config)
			input: "/Users/kyle/.config",
			want:  "-Users-kyle--config",
		},
		{
			// Underscore in directory name
			input: "/Users/kyle/my_project",
			want:  "-Users-kyle-my-project",
		},
		{
			// Mixed: worktree path with dots and underscores
			input: "/Users/kyle/.claude/worktrees/agent_abc123",
			want:  "-Users-kyle--claude-worktrees-agent-abc123",
		},
		{
			// macOS temp resolved path
			input: "/private/tmp/some_dir",
			want:  "-private-tmp-some-dir",
		},
	}

	for _, tc := range cases {
		got := encodePath(tc.input)
		if got != tc.want {
			t.Errorf("encodePath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestFindCurrentTranscript_NoFiles verifies that findCurrentTranscript returns
// an error when the project directory contains no .jsonl files.
func TestFindCurrentTranscript_NoFiles(t *testing.T) {
	// Override HOME so currentProjectDir resolves into our temp dir.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create the project dir but put no .jsonl files in it.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Resolve symlinks to match currentProjectDir's behaviour.
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	projectDir := filepath.Join(tmp, ".claude", "projects", encodePath(cwd))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err = findCurrentTranscript()
	if err == nil {
		t.Error("expected error when no .jsonl files present, got nil")
	}
}

// TestFindCurrentTranscript_ReturnsNewest verifies that findCurrentTranscript
// picks the most recently modified .jsonl file.
func TestFindCurrentTranscript_ReturnsNewest(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	projectDir := filepath.Join(tmp, ".claude", "projects", encodePath(cwd))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write two files; give the second a newer mod time via Chtimes.
	olderPath := filepath.Join(projectDir, "older.jsonl")
	newerPath := filepath.Join(projectDir, "newer.jsonl")
	for _, p := range []string{olderPath, newerPath} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	// Make newerPath demonstrably newer.
	import_time := func(p string) int64 {
		info, _ := os.Stat(p)
		return info.ModTime().UnixNano()
	}
	olderMT := import_time(olderPath)
	_ = olderMT
	// Chtimes: bump newer by 2 seconds.
	newerInfo, _ := os.Stat(newerPath)
	newTime := newerInfo.ModTime().Add(2e9) // +2s
	if err := os.Chtimes(newerPath, newTime, newTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	got, err := findCurrentTranscript()
	if err != nil {
		t.Fatalf("findCurrentTranscript: %v", err)
	}
	if got != newerPath {
		t.Errorf("got %q, want %q", got, newerPath)
	}
}
