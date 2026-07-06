package gather

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeRegistryEntry writes a minimal Claude Code session-registry file of
// the shape registry.Read expects ({pid}.json with camelCase fields).
func writeRegistryEntry(t *testing.T, dir, sessionID string, pid int, kind string) {
	t.Helper()
	entry := fmt.Sprintf(`{"pid":%d,"sessionId":%q,"cwd":"/tmp","kind":%q,"status":"idle"}`, pid, sessionID, kind)
	path := filepath.Join(dir, fmt.Sprintf("%d.json", pid))
	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		t.Fatalf("write registry entry: %v", err)
	}
}

func redirectPeersDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := peersSessionsDir
	peersSessionsDir = func() string { return dir }
	t.Cleanup(func() { peersSessionsDir = orig })
	return dir
}

func TestCountPeers_CountsLiveInteractive(t *testing.T) {
	dir := redirectPeersDir(t)

	// This test process's PID is alive; PID 1 (init/launchd) is alive but
	// signaling it fails with EPERM, which Alive treats as dead — use only
	// PIDs this process can signal.
	writeRegistryEntry(t, dir, "sess-other", os.Getpid(), "interactive")

	if got := countPeers("sess-own"); got != 1 {
		t.Fatalf("expected 1 peer, got %d", got)
	}
}

func TestCountPeers_SkipsOwnSession(t *testing.T) {
	dir := redirectPeersDir(t)

	writeRegistryEntry(t, dir, "sess-own", os.Getpid(), "interactive")

	if got := countPeers("sess-own"); got != 0 {
		t.Fatalf("expected 0 (own session excluded), got %d", got)
	}
}

func TestCountPeers_SkipsNonInteractive(t *testing.T) {
	dir := redirectPeersDir(t)

	writeRegistryEntry(t, dir, "sess-sdk", os.Getpid(), "sdk-cli")

	if got := countPeers("sess-own"); got != 0 {
		t.Fatalf("expected 0 (sdk-cli excluded), got %d", got)
	}
}

func TestCountPeers_SkipsDeadProcess(t *testing.T) {
	dir := redirectPeersDir(t)

	// A PID far above any real range: signal-0 fails, entry reads as dead.
	writeRegistryEntry(t, dir, "sess-gone", 1<<30, "interactive")

	if got := countPeers("sess-own"); got != 0 {
		t.Fatalf("expected 0 (dead process), got %d", got)
	}
}

func TestCountPeers_EmptyOrMissingDir(t *testing.T) {
	redirectPeersDir(t)
	if got := countPeers("sess-own"); got != 0 {
		t.Fatalf("expected 0 for empty dir, got %d", got)
	}

	peersSessionsDir = func() string { return "/nonexistent/sessions" }
	if got := countPeers("sess-own"); got != 0 {
		t.Fatalf("expected 0 for missing dir, got %d", got)
	}
}
