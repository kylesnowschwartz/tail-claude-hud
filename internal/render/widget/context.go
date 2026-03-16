package widget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var (
	dimStyle    = lipgloss.NewStyle().Faint(true)
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// colorStyle returns a lipgloss.Style using the given color string. If colorName
// is empty, the fallback style is returned unchanged. This lets callers apply
// config-driven color overrides without breaking the default palette.
func colorStyle(colorName string, fallback lipgloss.Style) lipgloss.Style {
	if colorName == "" {
		return fallback
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorName))
}

// contextThresholds returns the color style for the given usage percentage,
// selecting from the three provided styles based on the configured thresholds.
// warnAt and critAt are the percentage values at which colors shift.
func contextThresholds(pct, warnAt, critAt int, contextColor, warningColor, criticalColor lipgloss.Style) lipgloss.Style {
	switch {
	case pct >= critAt:
		return criticalColor
	case pct >= warnAt:
		return warningColor
	default:
		return contextColor
	}
}

// renderBar builds a block-character progress bar of the given width.
// Filled cells use █ and empty cells use ░.
// Example for 40% at width 10: "████░░░░░░"
func renderBar(pct, width int) string {
	if width <= 0 {
		width = 10
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := (pct * width) / 100
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// Context renders context window usage as a styled label, a block-character
// progress bar, or both, depending on cfg.Context.Display.
//
// Display modes (cfg.Context.Display):
//   - "text" (default): show the value label only
//   - "bar": show the progress bar only
//   - "both": show the bar followed by the value label
//
// The color shifts at configurable thresholds: normal below cfg.Thresholds.ContextWarning,
// warning between the two thresholds, critical at cfg.Thresholds.ContextCritical and above.
//
// The label format is controlled by cfg.Context.Value:
//   - "percent" (default): "42%"
//   - "tokens": "84k/200k"
//   - "remaining": "116k left"
//
// When context exceeds the critical threshold and cfg.Context.ShowBreakdown is
// true, a token breakdown is appended: " in:84k cr:12k rd:8k".
//
// Returns "" when both ContextPercent and ContextWindowSize are zero.
func Context(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.ContextPercent == 0 && ctx.ContextWindowSize == 0 {
		return ""
	}

	barWidth := cfg.Context.BarWidth
	if barWidth <= 0 {
		barWidth = 10
	}

	pct := ctx.ContextPercent

	// Resolve colors: prefer config overrides, fall back to package-level defaults.
	contextColor := colorStyle(cfg.Style.Colors.Context, greenStyle)
	warningColor := colorStyle(cfg.Style.Colors.Warning, yellowStyle)
	criticalColor := colorStyle(cfg.Style.Colors.Critical, redStyle)

	// Resolve thresholds with safe fallbacks.
	warnAt := cfg.Thresholds.ContextWarning
	critAt := cfg.Thresholds.ContextCritical
	if warnAt <= 0 {
		warnAt = 70
	}
	if critAt <= 0 {
		critAt = 85
	}

	activeStyle := contextThresholds(pct, warnAt, critAt, contextColor, warningColor, criticalColor)

	// Compute token totals used by "tokens" and "remaining" modes.
	used := ctx.InputTokens + ctx.CacheCreation + ctx.CacheRead
	total := ctx.ContextWindowSize

	// Build the value label based on the configured mode.
	var label string
	switch cfg.Context.Value {
	case "tokens":
		label = fmt.Sprintf("%s/%s", formatTokenCount(used), formatTokenCount(total))
	case "remaining":
		remaining := total - used
		label = fmt.Sprintf("%s left", formatTokenCount(remaining))
	default: // "percent" or empty
		label = fmt.Sprintf("%d%%", pct)
	}

	// Assemble the output according to the display mode.
	var result string
	switch cfg.Context.Display {
	case "bar":
		result = activeStyle.Render(renderBar(pct, barWidth))
	case "both":
		bar := activeStyle.Render(renderBar(pct, barWidth))
		result = bar + " " + activeStyle.Render(label)
	default: // "text" or empty
		result = activeStyle.Render(label)
	}

	// Append token breakdown when context exceeds the critical threshold and breakdown is enabled.
	if pct > critAt && cfg.Context.ShowBreakdown {
		breakdown := fmt.Sprintf(" in:%s cr:%s rd:%s",
			formatTokenCount(ctx.InputTokens),
			formatTokenCount(ctx.CacheCreation),
			formatTokenCount(ctx.CacheRead),
		)
		result += dimStyle.Render(breakdown)
	}

	return result
}

// formatTokenCount formats a token count into a compact human-readable string:
//   - < 1000: "123"
//   - < 100000: "12.3k" (one decimal place)
//   - >= 100000: "123k" (no decimal)
func formatTokenCount(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 100000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%dk", n/1000)
	}
}
