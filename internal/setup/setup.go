// Package setup handles registration of Claude Code hooks in settings.json.
//
// The --init flag calls RegisterHooks() after writing the TOML config.
// It patches ~/.claude/settings.json to register PermissionRequest, PostToolUse,
// and Stop hooks that invoke "tail-claude-hud hook <event>".
//
// The registration mechanics (dual-form idempotency, atomic write) live in
// the agent-ouija settings package; this package owns only WHICH hooks the
// HUD registers.
package setup

import (
	"fmt"

	"github.com/kylesnowschwartz/agent-ouija/claude/claudedir"
	"github.com/kylesnowschwartz/agent-ouija/claude/settings"
)

const binaryName = "tail-claude-hud"

// hookCommands lists the hook events the HUD needs and the subcommand each
// invokes, in exec form (command + args) to avoid shell interpolation.
var hookCommands = []settings.HookCommand{
	{Event: "PermissionRequest", Command: binaryName, Args: []string{"hook", "permission-request"}},
	{Event: "PostToolUse", Command: binaryName, Args: []string{"hook", "cleanup"}},
	{Event: "PostToolUseFailure", Command: binaryName, Args: []string{"hook", "cleanup"}},
	{Event: "Stop", Command: binaryName, Args: []string{"hook", "cleanup"}},
	{Event: "SessionStart", Command: binaryName, Args: []string{"hook", "session-start"}},
}

// RegisterHooks patches ~/.claude/settings.json to register the permission
// detection hooks. Returns a list of events that were newly registered.
// Returns nil, nil when all hooks are already present.
func RegisterHooks() ([]string, error) {
	root, err := claudedir.DefaultRoot()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return settings.RegisterHooks(root.SettingsPath(), hookCommands)
}
