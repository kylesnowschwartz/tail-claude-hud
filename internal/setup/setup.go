// Package setup handles registration of Claude Code hooks in settings.json.
//
// The --init flag calls RegisterHooks() after writing the TOML config.
// It patches ~/.claude/settings.json to register PermissionRequest, PostToolUse,
// and Stop hooks that invoke "tail-claude-hud hook <event>".
//
// Registration is idempotent: existing hooks with the same command are skipped.
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const binaryName = "tail-claude-hud"

// hookSpec defines one hook event and the subcommand it invokes.
type hookSpec struct {
	Event      string
	Subcommand string
}

var hooks = []hookSpec{
	{Event: "PermissionRequest", Subcommand: "permission-request"},
	{Event: "PreToolUse", Subcommand: "heartbeat"},
	{Event: "PostToolUse", Subcommand: "cleanup"},
	{Event: "PostToolUse", Subcommand: "heartbeat"},
	{Event: "Stop", Subcommand: "stop-cleanup"},
}

// RegisterHooks patches ~/.claude/settings.json to register the permission
// detection hooks. Returns a list of events that were newly registered.
// Returns nil, nil when all hooks are already present.
func RegisterHooks() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	settings, err := readSettings(settingsPath)
	if err != nil {
		return nil, err
	}

	hooksMap := ensureHooksMap(settings)

	var added []string
	for _, h := range hooks {
		command := fmt.Sprintf("%s hook %s", binaryName, h.Subcommand)
		if hasHookCommand(hooksMap, h.Event, command) {
			continue
		}
		appendHook(hooksMap, h.Event, command)
		added = append(added, h.Event)
	}

	if len(added) == 0 {
		return nil, nil
	}

	if err := writeSettings(settingsPath, settings); err != nil {
		return nil, err
	}
	return added, nil
}

// readSettings reads and parses settings.json. Returns an empty map if the
// file doesn't exist; returns an error for other read/parse failures.
func readSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return settings, nil
}

// ensureHooksMap returns the "hooks" sub-map from settings, creating it if absent.
func ensureHooksMap(settings map[string]any) map[string]any {
	v, ok := settings["hooks"]
	if !ok {
		m := make(map[string]any)
		settings["hooks"] = m
		return m
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	// Unexpected type — overwrite with an empty map.
	m := make(map[string]any)
	settings["hooks"] = m
	return m
}

// hasHookCommand checks whether the hooks map already contains a matching
// command for the given event. The Claude Code hooks schema is:
//
//	"EventName": [{"hooks": [{"type": "command", "command": "..."}]}]
func hasHookCommand(hooksMap map[string]any, event, command string) bool {
	arr, ok := hooksMap[event]
	if !ok {
		return false
	}
	entries, ok := arr.([]any)
	if !ok {
		return false
	}
	for _, entry := range entries {
		em, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		innerArr, ok := em["hooks"]
		if !ok {
			continue
		}
		innerHooks, ok := innerArr.([]any)
		if !ok {
			continue
		}
		for _, ih := range innerHooks {
			hm, ok := ih.(map[string]any)
			if !ok {
				continue
			}
			if cmd, ok := hm["command"].(string); ok && cmd == command {
				return true
			}
		}
	}
	return false
}

// appendHook adds a new hook entry for the given event.
func appendHook(hooksMap map[string]any, event, command string) {
	entry := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	}

	existing, ok := hooksMap[event]
	if !ok {
		hooksMap[event] = []any{entry}
		return
	}
	if arr, ok := existing.([]any); ok {
		hooksMap[event] = append(arr, entry)
	} else {
		hooksMap[event] = []any{entry}
	}
}

// writeSettings writes the settings map back to disk with readable formatting.
func writeSettings(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
