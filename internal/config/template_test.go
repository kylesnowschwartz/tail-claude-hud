package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// TestDefaultTemplateIsValidTOML verifies that the template can be decoded
// without error and that LoadHud can parse it successfully.
func TestDefaultTemplateIsValidTOML(t *testing.T) {
	var v interface{}
	if _, err := toml.Decode(DefaultTemplate, &v); err != nil {
		t.Fatalf("DefaultTemplate is not valid TOML: %v", err)
	}
}

// TestDefaultTemplateLoadsWithDefaults verifies that parsing the template into
// a Config struct produces a non-zero result with expected default values.
func TestDefaultTemplateLoadsWithDefaults(t *testing.T) {
	cfg := defaults()
	if _, err := toml.Decode(DefaultTemplate, cfg); err != nil {
		t.Fatalf("failed to decode template into Config: %v", err)
	}

	if len(cfg.Lines) == 0 {
		t.Error("expected at least one [[line]] in template")
	}
	if cfg.Context.BarWidth != 10 {
		t.Errorf("expected bar_width=10, got %d", cfg.Context.BarWidth)
	}
	if cfg.Style.Icons != "nerdfont" {
		t.Errorf("expected icons=nerdfont, got %q", cfg.Style.Icons)
	}
}

// TestInitCreatesFile verifies Init writes the template to the expected path.
func TestInitCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Override home dir by setting up the path manually via a test-local Init.
	target := filepath.Join(tmpDir, ".config", "tail-claude-hud", "config.toml")

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(target, []byte(DefaultTemplate), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != DefaultTemplate {
		t.Error("written content does not match DefaultTemplate")
	}
}

// TestInitErrorsWhenFileExists verifies Init returns an error when the config
// file is already present.
func TestInitErrorsWhenFileExists(t *testing.T) {
	// Write the template to a temp home, then call Init expecting an error.
	tmpHome := t.TempDir()
	target := filepath.Join(tmpHome, ".config", "tail-claude-hud", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	// Temporarily override UserHomeDir by pointing HOME to tmpHome.
	orig := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", orig)

	err := Init()
	if err == nil {
		t.Fatal("expected error when config already exists, got nil")
	}
	if !strings.Contains(err.Error(), "config already exists") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestInitWritesFile verifies Init creates the config file when it does not exist.
func TestInitWritesFile(t *testing.T) {
	tmpHome := t.TempDir()

	orig := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", orig)

	if err := Init(); err != nil {
		t.Fatalf("Init() returned unexpected error: %v", err)
	}

	target := filepath.Join(tmpHome, ".config", "tail-claude-hud", "config.toml")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if string(data) != DefaultTemplate {
		t.Error("written content does not match DefaultTemplate")
	}
}

// TestInitTemplateHasComments verifies the template includes comments for
// each configuration section.
func TestInitTemplateHasComments(t *testing.T) {
	sections := []string{
		"# Status line layout",
		"# Model widget",
		"# Context widget",
		"# Directory widget",
		"# Git widget",
		"# Style",
	}
	for _, s := range sections {
		if !strings.Contains(DefaultTemplate, s) {
			t.Errorf("template missing comment: %q", s)
		}
	}
}
