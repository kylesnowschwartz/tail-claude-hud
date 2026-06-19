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
	{Event: "PostToolUse", Subcommand: "cleanup"},
	{Event: "PostToolUseFailure", Subcommand: "cleanup"},
	{Event: "Stop", Subcommand: "cleanup"},
	{Event: "SessionStart", Subcommand: "session-start"},
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
		if hasHookCommand(hooksMap, h.Event, h.Subcommand) {
			continue
		}
		appendHook(hooksMap, h.Event, h.Subcommand)
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

// hasHookCommand checks whether the hooks map already contains a matching hook
// for the given event and subcommand. The Claude Code hooks schema is:
//
//	"EventName": [{"hooks": [{"type": "command", "command": "...", "args": [...]}]}]
//
// Both the legacy shell-string form ("tail-claude-hud hook <subcommand>") and the
// exec form (command "tail-claude-hud" with args ["hook", "<subcommand>"]) count
// as a match, so re-running --init after an upgrade does not duplicate hooks.
func hasHookCommand(hooksMap map[string]any, event, subcommand string) bool {
	arr, ok := hooksMap[event]
	if !ok {
		return false
	}
	entries, ok := arr.([]any)
	if !ok {
		return false
	}
	legacy := fmt.Sprintf("%s hook %s", binaryName, subcommand)
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
			cmd, _ := hm["command"].(string)
			if cmd == legacy {
				return true // legacy shell-string form
			}
			if cmd == binaryName && argsMatch(hm["args"], subcommand) {
				return true // exec form
			}
		}
	}
	return false
}

// argsMatch reports whether a decoded JSON args value equals ["hook", subcommand].
func argsMatch(v any, subcommand string) bool {
	arr, ok := v.([]any)
	if !ok || len(arr) != 2 {
		return false
	}
	first, _ := arr[0].(string)
	second, _ := arr[1].(string)
	return first == "hook" && second == subcommand
}

// appendHook adds a new exec-form hook entry for the given event. Exec form
// (command + args array) avoids shell interpolation of the subcommand.
func appendHook(hooksMap map[string]any, event, subcommand string) {
	entry := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": binaryName,
				"args":    []any{"hook", subcommand},
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
