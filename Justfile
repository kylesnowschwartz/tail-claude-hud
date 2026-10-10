# tail-claude-hud

# Default: run tests
default: test

# Build and install the binary to ~/go/bin
build:
    go install ./cmd/tail-claude-hud

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
    tail-claude-hud --dump-current

# Pipe sample stdin through the binary (no transcript)
run-sample:
    cat testdata/sample-stdin.json | go run ./cmd/tail-claude-hud

# The activity mod (a Claude Code function-hooks plugin) under mods/.
mod := "mods/tch-activity"

# Validate the mod and the marketplace, run the mod's tests, and typecheck it
mod-check:
    #!/usr/bin/env zsh
    set -e
    claude plugin validate --strict {{mod}}
    claude plugin validate --strict .
    claude plugin test {{mod}}
    if [[ ! -f {{mod}}/.claude-plugin/types/tsconfig.json ]]; then
        echo "No API types in {{mod}}/.claude-plugin/types: run 'just mod-dev' once, which writes them, then rerun."
        exit 1
    fi
    npx -y -p typescript tsc -p {{mod}}

# Run Claude Code with the mod from disk and whats-agent-doing off; TCH_HUD_BINARY sets the mod's binary
mod-dev *args:
    #!/usr/bin/env zsh
    set -e
    settings='{"enabledPlugins":{"whats-agent-doing@tzafrir":false}}'
    if [[ -n "${TCH_HUD_BINARY:-}" ]]; then
        settings=$(jq -cn --arg bin "$TCH_HUD_BINARY" \
            '{enabledPlugins:{"whats-agent-doing@tzafrir":false},pluginConfigs:{"tch-activity":{options:{binary:$bin}}}}')
    fi
    exec claude --plugin-dir {{mod}} --settings "$settings" {{args}}

# Run design evaluation
eval:
    go test ./internal/eval/ -run TestDesignEval -v -count=1

# Clean build artifacts
clean:
    rm -rf bin/
    rm -f $(go env GOPATH)/bin/tail-claude-hud

# Bump the version. Usage: just bump patch|minor|major
bump level:
    #!/usr/bin/env zsh
    set -e

    v=$(cat VERSION)
    M=${v%%.*}; rest=${v#*.}; m=${rest%%.*}; p=${rest#*.}

    case "{{level}}" in
        patch) new="$M.$m.$((p+1))" ;;
        minor) new="$M.$((m+1)).0" ;;
        major) new="$((M+1)).0.0" ;;
        *) echo "Usage: just bump patch|minor|major" && exit 1 ;;
    esac

    echo "Bumping $v → $new"
    echo "$new" > VERSION
    git add VERSION
    echo "Version bumped to $new. Run 'just release' to commit, tag, and push."

# Commit, tag, push, and create a GitHub release. Pass a notes file for custom release notes.
release notes="":
    #!/usr/bin/env zsh
    set -e

    v=$(cat VERSION)

    # Safety: must be on main and up to date
    branch=$(git branch --show-current)
    if [[ "$branch" != "main" ]]; then
        echo "Error: must be on main branch (currently on $branch)"
        exit 1
    fi

    git fetch origin main
    behind=$(git rev-list HEAD..origin/main --count)
    if [[ "$behind" -gt 0 ]]; then
        echo "Error: $behind commit(s) behind origin/main"
        echo "Run 'git pull --rebase' first"
        exit 1
    fi

    if git diff --cached --quiet; then
        echo "Error: nothing staged. Run 'just bump' first."
        exit 1
    fi

    tag="v$v"

    git commit -m "chore: bump version to $v"
    git tag "$tag"
    git push && git push --tags

    # Create GitHub release
    notes="{{notes}}"
    if [[ -n "$notes" && -f "$notes" ]]; then
        gh release create "$tag" --title "$tag" --notes-file "$notes" --latest
    else
        gh release create "$tag" --title "$tag" --generate-notes --latest
    fi

    # Prime the Go module proxy so `go install ...@latest` resolves immediately.
    # Uses the /lookup endpoint which only needs the tag to exist on GitHub —
    # no local auth required.
    curl -sf "https://proxy.golang.org/github.com/kylesnowschwartz/tail-claude-hud/@v/${tag}.info" > /dev/null || true
    curl -sf "https://proxy.golang.org/github.com/kylesnowschwartz/tail-claude-hud/@latest" > /dev/null || true

    echo "Released $tag"
