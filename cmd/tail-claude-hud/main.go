package main

import (
	"fmt"
	"os"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/stdin"
)

func main() {
	// 1. Read stdin — detect TTY (no pipe) vs piped JSON.
	input, err := stdin.Read(os.Stdin)
	if err != nil || input == nil {
		fmt.Println("[tail-claude-hud] Initializing...")
		return
	}

	// 2. Load HUD config (fast, single file read).
	cfg := config.LoadHud()

	// 3. Build a minimal RenderContext from stdin data.
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

	// 4. Render and print.
	render.Render(os.Stdout, ctx, cfg)
}
