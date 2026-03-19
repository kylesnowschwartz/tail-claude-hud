# tail-claude-hud

A terminal statusline for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions. Built with Go.

Reads JSON from Claude Code's stdin pipe on every tick, parses transcript state, gathers git/env data, and renders a styled multi-line statusline. The full cycle completes in single-digit milliseconds.

![tail-claude-hud demo](demo.gif)

## Requirements

- Go 1.25+
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) with statusline support
- A [Nerd Font](https://www.nerdfonts.com/) (optional, for icons in nerdfont/powerline modes)

## Install

```bash
go install github.com/kylesnowschwartz/tail-claude-hud/cmd/tail-claude-hud@latest
```

Or build from source:

```bash
git clone git@github.com:kylesnowschwartz/tail-claude-hud.git
cd tail-claude-hud
just build
```

## Update

```bash
go install github.com/kylesnowschwartz/tail-claude-hud/cmd/tail-claude-hud@latest
```

## Setup

Generate a default config and point Claude Code at the binary:

```bash
# Generate config
tail-claude-hud --init
```

This creates `~/.config/tail-claude-hud/config.toml`.

Add the statusline to your Claude Code settings (`~/.claude/settings.json`):

```json
{
  "statusline": {
    "command": "tail-claude-hud"
  }
}
```

If `$GOBIN` (typically `~/go/bin`) isn't on your `PATH`, use the full path:

```json
{
  "statusline": {
    "command": "~/go/bin/tail-claude-hud"
  }
}
```

## Quick start with presets

Presets are complete configurations you can apply without editing TOML. Five are built in:

| Preset | Lines | Description |
|---|---|---|
| `default` | 3 | Model, context, project, todos, duration / tools / agents |
| `compact` | 1 | Model, context, cost, duration |
| `detailed` | 4 | Everything: tokens, speed, messages, lines added/removed |
| `powerline` | 2 | Arrow segments with auto-detected light/dark theme |
| `minimal` | 1 | Model, context, duration. Space-separated, no backgrounds |

```bash
tail-claude-hud --preset powerline
tail-claude-hud --preset detailed
```

Custom presets go in `~/.config/tail-claude-hud/presets/*.toml` and are referenced by filename (without `.toml`).

## Widgets

18 widgets, each a pure function that returns a styled string or `""` when it has nothing to show.

**Session data** (from Claude Code's stdin JSON):

| Widget | Shows |
|---|---|
| `model` | Model name, optional context window size |
| `context` | Context usage as text, bar, percent, or both. Color-coded by threshold |
| `cost` | Session cost in USD, color-coded by threshold |
| `tokens` | Token count and cache hit ratio |
| `duration` | Elapsed session time |
| `lines` | Lines added/removed (green +N, red -N) |
| `speed` | Rolling tokens/sec over a configurable window |
| `messages` | Conversation turn count |
| `session` | Session name |
| `outputstyle` | Active output style name |

**Transcript data** (parsed incrementally from the JSONL transcript):

| Widget | Shows |
|---|---|
| `tools` | Running/completed tool invocations as a scrolling activity feed |
| `agents` | Sub-agents with elapsed time (running) or duration (completed) |
| `todos` | Task completion count, color-coded |
| `thinking` | Thinking block indicator and count |
| `skills` | Skill names invoked during the session |

**Environment** (gathered from the filesystem):

| Widget | Shows |
|---|---|
| `directory` | Working directory (full, fish-style abbreviation, or basename) |
| `git` | Branch, dirty indicator, ahead/behind counts |
| `project` | Composite of directory + git in a single segment |
| `env` | MCP servers, CLAUDE.md files, rule files, hooks (e.g. "3M 2C 4R 3H") |

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

Per-widget color overrides are supported in config:

```toml
[theme.overrides]
model = { fg = "#ffffff", bg = "#2d2d2d" }
tools = { fg = "#87ceeb", bg = "#2a2a2a" }
```

## Configuration

TOML at `~/.config/tail-claude-hud/config.toml`. Each `[[line]]` defines a row of widgets:

```toml
[[line]]
widgets = ["model", "context", "project", "todos", "duration"]

[[line]]
widgets = ["tools"]
mode = "powerline"  # per-line mode override

[style]
separator = " | "
icons = "nerdfont"     # nerdfont, unicode, ascii
mode = "plain"         # plain, powerline, minimal
theme = "default"

[context]
display = "both"       # text, bar, percent, both
bar_width = 10

[directory]
style = "fish"         # full, fish, basename
levels = 2

[git]
dirty = true
ahead_behind = true

[thresholds]
context_warning = 70
context_critical = 85
cost_warning = 5.00
cost_critical = 10.00
```

Config loading never fails. Missing or corrupt TOML yields defaults.

## CLI

```
tail-claude-hud [flags]
  --init           Generate default config file
  --preset NAME    Apply a built-in or custom preset
  --theme NAME     Override color theme
  --list-presets   Print available preset names
  --dump-current   Render from the current session's transcript snapshot
  --preview PATH   Render from a transcript file with mock stdin data
  --watch          Continuously re-render on transcript changes (with --preview)
```

`--dump-current` auto-discovers the most recent `.jsonl` transcript for the current directory. Useful for testing outside a live session.

`--preview` with `--watch` polls the transcript every 500ms and re-renders on change, for live development iteration.

## Development

Requires [just](https://github.com/casey/just) for task running.

```bash
just              # run tests
just build        # go build -o bin/tail-claude-hud ./cmd/tail-claude-hud
just test         # go test ./... -count=1
just test-race    # race detector
just bench        # benchmarks
just check        # fmt + vet + test
just dump         # build + render from current session
just run-sample   # pipe testdata through the binary
```

Run a single test:

```bash
go test ./internal/transcript/ -run TestExtractContentBlocks -count=1
```

## Architecture

Four-stage linear pipeline, each stage a separate package with no backward dependencies:

```
stdin -> gather -> render -> stdout
```

1. **stdin** -- Decode JSON from Claude Code, persist snapshot to disk.
2. **gather** -- Spawn goroutines only for data sources active widgets need.
3. **render** -- Walk configured lines, call each widget's render function, ANSI-truncate to terminal width.
4. **widget** -- Pure functions: `(RenderContext, Config) -> WidgetResult`.

Transcript processing reads incrementally (O(delta) not O(n)) by tracking byte offsets per file. State survives process restarts via disk snapshots.

## Related

- [tail-claude](https://github.com/kylesnowschwartz/tail-claude) -- Terminal TUI for reading Claude Code session logs

## License

[MIT](LICENSE)
