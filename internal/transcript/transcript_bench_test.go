package transcript_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/transcript"
)

// syntheticEntry returns a realistic JSONL line for entry i.
// Odd entries are assistant messages with tool_use blocks; even entries are
// user messages with tool_result blocks. This mirrors a real Claude Code session.
func syntheticEntry(i int) []byte {
	uuid := fmt.Sprintf("uuid-%08d", i)
	ts := "2024-03-15T14:22:33.123456789Z"
	slug := "bench-session"

	if i%2 == 1 {
		// Assistant message with two tool_use blocks.
		toolID1 := fmt.Sprintf("tu-%08d-a", i)
		toolID2 := fmt.Sprintf("tu-%08d-b", i)
		return []byte(fmt.Sprintf(
			`{"type":"assistant","uuid":%q,"timestamp":%q,"slug":%q,"message":{"role":"assistant","model":"claude-opus-4-5","stop_reason":"tool_use","content":[{"type":"tool_use","id":%q,"name":"Bash","input":{"command":"go test ./..."}},{"type":"tool_use","id":%q,"name":"Read","input":{"file_path":"/some/path/file.go"}}]}}`,
			uuid, ts, slug, toolID1, toolID2,
		))
	}
	// User message with two tool_result blocks matching the previous assistant entry.
	prevToolID1 := fmt.Sprintf("tu-%08d-a", i-1)
	prevToolID2 := fmt.Sprintf("tu-%08d-b", i-1)
	return []byte(fmt.Sprintf(
		`{"type":"user","uuid":%q,"timestamp":%q,"slug":%q,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":%q,"content":"ok","is_error":false},{"type":"tool_result","tool_use_id":%q,"content":"file contents here","is_error":false}]}}`,
		uuid, ts, slug, prevToolID1, prevToolID2,
	))
}

// syntheticTranscript returns n JSONL lines joined by newlines.
func syntheticTranscript(n int) []byte {
	var buf bytes.Buffer
	for i := 0; i < n; i++ {
		buf.Write(syntheticEntry(i))
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// --- Single-entry benchmarks ---

func BenchmarkParseEntry(b *testing.B) {
	b.ReportAllocs()
	line := syntheticEntry(1) // assistant message with tool_use blocks
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = transcript.ParseEntry(line)
	}
}

func BenchmarkExtractContentBlocks(b *testing.B) {
	b.ReportAllocs()
	line := syntheticEntry(1)
	e, err := transcript.ParseEntry(line)
	if err != nil {
		b.Fatalf("ParseEntry: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = transcript.ExtractContentBlocks(e)
	}
}

func BenchmarkProcessEntry(b *testing.B) {
	b.ReportAllocs()
	line := syntheticEntry(1)
	e, err := transcript.ParseEntry(line)
	if err != nil {
		b.Fatalf("ParseEntry: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Allocate a fresh state each iteration so we measure ProcessEntry in
		// isolation rather than the state growing unboundedly.
		es := transcript.NewExtractionState()
		es.ProcessEntry(e)
	}
}

// --- Scale benchmarks ---

func BenchmarkParseTranscriptFile_100(b *testing.B) {
	benchmarkParseTranscriptFile(b, 100)
}

func BenchmarkParseTranscriptFile_1000(b *testing.B) {
	benchmarkParseTranscriptFile(b, 1000)
}

func BenchmarkParseTranscriptFile_10000(b *testing.B) {
	benchmarkParseTranscriptFile(b, 10000)
}

func benchmarkParseTranscriptFile(b *testing.B, n int) {
	b.Helper()
	b.ReportAllocs()
	data := syntheticTranscript(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = transcript.ParseTranscriptFile(data)
	}
}

// --- Incremental benchmark ---

// BenchmarkIncremental_5NewLines writes a 10k-line file, saves state at that
// point, appends 5 new lines, then measures ReadIncremental (the common hot path
// that runs on every statusline tick).
func BenchmarkIncremental_5NewLines(b *testing.B) {
	b.ReportAllocs()

	// Build the base transcript (10k lines) and 5 additional lines.
	base := syntheticTranscript(10_000)
	extra := syntheticTranscript(5) // 5 new lines to append

	// Use a temp dir for both the transcript file and the state dir.
	dir := b.TempDir()
	transcriptPath := filepath.Join(dir, "session.jsonl")
	stateDir := filepath.Join(dir, "state")

	// Write the base file.
	if err := os.WriteFile(transcriptPath, base, 0o644); err != nil {
		b.Fatalf("write base transcript: %v", err)
	}

	// Consume and persist state at the 10k-line mark.
	sm := transcript.NewStateManager(stateDir)
	if _, err := sm.ReadIncremental(transcriptPath); err != nil {
		b.Fatalf("ReadIncremental (base): %v", err)
	}
	if err := sm.SaveState(transcriptPath); err != nil {
		b.Fatalf("SaveState: %v", err)
	}

	// Append the 5 new lines once (outside the loop — we re-read the same delta
	// each iteration to measure the cost of the incremental read itself).
	f, err := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		b.Fatalf("open for append: %v", err)
	}
	if _, err := f.Write(extra); err != nil {
		f.Close()
		b.Fatalf("append lines: %v", err)
	}
	f.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Restore state to the 10k-line mark before each iteration so every
		// run reads the same 5-line delta.
		smIter := transcript.NewStateManager(stateDir)
		lines, err := smIter.ReadIncremental(transcriptPath)
		if err != nil {
			b.Fatalf("ReadIncremental: %v", err)
		}
		if len(lines) != 5 {
			b.Fatalf("expected 5 new lines, got %d", len(lines))
		}
	}
}
