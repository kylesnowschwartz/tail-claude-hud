package heartbeat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	origDir := SessionsDir
	SessionsDir = func() string { return dir }
	return dir, func() { SessionsDir = origDir }
}

func TestWriteCreatesFile(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	h := Heartbeat{SessionID: "sess-123", Project: "my-project"}
	if err := Write(h); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "sess-123"))
	if err != nil {
		t.Fatalf("heartbeat file not found: %v", err)
	}

	var got Heartbeat
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SessionID != "sess-123" || got.Project != "my-project" {
		t.Fatalf("unexpected content: %+v", got)
	}
}

func TestRemoveDeletesFile(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	h := Heartbeat{SessionID: "sess-123", Project: "proj"}
	if err := Write(h); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := Remove("sess-123"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "sess-123")); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, got: %v", err)
	}
}

func TestRemoveIdempotent(t *testing.T) {
	_, cleanup := setupTestDir(t)
	defer cleanup()

	if err := Remove("nonexistent"); err != nil {
		t.Fatalf("Remove (idempotent): %v", err)
	}
}

func TestFindOthersSkipsSelf(t *testing.T) {
	_, cleanup := setupTestDir(t)
	defer cleanup()

	h := Heartbeat{SessionID: "sess-self", Project: "solo"}
	if err := Write(h); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := FindOthers("sess-self")
	if len(got) != 0 {
		t.Fatalf("expected empty, got %+v", got)
	}
}

func TestFindOthersReturnsAllMatches(t *testing.T) {
	_, cleanup := setupTestDir(t)
	defer cleanup()

	for _, h := range []Heartbeat{
		{SessionID: "sess-aaa", Project: "proj-a"},
		{SessionID: "sess-bbb", Project: "proj-b"},
		{SessionID: "sess-ccc", Project: "proj-c"},
	} {
		if err := Write(h); err != nil {
			t.Fatalf("Write %s: %v", h.SessionID, err)
		}
	}

	got := FindOthers("sess-self")
	if len(got) != 3 {
		t.Fatalf("expected 3 sessions, got %d: %+v", len(got), got)
	}
	// Should be sorted by project name.
	if got[0].Project != "proj-a" || got[1].Project != "proj-b" || got[2].Project != "proj-c" {
		t.Fatalf("unexpected order: %+v", got)
	}
}

func TestFindOthersSkipsStale(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	h := Heartbeat{SessionID: "sess-old", Project: "stale-proj"}
	if err := Write(h); err != nil {
		t.Fatalf("Write: %v", err)
	}

	staleTime := time.Now().Add(-staleTTL - time.Minute)
	os.Chtimes(filepath.Join(dir, "sess-old"), staleTime, staleTime)

	got := FindOthers("sess-other")
	if len(got) != 0 {
		t.Fatalf("expected empty for stale heartbeat, got %+v", got)
	}
}

func TestFindOthersClassifiesRunningVsIdle(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Running: fresh heartbeat (default modtime = now).
	if err := Write(Heartbeat{SessionID: "sess-run", Project: "proj-run"}); err != nil {
		t.Fatalf("Write running: %v", err)
	}

	// Idle: modtime between runningTTL and staleTTL.
	if err := Write(Heartbeat{SessionID: "sess-idle", Project: "proj-idle"}); err != nil {
		t.Fatalf("Write idle: %v", err)
	}
	idleTime := time.Now().Add(-runningTTL - 30*time.Second)
	os.Chtimes(filepath.Join(dir, "sess-idle"), idleTime, idleTime)

	got := FindOthers("sess-self")
	if len(got) != 2 {
		t.Fatalf("expected 2 sessions, got %d: %+v", len(got), got)
	}

	// Sorted by project: proj-idle, proj-run.
	if got[0].Project != "proj-idle" || got[0].Running {
		t.Errorf("expected idle session first, got %+v", got[0])
	}
	if got[1].Project != "proj-run" || !got[1].Running {
		t.Errorf("expected running session second, got %+v", got[1])
	}
}

func TestFindOthersSkipsDotfiles(t *testing.T) {
	dir, cleanup := setupTestDir(t)
	defer cleanup()

	// Write a dotfile (temp file artifact).
	os.WriteFile(filepath.Join(dir, ".tmp-abc123"), []byte(`{"session_id":"x"}`), 0o644)

	got := FindOthers("sess-self")
	if len(got) != 0 {
		t.Fatalf("expected empty (dotfile skipped), got %+v", got)
	}
}

func TestFindOthersEmptyDir(t *testing.T) {
	_, cleanup := setupTestDir(t)
	defer cleanup()

	got := FindOthers("sess-any")
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %+v", got)
	}
}

func TestFindOthersMissingDir(t *testing.T) {
	origDir := SessionsDir
	SessionsDir = func() string { return "/nonexistent/path/sessions" }
	defer func() { SessionsDir = origDir }()

	got := FindOthers("sess-any")
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %+v", got)
	}
}
