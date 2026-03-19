# Widgets

18 widgets, each a pure function that returns a styled string or `""` when it has nothing to show.

## Session data

From Claude Code's stdin JSON:

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

## Transcript data

Parsed incrementally from the JSONL transcript:

| Widget | Shows |
|---|---|
| `tools` | Running/completed tool invocations as a scrolling activity feed |
| `agents` | Sub-agents with elapsed time (running) or duration (completed) |
| `todos` | Task completion count, color-coded |
| `thinking` | Thinking block indicator and count |
| `skills` | Skill names invoked during the session |

## Environment

Gathered from the filesystem:

| Widget | Shows |
|---|---|
| `directory` | Working directory (full, fish-style abbreviation, or basename) |
| `git` | Branch, dirty indicator, ahead/behind counts |
| `project` | Composite of directory + git in a single segment |
| `env` | MCP servers, CLAUDE.md files, rule files, hooks (e.g. "3M 2C 4R 3H") |
