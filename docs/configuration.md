# Configuration

TOML at `~/.config/tail-claude-hud/config.toml`. Generate defaults with `tail-claude-hud --init`.

## Layout

Each `[[line]]` defines a row of widgets. Widgets render left to right in array order -- reorder the array to change the layout:

```toml
[[line]]
widgets = ["model", "context", "project", "todos", "duration"]

[[line]]
widgets = ["tools"]
mode = "powerline"  # per-line mode override
```

## Style

```toml
[style]
separator = " | "
icons = "nerdfont"     # nerdfont, unicode, ascii
mode = "plain"         # plain, powerline, minimal
theme = "default"
```

## Widget options

```toml
[context]
display = "both"       # text, bar, percent, both
bar_width = 10

[directory]
style = "fish"         # full, fish, basename
levels = 2

[git]
dirty = true
ahead_behind = true
```

## Usage (rate limits)

```toml
[usage]
five_hour_threshold = 0   # show when 5h usage >= this % (0 = always)
seven_day_threshold = 80  # append 7d window when >= this %
cache_ttl_seconds = 180   # how long to cache successful API responses
```

Requires OAuth credentials (macOS Keychain or `~/.claude/.credentials.json`). Only applies to plan subscribers (Pro, Max, Team). Returns empty for API users.

## Thresholds

```toml
[thresholds]
context_warning = 70
context_critical = 85
cost_warning = 5.00
cost_critical = 10.00
```

## Render modes

Three rendering styles, set globally or per-line:

- **`plain`** (default) -- Separator-joined, widgets apply their own ANSI styling.
- **`powerline`** -- Nerd Font arrow transitions between colored background segments. Auto-detects light/dark terminal background.
- **`minimal`** -- Space-separated, foreground color only, no backgrounds.

Modes can be mixed. The `powerline` preset uses powerline for line 1 and plain for the tools activity feed on line 2.

## Themes

Seven built-in color themes: `default`, `dark`, `light`, `nord`, `gruvbox`, `tokyo-night`, `rose-pine`.

```bash
tail-claude-hud --theme nord
```

In powerline mode without an explicit `--theme`, the terminal background is auto-detected (via OSC 11) and the theme switches between `light` and `dark` automatically.

Per-widget color overrides:

```toml
[theme.overrides]
model = { fg = "#ffffff", bg = "#2d2d2d" }
tools = { fg = "#87ceeb", bg = "#2a2a2a" }
```

## Fail-open

Config loading never fails. Missing or corrupt TOML yields defaults. The statusline always renders something.
