package styleexport

import (
	"maps"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/render/widget"
)

// modStyleSource is the tch-activity mod's look, drawn when the HUD binary is
// missing or answers badly.
const modStyleSource = "../../mods/tch-activity/hooks/style.ts"

var modIconEntry = regexp.MustCompile(`^\s*(\w+): '([^']*)',$`)

// builtinModIcons reads the icons block of the mod's BUILTIN_STYLE.
func builtinModIcons(t *testing.T) map[string]string {
	t.Helper()
	src, err := os.ReadFile(modStyleSource)
	if err != nil {
		t.Fatalf("read mod style: %v", err)
	}
	_, rest, ok := strings.Cut(string(src), "export const BUILTIN_STYLE")
	if !ok {
		t.Fatal("BUILTIN_STYLE not found in mod style")
	}
	_, rest, ok = strings.Cut(rest, "  icons: {\n")
	if !ok {
		t.Fatal("BUILTIN_STYLE icons block not found")
	}
	block, _, _ := strings.Cut(rest, "\n  },")

	icons := map[string]string{}
	for line := range strings.SplitSeq(block, "\n") {
		m := modIconEntry.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("unparsed line in BUILTIN_STYLE icons: %q", line)
		}
		icons[m[1]] = m[2]
	}
	return icons
}

// The mod's built-in look is the HUD's unicode icon set, key for key, so a
// session without the HUD binary draws the same glyphs as one with it.
func TestModBuiltinIconsMatchUnicodeSet(t *testing.T) {
	hud := widget.IconsFor("unicode").ByKey()
	mod := builtinModIcons(t)
	if !maps.Equal(hud, mod) {
		t.Errorf("mod BUILTIN_STYLE icons differ from the HUD unicode set\nHUD: %v\nmod: %v", hud, mod)
	}
}
