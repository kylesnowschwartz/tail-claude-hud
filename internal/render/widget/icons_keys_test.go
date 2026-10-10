package widget

import (
	"reflect"
	"testing"
)

// TestIconsByKey_CoversEveryField keeps ByKey in step with the Icons struct.
func TestIconsByKey_CoversEveryField(t *testing.T) {
	icons := IconsFor("ascii")
	if got, want := len(icons.ByKey()), reflect.TypeOf(icons).NumField(); got != want {
		t.Errorf("ByKey has %d entries, Icons has %d fields", got, want)
	}
}

// TestCategoryIconKey_MatchesCategoryIcon keeps the exported category-to-key
// mapping in step with the glyph CategoryIcon draws.
func TestCategoryIconKey_MatchesCategoryIcon(t *testing.T) {
	categories := append([]string{"Other", "Tool", "Unknown"}, IconCategories...)
	for _, mode := range []string{"nerdfont", "unicode", "ascii"} {
		icons := IconsFor(mode)
		byKey := icons.ByKey()
		for _, category := range categories {
			key := CategoryIconKey(category)
			if got, want := byKey[key], CategoryIcon(icons, category); got != want {
				t.Errorf("%s/%s: ByKey[%q] = %q, CategoryIcon = %q", mode, category, key, got, want)
			}
		}
	}
}

func TestAgentPalette_ReturnsCopy(t *testing.T) {
	p := AgentPalette()
	if len(p) != len(agentColors) || p[0] != agentColors[0] {
		t.Fatalf("AgentPalette() = %v, want %v", p, agentColors)
	}
	p[0] = "x"
	if agentColors[0] == "x" {
		t.Error("AgentPalette exposed the package palette")
	}
}
