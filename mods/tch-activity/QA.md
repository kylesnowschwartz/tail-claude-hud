# tch-activity QA checklist (tmux)

A script for checking the mod in a real Claude Code session driven through tmux. Each step names the keys to send and the text to expect in the band, the box just above the prompt. Glyphs below assume the HUD's Nerd Font icons; with the built-in look they are the unicode ones in brackets.

## Setup

1. Build a HUD binary that has `style --json`, without installing it:

   ```
   go build -o "$TMPDIR/tch-qa/tail-claude-hud" ./cmd/tail-claude-hud
   "$TMPDIR/tch-qa/tail-claude-hud" style --json | head -3    # expect "schema": 1
   ```

2. Start the session in a new window from the repository root:

   ```
   tmux new-window -d -n tch-activity-qa -c "$PWD" "TCH_HUD_BINARY=$TMPDIR/tch-qa/tail-claude-hud just mod-dev"
   tmux list-panes -t tch-activity-qa -F '#{pane_id} #{pane_current_command}'
   ```

   Target the pane whose command is `just` (or `claude`) by its id, `P=%NNNN`, in every later step. A sidebar pane may share the window; never send keys to it.

3. Wait until `tmux capture-pane -p -t $P` shows the prompt `❯`. If a folder trust question shows first, send `Enter`.

## Read the band

- Text: `tmux capture-pane -p -t $P | grep -v '^\s*$' | grep -A16 '╭'`
- Colors: `tmux capture-pane -p -e -t $P | grep -a '▸\|▾' | cat -v`. With the default HUD theme expect the border `38;5;8`, a check `38;5;2`, a running tool `38;5;3`, the first agent `38;5;12`, durations and the timer faint grey.
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
| 5 | Prompt: ``Launch one background subagent with the Agent tool (run_in_background: true, description "Count Go files") that runs `sleep 20` with Bash and then counts the .go files under internal/. Do not wait for it; just reply "launched".`` | While the agent works after the turn: `󱙺 Count Go files › <its latest action> \| Ns` (`◆`), icon and name in the first palette color (`38;5;12`). History: `󱙺 Running an agent: Count Go files`. |
| 6 | Wait for the agent to finish (about 25 s) | History gains `󱙺 Agent: Count Go files 25s`, then `› Agent message` for the turn its report starts. |
| 7 | Collapse: `C-x Tab`, `Enter` (or click the triangle) | `▸`, history hidden. `Escape` returns the keys to the prompt. |
| 8 | Optional, approval wait. Switch to manual mode (`send-keys BTab` until the footer says `manual mode on`), then prompt with a command your settings do not allow, for example ``Run exactly this with the Bash tool, description "Ask AppleScript for a number": osascript -e 'return 1'`` | While the approval dialog is up: `󰅸 Waiting for your approval: Ask AppleScript for a number` (`!`), icon, label and border in the permission color. Answer `Escape` (no): the history row starts with a muted `⊘`. If your allow rules or a permission hook approve it, no dialog appears and this step cannot be checked here; the automated test covers it. |
| 9 | Hot reload: `touch mods/tch-activity/hooks/text.ts` | The transcript shows `tch-activity: reloaded (11 hooks: …)` and no `refused` or `skipped` line. The history and the HUD look stay; the next prompt is tracked as before. |

## Finish

`tmux kill-window -t tch-activity-qa`. Report each step as pass or fail with the captured band text for any failure.
