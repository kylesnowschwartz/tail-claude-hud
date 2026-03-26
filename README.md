# tail-claude-hud

A terminal statusline for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions. Shows model, context usage, tools, agents, todos, git status, and more -- updated on every tick.

![tail-claude-hud demo](demo.gif)

## Install

Requires Go 1.25+.

```bash
go install github.com/kylesnowschwartz/tail-claude-hud/cmd/tail-claude-hud@latest
```

To update, run the same command. Note that if you're new to the `go` command, it will likely put the `tail-claude-hud` command
in `~/go/bin` which you might need to add to your path in your own `.bashrc` / `.zshrc` or similar.

To build from source:

```bash
git clone git@github.com:kylesnowschwartz/tail-claude-hud.git
cd tail-claude-hud
just build
```

## Setup

Add to `~/.claude/settings.json` (if there's already stuff there, just add the statusLine key and block to the end):

```json
{
  "statusLine": {
    "type": "command",
    "command": "tail-claude-hud"
  }
}
```

Works out of the box with the `default` preset. To customize, run `tail-claude-hud --init` to generate a config at `~/.config/tail-claude-hud/config.toml`.

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

- [Widgets](docs/widgets.md) -- 19 available widgets and what they display
- [Configuration](docs/configuration.md) -- TOML reference, render modes, themes
- [CLI](docs/cli.md) -- Flags and commands
- [Architecture](docs/architecture.md) -- Pipeline design and transcript processing
- [Development](docs/development.md) -- Building, testing, releasing

## Related

- [tail-claude](https://github.com/kylesnowschwartz/tail-claude) -- Terminal TUI for reading Claude Code session logs

## License

[MIT](LICENSE)
