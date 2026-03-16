// Package render walks config lines, calls widget functions, joins non-empty
// results with the configured separator, and writes each line to an io.Writer.
package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/logging"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render/widget"
)

// truncateSuffix is appended when a line is truncated to fit terminal width.
const truncateSuffix = "..."

// minTruncateWidth is the smallest terminal width at which truncation is
// applied. Below this threshold the suffix itself would consume most of the
// available space and produce output that is less useful than the raw text.
const minTruncateWidth = 20

// Render walks config lines, looks up widgets in the registry, joins non-empty
// results with the configured separator, and writes each line to w.
//
// Unknown widget names are skipped silently (logged at Debug level).
// Lines where all widgets return empty strings are skipped entirely.
//
// When ctx.TerminalWidth is at least minTruncateWidth (20), each output line
// is truncated to that width using ANSI-aware grapheme counting so that escape
// sequences and wide characters are measured correctly. Truncated lines gain a
// "..." suffix. Below the minimum, truncation is skipped so that very narrow
// terminals still receive content rather than collapsing to "...".
//
// The caller is expected to populate ctx.TerminalWidth before calling Render
// (the gather stage does this via terminalWidth() in gather.go).
func Render(w io.Writer, ctx *model.RenderContext, cfg *config.Config) {
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

		if ctx.TerminalWidth >= minTruncateWidth {
			output = ansi.Truncate(output, ctx.TerminalWidth, truncateSuffix)
		}

		fmt.Fprintln(w, output)
	}
}
