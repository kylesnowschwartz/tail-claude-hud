package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// readHooksMap loads the hooks map from the settings.json under the given home.
func readHooksMap(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	m, _ := settings["hooks"].(map[string]any)
	return m
}

func TestRegisterHooks_FreshExecForm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	added, err := RegisterHooks()
	if err != nil {
		t.Fatalf("RegisterHooks: %v", err)
	}
	if len(added) != len(hooks) {
		t.Errorf("added %d hooks, want %d", len(added), len(hooks))
	}

	hooksMap := readHooksMap(t, home)
	for _, h := range hooks {
		if !hasHookCommand(hooksMap, h.Event, h.Subcommand) {
			t.Errorf("hook %s/%s not registered", h.Event, h.Subcommand)
		}
	}

	// Verify the exec form was written (command + args), not a shell string.
	entries := hooksMap["PermissionRequest"].([]any)
	inner := entries[0].(map[string]any)["hooks"].([]any)
	hm := inner[0].(map[string]any)
	if hm["command"] != binaryName {
		t.Errorf("command = %v, want %q", hm["command"], binaryName)
	}
	if !argsMatch(hm["args"], "permission-request") {
		t.Errorf("args = %v, want [hook permission-request]", hm["args"])
	}
}

func TestRegisterHooks_Idempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := RegisterHooks(); err != nil {
		t.Fatalf("first RegisterHooks: %v", err)
	}
	added, err := RegisterHooks()
	if err != nil {
		t.Fatalf("second RegisterHooks: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("second run added %v, want none", added)
	}
}

func TestRegisterHooks_DetectsLegacyShellForm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Seed settings with a legacy shell-string PermissionRequest hook.
	settingsDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := map[string]any{
		"hooks": map[string]any{
			"PermissionRequest": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": binaryName + " hook permission-request"},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	added, err := RegisterHooks()
	if err != nil {
		t.Fatalf("RegisterHooks: %v", err)
	}
	for _, ev := range added {
		if ev == "PermissionRequest" {
			t.Error("legacy PermissionRequest hook was duplicated")
		}
	}
}
