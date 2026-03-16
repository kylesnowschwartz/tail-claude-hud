package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/gather"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/stdin"
)

func main() {
	dumpCurrent := flag.Bool("dump-current", false, "render the statusline from a transcript file instead of stdin")
	initConfig := flag.Bool("init", false, "generate a default config file at ~/.config/tail-claude-hud/config.toml")
	flag.Parse()

	if *initConfig {
		if err := config.Init(); err != nil {
			fmt.Fprintf(os.Stderr, "tail-claude-hud: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var input *model.StdinData
	var err error

	if *dumpCurrent {
		input, err = readFromFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "tail-claude-hud: %v\n", err)
			os.Exit(1)
		}
	} else {
		input, err = stdin.Read(os.Stdin)
		if err == nil && input != nil {
			stdin.SaveSnapshot(input)
		}
	}

	if err != nil || input == nil {
		fmt.Println("[tail-claude-hud] Initializing...")
		return
	}

	// Load HUD config (fast, single file read).
	cfg := config.LoadHud()

	// Collect data in parallel for configured widgets.
	ctx := gather.Gather(input, cfg)

	// Render and print.
	render.Render(os.Stdout, ctx, cfg)
}

// readFromFile loads the last-stdin snapshot (model, context window) and
// resolves the transcript path so the gather stage can parse tools/agents/todos.
// The snapshot is written on every live statusline invocation, so it reflects
// the most recent state from the active Claude Code session.
//
// Transcript path priority:
//  1. positional argument (first non-flag arg)
//  2. CLAUDE_TRANSCRIPT_PATH env var
//  3. snapshot's own TranscriptPath
//  4. auto-discover: most recently modified .jsonl in ~/.claude/projects/<cwd-slug>/
func readFromFile() (*model.StdinData, error) {
	// Start from the persisted snapshot when available. If missing, fall back
	// to an empty StdinData — dump still works, just without model/context.
	data, err := stdin.LoadSnapshot()
	if err != nil {
		data = &model.StdinData{}
	}

	// Resolve transcript path, allowing explicit overrides.
	path := flag.Arg(0)
	if path == "" {
		path = os.Getenv("CLAUDE_TRANSCRIPT_PATH")
	}
	if path == "" {
		path = data.TranscriptPath
	}
	if path == "" {
		path, err = findCurrentTranscript()
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("--dump-current: %w", err)
	}

	data.TranscriptPath = path
	if data.Cwd == "" {
		data.Cwd = mustCwd()
	}

	return data, nil
}

// findCurrentTranscript auto-discovers the most recently modified .jsonl file
// in ~/.claude/projects/<cwd-slug>/. The cwd-slug is computed from the current
// working directory using Claude Code's path encoding scheme.
func findCurrentTranscript() (string, error) {
	projectDir, err := currentProjectDir()
	if err != nil {
		return "", fmt.Errorf("--dump-current: resolve project dir: %w", err)
	}

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", fmt.Errorf("--dump-current: no transcript found (could not read %s): %w", projectDir, err)
	}

	var newest string
	var newestTime int64
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".jsonl") {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		if mt := info.ModTime().UnixNano(); mt > newestTime {
			newestTime = mt
			newest = filepath.Join(projectDir, de.Name())
		}
	}

	if newest == "" {
		return "", fmt.Errorf("--dump-current: no .jsonl transcript found in %s", projectDir)
	}
	return newest, nil
}

// currentProjectDir returns ~/.claude/projects/<encoded-cwd>. Symlinks in the
// cwd are resolved so the encoded path matches what Claude Code produces on
// disk (e.g. macOS /tmp -> /private/tmp).
func currentProjectDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Resolve symlinks so the encoded path matches Claude Code's on-disk output.
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}

	encoded := encodePath(cwd)
	return filepath.Join(home, ".claude", "projects", encoded), nil
}

// encodePath encodes an absolute filesystem path into a Claude Code project
// directory name. Three characters are replaced with "-": path separators (/),
// dots (.), and underscores (_). Ported from tail-claude's parser/session.go
// and verified empirically across 273 project directories.
func encodePath(absPath string) string {
	r := strings.NewReplacer(
		string(filepath.Separator), "-",
		".", "-",
		"_", "-",
	)
	return r.Replace(absPath)
}

// mustCwd returns the current working directory, resolving symlinks to match
// Claude Code's on-disk encoding (e.g. macOS /tmp -> /private/tmp).
func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	return cwd
}
