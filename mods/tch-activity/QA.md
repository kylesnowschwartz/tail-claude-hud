# tch-activity QA checklist (tmux)

A script for checking the mod in a real Claude Code session driven through tmux. Each step names the keys to send and the text to expect in the band, the box just above the prompt. Glyphs below assume the HUD's Nerd Font icons; with the built-in look they are the unicode ones in brackets.

## Setup

1. Start the session in a new window from the repository root. `just mod-dev` builds this checkout's HUD binary, which has `style --json`, and points the mod at it:

   ```
   tmux new-window -d -n tch-activity-qa -x 200 -y 60 -c "$PWD" "just mod-dev"
   tmux list-panes -t tch-activity-qa -F '#{pane_id} #{pane_current_command}'
   ```

   The window must be at least 160 columns wide so band rows are not truncated. Target the pane whose command is `just` (or `claude`) by its id, `P=%NNNN`, in every later step. A sidebar pane may share the window; never send keys to it.

2. Wait until `tmux capture-pane -p -t $P` shows the prompt `❯`. If a folder trust question shows first, send `Enter`.

## Read the band

- Text: `tmux capture-pane -p -t $P | grep -v '^\s*$' | grep -A16 '╭'`
- Colors: `tmux capture-pane -p -e -t $P | grep -a '▸\|▾' | cat -v`. Before piping captured text through `cut`, `awk` or `sed`, run `export LC_ALL=en_US.UTF-8`; without it they reject the glyphs as illegal byte sequences. With the default HUD theme expect the border `38;5;8` when idle and `38;5;3` while Claude works, a check `38;5;2`, a running tool's glyph `38;5;3`, the first agent `38;5;12`, labels with no color code and no bold (`1m`), and durations, timers and ` | ` separators faint (`2m`).
- Alignment: tmux and the engine both count a Nerd Font glyph as one column, so in a capture every row of the box ends its right border at the same column. Check it in code points (macOS `awk` counts bytes, which a glyph has three or four of): `tmux capture-pane -p -t $P | grep '│\|╭\|╰' | python3 -c "import sys; print({max(l.rfind(c) for c in '│╮╯') + 1 for l in sys.stdin})"` prints one number. A viewer whose font draws those glyphs two columns wide shows the right border of rows that hold one out of line; judge alignment by the number, not by how a capture looks.
- Wait for a state with a polling loop, not a fixed sleep: `for i in $(seq 1 30); do tmux capture-pane -p -t $P | grep -q 'Idle · last' && break; sleep 1; done`

## Submit a prompt

`tmux send-keys -t $P -l '<prompt>'`, then `tmux send-keys -t $P Enter`. If Enter does not submit, the prompt is in vim normal mode: send `i`, retype, then `Enter`. After `Escape` from the focused band the prompt is already in insert mode; an extra `i` lands in the prompt as text.

## Steps

