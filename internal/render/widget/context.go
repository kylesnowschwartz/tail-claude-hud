package widget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/color"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var (
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// colorStyle returns a lipgloss.Style using the given color string. If colorName
// is empty, the fallback style is returned unchanged. This lets callers apply
// config-driven color overrides without breaking the default palette.
// Named ANSI colors (e.g. "green", "red", "yellow") are resolved to their
// numeric equivalents so they render correctly in all terminal modes.
func colorStyle(colorName string, fallback lipgloss.Style) lipgloss.Style {
	if colorName == "" {
		return fallback
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color.ResolveColorName(colorName)))
}

// thresholdColors holds the three resolved lipgloss styles for threshold-based
// coloring. Used by widgets that color-shift based on a metric level.
type thresholdColors struct {
	context  lipgloss.Style
	warning  lipgloss.Style
	critical lipgloss.Style
}

// resolveThresholdColors resolves the three threshold colors from config,
// falling back to the default green/yellow/red ANSI palette.
func resolveThresholdColors(cfg *config.Config) thresholdColors {
	return thresholdColors{
		context:  colorStyle(cfg.Style.Colors.Context, greenStyle),
		warning:  colorStyle(cfg.Style.Colors.Warning, yellowStyle),
		critical: colorStyle(cfg.Style.Colors.Critical, redStyle),
	}
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

// thresholdFgColor returns the ANSI color string for threshold-based coloring.
// Uses config overrides when set, otherwise returns the default green/yellow/red.
// Config values are passed through resolveColor so named ANSI colors (e.g.
// "green") are converted to their numeric equivalents before the caller passes
// them to lipgloss.Color().
func thresholdFgColor(pct, warnAt, critAt int, cfgContext, cfgWarning, cfgCritical string) string {
	// Determine the effective color string for each tier.
	greenFg := "2"
	if cfgContext != "" {
		greenFg = color.ResolveColorName(cfgContext)
	}
	yellowFg := "3"
	if cfgWarning != "" {
		yellowFg = color.ResolveColorName(cfgWarning)
	}
	redFg := "1"
	if cfgCritical != "" {
		redFg = color.ResolveColorName(cfgCritical)
	}

	switch {
	case pct >= critAt:
		return redFg
	case pct >= warnAt:
		return yellowFg
	default:
		return greenFg
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
// Returns an empty WidgetResult when both ContextPercent and ContextWindowSize are zero.
// FgColor is left empty because the widget composes multiple styles internally;
// the renderer passes the pre-styled Text through as-is.
func Context(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.ContextPercent == 0 && ctx.ContextWindowSize == 0 {
		return WidgetResult{}
	}

	barWidth := cfg.Context.BarWidth
	if barWidth <= 0 {
		barWidth = 10
	}

	pct := ctx.ContextPercent

	colors := resolveThresholdColors(cfg)

	// Resolve thresholds with safe fallbacks.
	warnAt := cfg.Thresholds.ContextWarning
	critAt := cfg.Thresholds.ContextCritical
	if warnAt <= 0 {
		warnAt = 70
	}
	if critAt <= 0 {
		critAt = 85
	}

	activeStyle := contextThresholds(pct, warnAt, critAt, colors.context, colors.warning, colors.critical)
	if pct >= warnAt {
		activeStyle = activeStyle.Bold(true)
	}

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
	var text string
	switch cfg.Context.Display {
	case "bar":
		text = activeStyle.Render(renderBar(pct, barWidth))
	case "both":
		bar := activeStyle.Render(renderBar(pct, barWidth))
		text = bar + " " + activeStyle.Render(label)
	default: // "text" or empty
		text = activeStyle.Render(label)
	}

	// Prepend a circle-slice Nerd Font icon that fills proportionally with usage.
	if cfg.Style.Icons == "nerdfont" {
		text = text + " " + percentToIcon(pct)
	}

	// Build PlainText: the unstyled version for powerline/minimal rendering.
	var plainText string
	switch cfg.Context.Display {
	case "bar":
		plainText = renderBar(pct, barWidth)
	case "both":
		plainText = renderBar(pct, barWidth) + " " + label
	default:
		plainText = label
	}
	if cfg.Style.Icons == "nerdfont" {
		plainText = plainText + " " + percentToIcon(pct)
	}

	// Append token breakdown when context exceeds the critical threshold and breakdown is enabled.
	if pct > critAt && cfg.Context.ShowBreakdown {
		breakdown := fmt.Sprintf(" in:%s cr:%s rd:%s",
			formatTokenCount(ctx.InputTokens),
			formatTokenCount(ctx.CacheCreation),
			formatTokenCount(ctx.CacheRead),
		)
		text += DimStyle.Render(breakdown)
		plainText += breakdown
	}

	fgColor := thresholdFgColor(pct, warnAt, critAt,
		cfg.Style.Colors.Context, cfg.Style.Colors.Warning, cfg.Style.Colors.Critical)

	return WidgetResult{
		Text:      text,
		PlainText: plainText,
		FgColor:   fgColor,
	}
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
