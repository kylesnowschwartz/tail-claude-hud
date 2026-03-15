package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeLines writes newline-terminated lines to a file.
func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	for _, l := range lines {
		fmt.Fprintln(f, l)
	}
}

// appendLines appends newline-terminated lines to a file.
func appendLines(t *testing.T, path string, lines []string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	for _, l := range lines {
		fmt.Fprintln(f, l)
	}
}

// writePartial writes bytes without a trailing newline (simulates mid-write).
func writePartial(t *testing.T, path string, partial string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	fmt.Fprint(f, partial)
}

// jsonLine returns a valid JSON object string (no newline).
func jsonLine(v string) string {
	b, _ := json.Marshal(map[string]string{"v": v})
	return string(b)
}

// ---- Tests ----

// Spec 1: state file is keyed by first 12 chars of SHA-256 of the transcript path.
func TestPathHash(t *testing.T) {
	h := pathHash("/some/path/to/transcript.jsonl")
	if len(h) != 12 {
		t.Errorf("expected 12-char hash, got %d chars: %q", len(h), h)
	}
	// Same input must produce the same hash.
	h2 := pathHash("/some/path/to/transcript.jsonl")
	if h != h2 {
		t.Errorf("hash not deterministic: %q vs %q", h, h2)
	}
	// Different paths must produce different hashes.
	h3 := pathHash("/other/path/transcript.jsonl")
	if h == h3 {
		t.Errorf("collision: different paths produced same hash %q", h)
	}
}

func TestStateFilePath(t *testing.T) {
	sm := NewStateManager("/tmp/state")
	p := sm.stateFilePath("/some/transcript.jsonl")
	base := filepath.Base(p)
	if len(base) != len(".ts-")+12+len(".json") {
		t.Errorf("unexpected state file name: %q", base)
	}
	if base[:4] != ".ts-" {
		t.Errorf("state file name should start with .ts-: %q", base)
	}
	if base[len(base)-5:] != ".json" {
		t.Errorf("state file name should end with .json: %q", base)
	}
}

// Spec 2: missing state file starts from byte 0.
func TestReadIncremental_MissingStateStartsFromZero(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir)

	transcriptPath := filepath.Join(dir, "t.jsonl")
	lines := []string{jsonLine("a"), jsonLine("b"), jsonLine("c")}
	writeLines(t, transcriptPath, lines)

	got, err := sm.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("ReadIncremental: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(got), got)
	}
}

// Spec 2: corrupt state file starts from byte 0.
func TestReadIncremental_CorruptStateStartsFromZero(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir)

	transcriptPath := filepath.Join(dir, "t.jsonl")
	lines := []string{jsonLine("a"), jsonLine("b")}
	writeLines(t, transcriptPath, lines)

	// Write corrupt state file.
	stateFilePath := sm.stateFilePath(transcriptPath)
	if err := os.WriteFile(stateFilePath, []byte("not valid json!!!"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := sm.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("ReadIncremental: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 lines from byte 0, got %d: %v", len(got), got)
	}
}

// Incremental read: after saving state, only new lines are returned.
func TestReadIncremental_OnlyNewLines(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir)

	transcriptPath := filepath.Join(dir, "t.jsonl")
	first := []string{jsonLine("a"), jsonLine("b")}
	writeLines(t, transcriptPath, first)

	// First read.
	got1, err := sm.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("first ReadIncremental: %v", err)
	}
	if len(got1) != 2 {
		t.Errorf("first read: expected 2 lines, got %d", len(got1))
	}

	// Save state.
	if err := sm.SaveState(transcriptPath); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Append new lines.
	appendLines(t, transcriptPath, []string{jsonLine("c"), jsonLine("d")})

	// Second read: new StateManager simulates a new process.
	sm2 := NewStateManager(dir)
	got2, err := sm2.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("second ReadIncremental: %v", err)
	}
	if len(got2) != 2 {
		t.Errorf("second read: expected 2 new lines, got %d: %v", len(got2), got2)
	}
}

