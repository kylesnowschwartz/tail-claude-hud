package widget

import (
	"fmt"
	"strings"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

const maxVisibleTools = 5

// Tools renders running and recently-completed tool invocations as a HUD activity feed.
// Running tools show a category icon + name + elapsed indicator in yellow.
// Completed tools show a dim category icon + name + duration.
// Error tools show the error icon + name + duration + "err" in red.
// Returns "" when ctx.Transcript is nil or there are no tools to show.
func Tools(ctx *model.RenderContext, cfg *config.Config) string {
	if ctx.Transcript == nil {
		return ""
	}

	icons := IconsFor(cfg.Style.Icons)
	tools := ctx.Transcript.Tools

	if len(tools) == 0 {
		return ""
	}

	// Separate running (Count==0) from completed/error (Count>0).
	var running []model.ToolEntry
	var completed []model.ToolEntry
	for _, t := range tools {
		if t.Count == 0 {
			running = append(running, t)
		} else {
			completed = append(completed, t)
		}
	}

	// Build the visible list: running tools take priority, then completed newest-first.
	reversed := make([]model.ToolEntry, len(completed))
	for i, t := range completed {
		reversed[len(completed)-1-i] = t
	}

	var visible []model.ToolEntry
	visible = append(visible, running...)
	visible = append(visible, reversed...)
	if len(visible) > maxVisibleTools {
		visible = visible[:maxVisibleTools]
	}

	var parts []string
	for _, t := range visible {
		parts = append(parts, renderToolEntry(icons, t))
	}

	return strings.Join(parts, " | ")
}

// renderToolEntry formats a single tool entry according to its state.
func renderToolEntry(icons Icons, t model.ToolEntry) string {
	catIcon := CategoryIcon(icons, t.Category)

	if t.Count == 0 {
		// Running: yellow category icon + name + elapsed indicator.
		icon := yellowStyle.Render(catIcon)
		name := yellowStyle.Render(t.Name)
		elapsed := yellowStyle.Render("...")
		if t.DurationMs > 0 {
			elapsed = yellowStyle.Render(formatDuration(t.DurationMs))
		}
		return fmt.Sprintf("%s %s %s", icon, name, elapsed)
	}

	if t.HasError {
		// Error: red error icon + name + duration + "err".
		icon := redStyle.Render(icons.Error)
		name := redStyle.Render(t.Name)
		dur := redStyle.Render(formatDuration(t.DurationMs))
		suffix := redStyle.Render("err")
		return fmt.Sprintf("%s %s %s %s", icon, name, dur, suffix)
	}

	// Completed: dim category icon + name + duration.
	icon := dimStyle.Render(catIcon)
	name := dimStyle.Render(t.Name)
	dur := dimStyle.Render(formatDuration(t.DurationMs))
	return fmt.Sprintf("%s %s %s", icon, name, dur)
}

// formatDuration converts a millisecond duration into a compact human-readable string.
//
//   - < 1000ms:           "0.Xs"  (tenths of a second)
//   - 1000ms – 59999ms:   "Xs" or "X.Ys" (seconds, optional tenth)
//   - >= 60000ms:          "Xm Ys"
func formatDuration(ms int) string {
	if ms <= 0 {
		return "0.0s"
	}
	if ms < 1000 {
		return fmt.Sprintf("0.%ds", ms/100)
	}
	if ms < 60000 {
		secs := ms / 1000
		frac := (ms % 1000) / 100
		if frac == 0 {
			return fmt.Sprintf("%ds", secs)
		}
		return fmt.Sprintf("%d.%ds", secs, frac)
	}
	mins := ms / 60000
	secs := (ms % 60000) / 1000
	return fmt.Sprintf("%dm %ds", mins, secs)
}

// formatTokenCost formats a token count as a compact cost string:
//   - < 1000:    printed as-is (e.g. "500")
//   - 1000–99999: "X.Yk" (one decimal place)
//   - >= 100000:  "Xk" (no decimal)
func formatTokenCost(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 100000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%dk", n/1000)
}
