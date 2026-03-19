package widget

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// usageWarningStyle is used for API errors and limit-reached states.
var usageWarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

// Usage renders 5-hour and 7-day rate-limit utilization from the Anthropic
// OAuth API. Each window shows a circle-fill icon, percentage, and reset
// countdown.
//
// The widget returns empty when:
//   - ctx.Usage is nil (no credentials, API user, widget not configured)
//   - Both windows are below their configured thresholds
//
// Display format (nerdfont): "5h 󰪞 25% 1h32m | 7d 󰪣 60% 2d5h"
func Usage(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Usage == nil {
		return WidgetResult{}
	}

	u := ctx.Usage

	// Handle API unavailable state.
	if u.APIUnavailable && u.APIError != "rate-limited" {
		errorHint := formatUsageError(u.APIError)
		label := "Usage " + errorHint
		return WidgetResult{
			Text:      usageWarningStyle.Render(label),
			PlainText: label,
			FgColor:   "3", // yellow
		}
	}

	// Check if either window has hit 100%.
	if u.FiveHourPercent >= 100 || u.SevenDayPercent >= 100 {
		return renderLimitReached(u, cfg)
	}

	// Apply thresholds: hide when both windows are below their threshold.
	fiveThreshold := cfg.Usage.FiveHourThreshold
	sevenThreshold := cfg.Usage.SevenDayThreshold

	effectiveUsage := max(u.FiveHourPercent, u.SevenDayPercent)
	if effectiveUsage < fiveThreshold {
		return WidgetResult{}
	}

	// Build the 5-hour segment.
	var parts []string
	var plainParts []string
	fgColor := ""

	if u.FiveHourPercent >= 0 {
		text, plain, fg := renderUsageWindow("5h", u.FiveHourPercent, u.FiveHourResetAt, cfg)
		parts = append(parts, text)
		plainParts = append(plainParts, plain)
		fgColor = fg
	}

	// Append 7-day segment only when it exceeds the threshold.
	if u.SevenDayPercent >= 0 && u.SevenDayPercent >= sevenThreshold {
		text, plain, fg := renderUsageWindow("7d", u.SevenDayPercent, u.SevenDayResetAt, cfg)
		parts = append(parts, text)
		plainParts = append(plainParts, plain)
		// Use the more critical color of the two windows.
		if usageColorPriority(fg) > usageColorPriority(fgColor) {
			fgColor = fg
		}
	}

	if len(parts) == 0 {
		return WidgetResult{}
	}

	// Append syncing hint when rate-limited but showing stale data.
	syncingSuffix := ""
	if u.APIError == "rate-limited" {
		syncingSuffix = " " + DimStyle.Render("(syncing...)")
	}

	separator := DimStyle.Render(" | ")
	text := strings.Join(parts, separator) + syncingSuffix
	plainText := strings.Join(plainParts, " | ")
	if u.APIError == "rate-limited" {
		plainText += " (syncing...)"
	}

	return WidgetResult{
		Text:      text,
		PlainText: plainText,
		FgColor:   fgColor,
	}
}

// renderUsageWindow renders a single usage window (5h or 7d).
// Returns the styled text, plain text, and foreground color.
func renderUsageWindow(label string, pct int, resetAt time.Time, cfg *config.Config) (string, string, string) {
	if pct < 0 {
		pct = 0
	}

	// Determine threshold color.
	fg := usageFgColor(pct, cfg)
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(fg))

	// Build: "5h 󰪞 25% 1h32m"
	var sb strings.Builder
	var plainSb strings.Builder

	// Label.
	sb.WriteString(DimStyle.Render(label))
	plainSb.WriteString(label)

	// Circle icon (nerdfont only).
	if cfg.Style.Icons == "nerdfont" {
		icon := percentToIcon(pct)
		sb.WriteString(" ")
		sb.WriteString(style.Render(icon))
		plainSb.WriteString(" ")
		plainSb.WriteString(icon)
	}

	// Percentage.
	pctStr := fmt.Sprintf(" %d%%", pct)
	sb.WriteString(style.Render(pctStr))
	plainSb.WriteString(pctStr)

	// Reset countdown.
	if reset := formatResetTime(resetAt); reset != "" {
		resetStr := " " + reset
		sb.WriteString(DimStyle.Render(resetStr))
		plainSb.WriteString(resetStr)
	}

	return sb.String(), plainSb.String(), fg
}

// renderLimitReached renders the limit-reached state with reset countdown.
func renderLimitReached(u *model.UsageInfo, cfg *config.Config) WidgetResult {
	critFg := "1" // red
	if cfg.Style.Colors.Critical != "" {
		critFg = cfg.Style.Colors.Critical
	}
	critStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(critFg)).Bold(true)

	// Determine which window hit the limit first.
	var resetAt time.Time
	if u.FiveHourPercent >= 100 {
		resetAt = u.FiveHourResetAt
	} else {
		resetAt = u.SevenDayResetAt
	}

	label := "Limit reached"
	if reset := formatResetTime(resetAt); reset != "" {
		label += fmt.Sprintf(" (resets %s)", reset)
	}

	text := critStyle.Render(label)
	return WidgetResult{
		Text:      text,
		PlainText: label,
		FgColor:   critFg,
	}
}

// usageFgColor returns the ANSI foreground color for a usage percentage.
// 0-49%: green, 50-79%: yellow, 80-100%: red.
func usageFgColor(pct int, cfg *config.Config) string {
	return thresholdFgColor(pct, 50, 80,
		cfg.Style.Colors.Context, cfg.Style.Colors.Warning, cfg.Style.Colors.Critical)
}

// usageColorPriority returns a numeric priority for color severity
// so we can pick the most urgent color when combining windows.
func usageColorPriority(fg string) int {
	switch fg {
	case "1": // red / critical
		return 3
	case "3": // yellow / warning
		return 2
	default: // green / normal
		return 1
	}
}

// formatResetTime formats a future timestamp as a compact duration string.
// Returns "" when the time is zero or in the past.
func formatResetTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	diff := time.Until(t)
	if diff <= 0 {
		return ""
	}

	totalMins := int(math.Ceil(diff.Minutes()))
	if totalMins < 60 {
		return fmt.Sprintf("%dm", totalMins)
	}

	hours := totalMins / 60
	mins := totalMins % 60

	if hours >= 24 {
		days := hours / 24
		remHours := hours % 24
		if remHours > 0 {
			return fmt.Sprintf("%dd %dh", days, remHours)
		}
		return fmt.Sprintf("%dd", days)
	}

	if mins > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dh", hours)
}

// formatUsageError formats an API error string for display.
func formatUsageError(apiError string) string {
	if apiError == "" {
		return ""
	}
	if apiError == "rate-limited" {
		return "(syncing...)"
	}
	if strings.HasPrefix(apiError, "http-") {
		return "(" + apiError[5:] + ")"
	}
	return "(" + apiError + ")"
}
