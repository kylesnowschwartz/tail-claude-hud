# Architecture

## Four-stage pipeline

Every invocation follows a strict linear pipeline. Each stage is a separate package with no backward dependencies.

```
stdin -> gather -> render -> stdout
```

1. **stdin** (`internal/stdin`) -- Decode JSON from Claude Code, persist snapshot to disk.
2. **gather** (`internal/gather`) -- Spawn goroutines only for data sources active widgets need.
3. **render** (`internal/render`) -- Walk configured lines, call each widget's render function, ANSI-truncate to terminal width.
4. **widget** (`internal/render/widget`) -- Pure functions: `(RenderContext, Config) -> WidgetResult`.

## Transcript processing

Three layers in `internal/transcript/`:

- **transcript.go** -- Parses JSONL entries and classifies content blocks (tool_use, tool_result, thinking, text). Filters sidechain sub-agent messages.
- **extractor.go** -- Stateful processor that accumulates tools, agents, and todos across entries. Handles agent lifecycle, todo mutations, and the scrolling divider counter.
- **state.go** -- Byte-offset persistence for incremental reads. Embeds the extraction snapshot so state survives across ticks.

Reads are incremental (O(delta) not O(n)) by tracking byte offsets per file.

## Design decisions

- **Fail-open config**: Missing or corrupt TOML yields defaults. The statusline must always render.
- **Conditional goroutines**: The gather stage checks which widgets are configured before spawning work. No transcript widgets active = no transcript parsing.
- **Never write to stderr**: Claude Code owns the terminal. Debug logging goes to `~/.claude/plugins/tail-claude-hud/debug.log`, gated behind `TAIL_CLAUDE_HUD_DEBUG=1`.
- **Single-digit millisecond budget**: The full cycle runs on every tick. No work that doesn't contribute to the current frame.
