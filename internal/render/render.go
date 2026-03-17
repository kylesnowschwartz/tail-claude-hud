// Package render walks config lines, calls widget functions, joins non-empty
// results with the configured separator, and writes each line to an io.Writer.
package render

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/logging"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render/widget"
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

// defaultPowerlineBg is the fallback xterm-256 background color (dark gray)
// used when a widget does not declare its own BgColor.
const defaultPowerlineBg = "236"

// ansiSetFg returns the ANSI escape sequence to set the foreground to a
// xterm-256 color code (e.g. "75"). Returns "" for empty input.
func ansiSetFg(color string) string {
	if color == "" {
		return ""
	}
	return "\x1b[38;5;" + color + "m"
}

// ansiSetBg returns the ANSI escape sequence to set the background to a
// xterm-256 color code. Returns "" for empty input.
func ansiSetBg(color string) string {
	if color == "" {
		return ""
	}
	return "\x1b[48;5;" + color + "m"
}

// effectiveBg returns r.BgColor when set, otherwise the default powerline bg.
func effectiveBg(r widget.WidgetResult) string {
	if r.BgColor != "" {
		return r.BgColor
	}
	return defaultPowerlineBg
}

// renderPowerline formats a slice of WidgetResults as a powerline-style line:
//
//   - Empty results (Text == "") are skipped.
//   - The first segment is preceded by a start cap (U+E0B2) whose color
//     matches the first segment's background.
//   - Adjacent segments are separated by a right-arrow (U+E0B0) whose fg is
//     the left segment's bg and whose bg is the right segment's bg.
//   - The last segment is followed by a reset and a closing arrow in the last
//     segment's background color (so the arrow "ends" the bar).
//
// When results is empty the function returns "".
func renderPowerline(results []widget.WidgetResult) string {
	// Filter empty segments first.
	var segs []widget.WidgetResult
	for _, r := range results {
		if !r.IsEmpty() {
			segs = append(segs, r)
		}
	}
	if len(segs) == 0 {
		return ""
	}

	var sb strings.Builder

	// Start cap: rendered with the first segment's bg as fg, default terminal bg.
	firstBg := effectiveBg(segs[0])
	sb.WriteString(ansiReset)
	sb.WriteString(ansiSetFg(firstBg))
	sb.WriteString(powerlineStartCap)

	for i, seg := range segs {
		bg := effectiveBg(seg)

		// Segment content: bg color, then fg color (if any), then padded text.
		sb.WriteString(ansiSetBg(bg))
		if seg.FgColor != "" {
			sb.WriteString(ansiSetFg(seg.FgColor))
		} else {
			// No explicit fg; reset fg so it uses the terminal default on this bg.
			sb.WriteString("\x1b[39m")
		}
		sb.WriteString(" ")
		sb.WriteString(seg.Text)
		sb.WriteString(" ")

		// Arrow transition (or end cap after the last segment).
		if i < len(segs)-1 {
			nextBg := effectiveBg(segs[i+1])
			// Arrow: fg = current bg, bg = next segment's bg.
			sb.WriteString(ansiReset)
			sb.WriteString(ansiSetBg(nextBg))
			sb.WriteString(ansiSetFg(bg))
			sb.WriteString(powerlineArrow)
		} else {
			// End cap: reset to default bg, fg = last segment's bg.
			sb.WriteString(ansiReset)
			sb.WriteString(ansiSetFg(bg))
			sb.WriteString(powerlineArrow)
			sb.WriteString(ansiReset)
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
		// Apply fg color only; ignore any bg from theme or widget result.
		// When FgColor is empty the widget pre-styled its own text with internal
		// ANSI codes — pass it through as-is to avoid double-wrapping escape
		// sequences. Only apply theme fg when FgColor is explicitly set (structured
		// output where the widget deferred color responsibility to the renderer).
		var text string
		if r.FgColor != "" {
			text = lipgloss.NewStyle().Foreground(lipgloss.Color(r.FgColor)).Render(r.Text)
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
// The caller is expected to populate ctx.TerminalWidth before calling Render
// (the gather stage does this via terminalWidth() in gather.go).
func Render(w io.Writer, ctx *model.RenderContext, cfg *config.Config) {
	sep := cfg.Style.Separator
	globalMode := cfg.Style.Mode

	for _, line := range cfg.Lines {
		mode := lineMode(line, globalMode)

		var results []widget.WidgetResult
		for _, name := range line.Widgets {
			fn, ok := widget.Registry[name]
			if !ok {
				logging.Debug("render: unknown widget %q, skipping", name)
				continue
			}
			results = append(results, fn(ctx, cfg))
		}

		var output string
		switch mode {
		case "powerline":
			output = renderPowerline(results)
		case "minimal":
			output = renderMinimal(results, line, cfg)
		default: // "plain" or any unknown value
			// Plain mode: apply widget styles and join with separator.
			var parts []string
			for i, r := range results {
				if r.IsEmpty() {
					continue
				}
				name := line.Widgets[i]
				parts = append(parts, applyWidgetStyle(r, name, cfg))
			}
			if len(parts) == 0 {
				continue
			}
			output = strings.Join(parts, sep)
		}

		if output == "" {
			continue
		}

		if ctx.TerminalWidth >= minTruncateWidth {
			output = ansi.Truncate(output, ctx.TerminalWidth, truncateSuffix)
		}

		// Prepend reset so our colors override Claude Code's dim styling.
		// Then replace spaces with non-breaking spaces (U+00A0) to prevent
		// VS Code's integrated terminal from trimming trailing whitespace.
		// ANSI escape sequences do not contain spaces, so this replacement
		// is safe to apply to the full line including escape codes.
		outLine := strings.ReplaceAll(ansiReset+output, " ", "\u00a0")
		fmt.Fprintln(w, outLine)
	}
}

// applyWidgetStyle converts a WidgetResult to a styled string, incorporating
// theme colors from the resolved config theme map.
//
// Color precedence (highest to lowest):
//  1. WidgetResult.FgColor / WidgetResult.BgColor — explicit per-render override
//  2. cfg.ResolvedTheme[widgetName].Fg / .Bg — theme default for this widget
//  3. Widget's own pre-styled ANSI output (FgColor == "" and no theme bg)
//
// When FgColor is empty the Text is returned as-is (the widget pre-styled it
// internally), unless a theme BgColor applies in which case the text is wrapped
// with that background. When FgColor is set, a fresh lipgloss.Style is built
// from FgColor and the resolved BgColor (widget > theme) and applied to Text.
func applyWidgetStyle(r widget.WidgetResult, widgetName string, cfg *config.Config) string {
	// Resolve background: widget result takes precedence over theme.
	bgColor := r.BgColor
	if bgColor == "" {
		if colors, ok := cfg.ResolvedTheme[widgetName]; ok {
			bgColor = colors.Bg
		}
	}

	if r.FgColor == "" {
		// Pre-styled output: only apply bg if theme provides one.
		if bgColor == "" {
			return r.Text
		}
		return lipgloss.NewStyle().Background(lipgloss.Color(bgColor)).Render(r.Text)
	}

	// Structured output: build full style from fg + resolved bg.
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(r.FgColor))
	if bgColor != "" {
		style = style.Background(lipgloss.Color(bgColor))
	}
	return style.Render(r.Text)
}
