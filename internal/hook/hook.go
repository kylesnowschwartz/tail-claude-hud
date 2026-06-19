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
	"os"
	"path/filepath"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/breadcrumb"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/git"
)

// payload is the common subset of fields from Claude Code's hook stdin JSON.
type payload struct {
	SessionID string `json:"session_id"`
	CWD       string `json:"cwd"`
	ToolName  string `json:"tool_name"`
	Source    string `json:"source"` // SessionStart: startup|resume|clear|compact
}

// sessionID returns the payload's session_id, falling back to the
// CLAUDE_CODE_SESSION_ID environment variable when the payload omits it.
// Without this fallback, hook invocations with an empty session_id silently
// no-op instead of writing or removing a breadcrumb.
func (p payload) sessionID() string {
	if p.SessionID != "" {
		return p.SessionID
	}
	return os.Getenv("CLAUDE_CODE_SESSION_ID")
}

// permissionOutput is the hook-output JSON shape used to emit a terminal escape
// sequence. terminalSequence is a top-level hook-output field (CC 2.1.141) that
// Claude Code writes directly to the terminal.
type permissionOutput struct {
	TerminalSequence string `json:"terminalSequence"`
}

// sessionStartOutput is the SessionStart hook-output JSON shape. sessionTitle
// (CC 2.1.152) sets the terminal session title; it is honored only when the
// session source is "startup" or "resume".
type sessionStartOutput struct {
	HookSpecificOutput struct {
		HookEventName string `json:"hookEventName"`
		SessionTitle  string `json:"sessionTitle"`
	} `json:"hookSpecificOutput"`
}

// HandlePermissionRequest reads the hook payload and writes a breadcrumb
// indicating this session is waiting for permission approval. When notifyBell is
// true, it also writes a hook-output document to w containing a
// desktop-notification + bell escape sequence so the user is alerted even when
// the statusline is not visible.
func HandlePermissionRequest(r io.Reader, w io.Writer, notifyBell bool) error {
	var p payload
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return err
	}
	sid := p.sessionID()
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
	return json.NewEncoder(w).Encode(permissionOutput{TerminalSequence: seq})
}

// HandleSessionStart reads the SessionStart payload and, when enabled is true,
// writes a sessionTitle hook output of the form "project · branch". The title is
// only honored by Claude Code on "startup" and "resume" sources, so other
// sources are skipped. Always succeeds.
func HandleSessionStart(r io.Reader, w io.Writer, enabled bool) error {
	var p payload
	if err := json.NewDecoder(r).Decode(&p); err != nil {
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

	var out sessionStartOutput
	out.HookSpecificOutput.HookEventName = "SessionStart"
	out.HookSpecificOutput.SessionTitle = title
	return json.NewEncoder(w).Encode(out)
}

// HandleCleanup reads the hook payload and removes any breadcrumb for the
// session. Called by PostToolUse, PostToolUseFailure, and Stop hooks. Removing a
// breadcrumb that doesn't exist is a no-op.
func HandleCleanup(r io.Reader) error {
	var p payload
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return err
	}
	sid := p.sessionID()
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
