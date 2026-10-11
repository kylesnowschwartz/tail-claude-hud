# tch-activity

A box above the Claude Code prompt that says what Claude is doing right now, drawn with tail-claude-hud's icons and colors. Press the triangle to see every step it has taken.

```
╭────────────────────────────────────────────────────────────────╮
│ ▾  Idle · last turn done in 12s, 2 actions                    │
│                                                                │
│ › Run the shell command `sleep 5 && echo done` with the Bash … │
│  Read VERSION 0.4s                                            │
│ 󰯠 Wait five seconds 5.7s                                       │
│  Wrote the reply (11 words) <0.1s                             │
│  Done in 12s, 2 actions                                       │
╰────────────────────────────────────────────────────────────────╯
```

It is a Claude Code mod: a plugin of function hooks, which are early access and load only where function hooks are enabled. It replaces [whats-agent-doing](https://github.com/tzafrir/whats-agent-doing) with the same states and history, restyled to match the statusline.

## Install

```
/plugin install tch-activity --marketplace kylesnowschwartz/tail-claude-hud
```

Answer `y` to add the marketplace, then pick a scope. If whats-agent-doing is installed too, disable it (`claude plugin disable whats-agent-doing@tzafrir`) or both boxes stack above the prompt.

## Where the look comes from

At session start the mod runs `tail-claude-hud style --json` once, in the background, and keeps the answer for the session; the box draws with the built-in look until the answer arrives. That output carries the HUD's resolved icons (Nerd Font, unicode or ascii, per your `[style] icons`), its semantic colors, the agent palette and the tool categories, so the box draws with whatever your HUD config draws with.

The binary is found in this order:

1. The `binary` option (`/plugin` → tch-activity → configure, or `pluginConfigs."tch-activity".options.binary` in settings).
2. `~/go/bin/tail-claude-hud`, where `go install` puts it. A Claude Code session runs with the login PATH, which often lacks Go's bin folder.
3. `tail-claude-hud` on the session's PATH.

If your `statusLine` command passes `--preset` (for example `tail-claude-hud --preset powerline`), set the `preset` option to the same value so the box reads the same look. The HUD's automatic light theme for powerline layouts on a light terminal background is not part of the export.

The box uses the HUD's state colors (running, thinking, needs you, success, error, muted) and its agent palette. The per-widget colors in your theme file (`theme-active.toml`) are not used.

If the binary is missing, too old to have `style --json`, or prints something unreadable, the box draws with a built-in unicode look (`✓ ✗ ⧗ ≡ $ ◆ ∴ ❝`) in the HUD's default colors. Its icons are the HUD's own `icons = "unicode"` set: one-column symbols, no emoji (see [Style](../../docs/configuration.md#style)). The expanded history then ends with one dim line saying why. Nothing else is reported. The mod asks again after a reload or an option change, so a newer binary is picked up without a new session.

## What it shows

| State | Headline |
|---|---|
| Before any turn | ` Idle, waiting for your prompt` (clock icon, muted) |
| Reading the prompt | `󰔟 Reading your prompt` |
| Thinking | ` Thinking: …the hook needs to await next` (the thought's latest sentence), or, while none has streamed, the turn's last action under the thinking icon (` Reading app.ts`), ` Reading your prompt` before there is one |
| Writing a tool call | the turn's last action under the running icon (`󰔟 Preparing the next action` before there is one) until the call's input says what it does, then `󰯠 Run the tests`, ` Writing register.tsx \| 2.3 KB` |
| Running a tool | the tool's category icon in the running color, then its label: `󰯠 Wait five seconds`, ` Reading app.ts`, `󰖟 Fetching example.org` |
| Parallel calls | the newest, then `(+N more)` |
| A subagent at work | `󱙺 Running an agent: find the bug \| Reading app.ts` |
| Background agents | `󱙺 2 agents working`, and a row per agent under the headline label: `󱙺 Draft the plan \| Searching the web \| 1m 12s`. Each agent's icon and name take the color the HUD's agents widget gives it: its Agent call's position among the session's last 10, wrapped at the palette's 8 colors. |
| Waiting on you | `󰅸 Waiting for your approval: Run the tests`, `󰅸 Waiting for your answer` (permission icon and color). In the terminal the approval dialog takes the band's place while it is up; a subagent's row says it waits until its call runs. |
| Between calls | the turn's last action under the running icon: `󰔟 Reading app.ts` |
| Writing the reply | ` Writing the reply \| 120 words` |
| Compacting | `󰔟 Compacting the conversation` |
| Between turns | `Idle · last turn done in 42s, 6 actions`, or interrupted, stopped on an error, declined the request, stopped at a declined call; the glyph is the turn's outcome |

Labels are drawn in the default text color. A state's color is on its glyph and on the border: yellow while Claude works or thinks, red while it or a subagent waits on you, muted grey when idle. Numbers, timers and separators are faint. A timer after the label (`| 4.2s`) counts how long the current state has lasted, in tenths of a second under ten seconds; a tool's timer leaves out the time its approval dialog was up. While a turn runs, the turn's total time in whole seconds is drawn faint just right of the box, on the headline's line (`│ · 1m 12s`). It leaves out the time Claude waited on your approval or answer, as the turn's closing `Done in` time does, and is left off when the band has no room for it beside the box. The box widens to fit its widest row and keeps that width for the rest of the session.

Focus the band with `ctrl+x tab` and press Enter, or click the triangle, to expand the history: each prompt (`›`), stretch of thinking, tool call, reply and turn ending, with durations. A tool row is in the past tense and has one glyph, as the HUD's tools widget draws a finished tool: the tool's category icon in the success color, or the error color when it failed, as in ` Read VERSION 0.4s`. A call you declined shows the HUD's declined icon and an interrupted one its interrupted icon (`⊘` and `■` in the unicode set, both muted) in place of the category icon, so those rows never rely on color alone. A question to you shows the bell, and an agent's row the robot, under the same rule. Thought, reply and compaction (`⇣`) icons are faint. A background agent gets a `Started an agent:` row when it is launched and a `Ran an agent:` row when it finishes. The turn's last row says how it ended with its time in the label, `Done in 12s, 2 actions`, or `⊘ Stopped at a declined call after 7.1s` when its last call was one you declined. A prompt the engine sends for you (`<agent-message …>`, `<task-notification>`) is listed by its tag, `Agent message`.

The box steps aside while a survey holds the band. Other mods that draw above the prompt keep their place beneath it.

In the terminal the mod hides Claude Code's spinner line (`✶ Ionizing… (13s · ↓ 574 tokens)`), with the task list, tips and API retry notices drawn under it. Set the `hideSpinner` option to `false` to keep it.

## Data and privacy

The mod runs one local command (`tail-claude-hud style --json`) when a session starts, and again after a reload while the HUD's look is not in place; it reads `HOME` to find the binary. It keeps its headline, history and look in the session's own state (`$.state`). It makes no network calls.

It writes one file: while a call of the main conversation waits on your approval, `~/.config/tail-claude-hud/waiting/<session id>` holds the session id, the working folder's name and the tool's name, the file the HUD's `hook permission-request` writes. Other sessions' status lines read it to show that this one waits on you. The mod removes it, by running `/bin/rm -f` on that path, as soon as you approve or decline the call and when the session ends or is cleared. A file it fails to remove is ignored by the HUD after two minutes.

Every hook passes its event on unchanged; it never answers a permission prompt.

The box shows your prompts, file names, URLs, command descriptions (or the command when it has none), MCP tool names and a line of the model's thinking where it streams. Labels are cleaned of control and invisible characters before they are drawn.

## Develop

From the repository root:

```
just mod-dev      # claude with this folder loaded (hot-reloads on save), whats-agent-doing off for the session
just mod-check    # validate (mod and marketplace), run the tests, typecheck
```

`just mod-dev` builds this checkout's HUD binary to a temporary folder and points the mod at it, and passes `--settings` with `"enabledPlugins":{"whats-agent-doing@tzafrir":false}`, which turns an installed whats-agent-doing off for that session alone. Set `TCH_HUD_BINARY=/path/to/tail-claude-hud` to use another HUD binary. Loading the mod writes the API's types to `.claude-plugin/types/` (git-ignored), which `tsconfig.json` extends. `just mod-check` typechecks only once that has happened; until then it validates and runs the tests, prints `type check skipped: run 'just mod-dev' once to write the API types`, and exits 0.

`QA.md` is a scripted checklist for driving the mod in tmux.

| File | What it does |
|---|---|
| `hooks/register.tsx` | The hooks module: every hook, every function that takes `$`, the state atoms, the style load and the drawing. The engine follows `$` and state references only within this file. |
| `hooks/live.ts` | The live view of the session and the headline it yields (pure) |
| `hooks/marks.ts` | The glyph and color for each state, outcome and history row (pure) |
| `hooks/style.ts` | The built-in look, parsing `style --json`, and icon and color lookups (pure) |
| `hooks/activity-of.ts` | A tool call, whole or streaming, as a short label |
| `hooks/text.ts` | Printable labels, thought snippets, word counts, sizes, durations |
| `hooks/waiting.ts` | The approval breadcrumb's path and contents (pure) |
| `types/index.d.ts` | The `$.state` contract |
| `tests/` | The turn state by state, the history toggle, hostile labels, approvals, agents and their palette, the HUD look and its fallback |

## License

MIT. Portions are adapted from whats-agent-doing by Tzafrir Rehan (MIT); see `NOTICE`.
