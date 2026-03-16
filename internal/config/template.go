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
#
# Default layout (C+D hybrid):
#   Line 1 (identity + health): model, context, project, todos, duration
#   Line 2 (per-agent activity): agents          — ephemeral, hides when empty
#   Line 3 (thinking + tools):  thinking, tools  — ephemeral, hides when empty
#
# All available widgets:
#   model     — active Claude model name
#   context   — token usage progress bar
#   project   — project name with optional git ahead/behind count
#   todos     — count of active TodoWrite todos
#   duration  — elapsed session time
#   agents    — per-agent activity feed (requires transcript)
#   thinking  — current thinking block excerpt (requires transcript)
#   tools     — recent tool use feed (requires transcript)
#   git       — full repository state (dirty, ahead/behind, file stats)
#   directory — current working directory path
#   env       — environment variable counts (opt-in)
#   speed     — rolling tokens/sec average (requires transcript)
[[line]]
widgets = ["model", "context", "project", "todos", "duration"]

[[line]]
widgets = ["agents"]

[[line]]
widgets = ["thinking", "tools"]

# Model widget — shows the active Claude model name.
[model]
# Show the context window size next to the model name.
show_context_size = true

# Context widget — visualises token usage as a progress bar.
[context]
# Width of the bar in characters (used when display is "bar" or "both").
bar_width = 10
# How to render context: "text" (default), "bar", or "both".
display = "text"
# How to display the text value: "percent" (default), "tokens", or "remaining".
value = "percent"
# Show the input/cache/output token breakdown alongside the bar.
show_breakdown = true

# Directory widget — shows the current working directory.
[directory]
# Number of path components to display (e.g. 1 shows only the last segment).
levels = 1
# How to display the path: "full" (default), "fish" (abbreviate intermediate
# segments to first character: ~/Code/my-projects/tail → ~/C/m/tail),
# or "basename" (last segment only, ignores levels).
style = "full"

# Git widget — shows repository state. Also controls project widget's git data.
[git]
# Show a dirty indicator when there are uncommitted changes.
dirty = true
# Show ahead/behind counts relative to the upstream branch.
# Enabled by default so the project widget can display ahead/behind counts.
ahead_behind = true
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

# Speed widget — shows rolling tokens/sec average.
[speed]
# Rolling window in seconds. Only tokens from the last N seconds are counted.
# Set to 0 to use the session average.
window_secs = 30

# Thresholds — controls when widget colors shift from normal to warning to critical.
[thresholds]
# Context usage percentage at which the context widget shifts to warning color.
context_warning = 70
# Context usage percentage at which the context widget shifts to critical color.
context_critical = 85
# Session cost in USD at which the cost widget shifts to warning color.
cost_warning = 5.00
# Session cost in USD at which the cost widget shifts to critical color.
cost_critical = 10.00

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
