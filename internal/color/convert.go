package color

import (
	"strings"
)

// namedANSIColors maps CSS/ANSI color names to their ANSI numeric string
// equivalents. lipgloss.Color() only accepts hex strings or numeric strings;
// named colors like "green" return noColor and produce no styling output.
// This table covers the 16 standard ANSI names plus common aliases advertised
// in the config template and theme override examples.
var namedANSIColors = map[string]string{
	"black":         "0",
	"red":           "1",
	"green":         "2",
	"yellow":        "3",
	"blue":          "4",
	"magenta":       "5",
	"cyan":          "6",
	"white":         "7",
	"brightblack":   "8",
	"gray":          "8",
	"grey":          "8",
	"brightred":     "9",
	"brightgreen":   "10",
	"brightyellow":  "11",
	"brightblue":    "12",
	"brightmagenta": "13",
	"brightcyan":    "14",
	"brightwhite":   "15",
}

// ResolveColorName converts a CSS/ANSI color name to the numeric string
// equivalent expected by lipgloss.Color(). Hex strings ("#rrggbb") and
// numeric strings ("2", "114") pass through unchanged. Unrecognised names
// also pass through, which lets lipgloss handle them (it returns noColor for
// unknown values, which is the same behaviour as before this function existed).
//
// This is the canonical resolver for all color strings in the config system.
// Both the widget and render packages call this before passing config color
// values to lipgloss.Color().
func ResolveColorName(colorName string) string {
	if num, ok := namedANSIColors[strings.ToLower(colorName)]; ok {
		return num
	}
	return colorName
}
