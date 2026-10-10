// Package widget provides the registry of render functions and shared icon helpers.
// Each widget is a RenderFunc — a function that receives a RenderContext and Config
// and returns a WidgetResult. Widgets return an empty WidgetResult when they have
// nothing to display, so callers can filter them out before joining with separators.
package widget

import (
	"slices"
	"strings"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

// WidgetResult holds structured output from a widget render.
//
// Widgets populate two representations of their content:
//   - Text: pre-styled ANSI string for plain mode (rendered with lipgloss internally)
//   - PlainText: unstyled content for powerline/minimal modes, styled by the renderer
//
// FgColor tells the renderer which foreground color to apply to PlainText.
// BgColor is an optional explicit background override (powerline bg normally
// comes from the theme; this field overrides it when set).
type WidgetResult struct {
	Text      string // pre-styled ANSI string (for plain mode)
	PlainText string // unstyled text (for powerline/minimal modes)
	FgColor   string // foreground color (lipgloss color string) for PlainText
	BgColor   string // background color (lipgloss color string); empty means use theme

	// StateColor is set when the widget is in a state that demands its own
	// color (a warning or critical threshold). It wins over a theme fg
	// override, which is the widget's identity color for its normal state.
	StateColor string
}

// IsEmpty reports whether the result has no content to display.
func (w WidgetResult) IsEmpty() bool {
	return w.Text == "" && w.PlainText == ""
}

// RenderFunc renders a single widget segment.
// Returns a WidgetResult with empty Text when the widget has nothing to display.
type RenderFunc func(ctx *model.RenderContext, cfg *config.Config) WidgetResult

// Registry maps widget names to their render functions.
var Registry = map[string]RenderFunc{
	"model":       Model,
	"context":     Context,
	"cost":        Cost,
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
	"lines":       Lines,
	"outputstyle": OutputStyle,
	"messages":    Messages,
	"skills":      Skills,
	"speed":       Speed,
	"permission":  Permission,
	"usage":       Usage,
	"worktree":    Worktree,
	"effort":      Effort,
	"peers":       Peers,
	"exec":        Exec,
	"cachettl":    CacheTTL,
}

// Icons holds the icon strings for a given display mode (nerdfont, unicode, ascii).
type Icons struct {
	Check   string
	Running string // static indicator for a running/in-progress state
	Clock   string
	Folder  string
	Branch  string

	// Per-tool category icons (one per tool category).
	Read       string
	Edit       string
	Write      string
	Bash       string
	Grep       string
	Glob       string
	Web        string
	Task       string
	Skill      string
	Thinking   string
	Other      string // fallback for unrecognized tools
	Error      string
	Permission string // bell icon for permission-waiting alert
}

// IconsFor returns the icon set for the given mode string.
// Unrecognised modes fall back to ascii.
func IconsFor(mode string) Icons {
	switch mode {
	case "nerdfont":
		return Icons{
			Check:      "\uf00c",     // nf-fa-check
			Running:    "\U000F051F", // nf-md-timer_sand: static running indicator
			Clock:      "\uf017",     // nf-fa-clock_o
			Folder:     "\uf07b",     // nf-fa-folder
			Branch:     "",          // nf-pl-branch
			Read:       "",          // nf-fae-book_open_o
			Edit:       "\uee75",     // nf-fa-pen_nib
			Write:      "\uee75",     // nf-fa-pen_nib (same glyph as Edit)
			Bash:       "\U000F0BE0", // nf-md-wrench_outline
			Grep:       "\U000F0968", // nf-md-folder_search
			Glob:       "\U000F0968", // nf-md-folder_search (same glyph as Grep)
			Web:        "\U000F059F", // nf-md-web
			Task:       "\U000F167A", // nf-md-robot_outline
			Skill:      "\uf0d0",     // nf-fa-magic
			Thinking:   "\uf0eb",     // nf-fa-lightbulb
			Other:      "\uf013",     // nf-fa-gear
			Error:      "\uf00d",     // nf-fa-cross
			Permission: "󰅸",          // nf-md-bell-alert
		}
	case "unicode":
		// One-column symbols with no emoji presentation, identical to the
		// tch-activity mod's built-in look (mods/tch-activity/hooks/style.ts).
		return Icons{
			Check:      "✓",
			Running:    "⧗",
			Clock:      "◷",
			Folder:     "⌂",
			Branch:     "⎇",
			Read:       "≡",
			Edit:       "✎",
			Write:      "✎",
			Bash:       "$",
			Grep:       "⌕",
			Glob:       "⌕",
			Web:        "◎",
			Task:       "◆",
			Skill:      "✦",
			Thinking:   "∴",
			Other:      "⚙",
			Error:      "✗",
			Permission: "!",
		}
	default: // ascii
		return Icons{
			Check:      "v",
			Running:    "~",
			Clock:      "@",
			Folder:     ">",
			Branch:     "#",
			Read:       "R",
			Edit:       "E",
			Write:      "W",
			Bash:       "$",
			Grep:       "?",
			Glob:       "?",
			Web:        "W",
			Task:       "@",
			Skill:      "&",
			Thinking:   "~",
			Other:      "*",
			Error:      "!",
			Permission: "!",
		}
	}
}

// CategoryIcon returns the icon for a tool category.
// Each tool category maps to its own icon so Read, Edit, and Write are
// visually distinct. Unknown categories fall back to Other.
func CategoryIcon(icons Icons, category string) string {
	switch category {
	case "Read":
		return icons.Read
	case "Edit":
		return icons.Edit
	case "Write":
		return icons.Write
	case "Bash":
		return icons.Bash
	case "Grep":
		return icons.Grep
	case "Glob":
		return icons.Glob
	case "Web":
		return icons.Web
	case "Task":
		return icons.Task
	case "Skill":
		return icons.Skill
	case "Thinking":
		return icons.Thinking
	default:
		return icons.Other
	}
}

// ByKey returns every icon keyed by its lower-camel field name ("check",
// "running", "read", ...). CategoryIconKey returns keys from this map.
func (i Icons) ByKey() map[string]string {
	return map[string]string{
		"check":      i.Check,
		"running":    i.Running,
		"clock":      i.Clock,
		"folder":     i.Folder,
		"branch":     i.Branch,
		"read":       i.Read,
		"edit":       i.Edit,
		"write":      i.Write,
		"bash":       i.Bash,
		"grep":       i.Grep,
		"glob":       i.Glob,
		"web":        i.Web,
		"task":       i.Task,
		"skill":      i.Skill,
		"thinking":   i.Thinking,
		"other":      i.Other,
		"error":      i.Error,
		"permission": i.Permission,
	}
}

// IconCategories lists the tool categories CategoryIcon gives a dedicated
// icon. Every other category renders with the "other" icon.
var IconCategories = []string{"Read", "Edit", "Write", "Bash", "Grep", "Glob", "Web", "Task", "Skill", "Thinking"}

// CategoryIconKey returns the ByKey key of the icon CategoryIcon draws for a
// tool category.
func CategoryIconKey(category string) string {
	if slices.Contains(IconCategories, category) {
		return strings.ToLower(category)
	}
	return "other"
}