// Spec 3: stored byte_offset > file size resets to byte 0.
func TestReadIncremental_OffsetExceedsFileSizeResetsToZero(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir)

	transcriptPath := filepath.Join(dir, "t.jsonl")
	lines := []string{jsonLine("x"), jsonLine("y")}
	writeLines(t, transcriptPath, lines)

	// First read to establish offset.
	if _, err := sm.ReadIncremental(transcriptPath); err != nil {
		t.Fatalf("first read: %v", err)
	}
	if err := sm.SaveState(transcriptPath); err != nil {
		t.Fatal(err)
	}

	// Truncate the transcript (simulate a new session replacing the file).
	newLines := []string{jsonLine("fresh")}
	writeLines(t, transcriptPath, newLines)

	// The saved offset is now beyond the file size.
	sm2 := NewStateManager(dir)
	got, err := sm2.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("ReadIncremental after truncation: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 line after truncation reset, got %d: %v", len(got), got)
	}
}

// Spec 4: last line with invalid JSON is discarded; offset not advanced past it.
func TestReadIncremental_PartialLastLineDiscarded(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir)

	transcriptPath := filepath.Join(dir, "t.jsonl")
	// Two complete valid lines + a partial (no trailing newline).
	writeLines(t, transcriptPath, []string{jsonLine("a"), jsonLine("b")})
	writePartial(t, transcriptPath, `{"partial":true`)

	got, err := sm.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("ReadIncremental: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 lines (partial discarded), got %d: %v", len(got), got)
	}

	// Save state. Offset should be at the end of the two complete lines,
	// not including the partial.
	if err := sm.SaveState(transcriptPath); err != nil {
		t.Fatal(err)
	}

	// Complete the partial line by appending the rest + newline, simulating
	// Claude Code finishing the write. Then add a new valid line.
	// NOTE: we write the closing brace + newline directly (the partial was
	// `{"partial":true` so the completed line is `{"partial":true}` which IS valid JSON).
	f, err := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f, `}`) // closes the object -> `{"partial":true}` on its own line
	f.Close()

	// Append a new valid line after.
	appendLines(t, transcriptPath, []string{jsonLine("c")})

	// New process reads from saved offset: should pick up the now-complete partial line
	// AND the new valid line. The partial is now `{"partial":true}` (valid JSON), so
	// both it and jsonLine("c") are returned.
	sm2 := NewStateManager(dir)
	got2, err := sm2.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("second ReadIncremental: %v", err)
	}
	// `{"partial":true}` is valid JSON and now has a newline -> included.
	// `{"v":"c"}` is also valid -> included.
	if len(got2) != 2 {
		t.Errorf("expected 2 lines after partial was completed, got %d: %v", len(got2), got2)
	}
}

// Spec 4 (variant): a trailing partial that is truly invalid JSON (no closing brace)
// is discarded on the next tick too, until a newline arrives.
func TestReadIncremental_PartialInvalidJSONNotAdvanced(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir)

	transcriptPath := filepath.Join(dir, "t.jsonl")
	writeLines(t, transcriptPath, []string{jsonLine("a")})
	writePartial(t, transcriptPath, `{"incomplete":`)

	got, err := sm.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("ReadIncremental: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 line (partial discarded), got %d: %v", len(got), got)
	}

	if err := sm.SaveState(transcriptPath); err != nil {
		t.Fatal(err)
	}

	// Read the saved offset. It must NOT include the partial bytes.
	stateData, err := os.ReadFile(sm.stateFilePath(transcriptPath))
	if err != nil {
		t.Fatal(err)
	}
	var sf stateFile
	if err := json.Unmarshal(stateData, &sf); err != nil {
		t.Fatal(err)
	}
	// The offset should equal the length of jsonLine("a") + 1 (newline).
	expected := int64(len(jsonLine("a")) + 1)
	if sf.ByteOffset != expected {
		t.Errorf("byte_offset should be %d (end of complete line), got %d", expected, sf.ByteOffset)
	}
}

