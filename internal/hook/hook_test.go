package hook

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/breadcrumb"
)

// redirectWaitingDir points the breadcrumb directory at a temp dir for the test.
func redirectWaitingDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := breadcrumb.WaitingDir
	breadcrumb.WaitingDir = func() string { return dir }
	t.Cleanup(func() { breadcrumb.WaitingDir = orig })

	sessions := t.TempDir()
	origSessions := SessionsDir
	SessionsDir = func() string { return sessions }
	t.Cleanup(func() { SessionsDir = origSessions })
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

func TestCleanup_RemovesStaleBreadcrumbsOfOtherSessions(t *testing.T) {
	dir := redirectWaitingDir(t)
	for _, sid := range []string{"crashed", "waiting-now"} {
		if err := breadcrumb.Write(breadcrumb.Breadcrumb{SessionID: sid}); err != nil {
			t.Fatalf("seed breadcrumb: %v", err)
		}
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "crashed"), old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	if err := HandleCleanup(strings.NewReader(`{"session_id":"abc"}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "crashed")); !os.IsNotExist(err) {
		t.Errorf("expected stale breadcrumb removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "waiting-now")); err != nil {
		t.Errorf("expected another session's fresh breadcrumb kept: %v", err)
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

// writeSession records a live-session file for "s1" with the given status and
// process id.
func writeSession(t *testing.T, name string, pid int, status string) {
	t.Helper()
	body := fmt.Sprintf(`{"pid":%d,"sessionId":"s1","cwd":"/p","status":%q}`, pid, status)
	if err := os.WriteFile(filepath.Join(SessionsDir(), name+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// deadPID is far above any pid the kernel assigns, so signal 0 finds no process.
const deadPID = 2147483000

func writeBreadcrumb(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(breadcrumb.WaitingDir(), "s1"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestCleanup_RemovesBreadcrumbUnlessSessionStillWaits(t *testing.T) {
	post := `{"session_id":"s1","hook_event_name":"PostToolUse"}`
	postFailure := `{"session_id":"s1","hook_event_name":"PostToolUseFailure"}`
	subagentPost := `{"session_id":"s1","agent_id":"a123","hook_event_name":"PostToolUse"}`
	stop := `{"session_id":"s1","hook_event_name":"Stop"}`
	live := os.Getpid()

	tests := []struct {
		name        string
		sessions    map[string]int // status -> pid, one file each
		breadcrumb  string
		payload     string
		wantRemoved bool
	}{
		{"main outcome, session waiting, keeps", map[string]int{"waiting": live}, `{"session_id":"s1"}`, post, false},
		{"main failure, session waiting, keeps", map[string]int{"waiting": live}, `{"session_id":"s1"}`, postFailure, false},
		{"subagent outcome, session waiting, keeps", map[string]int{"waiting": live}, `{"session_id":"s1"}`, subagentPost, false},
		{"session busy, removes", map[string]int{"busy": live}, `{"session_id":"s1"}`, post, true},
		{"session idle, removes", map[string]int{"idle": live}, `{"session_id":"s1"}`, post, true},
		{"no session file, removes", nil, `{"session_id":"s1"}`, post, true},
		{"waiting entry of a dead process only, removes", map[string]int{"waiting": deadPID}, `{"session_id":"s1"}`, post, true},
		{"unparseable breadcrumb, session busy, removes", map[string]int{"busy": live}, `not json`, post, true},
		{"unparseable breadcrumb, session waiting, keeps", map[string]int{"waiting": live}, `not json`, post, false},
		{"Stop removes while waiting", map[string]int{"waiting": live}, `{"session_id":"s1"}`, stop, true},
		{"unknown event removes while waiting", map[string]int{"waiting": live}, `{"session_id":"s1"}`, `{"session_id":"s1","hook_event_name":"Whatever"}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := redirectWaitingDir(t)
			for status, pid := range tt.sessions {
				writeSession(t, status, pid, status)
			}
			writeBreadcrumb(t, tt.breadcrumb)

			if err := HandleCleanup(strings.NewReader(tt.payload)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			_, err := os.Stat(filepath.Join(dir, "s1"))
			if removed := os.IsNotExist(err); removed != tt.wantRemoved {
				t.Errorf("breadcrumb removed = %v, want %v", removed, tt.wantRemoved)
			}
		})
	}
}

func TestCleanup_DeadWaitingEntryDoesNotShadowLiveBusyOne(t *testing.T) {
	dir := redirectWaitingDir(t)
	writeSession(t, "old", deadPID, "waiting")
	writeSession(t, "new", os.Getpid(), "busy")
	writeBreadcrumb(t, `{"session_id":"s1"}`)

	if err := HandleCleanup(strings.NewReader(`{"session_id":"s1","hook_event_name":"PostToolUse"}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "s1")); !os.IsNotExist(err) {
		t.Errorf("breadcrumb should be removed, stat err = %v", err)
	}
}

func TestCleanup_ToolOutcomeWithoutBreadcrumbReadsNoRegistry(t *testing.T) {
	redirectWaitingDir(t)
	origSessions := SessionsDir
	SessionsDir = func() string {
		t.Error("registry consulted although no breadcrumb exists")
		return ""
	}
	t.Cleanup(func() { SessionsDir = origSessions })

	if err := HandleCleanup(strings.NewReader(`{"session_id":"s1","hook_event_name":"PostToolUse"}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
