// Package render walks config lines, calls widget functions, joins non-empty
// results with the configured separator, and writes each line to an io.Writer.
package render

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/color"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/logging"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render/widget"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/theme"
)

// truncateSuffix is appended when a line is truncated to fit terminal width.
const truncateSuffix = "..."

// ansiReset is prepended to every output line so that our ANSI color codes
// render correctly even when Claude Code applies dim styling to plugin output.
// Without this, Claude Code's dim setting bleeds into the statusline colors.
const ansiReset = "\x1b[0m"

// minTruncateWidth is the smallest terminal width at which truncation is
// applied. Below this threshold the suffix itself would consume most of the
// available space and produce output that is less useful than the raw text.
const minTruncateWidth = 20

// Powerline characters (Nerd Font private-use area).
const (
	// powerlineArrow is U+E0B0 — the right-pointing filled triangle used as a
	// segment separator when transitioning between adjacent segments.
	powerlineArrow = "\ue0b0"

	// powerlineStartCap is U+E0B2 — the left-pointing filled triangle rendered
	// before the first segment as an opening cap.
	powerlineStartCap = "\ue0b2"
)

// resolveSegmentBg returns the effective background color for a widget segment,
// reading in priority order:
//  1. r.BgColor (widget-level explicit override from the WidgetResult)
//  2. cfg.ResolvedTheme[widgetName].Bg (per-widget theme entry)
//  3. theme.DefaultPowerlineBg (last-resort fallback, xterm-256 color 236)
func resolveSegmentBg(r widget.WidgetResult, widgetName string, cfg *config.Config) string {
	if r.BgColor != "" {
		return color.ResolveColorName(r.BgColor)
	}
	if cfg.ResolvedTheme != nil {
		if colors, ok := cfg.ResolvedTheme[widgetName]; ok && colors.Bg != "" {
			return color.ResolveColorName(colors.Bg)
		}
	}
	return theme.DefaultPowerlineBg
}

// resolveSegmentFg returns the effective foreground color for a widget segment:
//  1. cfg.ResolvedTheme[widgetName].Fg (user theme override — highest priority)
//  2. r.FgColor (widget-level explicit fg default)
//  3. "" (no explicit fg; let lipgloss use terminal default)
//
// Theme overrides take priority over widget defaults so that user config is
// never silently blocked by a widget's built-in FgColor.
func resolveSegmentFg(r widget.WidgetResult, widgetName string, cfg *config.Config) string {
	// Theme overrides take priority — user intent beats widget defaults.
	if cfg.ResolvedTheme != nil {
		if colors, ok := cfg.ResolvedTheme[widgetName]; ok && colors.Fg != "" {
			return color.ResolveColorName(colors.Fg)
		}
	}
	if r.FgColor != "" {
		return color.ResolveColorName(r.FgColor)
	}
	return ""
}

// renderPowerline formats a slice of (name, WidgetResult) pairs as a
// powerline-style line:
//
//   - Empty results (IsEmpty()) are skipped.
//   - The first segment is preceded by a start cap (U+E0B2) whose fg matches
//     the first segment's background, against the terminal default bg.
//   - Adjacent segments are separated by a right-arrow (U+E0B0) whose fg is
//     the left segment's bg and whose bg is the right segment's bg.
//   - The last segment is followed by a closing arrow whose fg is the last
//     segment's bg and bg is reset to terminal default.
//
// Each segment's bg is resolved via resolveSegmentBg (widget result > theme >
// fallback), so every widget can have a visually distinct background.
//
// When results is empty the function returns "".
func renderPowerline(results []widget.WidgetResult, names []string, cfg *config.Config) string {
	// Pair non-empty results with their widget names.
	type seg struct {
		name   string
		result widget.WidgetResult
		bg     string
		fg     string
	}

	var segs []seg
	for i, r := range results {
		if r.IsEmpty() {
			continue
		}
		name := names[i]
		segs = append(segs, seg{
			name:   name,
			result: r,
			bg:     resolveSegmentBg(r, name, cfg),
			fg:     resolveSegmentFg(r, name, cfg),
		})
	}

	if len(segs) == 0 {
		return ""
	}

	var sb strings.Builder

	// Start cap: fg=first-segment-bg, no bg (terminal default).
	firstBg := segs[0].bg
	startCapStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(firstBg))
	sb.WriteString(startCapStyle.Render(powerlineStartCap))

	for i, s := range segs {
		// Use PlainText (unstyled) for powerline segments so the bg/fg wrapping
		// is not disrupted by internal ANSI resets. Fall back to Text when
		// PlainText is not populated.
		content := s.result.PlainText
		if content == "" {
			content = s.result.Text
		}

		// Segment content: bg color, fg color (if known), padded text.
		segStyle := lipgloss.NewStyle().Background(lipgloss.Color(s.bg))
		if s.fg != "" {
			segStyle = segStyle.Foreground(lipgloss.Color(s.fg))
		}
		sb.WriteString(segStyle.Render(" " + content + " "))

		// Arrow after this segment.
		if i+1 < len(segs) {
			// Transition arrow: fg=this-bg, bg=next-bg.
			nextBg := segs[i+1].bg
			arrowStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(s.bg)).
				Background(lipgloss.Color(nextBg))
			sb.WriteString(arrowStyle.Render(powerlineArrow))
		} else {
			// End cap: fg=this-bg, no bg (terminal default).
			endCapStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(s.bg))
			sb.WriteString(endCapStyle.Render(powerlineArrow))
		}
	}

	return sb.String()
}

