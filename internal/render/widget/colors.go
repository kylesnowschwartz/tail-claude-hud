package widget

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// agentColors is the 8-color palette for agent identity, matching tail-claude's team colors.
// Indexed as: blue, green, red, yellow, purple, cyan, orange, pink.
var agentColors = [8]string{"75", "114", "196", "220", "135", "87", "208", "219"}

// AgentColorStyle returns a foreground lipgloss.Style for the given color index.
// The index wraps around the 8-color palette, so any non-negative integer is valid.
func AgentColorStyle(colorIndex int) lipgloss.Style {
	color := agentColors[colorIndex%8]
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

var (
	opusStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	sonnetStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	haikuStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	defaultModelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("87"))
)

// ModelFamily returns the model family name ("opus", "sonnet", "haiku") from
// a model identifier string. Returns "" for unrecognized models.
func ModelFamily(modelName string) string {
	lower := strings.ToLower(modelName)
	switch {
	case strings.Contains(lower, "opus"):
		return "opus"
	case strings.Contains(lower, "sonnet"):
		return "sonnet"
	case strings.Contains(lower, "haiku"):
		return "haiku"
	default:
		return ""
	}
}

// ModelFamilyColor returns a foreground lipgloss.Style based on the Claude model family.
// Detection is case-insensitive via strings.Contains on the lowercased model name:
//   - "opus"   → coral (204)
//   - "sonnet" → blue (75)
//   - "haiku"  → green (114)
//   - default  → cyan (87)
func ModelFamilyColor(modelName string) lipgloss.Style {
	switch ModelFamily(modelName) {
	case "opus":
		return opusStyle
	case "sonnet":
		return sonnetStyle
	case "haiku":
		return haikuStyle
	default:
		return defaultModelStyle
	}
}
