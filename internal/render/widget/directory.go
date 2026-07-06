package widget

import (
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

var dirStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)

// Directory renders the current working directory from ctx.Cwd using the
// style configured in cfg.Directory.Style:
//
//   - "full"     — last N segments (cfg.Directory.Levels), default
//   - "fish"     — all segments except the last abbreviated to first char
//   - "basename" — last segment only, ignores cfg.Directory.Levels
//
// The home directory is always substituted with "~" before applying the style.
// Returns an empty WidgetResult when ctx.Cwd is empty.
// FgColor is set: the widget is single-color, so a theme fg override may
// re-render PlainText (losing dirStyle's bold, which themes don't model).
func Directory(ctx *model.RenderContext, cfg *config.Config) WidgetResult {
	if ctx.Cwd == "" {
		return WidgetResult{}
	}

	path := substituteHome(ctx.Cwd)

	style := cfg.Directory.Style
	if style == "" {
		style = "full"
	}

	var display string
	switch style {
	case "fish":
		display = abbreviateFish(path)
	case "basename":
		display = filepath.Base(path)
	default: // "full" or unrecognized
		levels := cfg.Directory.Levels
		if levels <= 0 {
			levels = 1
		}
		display = lastNSegments(path, levels)
	}

	return WidgetResult{
		Text:      dirStyle.Render(display),
		PlainText: display,
		FgColor:   "13",
	}
}

// substituteHome replaces the user's home directory prefix with "~".
// If the home directory cannot be determined, the path is returned unchanged.
func substituteHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	// Ensure we only match a full path segment, not a prefix of a directory name.
	prefix := strings.TrimRight(home, "/")
	if path == prefix {
		return "~"
	}
	if strings.HasPrefix(path, prefix+"/") {
		return "~" + path[len(prefix):]
	}
	return path
}

// abbreviateFish abbreviates all path segments except the last to their first
// character, mirroring fish shell's prompt_pwd behaviour. The "~" segment and
// any empty leading segment (from an absolute path) are left intact.
//
// Examples:
//
//	~/Code/my-projects/tail-claude-hud → ~/C/m/tail-claude-hud
//	/usr/local/bin                      → /u/l/bin
func abbreviateFish(path string) string {
	parts := strings.Split(path, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "~" || parts[i] == "" {
			continue
		}
		if len(parts[i]) > 0 {
			parts[i] = string([]rune(parts[i])[:1])
		}
	}
	return strings.Join(parts, "/")
}

// lastNSegments returns the last n path segments from a slash-delimited path,
// joined with "/". Trailing slashes are trimmed before splitting.
func lastNSegments(path string, n int) string {
	path = strings.TrimRight(path, "/")
	if path == "" {
		return ""
	}

	parts := strings.Split(path, "/")

	// Remove any empty leading segment that results from an absolute path.
	if len(parts) > 0 && parts[0] == "" {
		parts = parts[1:]
	}

	if len(parts) == 0 {
		return ""
	}

	if n >= len(parts) {
		return strings.Join(parts, "/")
	}

	return strings.Join(parts[len(parts)-n:], "/")
}
