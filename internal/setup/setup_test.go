package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The registration mechanics (exec form, dual-form idempotency, atomic
// write) are tested in the agent-ouija settings package. This suite pins
// only what setup owns: the hook set the HUD registers and the settings
// path it targets.

func TestRegisterHooks_RegistersHudHookSet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	added, err := RegisterHooks()
	if err != nil {
		t.Fatalf("RegisterHooks: %v", err)
	}
	if len(added) != len(hookCommands) {
		t.Errorf("added %d hooks, want %d", len(added), len(hookCommands))
	}

	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	hooksMap, _ := settings["hooks"].(map[string]any)
	for _, h := range hookCommands {
		if _, ok := hooksMap[h.Event]; !ok {
			t.Errorf("event %s not registered", h.Event)
		}
	}
}

func TestRegisterHooks_SecondRunAddsNothing(t *testing.T) {
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
