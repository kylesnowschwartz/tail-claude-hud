// Package theme provides named color theme presets for the HUD statusline.
// Themes map widget names to fg/bg color pairs. The renderer applies bg colors
// as a wrapper; widgets apply their own fg colors by reading from the resolved
// config theme map.
//
// Built-in themes ship as Go maps so they require no external files and are
// always available regardless of the user's config directory.
package theme

// WidgetColors holds the foreground and background color for a single widget.
// Colors are CSS hex strings (e.g. "#4c566a") or ANSI 256-color numbers
// (e.g. "75"). Empty string means "no color set — use widget default".
type WidgetColors struct {
	Fg string `toml:"fg"`
	Bg string `toml:"bg"`
}

// Theme maps widget names to their colors.
type Theme map[string]WidgetColors

// builtins is the registry of built-in named themes. The key is the theme name
// as it appears in the config file (e.g. theme = "nord").
var builtins = map[string]Theme{
	"default":     defaultTheme,
	"dark":        darkTheme,
	"nord":        nordTheme,
	"gruvbox":     gruvboxTheme,
	"tokyo-night": tokyoNightTheme,
	"rose-pine":   rosePineTheme,
}

// defaultTheme uses the existing hardcoded widget colors as its baseline.
// fg colors match what widgets currently apply via lipgloss; bg is empty
// (transparent) so the terminal default applies.
var defaultTheme = Theme{
	"model":     {Fg: "87", Bg: ""}, // cyan
	"context":   {Fg: "42", Bg: ""}, // green (normal usage)
	"directory": {Fg: "110", Bg: ""},
	"git":       {Fg: "87", Bg: ""}, // cyan
	"project":   {Fg: "75", Bg: ""}, // blue
	"env":       {Fg: "135", Bg: ""},
	"duration":  {Fg: "244", Bg: ""},
	"tools":     {Fg: "75", Bg: ""},
	"agents":    {Fg: "114", Bg: ""},
	"todos":     {Fg: "220", Bg: ""},
	"session":   {Fg: "87", Bg: ""},
	"thinking":  {Fg: "220", Bg: ""},
}

// darkTheme is a high-contrast dark terminal palette derived from claude-powerline's
// dark theme, adjusted to ANSI 256-color indices for broad terminal compatibility.
var darkTheme = Theme{
	"model":     {Fg: "#ffffff", Bg: "#2d2d2d"},
	"context":   {Fg: "#cbd5e0", Bg: "#4a5568"},
	"directory": {Fg: "#ffffff", Bg: "#8b4513"},
	"git":       {Fg: "#ffffff", Bg: "#404040"},
	"project":   {Fg: "#87ceeb", Bg: "#2a2a2a"},
	"env":       {Fg: "#d0a0d0", Bg: "#2d2d3d"},
	"duration":  {Fg: "#d1d5db", Bg: "#374151"},
	"tools":     {Fg: "#87ceeb", Bg: "#2a2a2a"},
	"agents":    {Fg: "#87ceeb", Bg: "#2a2a2a"},
	"todos":     {Fg: "#98fb98", Bg: "#1a1a1a"},
	"session":   {Fg: "#00ffff", Bg: "#202020"},
	"thinking":  {Fg: "#87ceeb", Bg: "#2a2a2a"},
}

// nordTheme uses the Nord color palette (https://www.nordtheme.com/).
// A cool, bluish palette with muted greens and soft purples.
var nordTheme = Theme{
	"model":     {Fg: "#81a1c1", Bg: "#4c566a"},
	"context":   {Fg: "#eceff4", Bg: "#5e81ac"},
	"directory": {Fg: "#d8dee9", Bg: "#434c5e"},
	"git":       {Fg: "#a3be8c", Bg: "#3b4252"},
	"project":   {Fg: "#88c0d0", Bg: "#434c5e"},
	"env":       {Fg: "#b48ead", Bg: "#3b4252"},
	"duration":  {Fg: "#d8dee9", Bg: "#3b4252"},
	"tools":     {Fg: "#81a1c1", Bg: "#3b4252"},
	"agents":    {Fg: "#88c0d0", Bg: "#2e3440"},
	"todos":     {Fg: "#8fbcbb", Bg: "#2e3440"},
	"session":   {Fg: "#88c0d0", Bg: "#2e3440"},
	"thinking":  {Fg: "#81a1c1", Bg: "#3b4252"},
}

