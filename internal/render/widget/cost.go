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
// Returns "" when SessionCostUSD is zero (no cost data available).
func Cost(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.SessionCostUSD == 0 {
		return ""
	}

	// Resolve colors: prefer config overrides, fall back to package-level defaults.
	contextColor := colorStyle(cfg.Style.Colors.Context, greenStyle)
	warningColor := colorStyle(cfg.Style.Colors.Warning, yellowStyle)
	criticalColor := colorStyle(cfg.Style.Colors.Critical, redStyle)

	// Resolve thresholds with safe fallbacks.
	warnAt := cfg.Thresholds.CostWarning
	critAt := cfg.Thresholds.CostCritical
	if warnAt <= 0 {
		warnAt = 5.00
	}
	if critAt <= 0 {
		critAt = 10.00
	}

	cost := ctx.SessionCostUSD
	activeStyle := contextColor
	if cost >= critAt {
		activeStyle = criticalColor
	} else if cost >= warnAt {
		activeStyle = warningColor
	}

	return activeStyle.Render(fmt.Sprintf("$%.2f", cost))
}
