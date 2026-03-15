package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/stdin"
)

func main() {
	dumpCurrent := flag.Bool("dump-current", false, "render the statusline from a transcript file instead of stdin")
	flag.Parse()

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
	}

	if err != nil || input == nil {
		fmt.Println("[tail-claude-hud] Initializing...")
		return
	}

	// Load HUD config (fast, single file read).
	cfg := config.LoadHud()

	// Build a minimal RenderContext from stdin data.
	// No gather coordinator yet — that comes in Phase 4.
	ctx := &model.RenderContext{
		ContextPercent: input.ContextPercent,
		Cwd:            input.Cwd,
	}
	if input.Model != nil {
		ctx.ModelID = input.Model.ID
		ctx.ModelDisplayName = input.Model.DisplayName
	}
	if input.ContextWindow != nil {
		ctx.ContextWindowSize = input.ContextWindow.Size
	}

	// Render and print.
	render.Render(os.Stdout, ctx, cfg)
}

// readFromFile resolves the transcript path from CLI args or the environment
// and delegates to readFileInput. It prefers a positional argument (first
// non-flag arg), then falls back to CLAUDE_TRANSCRIPT_PATH.
func readFromFile() (*model.StdinData, error) {
	path := flag.Arg(0)
	if path == "" {
		path = os.Getenv("CLAUDE_TRANSCRIPT_PATH")
	}
	if path == "" {
		return nil, fmt.Errorf("--dump-current requires a transcript path argument or CLAUDE_TRANSCRIPT_PATH env var")
	}
	return readFileInput(path)
}

// readFileInput opens path and parses it through the stdin pipeline.
// Extracted so tests can exercise file reading without flag state.
func readFileInput(path string) (*model.StdinData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("--dump-current: open %q: %w", path, err)
	}
	defer f.Close()

	return stdin.Read(f)
}