// Spec 5: state file written atomically via temp file + os.Rename.
// Verify the temp file does not persist after SaveState.
func TestSaveState_Atomic(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir)

	transcriptPath := filepath.Join(dir, "t.jsonl")
	writeLines(t, transcriptPath, []string{jsonLine("a")})

	if _, err := sm.ReadIncremental(transcriptPath); err != nil {
		t.Fatal(err)
	}
	if err := sm.SaveState(transcriptPath); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// The target state file must exist.
	target := sm.stateFilePath(transcriptPath)
	if _, err := os.Stat(target); err != nil {
		t.Errorf("state file not found after SaveState: %v", err)
	}

	// The temp file must not exist after rename.
	tmp := target + ".tmp"
	if _, err := os.Stat(tmp); err == nil {
		t.Errorf("temp file still exists after SaveState: %s", tmp)
	}

	// State file must contain valid JSON with correct fields.
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		t.Errorf("state file not valid JSON: %v", err)
	}
	if sf.TranscriptPath != transcriptPath {
		t.Errorf("transcript_path mismatch: got %q, want %q", sf.TranscriptPath, transcriptPath)
	}
	if sf.ByteOffset == 0 {
		t.Errorf("byte_offset should be > 0 after reading non-empty transcript")
	}
}

// Path differs: stored path doesn't match -> start from byte 0.
func TestReadIncremental_DifferentPathResetsToZero(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir)

	transcriptPath := filepath.Join(dir, "t.jsonl")
	writeLines(t, transcriptPath, []string{jsonLine("a"), jsonLine("b"), jsonLine("c")})

	// Read and save state for this path.
	if _, err := sm.ReadIncremental(transcriptPath); err != nil {
		t.Fatal(err)
	}
	if err := sm.SaveState(transcriptPath); err != nil {
		t.Fatal(err)
	}

	// Manually overwrite the state file to point to a different path.
	sf := stateFile{
		TranscriptPath: "/different/session/transcript.jsonl",
		ByteOffset:     9999,
	}
	data, _ := json.Marshal(sf)
	if err := os.WriteFile(sm.stateFilePath(transcriptPath), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// ReadIncremental: path mismatch -> reset to byte 0.
	sm2 := NewStateManager(dir)
	got, err := sm2.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 lines from byte 0 on path mismatch, got %d: %v", len(got), got)
	}
}

// Empty transcript returns no lines and no error.
func TestReadIncremental_EmptyTranscript(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir)

	transcriptPath := filepath.Join(dir, "t.jsonl")
	if err := os.WriteFile(transcriptPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := sm.ReadIncremental(transcriptPath)
	if err != nil {
		t.Fatalf("ReadIncremental on empty file: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 lines, got %d", len(got))
	}
}

// Non-existent transcript returns an error.
func TestReadIncremental_MissingTranscript(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(dir)

	_, err := sm.ReadIncremental(filepath.Join(dir, "does-not-exist.jsonl"))
	if err == nil {
		t.Error("expected error for missing transcript, got nil")
	}
}

// splitLines correctly handles a file with no trailing newline but valid JSON last line.
func TestSplitLines_NoTrailingNewline(t *testing.T) {
	a := jsonLine("a")
	b := jsonLine("b")
	// b has no trailing newline.
	data := []byte(a + "\n" + b)

	lines, consumed := splitLines(data)
	// Only a (with newline) is a confirmed-complete line.
	// b has no trailing newline -> treated as potentially partial, discarded.
	if len(lines) != 1 {
		t.Errorf("expected 1 confirmed line, got %d: %v", len(lines), lines)
	}
	if consumed != int64(len(a)+1) { // a + newline
		t.Errorf("consumed should be %d, got %d", len(a)+1, consumed)
	}
}

// splitLines handles multiple valid lines with trailing newline.
func TestSplitLines_AllComplete(t *testing.T) {
	a := jsonLine("a")
	b := jsonLine("b")
	data := []byte(a + "\n" + b + "\n")

	lines, consumed := splitLines(data)
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
	if consumed != int64(len(data)) {
		t.Errorf("consumed should equal data length %d, got %d", len(data), consumed)
	}
}
