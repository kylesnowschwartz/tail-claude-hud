package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/breadcrumb"
)

// redirectWaitingDir points the breadcrumb directory at a temp dir for the test.
func redirectWaitingDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := breadcrumb.WaitingDir
	breadcrumb.WaitingDir = func() string { return dir }
	t.Cleanup(func() { breadcrumb.WaitingDir = orig })
	return dir
}

func TestPermissionRequest_WritesBreadcrumb(t *testing.T) {
	dir := redirectWaitingDir(t)
	in := strings.NewReader(`{"session_id":"abc","cwd":"/home/me/proj","tool_name":"Bash"}`)
	var out bytes.Buffer

	if err := HandlePermissionRequest(in, &out, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "abc")); err != nil {
		t.Errorf("expected breadcrumb file for session abc: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output when notifyBell is false, got %q", out.String())
	}
}

func TestPermissionRequest_EmitsTerminalSequenceWhenEnabled(t *testing.T) {
	redirectWaitingDir(t)
	in := strings.NewReader(`{"session_id":"abc","cwd":"/home/me/proj","tool_name":"Bash"}`)
	var out bytes.Buffer

	if err := HandlePermissionRequest(in, &out, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "terminalSequence") {
		t.Errorf("expected terminalSequence in output, got %q", s)
	}
	if !strings.Contains(s, "Bash") {
		t.Errorf("expected tool name in notification, got %q", s)
	}
}

func TestPermissionRequest_SessionIDEnvFallback(t *testing.T) {
	dir := redirectWaitingDir(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "from-env")
	in := strings.NewReader(`{"cwd":"/home/me/proj","tool_name":"Bash"}`) // no session_id
	var out bytes.Buffer

	if err := HandlePermissionRequest(in, &out, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "from-env")); err != nil {
		t.Errorf("expected breadcrumb keyed by env session id: %v", err)
	}
}

func TestCleanup_RemovesBreadcrumb(t *testing.T) {
	dir := redirectWaitingDir(t)
	if err := breadcrumb.Write(breadcrumb.Breadcrumb{SessionID: "abc"}); err != nil {
		t.Fatalf("seed breadcrumb: %v", err)
	}

	in := strings.NewReader(`{"session_id":"abc"}`)
	if err := HandleCleanup(in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "abc")); !os.IsNotExist(err) {
		t.Errorf("expected breadcrumb removed, stat err = %v", err)
	}
}

func TestSessionStart_DisabledEmitsNothing(t *testing.T) {
	in := strings.NewReader(`{"cwd":"/home/me/proj","source":"startup"}`)
	var out bytes.Buffer
	if err := HandleSessionStart(in, &out, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output when disabled, got %q", out.String())
	}
}

func TestSessionStart_SkipsNonStartupSource(t *testing.T) {
	in := strings.NewReader(`{"cwd":"/home/me/proj","source":"clear"}`)
	var out bytes.Buffer
	if err := HandleSessionStart(in, &out, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output on clear source, got %q", out.String())
	}
}

func TestSessionStart_EmitsTitleOnStartup(t *testing.T) {
	in := strings.NewReader(`{"cwd":"/home/me/my-proj","source":"startup"}`)
	var out bytes.Buffer
	if err := HandleSessionStart(in, &out, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "sessionTitle") || !strings.Contains(s, "my-proj") {
		t.Errorf("expected sessionTitle with project name, got %q", s)
	}
	if !strings.Contains(s, "SessionStart") {
		t.Errorf("expected hookEventName SessionStart, got %q", s)
	}
}
