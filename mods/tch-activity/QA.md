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
- Colors: `tmux capture-pane -p -e -t $P | grep -a '▸\|▾' | cat -v`. Before piping captured text through `cut`, `awk` or `sed`, run `export LC_ALL=en_US.UTF-8`; without it they reject the glyphs as illegal byte sequences. With the default HUD theme expect the border `38;5;8`, a check `38;5;2`, a running tool `38;5;3`, the first agent `38;5;12`, durations and the timer faint grey.
- Alignment: tmux and the engine both count a Nerd Font glyph as one column, so in a capture every row of the box ends its right border at the same column. Check it in code points (macOS `awk` counts bytes, which a glyph has three or four of): `tmux capture-pane -p -t $P | grep '│\|╭\|╰' | python3 -c "import sys; print({max(l.rfind(c) for c in '│╮╯') + 1 for l in sys.stdin})"` prints one number. A viewer whose font draws those glyphs two columns wide shows the right border of rows that hold one out of line; judge alignment by the number, not by how a capture looks.
- Wait for a state with a polling loop, not a fixed sleep: `for i in $(seq 1 30); do tmux capture-pane -p -t $P | grep -q 'Idle · last' && break; sleep 1; done`

## Submit a prompt

`tmux send-keys -t $P -l '<prompt>'`, then `tmux send-keys -t $P Enter`. If Enter does not submit, the prompt is in vim normal mode: send `i`, retype, then `Enter`. After `Escape` from the focused band the prompt is already in insert mode; an extra `i` lands in the prompt as text.

## Steps

| # | Do | Expect in the band |
|---|---|---|
| 1 | Nothing, right after start | `▸  Idle, waiting for your prompt` (`◷`), one box. No second box reading `Claude: Idle`: that one is whats-agent-doing, which `just mod-dev` turns off. |
| 2 | Focus the band and expand: `send-keys C-x Tab`, then `Enter` | `▾`, then `No actions yet`. No `Built-in look:` line (that line means the HUD binary was not used; it says why). |
| 3 | Prompt: ``Run the shell command `sleep 5 && echo done` with the Bash tool, description "Wait five seconds". Then read the file VERSION and reply in one short sentence.`` Capture at about 4 s. | `󰯠 Wait five seconds \| 4s` (`$`), icon and label in the running color, label bold; the timer counts up each second. |
| 4 | Wait for `Idle · last` | `Idle · last turn done in 12.5s, 2 actions` (`✓`, success color). History rows: `› Run the shell command …`, `Reading VERSION 0.7s` (`✓ ≡`), `󰯠 Wait five seconds 6.1s`, `Wrote the reply (N words)`, `Done in …`. |
| 5 | Prompt: ``Launch one background subagent with the Agent tool (run_in_background: true, description "Count Go files") that runs `sleep 20` with Bash and then counts the .go files under internal/. Do not wait for it; just reply "launched".`` | While the agent works after the turn: `󱙺 Count Go files › <its latest action> \| Ns` (`◆`), icon and name in the palette color of the session's first Agent call (`38;5;12` in a fresh session); the ` › <action>` part stays in the default color. Match `Count Go files ›`, not the bare name: the first line containing `Count Go files` is the `Running an agent:` history row, drawn in the running color. The headline lasts only about 20 s, so sample the color inside the polling loop: `for i in $(seq 1 30); do L=$(tmux capture-pane -p -e -t $P \| grep -a '▸'); [[ $L == *'Count Go files ›'* ]] && { print -r -- $L \| cat -v; break; }; sleep 1; done`. History: `󱙺 Running an agent: Count Go files`. |
| 6 | Wait for the agent to finish (about 25 s) | History gains `› Agent message` for the turn its report starts, then `󱙺 Agent: Count Go files 25s` below it. |
| 7 | Collapse: `C-x Tab`, `Enter` (or click the triangle) | `▸`, history hidden. `Escape` returns the keys to the prompt. |
| 8 | Approval wait, in a session of its own (close the first with `tmux kill-window -t tch-activity-qa`): build this checkout's HUD binary the way `just mod-dev` does, `bin="${TMPDIR:-/tmp}/tch-activity-dev/tail-claude-hud"; go build -o "$bin" ./cmd/tail-claude-hud`, then `tmux new-window -d -n tch-activity-qa -x 200 -y 60 -c "$PWD" "claude --plugin-dir mods/tch-activity --permission-mode default --settings '$(jq -cn --arg bin "$bin" '{enabledPlugins:{"whats-agent-doing@tzafrir":false},permissions:{ask:["Bash(osascript:*)"]},pluginConfigs:{"tch-activity":{options:{binary:$bin}}}}')'"`. The `ask` rule makes the dialog appear whatever your allow rules say. Poll for the dialog and for the band with a loop of 60 or more iterations: an API retry can stretch the turn well past the usual few seconds. Prompt: ``Run exactly this with the Bash tool, description "Wait in AppleScript": osascript -e 'delay 4' -e 'return 1'`` | While the dialog (`Do you want to proceed?`) is up, the band is not drawn: the dialog takes its place. Send `Enter` (Yes): the band returns showing `󰯠 Wait in AppleScript \| Ns` with no `Waiting for approval`. Run it again and answer `Escape` (No): the history row starts with a muted `⊘`. |
| 9 | Hot reload: change the module's content (a new modification time alone does not reload it): `printf '\n' >> mods/tch-activity/hooks/text.ts`, wait for the line below, then `git checkout -- mods/tch-activity/hooks/text.ts` | The transcript shows `tch-activity: reloaded (N hooks: …)` and no `refused` or `skipped` line. The headline, the history, the agent colors and the HUD look stay (`Idle · last turn …` keeps its last turn). A turn running when the reload lands is taken up at its next model request and ends idle. |

## Finish

`tmux kill-window -t tch-activity-qa`. Report each step as pass or fail with the captured band text for any failure.
