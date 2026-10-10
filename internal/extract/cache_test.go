package extract

import (
	"testing"
	"time"

	"github.com/kylesnowschwartz/agent-ouija/claude/transcript"
)

// feedLines parses each JSONL line and runs it through es in order.
func feedLines(t *testing.T, es *ExtractionState, lines ...string) {
	t.Helper()
	for _, line := range lines {
		e, ok := transcript.ParseEntryLenient([]byte(line))
		if !ok {
			t.Fatalf("ParseEntryLenient rejected fixture line: %s", line)
		}
		es.ProcessEntry(e)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

const (
	cacheUserPrompt  = `{"type":"user","uuid":"u1","timestamp":"2026-10-10T09:00:00.000Z","message":{"role":"user","content":"hi"}}`
	cacheAssistant1h = `{"type":"assistant","uuid":"a1","timestamp":"2026-10-10T09:00:05.000Z","message":{"id":"msg_A","role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":2,"output_tokens":10,"cache_creation_input_tokens":900,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":900}}}}`
)

func TestCache_RequestTimeIsPrecedingUserEntry(t *testing.T) {
	es := NewExtractionState()
	feedLines(t, es, cacheUserPrompt, cacheAssistant1h)

	td := es.ToTranscriptData()
	if want := mustTime(t, "2026-10-10T09:00:00Z"); !td.LastRequestAt.Equal(want) {
		t.Errorf("LastRequestAt = %v, want %v (the user prompt)", td.LastRequestAt, want)
	}
	if td.CacheTTL != time.Hour {
		t.Errorf("CacheTTL = %v, want 1h", td.CacheTTL)
	}
}

func TestCache_EmptyBeforeAnyResponse(t *testing.T) {
	es := NewExtractionState()
	feedLines(t, es, cacheUserPrompt)

	td := es.ToTranscriptData()
	if !td.LastRequestAt.IsZero() || td.CacheTTL != 0 {
		t.Errorf("LastRequestAt = %v, CacheTTL = %v; want both zero", td.LastRequestAt, td.CacheTTL)
	}
}

// One response is written as several rows sharing message.id; a tool result
// written between them must not restart the request time.
func TestCache_MultiRowResponseCountedOnce(t *testing.T) {
	es := NewExtractionState()
	feedLines(t, es,
		cacheUserPrompt,
		`{"type":"assistant","uuid":"a1","timestamp":"2026-10-10T09:00:05.000Z","message":{"id":"msg_A","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Read","input":{}}],"usage":{"input_tokens":2,"output_tokens":10}}}`,
		`{"type":"user","uuid":"u2","timestamp":"2026-10-10T09:00:06.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"x"}]}}`,
		`{"type":"assistant","uuid":"a2","timestamp":"2026-10-10T09:00:07.000Z","message":{"id":"msg_A","role":"assistant","content":[{"type":"tool_use","id":"t2","name":"Read","input":{}}],"usage":{"input_tokens":2,"output_tokens":10}}}`,
	)

	if got, want := es.ToTranscriptData().LastRequestAt, mustTime(t, "2026-10-10T09:00:00Z"); !got.Equal(want) {
		t.Errorf("LastRequestAt = %v, want %v (first row of msg_A)", got, want)
	}

	// The next response is a new request, carried by the latest tool result.
	feedLines(t, es,
		`{"type":"user","uuid":"u3","timestamp":"2026-10-10T09:00:08.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"y"}]}}`,
		`{"type":"assistant","uuid":"a3","timestamp":"2026-10-10T09:00:12.000Z","message":{"id":"msg_B","role":"assistant","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":2,"output_tokens":10}}}`,
	)
	if got, want := es.ToTranscriptData().LastRequestAt, mustTime(t, "2026-10-10T09:00:08Z"); !got.Equal(want) {
		t.Errorf("LastRequestAt = %v, want %v (tool result before msg_B)", got, want)
	}
}

func TestCache_SidechainEntriesIgnored(t *testing.T) {
	es := NewExtractionState()
	feedLines(t, es,
		cacheUserPrompt,
		cacheAssistant1h,
		`{"type":"user","uuid":"s1","timestamp":"2026-10-10T09:10:00.000Z","isSidechain":true,"message":{"role":"user","content":"sub task"}}`,
		`{"type":"assistant","uuid":"s2","timestamp":"2026-10-10T09:10:05.000Z","isSidechain":true,"message":{"id":"msg_S","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":2,"output_tokens":10,"cache_creation":{"ephemeral_5m_input_tokens":400,"ephemeral_1h_input_tokens":0}}}}`,
	)

	td := es.ToTranscriptData()
	if want := mustTime(t, "2026-10-10T09:00:00Z"); !td.LastRequestAt.Equal(want) {
		t.Errorf("LastRequestAt = %v, want %v (sidechain skipped)", td.LastRequestAt, want)
	}
	if td.CacheTTL != time.Hour {
		t.Errorf("CacheTTL = %v, want 1h (sidechain 5m write skipped)", td.CacheTTL)
	}
}

func TestCache_SyntheticAndUsagelessRowsIgnored(t *testing.T) {
	es := NewExtractionState()
	feedLines(t, es,
		cacheUserPrompt,
		cacheAssistant1h,
		`{"type":"user","uuid":"u2","timestamp":"2026-10-10T09:05:00.000Z","message":{"role":"user","content":"again"}}`,
		`{"type":"assistant","uuid":"a2","timestamp":"2026-10-10T09:05:01.000Z","message":{"id":"msg_X","role":"assistant","model":"<synthetic>","content":[{"type":"text","text":"API Error"}],"usage":{"input_tokens":0,"output_tokens":0}}}`,
		`{"type":"assistant","uuid":"a3","timestamp":"2026-10-10T09:05:02.000Z","message":{"id":"msg_Y","role":"assistant","content":[{"type":"text","text":"no usage"}]}}`,
	)

	if got, want := es.ToTranscriptData().LastRequestAt, mustTime(t, "2026-10-10T09:00:00Z"); !got.Equal(want) {
		t.Errorf("LastRequestAt = %v, want %v (synthetic/usage-less rows skipped)", got, want)
	}
}

func TestCache_TTLFollowsLatestWrite(t *testing.T) {
	es := NewExtractionState()
	feedLines(t, es, cacheUserPrompt, cacheAssistant1h,
		`{"type":"user","uuid":"u2","timestamp":"2026-10-10T09:01:00.000Z","message":{"role":"user","content":"next"}}`,
		`{"type":"assistant","uuid":"a2","timestamp":"2026-10-10T09:01:02.000Z","message":{"id":"msg_B","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":2,"output_tokens":5,"cache_creation":{"ephemeral_5m_input_tokens":300,"ephemeral_1h_input_tokens":0}}}}`,
	)
	if got := es.ToTranscriptData().CacheTTL; got != 5*time.Minute {
		t.Fatalf("CacheTTL = %v, want 5m after a 5m write", got)
	}

	// A response that only reads the cache keeps the previous TTL.
	feedLines(t, es,
		`{"type":"user","uuid":"u3","timestamp":"2026-10-10T09:02:00.000Z","message":{"role":"user","content":"read only"}}`,
		`{"type":"assistant","uuid":"a3","timestamp":"2026-10-10T09:02:02.000Z","message":{"id":"msg_C","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":2,"output_tokens":5,"cache_read_input_tokens":1200,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":0}}}}`,
	)
	if got := es.ToTranscriptData().CacheTTL; got != 5*time.Minute {
		t.Errorf("CacheTTL = %v, want 5m kept after a read-only response", got)
	}
}

// The user entry and its response can land in different ticks, so the
// pending user timestamp and response id must survive the snapshot.
func TestCache_SnapshotRoundTripAcrossTicks(t *testing.T) {
	es1 := NewExtractionState()
	feedLines(t, es1, cacheUserPrompt, cacheAssistant1h,
		`{"type":"user","uuid":"u2","timestamp":"2026-10-10T09:03:00.000Z","message":{"role":"user","content":"next"}}`,
	)
	snap, err := es1.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	es2 := NewExtractionState()
	if err := es2.UnmarshalSnapshot(snap); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}
	td := es2.ToTranscriptData()
	if want := mustTime(t, "2026-10-10T09:00:00Z"); !td.LastRequestAt.Equal(want) {
		t.Errorf("restored LastRequestAt = %v, want %v", td.LastRequestAt, want)
	}
	if td.CacheTTL != time.Hour {
		t.Errorf("restored CacheTTL = %v, want 1h", td.CacheTTL)
	}

	feedLines(t, es2,
		`{"type":"assistant","uuid":"a2","timestamp":"2026-10-10T09:03:04.000Z","message":{"id":"msg_B","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":2,"output_tokens":5}}}`,
	)
	if got, want := es2.ToTranscriptData().LastRequestAt, mustTime(t, "2026-10-10T09:03:00Z"); !got.Equal(want) {
		t.Errorf("LastRequestAt = %v, want %v (user entry from the previous tick)", got, want)
	}
}
