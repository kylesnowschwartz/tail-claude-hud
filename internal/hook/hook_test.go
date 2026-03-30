package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/breadcrumb"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/heartbeat"
)

func TestHandleHeartbeat_WritesFile(t *testing.T) {
	dir := t.TempDir()
	origDir := heartbeat.SessionsDir
	heartbeat.SessionsDir = func() string { return dir }
	defer func() { heartbeat.SessionsDir = origDir }()

	input := `{"session_id":"sess-abc","cwd":"/home/user/my-project"}`
	err := HandleHeartbeat(strings.NewReader(input))
	if err != nil {
		t.Fatalf("HandleHeartbeat: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "sess-abc")); err != nil {
		t.Fatalf("heartbeat file not found: %v", err)
	}
}

func TestHandleHeartbeat_EmptySessionID(t *testing.T) {
	dir := t.TempDir()
	origDir := heartbeat.SessionsDir
	heartbeat.SessionsDir = func() string { return dir }
	defer func() { heartbeat.SessionsDir = origDir }()

	input := `{"session_id":"","cwd":"/home/user/proj"}`
	err := HandleHeartbeat(strings.NewReader(input))
	if err != nil {
		t.Fatalf("HandleHeartbeat: %v", err)
	}

	// No file should be created.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("expected no files, got %d", len(entries))
	}
}

func TestHandleStopCleanup_RemovesBoth(t *testing.T) {
	bDir := t.TempDir()
	hDir := t.TempDir()
	origBDir := breadcrumb.WaitingDir
	origHDir := heartbeat.SessionsDir
	breadcrumb.WaitingDir = func() string { return bDir }
	heartbeat.SessionsDir = func() string { return hDir }
	defer func() {
		breadcrumb.WaitingDir = origBDir
		heartbeat.SessionsDir = origHDir
	}()

	// Create both a breadcrumb and a heartbeat.
	breadcrumb.Write(breadcrumb.Breadcrumb{SessionID: "sess-xyz", Project: "proj"})
	heartbeat.Write(heartbeat.Heartbeat{SessionID: "sess-xyz", Project: "proj"})

	input := `{"session_id":"sess-xyz","cwd":"/home/user/proj"}`
	err := HandleStopCleanup(strings.NewReader(input))
	if err != nil {
		t.Fatalf("HandleStopCleanup: %v", err)
	}

	if _, err := os.Stat(filepath.Join(bDir, "sess-xyz")); !os.IsNotExist(err) {
		t.Fatal("breadcrumb should be removed")
	}
	if _, err := os.Stat(filepath.Join(hDir, "sess-xyz")); !os.IsNotExist(err) {
		t.Fatal("heartbeat should be removed")
	}
}

func TestHandleStopCleanup_EmptySessionID(t *testing.T) {
	input := `{"session_id":"","cwd":"/home/user/proj"}`
	err := HandleStopCleanup(strings.NewReader(input))
	if err != nil {
		t.Fatalf("HandleStopCleanup: %v", err)
	}
}
