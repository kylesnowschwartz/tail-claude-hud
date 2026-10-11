// Package hook handles Claude Code hook events dispatched via the CLI
// subcommand "tail-claude-hud hook <event>".
//
// Each handler reads JSON from stdin (the hook payload), performs a breadcrumb
// operation, and optionally writes a hook-output JSON document to stdout (for
// attention signals such as terminal bells and session titles). Handlers always
// succeed (exit 0) because a hook failure would block Claude Code, and they
// never write to stderr because Claude Code owns the terminal.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/kylesnowschwartz/agent-ouija/claude/claudedir"
	"github.com/kylesnowschwartz/agent-ouija/claude/hooks"
	"github.com/kylesnowschwartz/agent-ouija/claude/registry"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/breadcrumb"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/git"
)

// HandlePermissionRequest reads the hook payload and writes a breadcrumb
// indicating this session is waiting for permission approval. When notifyBell is
// true, it also writes a hook-output document to w containing a
// desktop-notification + bell escape sequence so the user is alerted even when
// the statusline is not visible.
func HandlePermissionRequest(r io.Reader, w io.Writer, notifyBell bool) error {
	p, err := hooks.Decode(r)
	if err != nil {
		return err
	}
	sid := p.EffectiveSessionID()
	if sid == "" {
		return nil // no session ID — can't write a meaningful breadcrumb
	}

	project := projectName(p.CWD)

	if err := breadcrumb.Write(breadcrumb.Breadcrumb{
		SessionID: sid,
		Project:   project,
		ToolName:  p.ToolName,
	}); err != nil {
		return err
	}

	if !notifyBell {
		return nil
	}

	// OSC 9 desktop notification (iTerm2/Konsole/Ghostty) followed by a BEL to
	// ring the audible bell. \a is the terminal bell (0x07); the trailing \a
	// after the OSC 9 string terminator produces the audible alert.
	msg := "Claude Code needs permission"
	if p.ToolName != "" {
		msg = fmt.Sprintf("Claude Code: permission needed for %s", p.ToolName)
	}
	seq := fmt.Sprintf("\x1b]9;%s\a\a", msg)
	return json.NewEncoder(w).Encode(hooks.TerminalSequenceOutput{TerminalSequence: seq})
}

// HandleSessionStart reads the SessionStart payload and, when enabled is true,
// writes a sessionTitle hook output of the form "project · branch". The title is
// only honored by Claude Code on "startup" and "resume" sources, so other
// sources are skipped. Always succeeds.
func HandleSessionStart(r io.Reader, w io.Writer, enabled bool) error {
	p, err := hooks.Decode(r)
	if err != nil {
		return err
	}

	if !enabled {
		return nil
	}
	if p.Source != "" && p.Source != "startup" && p.Source != "resume" {
		return nil
	}

	title := projectName(p.CWD)
	if title == "" {
		return nil
	}
	if st := git.GetStatus(p.CWD); st != nil && st.Branch != "" {
		title += " · " + st.Branch
	}

	return json.NewEncoder(w).Encode(hooks.NewSessionStartOutput(title))
}

// HandleCleanup reads the hook payload and removes the session's breadcrumb,
// then deletes stale breadcrumbs left by any session. Called by PostToolUse,
// PostToolUseFailure, and Stop hooks. Removing a breadcrumb that doesn't exist
// is a no-op.
//
// A tool outcome does not end the dialog that wrote the breadcrumb: a
// background subagent or a parallel call can finish while a dialog is still
// open, and hooks from subagents carry the parent's session ID. So a tool
// outcome removes the breadcrumb only when Claude Code no longer reports the
// session as waiting. Any other event ends the turn and removes it.
func HandleCleanup(r io.Reader) error {
	p, err := hooks.Decode(r)
	if err != nil {
		return err
	}
	if sid := p.EffectiveSessionID(); sid != "" {
		if err := removeBreadcrumb(sid, p.HookEventName); err != nil {
			return err
		}
	}
	return breadcrumb.RemoveStale()
}

// SessionsDir returns the directory of Claude Code's live-session files. It is
// a variable so tests can redirect to a temp directory.
var SessionsDir = func() string {
	root, err := claudedir.DefaultRoot()
	if err != nil {
		return ""
	}
	return root.SessionsDir()
}

// removeBreadcrumb removes the session's breadcrumb, except on a tool outcome
// while Claude Code still reports the session waiting. A declined dialog fires
// no hook, so the session status is the only signal that the dialog is over;
// when it cannot be read the breadcrumb is removed. The registry is read only
// when a breadcrumb exists.
func removeBreadcrumb(sid, event string) error {
	isToolOutcome := event == "PostToolUse" || event == "PostToolUseFailure"
	if isToolOutcome {
		if !breadcrumb.Exists(sid) {
			return nil
		}
		if isSessionWaiting(sid) {
			return nil
		}
	}
	return breadcrumb.Remove(sid)
}

// isSessionWaiting reports whether a running process of session sid records an
// open dialog. Session files outlive their process, so entries of dead
// processes (an earlier run of a resumed session) are skipped.
func isSessionWaiting(sid string) bool {
	dir := SessionsDir()
	if dir == "" {
		return false
	}
	for _, live := range registry.Read(dir) {
		if live.SessionID == sid && live.Alive() {
			return live.Status == "waiting"
		}
	}
	return false
}

// projectName returns the last path component of cwd, or "" when cwd is empty
// or resolves to a filesystem root.
func projectName(cwd string) string {
	project := filepath.Base(cwd)
	if project == "." || project == "/" {
		return ""
	}
	return project
}
