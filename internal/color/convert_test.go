package color

import (
	"testing"
)

// TestResolveColorName_NamedColorsMapToANSINumbers verifies that the 16
// standard ANSI color names and common aliases resolve to their numeric
// equivalents, which lipgloss.Color() can parse.
func TestResolveColorName_NamedColorsMapToANSINumbers(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"black", "0"},
		{"red", "1"},
		{"green", "2"},
		{"yellow", "3"},
		{"blue", "4"},
		{"magenta", "5"},
		{"cyan", "6"},
		{"white", "7"},
		{"brightblack", "8"},
		{"gray", "8"},
		{"grey", "8"},
		{"brightred", "9"},
		{"brightgreen", "10"},
		{"brightyellow", "11"},
		{"brightblue", "12"},
		{"brightmagenta", "13"},
		{"brightcyan", "14"},
		{"brightwhite", "15"},
	}
	for _, tc := range cases {
		got := ResolveColorName(tc.input)
		if got != tc.want {
			t.Errorf("ResolveColorName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestResolveColorName_CaseInsensitive verifies that mixed-case named colors
// resolve to the same numeric string as their lowercase equivalents.
func TestResolveColorName_CaseInsensitive(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Green", "2"},
		{"GREEN", "2"},
		{"Red", "1"},
		{"YELLOW", "3"},
		{"BrightCyan", "14"},
	}
	for _, tc := range cases {
		got := ResolveColorName(tc.input)
		if got != tc.want {
			t.Errorf("ResolveColorName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestResolveColorName_PassThrough verifies that hex strings, numeric strings,
// and unrecognised names are returned unchanged.
func TestResolveColorName_PassThrough(t *testing.T) {
	cases := []string{
		"#ff0000",
		"#abc",
		"114",
		"2",
		"",
		"unknown-color",
	}
	for _, input := range cases {
		got := ResolveColorName(input)
		if got != input {
			t.Errorf("ResolveColorName(%q) = %q, want pass-through %q", input, got, input)
		}
	}
}
