# tch-activity

A box above the Claude Code prompt that says what Claude is doing right now, drawn with tail-claude-hud's icons and colors. Press the triangle to see every step it has taken.

```
╭────────────────────────────────────────────────────────────────╮
│ ▾  Idle · last turn done in 12.3s, 2 actions                  │
│                                                                │
│ › Run the shell command `sleep 5 && echo done` with the Bash … │
│   Reading VERSION 0.4s                                       │
│  󰯠 Wait five seconds 5.7s                                     │
│  Wrote the reply (11 words) <0.1s                             │
│  Done in 12.3s, 2 actions 12.3s                               │
╰────────────────────────────────────────────────────────────────╯
```

It is a Claude Code mod: a plugin of function hooks, which are early access and load only where function hooks are enabled. It replaces [whats-agent-doing](https://github.com/tzafrir/whats-agent-doing) with the same states and history, restyled to match the statusline.

## Install

```
/plugin install tch-activity --marketplace kylesnowschwartz/tail-claude-hud
```

Answer `y` to add the marketplace, then pick a scope. If whats-agent-doing is installed too, disable it (`claude plugin disable whats-agent-doing@tzafrir`) or both boxes stack above the prompt.

## Where the look comes from

At session start the mod runs `tail-claude-hud style --json` once and keeps the answer for the session. That output carries the HUD's resolved icons (Nerd Font, unicode or ascii, per your `[style] icons`), its semantic colors, the agent palette and the tool categories, so the box draws with whatever your HUD config draws with.

The binary is found in this order:

1. The `binary` option (`/plugin` → tch-activity → configure, or `pluginConfigs."tch-activity".options.binary` in settings).
2. `~/go/bin/tail-claude-hud`, where `go install` puts it. A Claude Code session runs with the login PATH, which often lacks Go's bin folder.
3. `tail-claude-hud` on the session's PATH.

If the binary is missing, too old to have `style --json`, or prints something unreadable, the box draws with a built-in unicode look (`✓ ✗ ◐ ≡ $ ◆ ∴`) in the HUD's default colors. The expanded history then ends with one dim line saying why. Nothing else is reported.

## What it shows

| State | Headline |
|---|---|
| Before any turn | `◷ Idle, waiting for your prompt` (clock icon, muted) |
| Reading the prompt | `󰪠 Reading your prompt` |
| Thinking | `Thinking: …the hook needs to await next` (the thought's latest sentence; with none streamed, the last action stands) |
| Writing a tool call, once its input says what it does | `󰯠 Run the tests`, `Writing register.tsx · 2.3 KB` |
| Running a tool | the tool's category icon and label, bold in the running color: `󰯠 Wait five seconds`, `Reading app.ts`, `󰖟 Fetching example.org` |
| Parallel calls | the newest, then `(+N more)` |
| A subagent at work | `󱙺 Running an agent: find the bug › Reading app.ts` |
| Background agents | `󱙺 2 agents working`, and a row per agent: `󱙺 Draft the plan › Searching the web | 1m 12s` |
| Waiting on you | `󰅸 Waiting for your approval: Run the tests`, `󰅸 Waiting for your answer` (permission icon and color; the border turns that color too) |
| Reading tool results | `󰪠 Reviewing the results of 3 actions` |
| Writing the reply | `Writing the reply · 120 words` |
| Compacting | `󰪠 Compacting the conversation` |
| Between turns | `Idle · last turn done in 42s, 6 actions`, or interrupted, stopped on an error, declined; the glyph is the turn's outcome |

A timer after the label (`| 4s`) counts how long the current state has lasted.

Focus the band with `ctrl+x tab` and press Enter, or click the triangle, to expand the history: each prompt (`›`), stretch of thinking, tool call, reply and turn ending, with durations. A tool row reads `<outcome> <category icon> <label> <duration>`, the outcome being the HUD's check (success color), its error icon (error color), `⊘` denied or `■` interrupted (both muted). Background agents get a history row when they finish. A prompt the engine sends for you (`<agent-message …>`, `<task-notification>`) is listed by its tag, `Agent message`.

The box steps aside while a survey holds the band. Other mods that draw above the prompt keep their place beneath it.

## Data and privacy

The mod runs one local command per session (`tail-claude-hud style --json`) and reads `HOME` to find it. It keeps its headline, history and look in the session's own state (`$.state`). It makes no network calls and writes no files. Every hook is display only and passes its event on unchanged; it never answers a permission prompt.

The box shows your prompts, file names, URLs, command descriptions (or the command when it has none), MCP tool names and a line of the model's thinking where it streams. Labels are cleaned of control and invisible characters before they are drawn.

## Develop

From the repository root:

```
just mod-dev      # claude with this folder loaded (hot-reloads on save), whats-agent-doing off for the session
just mod-check    # validate (mod and marketplace), run the tests, typecheck
```

`just mod-dev` builds this checkout's HUD binary to a temporary folder and points the mod at it, and passes `--settings` with `"enabledPlugins":{"whats-agent-doing@tzafrir":false}`, which turns an installed whats-agent-doing off for that session alone. Set `TCH_HUD_BINARY=/path/to/tail-claude-hud` to use another HUD binary. Loading the mod writes the API's types to `.claude-plugin/types/` (git-ignored), which `tsconfig.json` extends; `just mod-check` typechecks only once that has happened.

`QA.md` is a scripted checklist for driving the mod in tmux.

| File | What it does |
|---|---|
| `hooks/register.tsx` | The hooks module: every hook, every function that takes `$`, the state atoms, the style load and the drawing. The engine follows `$` and state references only within this file. |
| `hooks/live.ts` | The live view of the session and the headline it yields (pure) |
| `hooks/marks.ts` | The glyph and color for each state, outcome and history row (pure) |
| `hooks/style.ts` | The built-in look, parsing `style --json`, and icon and color lookups (pure) |
| `hooks/activity-of.ts` | A tool call, whole or streaming, as a short label |
| `hooks/text.ts` | Printable labels, thought snippets, word counts, sizes, durations |
| `types/index.d.ts` | The `$.state` contract |
| `tests/` | The turn state by state, the history toggle, hostile labels, approvals, agents and their palette, the HUD look and its fallback |

## License

MIT. Portions are adapted from whats-agent-doing by Tzafrir Rehan (MIT); see `NOTICE`.
