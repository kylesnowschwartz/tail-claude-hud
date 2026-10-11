# CLI Reference

```
tail-claude-hud [flags]
  --init           Generate default config and register Claude Code hooks
  --preset NAME    Apply a built-in or custom preset
  --theme NAME     Override color theme
  --list-presets   Print available preset names
  --dump-current   Render from the current session's transcript snapshot
  --dump-raw       Like --dump-current but print ANSI escapes as visible text
  --preview PATH   Render from a transcript file with mock stdin data
  --watch          Continuously re-render on transcript changes (with --preview)
```

## Hook subcommand

The binary handles Claude Code hook events directly:

```
tail-claude-hud hook permission-request   # write breadcrumb; notify when [notify] permission_bell is on (PermissionRequest hook)
tail-claude-hud hook cleanup              # remove this session's breadcrumb (a tool outcome keeps it while the session still waits on a dialog) and any breadcrumb older than 10 minutes (PostToolUse, PostToolUseFailure, Stop hooks)
tail-claude-hud hook session-start        # set the terminal title when [notify] session_title is on (SessionStart hook)
```

`--init` registers these hooks in `~/.claude/settings.json`. Re-running is idempotent.

## Other commands

```
tail-claude-hud style --json [--preset NAME] [--theme NAME]   # print resolved icons, colors, agent palette, and tool categories as JSON
tail-claude-hud version                                       # print the current version
tail-claude-hud update                                        # install the latest version with go install
```

`style --json` reads the same config as the statusline. Pass the same `--preset` the `statusLine` command uses to get the same look. The automatic light theme for powerline on a light terminal background is not applied. The [tch-activity](../mods/tch-activity/README.md) mod reads its look from this command.

## Notes

`--dump-current` auto-discovers the most recent `.jsonl` transcript for the current directory. Useful for testing outside a live session.

`--preview` with `--watch` polls the transcript every 500ms and re-renders on change, for live development iteration.
