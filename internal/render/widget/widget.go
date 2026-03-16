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
	"model":       Model,
	"context":     Context,
	"directory":   Directory,
	"git":         Git,
	"project":     Project,
	"env":         Env,
	"duration":    Duration,
	"tools":       Tools,
	"agents":      Agents,
	"todos":       Todos,
	"session":     Session,
	"thinking":    Thinking,
	"tokens":      Tokens,
	"cost":        Cost,
	"lines":       Lines,
	"outputstyle": OutputStyle,
	"messages":    Messages,
	"skills":      Skills,
	"speed":       Speed,
}

// Icons holds the icon strings for a given display mode (nerdfont, unicode, ascii).
type Icons struct {
	Check   string
	Running string // static indicator for a running/in-progress state
	Clock   string
	Folder  string
	Branch  string

	// Tool category icons
	Read     string
	Edit     string
	Shell    string
	Search   string
	Web      string
	Agent    string
	Gear     string
	Thinking string
	Error    string
}

// IconsFor returns the icon set for the given mode string.
// Unrecognised modes fall back to ascii.
func IconsFor(mode string) Icons {
	switch mode {
	case "nerdfont":
		return Icons{
			Check:    "\uf00c",     // nf-fa-check
			Running:  "󰪠",          // half-filled circle: static running indicator
			Clock:    "\uf017",     // nf-fa-clock_o
			Folder:   "\uf07b",     // nf-fa-folder
			Branch:   "\ue0a0",     // nf-pl-branch
			Read:     "\ue28b",     // book
			Edit:     "\uee75",     // pen
			Shell:    "\U000F0BE0", // wrench
			Search:   "\U000F0968", // folder-search
			Web:      "\U000F059F", // web
			Agent:    "\U000F167A", // robot
			Gear:     "\uf013",     // gear
			Thinking: "\uf0eb",     // lightbulb
			Error:    "\uf00d",     // cross
		}
	case "unicode":
		return Icons{
			Check:    "✓",
			Running:  "◐",
			Clock:    "⏱",
			Folder:   "📁",
			Branch:   "⎇",
			Read:     "📖",
			Edit:     "✎",
			Shell:    "⚒",
			Search:   "🔍",
			Web:      "🌐",
			Agent:    "🤖",
			Gear:     "⚙",
			Thinking: "🧠",
			Error:    "✗",
		}
	default: // ascii
		return Icons{
			Check:    "v",
			Running:  "~",
			Clock:    "@",
			Folder:   ">",
			Branch:   "#",
			Read:     "R",
			Edit:     "E",
			Shell:    "$",
			Search:   "?",
			Web:      "W",
			Agent:    "@",
			Gear:     "*",
			Thinking: "~",
			Error:    "!",
		}
	}
}

// CategoryIcon returns the icon for a tool category.
// Recognized categories: "file", "shell", "search", "web", "agent", "internal".
// Unknown categories default to the Gear icon.
func CategoryIcon(icons Icons, category string) string {
	switch category {
	case "file":
		return icons.Read
	case "shell":
		return icons.Shell
	case "search":
		return icons.Search
	case "web":
		return icons.Web
	case "agent":
		return icons.Agent
	case "internal":
		return icons.Gear
	default:
		return icons.Gear
	}
}
