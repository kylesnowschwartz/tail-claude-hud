package styleexport

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
)

func TestModColor(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"1", "ansi256(1)"},
		{"15", "ansi256(15)"},
		{"75", "ansi256(75)"},
		{"255", "ansi256(255)"},
		{"256", ""},
		{"-1", ""},
		{"red", "ansi256(1)"},
		{"BrightBlue", "ansi256(12)"},
		{"#4C566A", "#4c566a"},
		{"#abc", "#aabbcc"},
		{"#12345", ""},
		{"#gggggg", ""},
		{"chartreuse", ""},
	}
	for _, tc := range cases {
		if got := ModColor(tc.in); got != tc.want {
			t.Errorf("ModColor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// loadConfig points HOME at a temp dir holding the given config files and
// loads the config the way the binary does.
func loadConfig(t *testing.T, files map[string]string) *config.Config {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "tail-claude-hud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	return config.LoadHud()
}

// decode round-trips the export through JSON so assertions see exactly what
// a consumer parses.
func decode(t *testing.T, cfg *config.Config) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	if err := Write(&buf, cfg, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, buf.String())
	}
	return doc
}

func field(t *testing.T, doc map[string]any, path ...string) any {
	t.Helper()
	var cur any = doc
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("%v: %v is not an object", path, cur)
		}
		cur, ok = m[key]
		if !ok {
			t.Fatalf("%v: missing key %q", path, key)
		}
	}
	return cur
}

func TestWrite_DefaultConfig(t *testing.T) {
	doc := decode(t, loadConfig(t, nil))

	checks := []struct {
		path []string
		want any
	}{
		{[]string{"schema"}, float64(SchemaVersion)},
		{[]string{"version"}, "1.2.3"},
		{[]string{"iconMode"}, "nerdfont"},
		{[]string{"icons", "check"}, ""},
		{[]string{"icons", "task"}, "\U000F167A"},
		{[]string{"colors", "running"}, "ansi256(3)"},
		{[]string{"colors", "success"}, "ansi256(2)"},
		{[]string{"colors", "error"}, "ansi256(1)"},
		{[]string{"colors", "thinking"}, "ansi256(3)"},
		{[]string{"colors", "permission"}, "ansi256(1)"},
		{[]string{"colors", "muted"}, "ansi256(8)"},
		{[]string{"colors", "normal"}, "ansi256(2)"},
		{[]string{"colors", "warning"}, "ansi256(3)"},
		{[]string{"colors", "critical"}, "ansi256(1)"},
		{[]string{"widgetColors", "tools", "fg"}, "ansi256(75)"},
		{[]string{"widgetColors", "tools", "bg"}, ""},
		{[]string{"toolCategories", "default"}, "Other"},
		{[]string{"toolCategories", "byName", "Read"}, "Read"},
		{[]string{"toolCategories", "byName", "NotebookEdit"}, "Write"},
		{[]string{"toolCategories", "byName", "WebSearch"}, "Web"},
		{[]string{"toolCategories", "byName", "Agent"}, "Task"},
		{[]string{"toolCategories", "byName", "Skill"}, "Skill"},
		{[]string{"toolCategories", "icons", "Web"}, "web"},
		{[]string{"toolCategories", "icons", "Thinking"}, "thinking"},
	}
	for _, c := range checks {
		if got := field(t, doc, c.path...); got != c.want {
			t.Errorf("%v = %#v, want %#v", c.path, got, c.want)
		}
	}

	agents, ok := field(t, doc, "agentColors").([]any)
	if !ok || len(agents) != 8 || agents[0] != "ansi256(12)" || agents[7] != "ansi256(5)" {
		t.Errorf("agentColors = %#v, want 8 entries from ansi256(12) to ansi256(5)", agents)
	}

	// Every category icon key must resolve in the icons map.
	icons := field(t, doc, "icons").(map[string]any)
	for category, key := range field(t, doc, "toolCategories", "icons").(map[string]any) {
		if _, ok := icons[key.(string)]; !ok {
			t.Errorf("category %s icon key %q missing from icons", category, key)
		}
	}
}

func TestWrite_ConfigOverridesChangeColors(t *testing.T) {
	doc := decode(t, loadConfig(t, map[string]string{
		"config.toml": `
[style]
icons = "ascii"
theme = "nord"

[style.colors]
warning = "#ff8800"

[theme.overrides.tools]
fg = "brightmagenta"
bg = "#112233"
`,
		"theme-active.toml": `
[agents]
fg = "200"
`,
	}))

	checks := []struct {
		path []string
		want any
	}{
		{[]string{"iconMode"}, "ascii"},
		{[]string{"icons", "check"}, "v"},
		{[]string{"colors", "warning"}, "#ff8800"},
		{[]string{"colors", "critical"}, "ansi256(1)"},
		{[]string{"widgetColors", "tools", "fg"}, "ansi256(13)"},
		{[]string{"widgetColors", "tools", "bg"}, "#112233"},
		{[]string{"widgetColors", "agents", "fg"}, "ansi256(200)"},
		{[]string{"widgetColors", "model", "fg"}, "#81a1c1"},
	}
	for _, c := range checks {
		if got := field(t, doc, c.path...); got != c.want {
			t.Errorf("%v = %#v, want %#v", c.path, got, c.want)
		}
	}
}

func TestWrite_CorruptConfigFallsBackToDefaults(t *testing.T) {
	doc := decode(t, loadConfig(t, map[string]string{"config.toml": "[[[ not toml"}))
	if got := field(t, doc, "iconMode"); got != "nerdfont" {
		t.Errorf("iconMode = %#v, want nerdfont", got)
	}
	if got := field(t, doc, "widgetColors", "tools", "fg"); got != "ansi256(75)" {
		t.Errorf("tools fg = %#v, want ansi256(75)", got)
	}
}
