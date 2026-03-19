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
#   Line 2 (per-agent activity): agents  — ephemeral, hides when empty
#   Line 3 (tools):              tools   — ephemeral, hides when empty
#
# All available widgets:
#   model       — active Claude model name
#   context     — token usage progress bar
#   cost        — session cost in USD; color shifts at cost warning/critical thresholds
#   project     — project name with optional git ahead/behind count
#   todos       — count of active TodoWrite todos
#   duration    — elapsed session time
#   agents      — per-agent activity feed (requires transcript)
#   tools       — recent tool use feed, including thinking blocks (requires transcript)
#   thinking    — thinking-block indicator; yellow while active, dim with count when done (requires transcript)
#   tokens      — per-call token count and cache hit ratio (requires transcript)
#   lines       — lines added/removed this session
#   messages    — conversational turn count (requires transcript)
#   skills      — skills invoked this session, newest-first (requires transcript)
#   session     — session name (requires transcript)
#   outputstyle — current output style name (e.g. "concise")
#   git         — full repository state (dirty, ahead/behind, file stats)
#   directory   — current working directory path
#   env         — environment variable counts (opt-in)
#   speed       — rolling tokens/sec average (requires transcript)
#   permission  — red alert when another Claude session is waiting for permission
#
# Each [[line]] can optionally override the global style.mode:
#   mode = "powerline"  — use powerline arrows for this line only
[[line]]
widgets = ["model", "context", "project", "todos", "duration"]

[[line]]
widgets = ["agents"]

[[line]]
widgets = ["tools"]

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

# Style — controls separators, icons, colour thresholds, and rendering mode.
[style]
# String rendered between widgets on each line (used in plain mode).
separator = " | "
# Icon set: "nerdfont", "unicode", or "ascii".
icons = "nerdfont"
# Color depth: "auto" detects from the environment (COLORTERM, TERM_PROGRAM, TERM).
# Override with "truecolor", "256", or "basic" to force a specific level.
color_level = "auto"
# Color theme: "default", "dark", "nord", "gruvbox", "tokyo-night", "rose-pine".
# The theme sets fg/bg colors for each widget. Use [theme.overrides] to
# override individual widget colors on top of the selected theme.
theme = "default"
# Segment decoration mode for all lines (individual lines can override with mode = "..."):
#   plain     — separator-joined widgets (default)
#   powerline — arrow transitions between segments (requires a Nerd Font terminal)
#   minimal   — space-separated, fg color only, no decorators
mode = "plain"

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

# Theme overrides — per-widget color customisation on top of style.theme.
# Each key is a widget name. fg and bg are string values that accept:
#   - Hex:        "#rrggbb"  e.g. fg = "#ff8800"
#   - 256-color:  "N"        e.g. fg = "114"  (ANSI 256-color index)
#   - Named ANSI: name       e.g. fg = "green", fg = "cyan", fg = "red"
#
# Omitting a field sets it to empty (terminal default). Omitting both fg and bg
# for a widget keeps the entry in the table but applies no colors.
# To keep a theme's bg while changing only fg, copy the theme's bg value.
#
# Examples:
#   [theme.overrides.git]
#   fg = "#ffffff"        # hex fg; bg uses terminal default
#   bg = "#0a1628"        # hex bg
#
#   [theme.overrides.model]
#   fg = "#ff8800"        # fg-only hex override (bg becomes empty)
#
#   [theme.overrides.duration]
#   bg = "235"            # bg-only 256-color override (fg becomes empty)
#
#   [theme.overrides.context]
#   fg = "cyan"           # named ANSI fg
#   bg = "#1a1a2e"        # hex bg
#
# [theme.overrides]

# Permission widget — shows when another Claude session is waiting for approval.
[permission]
# Show the project name of the waiting session next to the icon.
# When false, only the icon is shown.
show_project = true

[extra]
# Uncomment to run a shell command and append its output to the statusline.
# The command must print a JSON object with a "label" field: {"label": "my text"}
# ANSI SGR color codes in the label are preserved; other escape sequences are stripped.
# The command is run with a 3-second timeout; empty string or errors produce no output.
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
