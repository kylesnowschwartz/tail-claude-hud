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

Three layers, two from the shared [agent-ouija](https://github.com/kylesnowschwartz/agent-ouija) library and one app-side:

- **`claude/transcript`** (library) -- Parses JSONL entries and classifies content blocks (tool_use, tool_result, thinking, text). The HUD uses the lenient parse path, which accepts uuid-less entries.
- **`internal/extract`** (app) -- Stateful processor that accumulates tools, agents, and todos across entries. Handles agent lifecycle, todo mutations, and the scrolling divider counter. Owns the snapshot `SchemaVersion`.
- **`offsetstore`** (library) -- Byte-offset persistence for incremental reads. Embeds the extraction snapshot so state survives across ticks. Always defers an unterminated final line to the next tick.

Reads are incremental (O(delta) not O(n)) by tracking byte offsets per file.

## Design decisions

- **Fail-open config**: Missing or corrupt TOML yields defaults. The statusline must always render.
- **Conditional goroutines**: The gather stage checks which widgets are configured before spawning work. No transcript widgets active = no transcript parsing.
- **Never write to stderr**: Claude Code owns the terminal. Debug logging goes to `~/.claude/plugins/tail-claude-hud/debug.log`, gated behind `TAIL_CLAUDE_HUD_DEBUG=1`.
- **Hook-based permission detection**: The binary doubles as a Claude Code hook handler (`tail-claude-hud hook <event>`). PermissionRequest writes a breadcrumb file; PostToolUse and Stop remove it. The gather stage scans breadcrumb files instead of the process table, keeping the project free of cgo.
- **Single-digit millisecond budget**: The full cycle runs on every tick. No work that doesn't contribute to the current frame.
