// Package render walks config lines, calls widget functions, joins non-empty
// results with the configured separator, and writes each line to an io.Writer.
package render

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/logging"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render/widget"
)

// truncateSuffix is appended when a line is truncated to fit terminal width.
const truncateSuffix = "..."

// readTerminalWidth returns the value of the COLUMNS environment variable, or
// 0 if COLUMNS is unset or not a positive integer. A width of 0 means no
// truncation is applied.
func readTerminalWidth() int {
	s := os.Getenv("COLUMNS")
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// Render walks config lines, looks up widgets in the registry, joins non-empty
// results with the configured separator, and writes each line to w.
//
// Unknown widget names are skipped silently (logged at Debug level).
// Lines where all widgets return empty strings are skipped entirely.
//
// If ctx.TerminalWidth is zero, Render reads the COLUMNS environment variable
// to populate it. When TerminalWidth is positive, each output line is
// truncated to that width using ANSI-aware grapheme counting so that escape
// sequences and wide characters are measured correctly. Truncated lines gain a
// "..." suffix.
func Render(w io.Writer, ctx *model.RenderContext, cfg *config.Config) {
	// Populate terminal width from environment when the caller hasn't set it.
	if ctx.TerminalWidth == 0 {
		ctx.TerminalWidth = readTerminalWidth()
	}

	sep := cfg.Style.Separator

	for _, line := range cfg.Lines {
		var parts []string
		for _, name := range line.Widgets {
			fn, ok := widget.Registry[name]
			if !ok {
				logging.Debug("render: unknown widget %q, skipping", name)
				continue
			}
			if s := fn(ctx, cfg); s != "" {
				parts = append(parts, s)
			}
		}
		if len(parts) == 0 {
			continue // skip lines where every widget returned empty
		}

		output := strings.Join(parts, sep)

		if ctx.TerminalWidth > 0 {
			output = ansi.Truncate(output, ctx.TerminalWidth, truncateSuffix)
		}

		fmt.Fprintln(w, output)
	}
}
