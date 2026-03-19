# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

tail-claude-hud is a Go binary that renders a terminal statusline for Claude Code sessions. Claude Code pipes JSON to stdin on every tick; this binary parses it, gathers supplementary data (transcript state, git status, environment counts), and prints a styled multi-line statusline to stdout. The entire cycle must complete in single-digit milliseconds because it runs on every keypress/tick.

It combines techniques from two reference projects in `.cloned-sources/`: `tail-claude` (Go transcript parsing) and `claude-hud` (TypeScript statusline plugin with HUD-first design).

## Build & Test

Uses `just` (justfile) for all tasks:

```sh
just              # default: run tests
just build        # go build -o bin/tail-claude-hud ./cmd/tail-claude-hud
just test         # go test ./... -count=1
just test-race    # go test -race ./... -count=1
just bench        # go test -bench=. -benchmem ./internal/... -count=1
just check        # fmt + vet + test
just dump         # build + render from current session's transcript
just run-sample   # pipe testdata/sample-stdin.json through the binary
```

Run a single test:
```sh
go test ./internal/transcript/ -run TestExtractContentBlocks -count=1
```

## Architecture: The Four-Stage Pipeline

Every invocation follows a strict linear pipeline. Each stage is a separate package with no backward dependencies.

```
stdin → gather → render → stdout
```

1. **stdin** (`internal/stdin`): Decodes JSON from Claude Code, computes context percentage, persists a snapshot to disk for `--dump-current` mode.

2. **gather** (`internal/gather`): Inspects which widgets are active in the config, spawns goroutines *only* for the data sources those widgets need (transcript, git, env). A `sync.WaitGroup` gates the render stage.

3. **render** (`internal/render`): Walks configured lines, calls each widget's `RenderFunc` from the registry, joins non-empty results with the separator, and ANSI-truncates to terminal width.

4. **widget** (`internal/render/widget`): 21 registered widgets (model, context, cost, directory, git, project, env, duration, tools, agents, todos, session, thinking, tokens, lines, outputstyle, messages, skills, speed, permission, usage). Each is a pure function: `(RenderContext, Config) -> WidgetResult`. Returns empty when it has nothing to show.

## Key Design Decisions

**Fail-open config**: `config.LoadHud()` never returns nil or an error. Missing or corrupt TOML yields defaults. The statusline must always render something.

**Incremental transcript reads**: `transcript.StateManager` tracks byte offsets per transcript path (keyed by SHA-256 hash). Each tick reads only new bytes (O(delta) not O(n)). Extraction state is snapshotted to disk so the full tool/agent/todo history survives process restarts.

**Never write to stderr**: Claude Code owns the terminal. Any stderr output corrupts the display. Debug logging goes to `~/.claude/plugins/tail-claude-hud/debug.log` and is gated behind `TAIL_CLAUDE_HUD_DEBUG=1`.

**Conditional goroutines**: The gather stage checks which widgets are configured before spawning work. If no transcript widgets are active, no transcript parsing runs.

**Hook-based permission detection**: The binary doubles as a Claude Code hook handler via `tail-claude-hud hook <event>`. The `PermissionRequest` hook writes a breadcrumb file to `~/.config/tail-claude-hud/waiting/{session_id}`; `PostToolUse` and `Stop` hooks remove it. The statusline gather stage scans this directory (skipping its own session) to detect other sessions blocked on permission approval. Breadcrumbs older than 120s are ignored (covers hard crashes). This replaced a cgo-based process table scanner, enabling `CGO_ENABLED=0` builds.

## Transcript Processing (Three Layers)

The transcript package has three distinct responsibilities:

- **transcript.go** — Parses individual JSONL entries and classifies content blocks (tool_use, tool_result, thinking, text). Handles sidechain filtering (sub-agent user messages are excluded).
- **extractor.go** — Stateful processor that accumulates tools, agents, and todos across entries. Handles agent lifecycle (launch, async results, task notifications), todo mutations (TodoWrite replaces all, TaskCreate/TaskUpdate mutate), and the scrolling divider counter.
- **state.go** — Byte-offset persistence for incremental reads. Embeds the extraction snapshot so state survives across ticks.

## Config

TOML at `~/.config/tail-claude-hud/config.toml` (or legacy `~/.claude/plugins/tail-claude-hud/config.toml`). Generate defaults with `tail-claude-hud --init`.

Layout is configured as `[[line]]` arrays with widget name lists. Default is three lines: summary, agents, tools.

## Stdin JSON Contract

Claude Code pipes this JSON to stdin on every tick. Canonical reference: https://code.claude.com/docs/en/statusline#available-data

| Field | Description |
|---|---|
| `model.id`, `model.display_name` | Model identifier and human-readable name |
| `cwd`, `workspace.current_dir` | Current working directory (identical; prefer `workspace.current_dir`) |
| `workspace.project_dir` | Directory where Claude Code was launched |
| `session_id` | Unique session identifier |
| `transcript_path` | Path to conversation transcript JSONL file |
| `version` | Claude Code version |
| `cost.total_cost_usd` | Accumulated session cost in USD |
| `cost.total_duration_ms` | Wall-clock time since session start (ms) |
| `cost.total_api_duration_ms` | Time spent waiting for API responses (ms) |
| `cost.total_lines_added` | Lines of code added this session |
| `cost.total_lines_removed` | Lines of code removed this session |
| `context_window.context_window_size` | Max context in tokens (200k or 1M) |
| `context_window.used_percentage` | Pre-calculated context usage % (from input tokens only) |
| `context_window.remaining_percentage` | Pre-calculated context remaining % |
| `context_window.total_input_tokens` | **Cumulative** input tokens across the session |
| `context_window.total_output_tokens` | **Cumulative** output tokens across the session |
| `context_window.current_usage` | Token counts from the **most recent** API call (null until first call) |
| `current_usage.input_tokens` | Non-cached input tokens (after last cache breakpoint) |
| `current_usage.output_tokens` | Output tokens generated |
| `current_usage.cache_creation_input_tokens` | Tokens written to prompt cache |
| `current_usage.cache_read_input_tokens` | Tokens served from prompt cache |
| `exceeds_200k_tokens` | Whether combined tokens from last API response exceed 200k |
| `output_style.name` | Current output style name |
| `vim.mode` | Vim mode ("NORMAL"/"INSERT") when enabled |
| `agent.name` | Agent name (when `--agent` flag is used) |
| `worktree.*` | Worktree metadata: name, path, branch, original_cwd, original_branch |

**Token semantics**: `current_usage` fields are per-call snapshots, not session totals. With prompt caching, `input_tokens` is only the uncacheable tail after the last cache breakpoint. `used_percentage` = `(input_tokens + cache_creation + cache_read) / context_window_size`. Session-level cumulative totals are in `total_input_tokens` and `total_output_tokens`.

## Reference Projects

`.cloned-sources/claude-hud/` — Original TypeScript plugin. Reference for UI patterns, color choices, widget behavior.
`.cloned-sources/tail-claude/` — Go predecessor. Reference for transcript entry schema, content block parsing, tool categorization.
