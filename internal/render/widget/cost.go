package widget

import (
	"fmt"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// Cost renders the session cost as a dollar amount. The color shifts from the
// normal context color to warning at cfg.Thresholds.CostWarning USD, and to
// critical at cfg.Thresholds.CostCritical USD.
//
// Returns an empty WidgetResult when SessionCostUSD is zero (no cost data available).
// FgColor is left empty because the widget selects among multiple styles dynamically;
// the renderer passes the pre-styled Text through as-is.
func Cost(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.SessionCostUSD == 0 {
		return WidgetResult{}
	}

	// Resolve thresholds with safe fallbacks.
	warnAt := cfg.Thresholds.CostWarning
	critAt := cfg.Thresholds.CostCritical
	if warnAt <= 0 {
		warnAt = 5.00
	}
	if critAt <= 0 {
		critAt = 10.00
	}

	// Map cost to a 0-100 scale relative to the critical threshold so we
	// can reuse the shared integer-based threshold helpers.
	cost := ctx.SessionCostUSD
	scaledPct := int(cost / critAt * 100)

	// Scale warning threshold to the same 0-100 range.
	scaledWarn := int(warnAt / critAt * 100)

	colors := resolveThresholdColors(cfg)
	activeStyle := contextThresholds(scaledPct, scaledWarn, 100, colors.context, colors.warning, colors.critical)

	plain := fmt.Sprintf("$%.2f", cost)
	fgColor := thresholdFgColor(scaledPct, scaledWarn, 100,
		cfg.Style.Colors.Context, cfg.Style.Colors.Warning, cfg.Style.Colors.Critical)

	return WidgetResult{
		Text:      activeStyle.Render(plain),
		PlainText: plain,
		FgColor:   fgColor,
	}
}
