// Package styleexport describes the HUD's resolved look (icons, colors, agent
// palette, tool categories) as JSON for programs that draw HUD-style output
// without linking Go, such as a Claude Code mod. It reads the same config,
// theme, and widget definitions the statusline renders with. The caller
// applies the same --preset and --theme the statusline command uses; the
// statusline's automatic light theme for powerline layouts on a light
// terminal background is not applied, since the export has no terminal to
// query.
//
// Colors are emitted in the mod color syntax: "ansi256(N)" for terminal
// palette indices and "#rrggbb" for hex. An empty string means "no color":
// draw with the terminal's default foreground.
package styleexport

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/color"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/extract"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render/widget"
)

// SchemaVersion changes whenever a field is removed or changes meaning.
// Adding fields does not bump it.
const SchemaVersion = 1

// Export is the JSON document printed by `tail-claude-hud style --json`.
type Export struct {
	Schema  int    `json:"schema"`
	Version string `json:"version"`

	// IconMode is the configured [style] icons value: nerdfont, unicode, or
	// ascii (any other value renders as ascii).
	IconMode string `json:"iconMode"`
	// Icons maps lower-camel icon keys (check, running, read, ...) to glyphs.
	Icons map[string]string `json:"icons"`

	Colors Colors `json:"colors"`
	// WidgetColors is the per-widget fg/bg theme after merging the built-in
	// theme, [theme.overrides], and theme-active.toml.
	WidgetColors map[string]WidgetColors `json:"widgetColors"`
	// AgentColors is the agent identity palette in assignment order: the
	// N-th subagent (0-based) uses AgentColors[N % len(AgentColors)].
	AgentColors []string `json:"agentColors"`

	ToolCategories ToolCategories `json:"toolCategories"`
}

// Colors are the semantic colors of the activity lines (tools, agents,
// thinking) and the threshold tiers.
type Colors struct {
	// Running colors a running tool's icon and name (drawn bold). Running
	// agents use their AgentColors entry instead.
	Running string `json:"running"`
	// Success colors a completed tool's icon and a completed agent's check.
	Success string `json:"success"`
	// Error colors a failed tool's icon, name, and duration.
	Error string `json:"error"`
	// Thinking colors the thinking icon while a thinking block is active.
	Thinking string `json:"thinking"`
	// Permission colors the alert for another session waiting on approval.
	Permission string `json:"permission"`
	// Muted is static metadata text. Completed tool names, durations, and
	// separators use the terminal's faint attribute instead of a color; a mod
	// without a faint style should use Muted for them.
	Muted string `json:"muted"`

	// Normal, Warning, and Critical are the threshold tiers for context,
	// cost, and usage, from [style.colors].
	Normal   string `json:"normal"`
	Warning  string `json:"warning"`
	Critical string `json:"critical"`
}

// WidgetColors is one widget's theme entry.
type WidgetColors struct {
	Fg string `json:"fg"`
	Bg string `json:"bg"`
}

// ToolCategories describes how the tools widget picks an icon for a tool.
type ToolCategories struct {
	// ByName maps known tool names to their category.
	ByName map[string]string `json:"byName"`
	// Default is the category of any tool name absent from ByName, including
	// MCP tools (mcp__server__tool).
	Default string `json:"default"`
	// Icons maps each category to its key in the top-level icons map.
	// Categories absent here use the "other" icon.
	Icons map[string]string `json:"icons"`
}

// knownToolNames are the Claude Code tool names whose category is not the
// default. Each one is categorized through extract.ToolCategory, the function
// the transcript extractor uses, so only the name list lives here.
var knownToolNames = []string{
	"Read", "Edit", "Write", "NotebookEdit",
	"Bash", "Grep", "Glob",
	"WebFetch", "WebSearch",
	"Task", "Agent", "Skill", "Workflow",
}

// unknownToolProbe is a name no tool uses, so its category is the default.
const unknownToolProbe = "mcp__styleexport__probe"

// Build assembles the export for cfg. version is the binary's version string.
func Build(cfg *config.Config, version string) Export {
	icons := widget.IconsFor(cfg.Style.Icons)
	thresholds := widget.ResolveThresholdFgColors(
		cfg.Style.Colors.Context, cfg.Style.Colors.Warning, cfg.Style.Colors.Critical)

	widgetColors := make(map[string]WidgetColors, len(cfg.ResolvedTheme))
	for name, c := range cfg.ResolvedTheme {
		widgetColors[name] = WidgetColors{Fg: ModColor(c.Fg), Bg: ModColor(c.Bg)}
	}

	palette := widget.AgentPalette()
	agentColors := make([]string, len(palette))
	for i, c := range palette {
		agentColors[i] = ModColor(c)
	}

	return Export{
		Schema:   SchemaVersion,
		Version:  version,
		IconMode: cfg.Style.Icons,
		Icons:    icons.ByKey(),
		Colors: Colors{
			Running:    ModColor(widget.YellowColor),
			Success:    ModColor(widget.GreenColor),
			Error:      ModColor(widget.RedColor),
			Thinking:   ModColor(widget.YellowColor),
			Permission: ModColor(widget.RedColor),
			Muted:      ModColor(widget.MutedColor),
			Normal:     ModColor(thresholds.Normal),
			Warning:    ModColor(thresholds.Warning),
			Critical:   ModColor(thresholds.Critical),
		},
		WidgetColors:   widgetColors,
		AgentColors:    agentColors,
		ToolCategories: toolCategories(),
	}
}

func toolCategories() ToolCategories {
	byName := make(map[string]string, len(knownToolNames))
	for _, name := range knownToolNames {
		byName[name] = extract.ToolCategory(name)
	}

	categoryIcons := make(map[string]string, len(widget.IconCategories))
	for _, category := range widget.IconCategories {
		categoryIcons[category] = widget.CategoryIconKey(category)
	}

	return ToolCategories{
		ByName:  byName,
		Default: extract.ToolCategory(unknownToolProbe),
		Icons:   categoryIcons,
	}
}

// Write encodes the export for cfg as indented JSON.
func Write(w io.Writer, cfg *config.Config, version string) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(Build(cfg, version))
}

// ModColor converts a config or lipgloss color string to mod color syntax.
// ANSI names ("red", "brightblue") and indices 0-255 become "ansi256(N)";
// hex strings become lowercase "#rrggbb". Empty and unrecognized values return "", matching
// lipgloss, which draws them with no color.
func ModColor(c string) string {
	c = strings.TrimSpace(color.ResolveColorName(c))
	if c == "" {
		return ""
	}
	if digits, ok := strings.CutPrefix(c, "#"); ok {
		if !isHexColor(digits) {
			return ""
		}
		if len(digits) == 3 {
			digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
		}
		return "#" + strings.ToLower(digits)
	}
	n, err := strconv.Atoi(c)
	if err != nil || n < 0 || n > 255 {
		return ""
	}
	return "ansi256(" + strconv.Itoa(n) + ")"
}

// isHexColor reports whether digits is a 3- or 6-digit hex color body.
func isHexColor(digits string) bool {
	if len(digits) != 3 && len(digits) != 6 {
		return false
	}
	for _, r := range digits {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}
