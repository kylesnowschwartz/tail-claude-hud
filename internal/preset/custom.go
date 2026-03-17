package preset

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
)

// presetFile mirrors the TOML schema for a preset file. Only preset-relevant
// fields are decoded; data-source settings stay in the user's config.
type presetFile struct {
	Name      string        `toml:"name"`
	Lines     []config.Line `toml:"line"`
	Style     presetStyle   `toml:"style"`
	Directory presetDir     `toml:"directory"`
}

type presetStyle struct {
	Separator string `toml:"separator"`
	Icons     string `toml:"icons"`
	Mode      string `toml:"mode"`
	Theme     string `toml:"theme"`
}

type presetDir struct {
	Style string `toml:"style"`
}

// CustomPresetDir returns the directory where custom preset TOML files are
// stored: ~/.config/tail-claude-hud/presets/.
func CustomPresetDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}
	return filepath.Join(home, ".config", "tail-claude-hud", "presets")
}

// LoadFromFile parses a TOML preset file at path and returns a Preset. The
// preset Name defaults to the file's base name (without extension) when the
// TOML does not set a name field.
func LoadFromFile(path string) (Preset, error) {
	var pf presetFile
	if _, err := toml.DecodeFile(path, &pf); err != nil {
		return Preset{}, fmt.Errorf("preset: decode %s: %w", path, err)
	}

	name := pf.Name
	if name == "" {
		base := filepath.Base(path)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return Preset{
		Name:           name,
		Lines:          pf.Lines,
		Separator:      pf.Style.Separator,
		Icons:          pf.Style.Icons,
		Mode:           pf.Style.Mode,
		Theme:          pf.Style.Theme,
		DirectoryStyle: pf.Directory.Style,
	}, nil
}

// LoadCustom loads a preset by name from the CustomPresetDir. It looks for
// name.toml in that directory and delegates to LoadFromFile.
func LoadCustom(name string) (Preset, error) {
	path := filepath.Join(CustomPresetDir(), name+".toml")
	p, err := LoadFromFile(path)
	if err != nil {
		return Preset{}, fmt.Errorf("preset: load custom %q: %w", name, err)
	}
	return p, nil
}

// ListCustom returns the sorted names of all .toml files in CustomPresetDir,
// without the .toml extension. If the directory does not exist, it returns an
// empty slice rather than an error.
func ListCustom() []string {
	dir := CustomPresetDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}
		}
		return []string{}
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".toml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".toml"))
		}
	}
	sort.Strings(names)
	return names
}

// ListAll returns built-in preset names followed by custom preset names. Both
// halves are sorted. Duplicates are removed (built-in takes precedence).
func ListAll() []string {
	builtins := BuiltinNames()
	customs := ListCustom()

	seen := make(map[string]bool, len(builtins))
	result := make([]string, 0, len(builtins)+len(customs))

	for _, n := range builtins {
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}
	for _, n := range customs {
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}
	return result
}
