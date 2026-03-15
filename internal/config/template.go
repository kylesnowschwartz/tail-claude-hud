package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultTemplate is the default config.toml content written by --init.
// It is valid TOML that LoadHud can parse without error.
const DefaultTemplate = `# tail-claude-hud configuration
# Run 'tail-claude-hud --init' to regenerate this file.

# Status line layout — each [[line]] entry is a rendered row of widgets.
# Widgets are rendered left-to-right in the order listed.
# Available widgets: model, context, directory, env, duration, tools, git, usage
[[line]]
widgets = ["model", "context", "directory", "env", "duration"]

[[line]]
widgets = ["tools"]

# Model widget — shows the active Claude model name.
[model]
# Show the context window size next to the model name.
show_context_size = true

# Context widget — visualises token usage as a progress bar.
[context]
# Width of the bar in characters.
bar_width = 10
# How to display context usage: "percent" or "tokens".
value = "percent"
# Show the input/cache/output token breakdown alongside the bar.
show_breakdown = true

# Directory widget — shows the current working directory.
[directory]
# Number of path components to display (e.g. 1 shows only the last segment).
levels = 1

# Git widget — shows repository state.
[git]
# Show a dirty indicator when there are uncommitted changes.
dirty = true
# Show ahead/behind counts relative to the upstream branch.
ahead_behind = false
# Show per-file change statistics.
file_stats = false

# Style — controls separators, icons, and colour thresholds.
[style]
# String rendered between widgets on each line.
separator = " | "
# Icon set: "nerdfont", "unicode", or "ascii".
icons = "nerdfont"

[style.colors]
# Colour for normal context usage.
context = "green"
# Colour when context usage passes the warning threshold.
warning = "yellow"
# Colour when context usage passes the critical threshold.
critical = "red"

[extra]
# Uncomment to run a shell command and append its output to the statusline.
# command = "my-custom-command"
`

// Init writes the default config template to ~/.config/tail-claude-hud/config.toml.
// It returns an error if the file already exists or if the write fails.
func Init() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	target := filepath.Join(home, ".config", "tail-claude-hud", "config.toml")

	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("config already exists at %s", target)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.WriteFile(target, []byte(DefaultTemplate), 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	fmt.Printf("Created config at %s\n", target)
	return nil
}