// gruvboxTheme uses the Gruvbox color palette (https://github.com/morhetz/gruvbox).
// Warm retro colors with earthy tones and high contrast.
var gruvboxTheme = Theme{
	"model":     {Fg: "#83a598", Bg: "#665c54"},
	"context":   {Fg: "#ebdbb2", Bg: "#458588"},
	"directory": {Fg: "#ebdbb2", Bg: "#504945"},
	"git":       {Fg: "#b8bb26", Bg: "#3c3836"},
	"project":   {Fg: "#8ec07c", Bg: "#504945"},
	"env":       {Fg: "#d3869b", Bg: "#3c3836"},
	"duration":  {Fg: "#ebdbb2", Bg: "#3c3836"},
	"tools":     {Fg: "#83a598", Bg: "#3c3836"},
	"agents":    {Fg: "#8ec07c", Bg: "#282828"},
	"todos":     {Fg: "#fabd2f", Bg: "#282828"},
	"session":   {Fg: "#8ec07c", Bg: "#282828"},
	"thinking":  {Fg: "#83a598", Bg: "#3c3836"},
}

// tokyoNightTheme uses the Tokyo Night color palette
// (https://github.com/folke/tokyonight.nvim).
// Deep navy backgrounds with vivid neon accents.
var tokyoNightTheme = Theme{
	"model":     {Fg: "#fca7ea", Bg: "#191b29"},
	"context":   {Fg: "#c0caf5", Bg: "#414868"},
	"directory": {Fg: "#82aaff", Bg: "#2f334d"},
	"git":       {Fg: "#c3e88d", Bg: "#1e2030"},
	"project":   {Fg: "#bb9af7", Bg: "#292e42"},
	"env":       {Fg: "#fca7ea", Bg: "#24283b"},
	"duration":  {Fg: "#c0caf5", Bg: "#3d59a1"},
	"tools":     {Fg: "#7aa2f7", Bg: "#2d3748"},
	"agents":    {Fg: "#86e1fc", Bg: "#222436"},
	"todos":     {Fg: "#4fd6be", Bg: "#1a202c"},
	"session":   {Fg: "#86e1fc", Bg: "#222436"},
	"thinking":  {Fg: "#7aa2f7", Bg: "#2d3748"},
}

// rosePineTheme uses the Rosé Pine color palette (https://rosepinetheme.com/).
// A sooty dark theme with muted purples, teal accents, and soft rose highlights.
var rosePineTheme = Theme{
	"model":     {Fg: "#ebbcba", Bg: "#191724"},
	"context":   {Fg: "#e0def4", Bg: "#393552"},
	"directory": {Fg: "#c4a7e7", Bg: "#26233a"},
	"git":       {Fg: "#9ccfd8", Bg: "#1f1d2e"},
	"project":   {Fg: "#c4a7e7", Bg: "#2a273f"},
	"env":       {Fg: "#eb6f92", Bg: "#21202e"},
	"duration":  {Fg: "#e0def4", Bg: "#524f67"},
	"tools":     {Fg: "#eb6f92", Bg: "#2a273f"},
	"agents":    {Fg: "#f6c177", Bg: "#26233a"},
	"todos":     {Fg: "#9ccfd8", Bg: "#232136"},
	"session":   {Fg: "#f6c177", Bg: "#26233a"},
	"thinking":  {Fg: "#eb6f92", Bg: "#2a273f"},
}

// Load returns the named built-in theme. If the name is not recognized,
// the default theme is returned. Never returns nil.
func Load(name string) Theme {
	if t, ok := builtins[name]; ok {
		return t
	}
	return defaultTheme
}

// BuiltinNames returns the sorted list of built-in theme names.
func BuiltinNames() []string {
	return []string{"default", "dark", "gruvbox", "nord", "rose-pine", "tokyo-night"}
}

// MergeOverrides returns a new Theme that starts from base and then applies
// the per-widget overrides on top. Widget entries in overrides replace the
// corresponding base entry entirely (both Fg and Bg), so a partial override
// must copy the fields it wants to keep.
func MergeOverrides(base Theme, overrides map[string]WidgetColors) Theme {
	merged := make(Theme, len(base))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}
