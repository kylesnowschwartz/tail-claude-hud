# Widgets

26 widgets, each a pure function that returns a styled string or `""` when it has nothing to show.

`context`, `cost`, `usage`, and `cachettl` switch to a warning or critical color at their thresholds. That state color wins over a theme `fg`, and in powerline mode the segment is drawn on the state color (see [Themes](configuration.md#themes)).

## Session data

From Claude Code's stdin JSON:

| Widget | Shows | Config |
|---|---|---|
| `model` | Model name, optional context window size | `show_context_size` (bool, default: true) |
| `context` | Context usage, color-coded by threshold; with Nerd Font icons a pie icon comes before the value | `display` (text/bar/percent/both), `bar_width` (int), `value` (percent/tokens), `show_breakdown` (bool) |
| `cost` | Session cost in USD, color-coded by threshold | -- |
| `tokens` | Token count and cache hit ratio (`136k tok · 40% cached`) | -- |
| `duration` | Elapsed session time (`45s`, `1m 12s`, `1h 3m`) | -- |
| `lines` | Lines added/removed (green +N, red -N) | -- |
| `speed` | Output tokens/sec averaged over the window, with a ↑/↓ trend arrow when generation is ramping or slowing; hides when idle past the window | `window_secs` (int, default: 30; 0 = session average, no arrow) |
| `messages` | Conversation turn count | -- |
| `session` | Session name | -- |
| `outputstyle` | Active output style name | -- |
| `effort` | Reasoning effort level (low/medium/high/xhigh/max), falling back to `CLAUDE_EFFORT` | -- |
| `worktree` | Worktree name (`wt:fix-bug`) and branch when running in a git worktree | `show_branch` (bool, default: true) |

## Transcript data

Parsed incrementally from the JSONL transcript:

| Widget | Shows | Config |
|---|---|---|
| `tools` | Running/completed tool invocations as a scrolling activity feed, with each finished call's duration (`0.3s`, `12.3s`, `1m 12s`) | -- |
| `agents` | Sub-agents with model family (`(Opus)`) and elapsed time (running) or duration (completed), in the same format as `duration` | -- |
| `todos` | Task completion count, color-coded | -- |
| `thinking` | Thinking block indicator and count | -- |
| `skills` | Skill names invoked during the session, newest first | `max_age_mins` (int, default: 0 = show all; >0 hides invocations older than N minutes) |
| `cachettl` | Time left before the main conversation's prompt cache expires: a ring that empties plus `47m`, `0:42` in the last minute, or `◌ Cold` once expired (the next message re-writes the whole conversation to cache). Warning color in the last 20% of the TTL; hidden before the first request | `ttl` (`"auto"` default follows the transcript's latest cache write and assumes 1h until one is seen; `"5m"` or `"1h"` forces it) |

## Environment

Gathered from the filesystem:

| Widget | Shows | Config |
|---|---|---|
| `directory` | Working directory; `fish` shortens parent segments to one letter, hidden ones to the dot and one letter (`~/C/.c/w/wt`) | `style` (full/fish/basename), `levels` (int) |
| `git` | Branch, dirty indicator, ahead/behind counts, uncommitted line deltas (`+100 -55`) | `dirty` (bool), `ahead_behind` (bool), `file_stats` (bool), `line_stats` (bool) |
| `project` | Composite of directory + git in a single segment | inherits `[directory]` and `[git]` config |
| `env` | MCP servers, CLAUDE.md files, rule files, hooks (e.g. "3M 2C 4R 3H") | -- |
| `permission` | Red alert when another session needs approval (requires hooks via `--init`) | `show_project` (bool, default: true) |
| `usage` | Anthropic 5-hour and 7-day rate-limit utilization; warning color at 50%, critical at 80% | `five_hour_threshold` (int, default: 0), `seven_day_threshold` (int, default: 80), `cache_ttl_seconds` (int, default: 180), `display` (text/bar), `bar_width` (int, default: 8) |
| `peers` | Count of other running interactive Claude Code sessions (`×2`) | -- |
| `exec` | First stdout line of a user command that receives the stdin JSON | `command` (string, absolute path), `timeout_ms` (int, default: 100) |

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

[cachettl]
ttl = "auto"   # or "5m" / "1h" to force the cache lifetime

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
