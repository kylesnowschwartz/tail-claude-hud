# Development

Requires Go 1.25+ and [just](https://github.com/casey/just) for task running.

## Commands

```bash
just              # run tests
just build        # go install ./cmd/tail-claude-hud (to ~/go/bin)
just test         # go test ./... -count=1
just test-race    # race detector
just bench        # benchmarks
just check        # fmt + vet + test
just dump         # build + render from current session
just run-sample   # pipe testdata through the binary
just mod-check    # validate the tch-activity mod and the marketplace, run the mod's tests, typecheck
just mod-dev      # run claude with the mod from disk and this checkout's HUD binary
```

`just mod-check` typechecks the mod only after one `just mod-dev` run has written the mod API's types to `mods/tch-activity/.claude-plugin/types/`. Until then it prints `type check skipped: run 'just mod-dev' once to write the API types` and exits 0 after validation and tests.

## Running a single test

```bash
go test ./internal/extract/ -run TestProcessEntry -count=1
```

## Releasing

```bash
just bump patch   # or minor, major
just release      # commit, tag, push, create GitHub release
```
