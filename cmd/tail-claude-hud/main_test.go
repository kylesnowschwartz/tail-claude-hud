package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kylesnowschwartz/agent-ouija/claude/claudedir"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/preset"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/stdin"
)

// writeTempTranscript creates a temporary .jsonl transcript file and returns its path.
func writeTempTranscript(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "transcript-*.jsonl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestReadFromFile_EnvFallback(t *testing.T) {
	path := writeTempTranscript(t, `{"type":"init"}`)
	t.Setenv("CLAUDE_TRANSCRIPT_PATH", path)

	data, err := readFromFile()
	if err != nil {
		t.Fatalf("readFromFile: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil StdinData")
	}
	if data.TranscriptPath != path {
		t.Errorf("TranscriptPath: got %q, want %q", data.TranscriptPath, path)
	}
	if data.CWD == "" {
		t.Error("expected Cwd to be populated")
	}
}

func TestReadFromFile_MissingFile_ReturnsError(t *testing.T) {
	t.Setenv("CLAUDE_TRANSCRIPT_PATH", "/nonexistent/path/transcript.jsonl")

	_, err := readFromFile()
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestFindCurrentTranscript_NoFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	projectDir := filepath.Join(tmp, ".claude", "projects", claudedir.EncodeProjectPath(cwd))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err = findCurrentTranscript()
	if err == nil {
		t.Error("expected error when no .jsonl files present, got nil")
	}
}

func TestResolvePreset_BuiltinName(t *testing.T) {
	// "default" is always a built-in preset.
	names := preset.BuiltinNames()
	if len(names) == 0 {
		t.Skip("no built-in presets defined")
	}
	p, err := resolvePreset(names[0])
	if err != nil {
		t.Fatalf("resolvePreset(%q): %v", names[0], err)
	}
	if p.Name == "" {
		t.Error("expected non-empty preset Name")
	}
}

func TestResolvePreset_UnknownName(t *testing.T) {
	_, err := resolvePreset("this-preset-definitely-does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown preset, got nil")
	}
	if !strings.Contains(err.Error(), "unknown preset") {
		t.Errorf("error message should mention 'unknown preset', got: %v", err)
	}
}

func TestResolvePreset_FilePath(t *testing.T) {
	dir := t.TempDir()
	presetFile := filepath.Join(dir, "my-preset.toml")
	content := `
[[line]]
widgets = ["model", "context"]

[style]
separator = " | "
`
	if err := os.WriteFile(presetFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write preset file: %v", err)
	}

	p, err := resolvePreset(presetFile)
	if err != nil {
		t.Fatalf("resolvePreset(%q): %v", presetFile, err)
	}
	if p.Name != "my-preset" {
		t.Errorf("Name: got %q, want %q", p.Name, "my-preset")
	}
	if p.Separator != " | " {
		t.Errorf("Separator: got %q, want %q", p.Separator, " | ")
	}
}

func TestResolvePreset_TomlSuffix(t *testing.T) {
	dir := t.TempDir()
	// A path ending in .toml without "/" should still be treated as a file path.
	presetFile := filepath.Join(dir, "custom.toml")
	if err := os.WriteFile(presetFile, []byte(`[[line]]\nwidgets = ["model"]`), 0o644); err != nil {
		t.Fatalf("write preset file: %v", err)
	}

	// Verify the resolution logic routes to LoadFromFile for .toml paths.
	// (The file content is minimal so it may not parse perfectly, but errors
	//  should come from TOML parsing, not "unknown preset".)
	_, err := resolvePreset(presetFile)
	if err != nil && strings.Contains(err.Error(), "unknown preset") {
		t.Error("a .toml-suffixed path should route to LoadFromFile, not name lookup")
	}
}

func TestStatFile_ExistingFile(t *testing.T) {
	path := writeTempTranscript(t, `{"type":"init"}`)

	size, mod := statFile(path)
	if size <= 0 {
		t.Errorf("expected positive size, got %d", size)
	}
	if mod == 0 {
		t.Error("expected non-zero mtime")
	}
}

func TestStatFile_MissingFile(t *testing.T) {
	size, mod := statFile("/nonexistent/path/file.jsonl")
	if size != -1 || mod != -1 {
		t.Errorf("expected -1,-1 for missing file, got %d,%d", size, mod)
	}
}

// TestWatchAndRender_DetectsFileChange verifies that watchAndRender triggers
// a re-render when the transcript file changes. It sends SIGINT after the
// re-render to exit the loop.
func TestWatchAndRender_DetectsFileChange(t *testing.T) {
	path := writeTempTranscript(t, `{"type":"init"}`)
	input := stdin.MockStdinData(path)
	cfg := config.LoadHud()

	// Capture stdout to confirm re-render produces output.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan struct{})
	go func() {
		defer close(done)
		watchAndRender(path, input, cfg)
	}()

	// Wait for the watcher to start, then modify the file.
	time.Sleep(150 * time.Millisecond)
	if err := os.WriteFile(path, []byte(`{"type":"init"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Give the watcher up to 1s to detect the change and re-render.
	time.Sleep(700 * time.Millisecond)

	// Signal exit.
	w.Close()
	os.Stdout = origStdout

	// Read what was written.
	var buf bytes.Buffer
	buf.ReadFrom(r)
	r.Close()

	select {
	case <-done:
		// watchAndRender exited — send SIGINT if it's still running
	default:
		// Send SIGINT to the process to stop the watcher.
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(os.Interrupt)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("watchAndRender did not exit within 2s after SIGINT")
		}
	}

	// The clear-screen escape should appear in the captured output if a re-render occurred.
	if !strings.Contains(buf.String(), "\x1b[2J") {
		t.Error("expected clear-screen ANSI sequence in output after file change")
	}
}

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
	projectDir := filepath.Join(tmp, ".claude", "projects", claudedir.EncodeProjectPath(cwd))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	olderPath := filepath.Join(projectDir, "older.jsonl")
	newerPath := filepath.Join(projectDir, "newer.jsonl")
	for _, p := range []string{olderPath, newerPath} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	// Make newerPath demonstrably newer.
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
