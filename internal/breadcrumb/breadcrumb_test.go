package breadcrumb

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndRemove(t *testing.T) {
	dir := t.TempDir()
	origDir := WaitingDir
	WaitingDir = func() string { return dir }
	defer func() { WaitingDir = origDir }()

	b := Breadcrumb{
		SessionID: "sess-123",
		Project:   "my-project",
		ToolName:  "Bash",
	}

	if err := Write(b); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// File should exist.
	if _, err := os.Stat(filepath.Join(dir, "sess-123")); err != nil {
		t.Fatalf("breadcrumb file not found: %v", err)
	}

	// Remove should succeed.
	if err := Remove("sess-123"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// File should be gone.
	if _, err := os.Stat(filepath.Join(dir, "sess-123")); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, got: %v", err)
	}

	// Remove again should be a no-op.
	if err := Remove("sess-123"); err != nil {
		t.Fatalf("Remove (idempotent): %v", err)
	}
}

func TestFindWaiting(t *testing.T) {
	dir := t.TempDir()
	origDir := WaitingDir
	WaitingDir = func() string { return dir }
	defer func() { WaitingDir = origDir }()

	// Write breadcrumbs for two sessions.
	b1 := Breadcrumb{SessionID: "sess-aaa", Project: "project-a", ToolName: "Bash"}
	b2 := Breadcrumb{SessionID: "sess-bbb", Project: "project-b", ToolName: "Edit"}

	if err := Write(b1); err != nil {
		t.Fatalf("Write b1: %v", err)
	}
	if err := Write(b2); err != nil {
		t.Fatalf("Write b2: %v", err)
	}

	// Searching as sess-aaa should find sess-bbb (skip self).
	found := FindWaiting("sess-aaa")
	if found == nil {
		t.Fatal("expected to find a waiting session")
	}
	if found.SessionID != "sess-bbb" {
		t.Fatalf("expected sess-bbb, got %s", found.SessionID)
	}
}

func TestFindWaiting_SkipsSelf(t *testing.T) {
	dir := t.TempDir()
	origDir := WaitingDir
	WaitingDir = func() string { return dir }
	defer func() { WaitingDir = origDir }()

	b := Breadcrumb{SessionID: "sess-only", Project: "solo"}
	if err := Write(b); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Only breadcrumb is our own session — should find nothing.
	if found := FindWaiting("sess-only"); found != nil {
		t.Fatalf("expected nil, got %+v", found)
	}
}

func TestFindWaiting_SkipsStale(t *testing.T) {
	dir := t.TempDir()
	origDir := WaitingDir
	WaitingDir = func() string { return dir }
	defer func() { WaitingDir = origDir }()

	b := Breadcrumb{SessionID: "sess-old", Project: "stale-project"}
	if err := Write(b); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Backdate the file to make it stale.
	staleTime := time.Now().Add(-staleTTL - time.Minute)
	os.Chtimes(filepath.Join(dir, "sess-old"), staleTime, staleTime)

	if found := FindWaiting("sess-other"); found != nil {
		t.Fatalf("expected nil for stale breadcrumb, got %+v", found)
	}
}

func TestFindWaiting_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	origDir := WaitingDir
	WaitingDir = func() string { return dir }
	defer func() { WaitingDir = origDir }()

	if found := FindWaiting("sess-any"); found != nil {
		t.Fatalf("expected nil for empty dir, got %+v", found)
	}
}

func TestRemoveStale_RemovesOnlyStaleFiles(t *testing.T) {
	dir := t.TempDir()
	origDir := WaitingDir
	WaitingDir = func() string { return dir }
	defer func() { WaitingDir = origDir }()

	for _, sid := range []string{"sess-old", "sess-fresh"} {
		if err := Write(Breadcrumb{SessionID: sid}); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	// A temp file left by a crashed write is retired the same way.
	leftover := filepath.Join(dir, ".tmp-123")
	if err := os.WriteFile(leftover, []byte("{"), 0o644); err != nil {
		t.Fatalf("seed temp file: %v", err)
	}

	staleTime := time.Now().Add(-staleTTL - time.Minute)
	for _, path := range []string{filepath.Join(dir, "sess-old"), leftover} {
		if err := os.Chtimes(path, staleTime, staleTime); err != nil {
			t.Fatalf("Chtimes: %v", err)
		}
	}

	if err := RemoveStale(); err != nil {
		t.Fatalf("RemoveStale: %v", err)
	}

	for _, gone := range []string{"sess-old", ".tmp-123"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("expected %s removed, stat err = %v", gone, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "sess-fresh")); err != nil {
		t.Errorf("expected fresh breadcrumb kept: %v", err)
	}
}

func TestRemoveStale_MissingDir(t *testing.T) {
	origDir := WaitingDir
	WaitingDir = func() string { return filepath.Join(t.TempDir(), "absent") }
	defer func() { WaitingDir = origDir }()

	if err := RemoveStale(); err != nil {
		t.Fatalf("expected nil for a missing directory, got %v", err)
	}
}
