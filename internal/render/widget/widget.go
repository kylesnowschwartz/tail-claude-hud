// Package widget provides the registry of render functions and shared icon helpers.
// Each widget is a RenderFunc — a function that receives a RenderContext and Config
// and returns a lipgloss-styled string. Widgets return an empty string when they
// have nothing to display, so callers can filter them out before joining with separators.
package widget

import (
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// RenderFunc renders a single widget segment.
// Returns empty string when the widget has nothing to display.
type RenderFunc func(ctx *model.RenderContext, cfg *config.Config) string

// Registry maps widget names to their render functions.
var Registry = map[string]RenderFunc{
	"model":     Model,
	"context":   Context,
	"directory": Directory,
	"git":       Git,
	"env":       Env,
	"duration":  Duration,
	"usage":     Usage,
	"tools":     Tools,
	"agents":    Agents,
	"todos":     Todos,
}

// Icons holds the icon strings for a given display mode (nerdfont, unicode, ascii).
type Icons struct {
	Check   string
	Spinner string
	Clock   string
	Folder  string
	Branch  string
}

// IconsFor returns the icon set for the given mode string.
// Unrecognised modes fall back to ascii.
func IconsFor(mode string) Icons {
	switch mode {
	case "nerdfont":
		return Icons{
			Check:   "\uf00c", // nf-fa-check
			Spinner: "\uf110", // nf-fa-spinner
			Clock:   "\uf017", // nf-fa-clock_o
			Folder:  "\uf07b", // nf-fa-folder
			Branch:  "\ue0a0", // nf-pl-branch
		}
	case "unicode":
		return Icons{
			Check:   "✓",
			Spinner: "◐",
			Clock:   "⏱",
			Folder:  "📁",
			Branch:  "⎇",
		}
	default: // ascii
		return Icons{
			Check:   "v",
			Spinner: "*",
			Clock:   "@",
			Folder:  ">",
			Branch:  "#",
		}
	}
}