// renderMinimal joins non-empty segments with a single space. No decorators,
// no background colors — just the widget text with its fg color applied.
//
// When results is empty the function returns "".
func renderMinimal(results []widget.WidgetResult, line config.Line, cfg *config.Config) string {
	var parts []string
	for _, r := range results {
		if r.IsEmpty() {
			continue
		}
		// Use PlainText with FgColor for clean minimal rendering. Fall back
		// to pre-styled Text when PlainText is not available.
		var text string
		if r.PlainText != "" && r.FgColor != "" {
			text = lipgloss.NewStyle().Foreground(lipgloss.Color(color.ResolveColorName(r.FgColor))).Render(r.PlainText)
		} else if r.FgColor != "" {
			text = lipgloss.NewStyle().Foreground(lipgloss.Color(color.ResolveColorName(r.FgColor))).Render(r.Text)
		} else {
			text = r.Text
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// lineMode returns the effective render mode for a line, applying the per-line
// override when present, otherwise falling back to the global style mode.
func lineMode(line config.Line, globalMode string) string {
	if line.Mode != "" {
		return line.Mode
	}
	if globalMode != "" {
		return globalMode
	}
	return "plain"
}

// Render walks config lines, looks up widgets in the registry, joins non-empty
// results with the configured separator, and writes each line to w.
//
// Unknown widget names are skipped silently (logged at Debug level).
// Lines where all widgets return empty strings are skipped entirely.
//
// The rendering style for each line is controlled by cfg.Style.Mode (global)
// and line.Mode (per-line override). Accepted modes:
//   - "plain"     (default): separator-joined pre-styled strings
//   - "powerline": Nerd Font arrow transitions between segments
//   - "minimal":   space-separated, fg color only, no background or decorators
//
// When ctx.TerminalWidth is at least minTruncateWidth (20), each output line
// is truncated to that width using ANSI-aware grapheme counting so that escape
// sequences and wide characters are measured correctly. Truncated lines gain a
// "..." suffix. Below the minimum, truncation is skipped so that very narrow
// terminals still receive content rather than collapsing to "...".
//
// When ctx.TerminalWidth is 0 (width detection failed), a defaultTerminalWidth
// of 120 is used as a safe fallback. Skipping truncation entirely when the
// width is unknown risks lines wrapping, which causes Claude Code to hide the
// whole HUD.
//
// The caller is expected to populate ctx.TerminalWidth before calling Render
// (the gather stage does this via terminalWidth() in gather.go).
func Render(w io.Writer, ctx *model.RenderContext, cfg *config.Config) {
	// Apply color_level from config to lipgloss's global writer so that ANSI
	// escape codes emitted by Style.Render() are downsampled (or passed
	// through at full fidelity) according to the user's explicit preference.
	// Without this, lipgloss auto-detects from os.Stdout at package init time,
	// which in pipe mode (how Claude Code runs this binary) may produce a
	// NoTTY profile and strip all colors.
	lipgloss.Writer.Profile = color.LevelFromConfig(cfg.Style.ColorLevel).ColorProfile()

	sep := cfg.Style.Separator
	globalMode := cfg.Style.Mode

	for _, line := range cfg.Lines {
		mode := lineMode(line, globalMode)

		var results []widget.WidgetResult
		var names []string
		for _, name := range line.Widgets {
			fn, ok := widget.Registry[name]
			if !ok {
				logging.Debug("render: unknown widget %q, skipping", name)
				continue
			}
			results = append(results, fn(ctx, cfg))
			names = append(names, name)
		}

		var output string
		switch mode {
		case "powerline":
			output = renderPowerline(results, names, cfg)
		case "minimal":
			output = renderMinimal(results, line, cfg)
		default: // "plain" or any unknown value
			// Plain mode: apply widget styles and join with separator.
			var parts []string
			for i, r := range results {
				if r.IsEmpty() {
					continue
				}
				parts = append(parts, applyWidgetStyle(r, names[i], cfg))
			}
			if len(parts) == 0 {
				continue
			}
			output = strings.Join(parts, sep)
		}

		if output == "" {
			continue
		}

		// Truncate only when the terminal width is known. When width
		// detection fails (returns 0), output the raw line and let Claude
		// Code handle it — matching claude-hud's behavior.
		if ctx.TerminalWidth >= minTruncateWidth {
			output = ansi.Truncate(output, ctx.TerminalWidth, truncateSuffix)
		}

		// Prepend reset so our colors aren't affected by Claude Code's dim styling.
		// Append reset + erase-to-EOL (\x1b[K] so background colors don't bleed
		// through the newline into the next line. The erase-to-EOL tells the terminal
		// to fill the remainder of the line with the default background.
		outLine := ansiReset + output + ansiReset + "\x1b[K"
		fmt.Fprintln(w, outLine)
	}

	// Extra command output: rendered as a final line when non-empty.
	// The output is already sanitized by extracmd.Run (only printable chars
	// and safe ANSI color sequences are retained).
	if ctx.ExtraOutput != "" {
		extraLine := ansiReset + ctx.ExtraOutput + ansiReset + "\x1b[K"
		fmt.Fprintln(w, extraLine)
	}
}

// applyWidgetStyle converts a WidgetResult to a styled string for plain mode.
//
// When ResolvedTheme has an fg override for this widget AND the widget signals
// it accepts theme coloring (r.FgColor != ""), PlainText is re-rendered with
// the override color. This avoids double-wrapping: r.Text already has the
// widget's ANSI codes baked in, so wrapping it again with a different fg
// produces conflicting escape sequences.
//
// When r.FgColor == "", the widget composes multiple internal styles (e.g.
// tools with yellow/dim separators, green/red icons). Applying a theme fg
// override would flatten those per-element ANSI codes into a single color, so
// r.Text is passed through unchanged regardless of any theme fg override.
//
// When no fg override is present the pre-styled r.Text is passed through as-is
// (widget's own styling is preserved). A bg override, if present, is applied
// around whichever text was selected.
func applyWidgetStyle(r widget.WidgetResult, widgetName string, cfg *config.Config) string {
	var fgOverride, bgOverride string
	if cfg.ResolvedTheme != nil {
		if colors, ok := cfg.ResolvedTheme[widgetName]; ok {
			fgOverride = colors.Fg
			bgOverride = colors.Bg
		}
	}
	// Fall back to widget-level bg when no theme bg override.
	if bgOverride == "" {
		bgOverride = r.BgColor
	}

	// When the widget returns FgColor == "", it composes multiple internal
	// styles (e.g. tools with yellow/dim separators, green/red icons).
	// Skip the theme fg override to preserve per-element ANSI styling.
	// Widgets that want theme fg to apply set FgColor to a non-empty value.
	if fgOverride != "" && r.FgColor != "" {
		// Re-render from PlainText to avoid double-styling.
		text := r.PlainText
		if text == "" {
			text = r.Text // fallback if PlainText not set
		}
		s := lipgloss.NewStyle().Foreground(lipgloss.Color(fgOverride))
		if bgOverride != "" {
			s = s.Background(lipgloss.Color(bgOverride))
		}
		return s.Render(text)
	}

	// No fg override (or widget is pre-styled) — use pre-styled Text, optionally with bg.
	if bgOverride != "" {
		return lipgloss.NewStyle().Background(lipgloss.Color(bgOverride)).Render(r.Text)
	}
	return r.Text
}
