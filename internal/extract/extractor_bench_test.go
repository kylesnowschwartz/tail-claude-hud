package extract

import (
	"fmt"
	"testing"

	"github.com/kylesnowschwartz/agent-ouija/claude/transcript"
)

// syntheticAssistantLine returns a realistic assistant JSONL line with two
// tool_use blocks, mirroring the deleted internal/transcript bench corpus so
// before/after perf comparisons stay meaningful.
func syntheticAssistantLine(i int) []byte {
	uuid := fmt.Sprintf("uuid-%08d", i)
	ts := "2024-03-15T14:22:33.123456789Z"
	toolID1 := fmt.Sprintf("tu-%08d-a", i)
	toolID2 := fmt.Sprintf("tu-%08d-b", i)
	return []byte(fmt.Sprintf(
		`{"type":"assistant","uuid":%q,"timestamp":%q,"slug":"bench-session","message":{"role":"assistant","model":"claude-opus-4-5","stop_reason":"tool_use","content":[{"type":"tool_use","id":%q,"name":"Bash","input":{"command":"go test ./..."}},{"type":"tool_use","id":%q,"name":"Read","input":{"file_path":"/some/path/file.go"}}]}}`,
		uuid, ts, toolID1, toolID2,
	))
}

// BenchmarkProcessEntry measures the per-line extraction hot path that runs
// on every statusline tick. ParseEntry and ExtractContentBlocks benchmarks
// live in the agent-ouija transcript package.
func BenchmarkProcessEntry(b *testing.B) {
	b.ReportAllocs()
	e, ok := transcript.ParseEntryLenient(syntheticAssistantLine(1))
	if !ok {
		b.Fatal("ParseEntryLenient failed")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Allocate a fresh state each iteration so we measure ProcessEntry in
		// isolation rather than the state growing unboundedly.
		es := NewExtractionState()
		es.ProcessEntry(e)
	}
}
