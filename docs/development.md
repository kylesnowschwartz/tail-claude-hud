# Development

Requires Go 1.25+ and [just](https://github.com/casey/just) for task running.

## Commands

```bash
just              # run tests
just build        # go build -o bin/tail-claude-hud ./cmd/tail-claude-hud
just test         # go test ./... -count=1
just test-race    # race detector
just bench        # benchmarks
just check        # fmt + vet + test
just dump         # build + render from current session
just run-sample   # pipe testdata through the binary
```

## Running a single test

```bash
go test ./internal/transcript/ -run TestExtractContentBlocks -count=1
```

## Releasing

```bash
just bump patch   # or minor, major
just release      # commit, tag, push, create GitHub release
```
