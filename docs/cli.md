# CLI Reference

```
tail-claude-hud [flags]
  --init           Generate default config file
  --preset NAME    Apply a built-in or custom preset
  --theme NAME     Override color theme
  --list-presets   Print available preset names
  --dump-current   Render from the current session's transcript snapshot
  --dump-raw       Like --dump-current but print ANSI escapes as visible text
  --preview PATH   Render from a transcript file with mock stdin data
  --watch          Continuously re-render on transcript changes (with --preview)
```

## Notes

`--dump-current` auto-discovers the most recent `.jsonl` transcript for the current directory. Useful for testing outside a live session.

`--preview` with `--watch` polls the transcript every 500ms and re-renders on change, for live development iteration.
