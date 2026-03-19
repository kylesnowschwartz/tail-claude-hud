# tail-claude-hud

A terminal statusline for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions. Shows model, context usage, tools, agents, todos, git status, and more -- updated on every tick.

![tail-claude-hud demo](demo.gif)

## Install

Requires Go 1.25+.

```bash
go install github.com/kylesnowschwartz/tail-claude-hud/cmd/tail-claude-hud@latest
```

To update, run the same command. To build from source:

```bash
git clone git@github.com:kylesnowschwartz/tail-claude-hud.git
cd tail-claude-hud
just build
```

## Setup

Generate a config and add the statusline to Claude Code:

```bash
tail-claude-hud --init
```

Then add to `~/.claude/settings.json`:

```json
{
  "statusline": {
    "command": "tail-claude-hud"
  }
}
```

## Presets

Five built-in presets. Apply one without editing TOML:

| Preset | Lines | Description |
|---|---|---|
| `default` | 3 | Model, context, project, todos, duration / tools / agents |
| `compact` | 1 | Model, context, cost, duration |
| `detailed` | 4 | Everything: tokens, speed, messages, lines added/removed |
| `powerline` | 2 | Arrow segments with auto-detected light/dark theme |
| `minimal` | 1 | Model, context, duration. Space-separated, no backgrounds |

```bash
tail-claude-hud --preset powerline
```

Custom presets go in `~/.config/tail-claude-hud/presets/*.toml`.

## Documentation

- [Widgets](docs/widgets.md) -- 18 available widgets and what they display
- [Configuration](docs/configuration.md) -- TOML reference, render modes, themes
- [CLI](docs/cli.md) -- Flags and commands
- [Architecture](docs/architecture.md) -- Pipeline design and transcript processing
- [Development](docs/development.md) -- Building, testing, releasing

## Related

- [tail-claude](https://github.com/kylesnowschwartz/tail-claude) -- Terminal TUI for reading Claude Code session logs

## License

[MIT](LICENSE)
