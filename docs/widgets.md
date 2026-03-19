# Widgets

19 widgets, each a pure function that returns a styled string or `""` when it has nothing to show.

## Session data

From Claude Code's stdin JSON:

| Widget | Shows | Config |
|---|---|---|
| `model` | Model name, optional context window size | `show_context_size` (bool, default: true) |
| `context` | Context usage, color-coded by threshold | `display` (text/bar/percent/both), `bar_width` (int), `value` (percent/tokens), `show_breakdown` (bool) |
| `cost` | Session cost in USD, color-coded by threshold | -- |
| `tokens` | Token count and cache hit ratio | -- |
| `duration` | Elapsed session time | -- |
| `lines` | Lines added/removed (green +N, red -N) | -- |
| `speed` | Rolling tokens/sec | `window_secs` (int, default: 30) |
| `messages` | Conversation turn count | -- |
| `session` | Session name | -- |
| `outputstyle` | Active output style name | -- |

## Transcript data

Parsed incrementally from the JSONL transcript:

| Widget | Shows | Config |
|---|---|---|
| `tools` | Running/completed tool invocations as a scrolling activity feed | -- |
| `agents` | Sub-agents with elapsed time (running) or duration (completed) | -- |
| `todos` | Task completion count, color-coded | -- |
| `thinking` | Thinking block indicator and count | -- |
| `skills` | Skill names invoked during the session | -- |

## Environment

Gathered from the filesystem:

| Widget | Shows | Config |
|---|---|---|
| `directory` | Working directory | `style` (full/fish/basename), `levels` (int) |
| `git` | Branch, dirty indicator, ahead/behind counts | `dirty` (bool), `ahead_behind` (bool), `file_stats` (bool) |
| `project` | Composite of directory + git in a single segment | inherits `[directory]` and `[git]` config |
| `env` | MCP servers, CLAUDE.md files, rule files, hooks (e.g. "3M 2C 4R 3H") | -- |
| `permission` | Red alert when another session needs approval | `show_project` (bool, default: true) |
| `usage` | Anthropic 5-hour and 7-day rate-limit utilization | `five_hour_threshold` (int, default: 0), `seven_day_threshold` (int, default: 80), `cache_ttl_seconds` (int, default: 180) |

## Config sections

Widget options live under a TOML section matching the widget name:

```toml
[model]
show_context_size = false

[context]
display = "bar"
bar_width = 15

[speed]
window_secs = 10

[directory]
style = "fish"
levels = 2

[git]
dirty = true
ahead_behind = true
file_stats = false

[permission]
show_project = false   # show only the bell icon

[usage]
five_hour_threshold = 0   # always show when credentials are available
seven_day_threshold = 80  # only show 7-day window when >= 80%
cache_ttl_seconds = 180   # cache successful API responses for 3 minutes
```
