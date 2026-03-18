package preset

import "github.com/kylesnowschwartz/tail-claude-hud/internal/config"

// builtins is the registry of named built-in presets.
var builtins = map[string]Preset{
	"default": {
		Name: "default",
		Lines: []config.Line{
			{Widgets: []string{"model", "context", "project", "todos", "duration"}},
			{Widgets: []string{"agents"}},
			{Widgets: []string{"tools"}},
		},
		Separator:      " | ",
		Icons:          "nerdfont",
		Mode:           "plain",
		Theme:          "default",
		DirectoryStyle: "full",
	},
	"compact": {
		Name: "compact",
		Lines: []config.Line{
			{Widgets: []string{"model", "context", "cost", "duration"}},
		},
		Separator:      " | ",
		Icons:          "nerdfont",
		Mode:           "plain",
		Theme:          "default",
		DirectoryStyle: "basename",
	},
	"detailed": {
		Name: "detailed",
		Lines: []config.Line{
			{Widgets: []string{"model", "context", "cost", "duration", "speed"}},
			{Widgets: []string{"directory", "git", "lines", "outputstyle"}},
			{Widgets: []string{"agents", "messages", "skills"}},
			{Widgets: []string{"tools"}},
		},
		Separator:      " | ",
		Icons:          "nerdfont",
		Mode:           "plain",
		Theme:          "default",
		DirectoryStyle: "fish",
	},
	"powerline": {
		Name: "powerline",
		Lines: []config.Line{
			// Line 1: identity bar with powerline-style arrow transitions between segments.
			// Per-line mode overrides the global mode so line 2 can use plain mode.
			{
				Widgets: []string{"model", "context", "project", "git", "cost", "duration"},
				Mode:    "powerline",
			},
			// Line 2: activity feed in plain mode — powerline style is too
			// heavy for rapidly-changing tool output.
			{Widgets: []string{"tools"}},
		},
		Separator:      " | ",
		Icons:          "nerdfont",
		Mode:           "plain", // default for lines without explicit mode
		Theme:          "dark",
		DirectoryStyle: "basename",
	},
	"minimal": {
		Name: "minimal",
		Lines: []config.Line{
			{Widgets: []string{"model", "context", "duration"}},
		},
		Separator:      " ",
		Icons:          "nerdfont",
		Mode:           "minimal",
		Theme:          "default",
		DirectoryStyle: "basename",
	},
}
