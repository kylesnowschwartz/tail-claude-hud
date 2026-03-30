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

	"github.com/kylesnowschwartz/agent-ouija/claude/hooks"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/breadcrumb"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/git"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/heartbeat"
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

// HandleCleanup reads the hook payload and removes any breadcrumb for the
// session. Called by PostToolUse, PostToolUseFailure, and Stop hooks. Removing a
// breadcrumb that doesn't exist is a no-op.
func HandleCleanup(r io.Reader) error {
	p, err := hooks.Decode(r)
	if err != nil {
		return err
	}
	sid := p.EffectiveSessionID()
	if sid == "" {
		return nil
	}
	return breadcrumb.Remove(sid)
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

// HandleHeartbeat reads the hook payload and writes a heartbeat file for the
// session. Called by PreToolUse and PostToolUse hooks to keep the session
// heartbeat fresh.
func HandleHeartbeat(r io.Reader) error {
	p, err := hooks.Decode(r)
	if err != nil {
		return err
	}
	sid := p.EffectiveSessionID()
	if sid == "" {
		return nil
	}
	return heartbeat.Write(heartbeat.Heartbeat{
		SessionID: sid,
		Project:   projectName(p.CWD),
	})
}

// HandleStopCleanup reads the hook payload and removes both the breadcrumb
// and heartbeat for the session. Called by the Stop hook to clean up all
// session markers on exit.
func HandleStopCleanup(r io.Reader) error {
	p, err := hooks.Decode(r)
	if err != nil {
		return err
	}
	sid := p.EffectiveSessionID()
	if sid == "" {
		return nil
	}

	// Remove both markers. Each removal is idempotent.
	breadcrumb.Remove(sid)
	heartbeat.Remove(sid)
	return nil
}