| # | Do | Expect in the band |
|---|---|---|
| 1 | Nothing, right after start | `▸  Idle, waiting for your prompt` (`◷`), one box. No second box reading `Claude: Idle`: that one is whats-agent-doing, which `just mod-dev` turns off. |
| 2 | Focus the band and expand: `send-keys C-x Tab`, then `Enter` | `▾`, then `No actions yet`. No `Built-in look:` line (that line means the HUD binary was not used; it says why). |
| 3 | Prompt: ``Run the shell command `sleep 5 && echo done` with the Bash tool, description "Wait five seconds". Then read the file VERSION and reply in one short sentence.`` Capture at about 4 s. | `󰯠 Wait five seconds \| 4.2s` (`$`): the icon and the border in the running color, the label in the default color and not bold; the timer counts in tenths of a second. |
| 4 | Wait for `Idle · last` | `Idle · last turn done in 12s, 2 actions` (`✓`, success color), border muted. History rows, in the past tense, each with one glyph: `› Run the shell command …`, ` Read VERSION 0.7s` (`≡`) and `󰯠 Wait five seconds 6.1s` (`$`), the category icons in the success color (`38;5;2`), `Wrote the reply (N words)` (`❝`, faint), then `Done in 12s, 2 actions` with no duration after it. |
| 5 | Prompt: ``Launch two background subagents with the Agent tool (run_in_background: true), description "Count Go files" and "Count Markdown files". Each runs `sleep 20` with Bash and then counts its files under internal/. Do not wait for them; just reply "launched".`` | While the agents work after the turn: the headline `󱙺 2 agents working \| Ns` and a row per agent, `󱙺 Count Go files \| <its latest action> \| Ns` (`◆`). Each row's icon starts in the column of the headline label, not under the triangle. Icon and name are in the palette color of the agent's Agent call (`38;5;12` and `38;5;10` in a fresh session), not bold; the action is in the default color and both ` \| ` separators are faint. Match `Count Go files \|`, not the bare name: the first line containing `Count Go files` is the history row. The rows last only about 20 s, so sample them inside the polling loop: `for i in $(seq 1 30); do L=$(tmux capture-pane -p -e -t $P \| grep -a 'Count Go files'); [[ $L == *'Count Go files'*'|'* ]] && { print -r -- $L \| cat -v; break; }; sleep 1; done`. History:`󱙺 Started an agent: Count Go files`. |
| 6 | Wait for the agent to finish (about 25 s) | History gains `› Agent message` for the turn its report starts, then `󱙺 Ran an agent: Count Go files 25s` below it, the robot icon in the success color. |
| 7 | Collapse: `C-x Tab`, `Enter` (or click the triangle) | `▸`, history hidden, and the box keeps the width it had with the history open: its right border stays in the same column (the alignment check prints the same number as before). `Escape` returns the keys to the prompt. |
| 8 | Approval wait, in a session of its own (close the first with `tmux kill-window -t tch-activity-qa`): build this checkout's HUD binary the way `just mod-dev` does, `bin="${TMPDIR:-/tmp}/tch-activity-dev/tail-claude-hud"; go build -o "$bin" ./cmd/tail-claude-hud`, then `tmux new-window -d -n tch-activity-qa -x 200 -y 60 -c "$PWD" "claude --plugin-dir mods/tch-activity --permission-mode default --settings '$(jq -cn --arg bin "$bin" '{enabledPlugins:{"whats-agent-doing@tzafrir":false},permissions:{ask:["Bash(osascript:*)"]},pluginConfigs:{"tch-activity":{options:{binary:$bin}}}}')'"`. The `ask` rule makes the dialog appear whatever your allow rules say. Poll for the dialog and for the band with a loop of 60 or more iterations: an API retry can stretch the turn well past the usual few seconds. Prompt: ``Run exactly this with the Bash tool, description "Wait in AppleScript": osascript -e 'delay 4' -e 'return 1'`` | While the dialog (`Do you want to proceed?`) is up, the band is not drawn: the dialog takes its place. Send `Enter` (Yes): the band returns showing `󰯠 Wait in AppleScript \| N.Ns` with no `Waiting for your approval`, the timer starting from about zero however long the dialog was up; its history row's duration is about 4 s. Run it again, wait a few seconds, and answer `Escape` (No): the history row reads `⊘ Wait in AppleScript` (the muted mark in place of the category icon) with a duration near zero, the turn ends with `⊘ Stopped at a declined call after N.Ns`, and the headline reads `Idle · last turn stopped at a declined call after N.Ns` under a muted `⊘`. |
| 9 | Hot reload: change the module's content (a new modification time alone does not reload it): `printf '\n' >> mods/tch-activity/hooks/text.ts`, wait for the line below, then `git checkout -- mods/tch-activity/hooks/text.ts` | The transcript shows `tch-activity: reloaded (N hooks: …)` and no `refused` or `skipped` line. The headline, the history, the agent colors and the HUD look stay (`Idle · last turn …` keeps its last turn), and so does the box's width. A turn running when the reload lands is taken up at its next model request and ends idle. |

## Finish

`tmux kill-window -t tch-activity-qa`. Report each step as pass or fail with the captured band text for any failure.
