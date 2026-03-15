# tail-claude-hud

# Default: run tests
default: test

# Build the binary
build:
    go build -o bin/tail-claude-hud ./cmd/tail-claude-hud

# Run all tests
test:
    go test ./... -count=1

# Run tests with race detector
test-race:
    go test -race ./... -count=1

# Run benchmarks
bench:
    go test -bench=. -benchmem ./internal/... -count=1

# Format code
fmt:
    go fmt ./...

# Vet code
vet:
    go vet ./...

# Format, vet, and test
check: fmt vet test

# Render the statusline from the current session's transcript
dump: build
    ./bin/tail-claude-hud --dump-current

# Pipe sample stdin through the binary (no transcript)
run-sample:
    cat testdata/sample-stdin.json | go run ./cmd/tail-claude-hud

# Clean build artifacts
clean:
    rm -rf bin/
