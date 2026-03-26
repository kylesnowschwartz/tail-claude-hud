package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/color"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/config"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/gather"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/hook"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/preset"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/render"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/stdin"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/version"
	"github.com/lucasb-eyer/go-colorful"
)

// installPath is the go install target for the update command.
const installPath = "github.com/kylesnowschwartz/tail-claude-hud/cmd/tail-claude-hud@latest"

func main() {
	// Subcommands are dispatched before flag.Parse() so they don't interfere
	// with flags. Each exits directly when handled.
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "hook":
			runHook()
			return
		case "version", "--version", "-version", "-v":
			fmt.Println(version.String())
			return
		case "update":
			runUpdate()
			return
		}
	}

	flag.Usage = usage
	dumpCurrent := flag.Bool("dump-current", false, "render the statusline from a transcript file instead of stdin")
	dumpRaw := flag.Bool("dump-raw", false, "like --dump-current but print ANSI escape sequences as visible text for debugging")
	initConfig := flag.Bool("init", false, "generate a default config file at ~/.config/tail-claude-hud/config.toml")
	listPresets := flag.Bool("list-presets", false, "print available preset names and exit")
	previewPath := flag.String("preview", "", "render statusline from a transcript file using mock stdin data")
	presetName := flag.String("preset", "", "apply a named preset or TOML file path")
	themeName := flag.String("theme", "", "override the color theme (e.g. light, dark, nord)")
	watch := flag.Bool("watch", false, "continuously re-render on transcript changes (requires --preview)")
	flag.Parse()

	if *listPresets {
		for _, name := range preset.ListAll() {
			fmt.Println(name)
		}
		return
	}

	if *dumpRaw {
		*dumpCurrent = true
	}

	if *watch && *previewPath == "" {
		fmt.Fprintf(os.Stderr, "tail-claude-hud: --watch requires --preview\n")
		os.Exit(1)
	}

	if *initConfig {
		if err := config.Init(); err != nil {
			fmt.Fprintf(os.Stderr, "tail-claude-hud: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Load config and apply preset (if any) before choosing the data source.
	cfg := config.LoadHud()

	if *presetName != "" {
		p, err := resolvePreset(*presetName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tail-claude-hud: %v\n", err)
			os.Exit(1)
		}
		preset.ApplyPreset(cfg, p)
	}

	// CLI --theme overrides the theme from config/preset.
	// When no explicit --theme is given, auto-detect the terminal background
	// and switch between "light" and "dark" themes for powerline mode.
	if *themeName != "" {
		cfg.Style.Theme = *themeName
		config.ResolveTheme(cfg)
	} else if cfg.Style.Mode == "powerline" || hasPowerlineLines(cfg) {
		if detectLightBackground() {
			cfg.Style.Theme = "light"
			config.ResolveTheme(cfg)
		}
	}

	// Resolve input data from one of three sources.
	var input *model.StdinData
	var err error

	if *previewPath != "" {
		if _, err := os.Stat(*previewPath); err != nil {
			fmt.Fprintf(os.Stderr, "tail-claude-hud: --preview: %v\n", err)
			os.Exit(1)
		}
		input = stdin.MockStdinData(*previewPath)
	} else if *dumpCurrent {
		input, err = readFromFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "tail-claude-hud: %v\n", err)
			os.Exit(1)
		}
	} else {
		input, err = stdin.Read(os.Stdin)
		if err == nil && input != nil {
			stdin.SaveSnapshot(input)
		}
	}

	if err != nil || input == nil {
		fmt.Println("[tail-claude-hud] Initializing...")
		return
	}

	// Collect data in parallel for configured widgets.
	ctx := gather.Gather(input, cfg)

	// Render and print.
	if *dumpRaw {
		var buf bytes.Buffer
		render.Render(&buf, ctx, cfg)
		printRaw(os.Stdout, buf.Bytes())
	} else {
		render.Render(os.Stdout, ctx, cfg)
	}

	if *watch && *previewPath != "" {
		watchAndRender(*previewPath, input, cfg)
	}
}

// resolvePreset loads a preset by name or file path.
// When value contains "/" or ends in ".toml", it is treated as a file path.
// Otherwise, built-in presets are tried first, then custom presets.
func resolvePreset(value string) (preset.Preset, error) {
	if strings.Contains(value, "/") || strings.HasSuffix(value, ".toml") {
		return preset.LoadFromFile(value)
	}

	if p, ok := preset.Load(value); ok {
		return p, nil
	}

	if p, err := preset.LoadCustom(value); err == nil {
		return p, nil
	}

	available := preset.ListAll()
	return preset.Preset{}, fmt.Errorf("--preset: unknown preset %q (available: %s)", value, strings.Join(available, ", "))
}

// watchAndRender polls the transcript file every 500ms and re-renders the
// statusline whenever the file size or mtime changes. It exits cleanly on
// SIGINT or SIGTERM. The mock stdin data (model, cost, context window) stays
// constant — only the transcript content changes between re-renders.
func watchAndRender(path string, input *model.StdinData, cfg *config.Config) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	lastSize, lastMod := statFile(path)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	fmt.Fprintf(os.Stderr, "\x1b[2m Watching %s — Ctrl+C to stop\x1b[0m\n", filepath.Base(path))

	for {
		select {
		case <-sigCh:
			return
		case <-ticker.C:
			size, mod := statFile(path)
			if size == lastSize && mod == lastMod {
				continue
			}
			lastSize, lastMod = size, mod
			// Clear screen and move cursor to top-left before re-rendering.
			fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
			ctx := gather.Gather(input, cfg)
			render.Render(os.Stdout, ctx, cfg)
			fmt.Fprintf(os.Stderr, "\x1b[2m Watching %s — Ctrl+C to stop\x1b[0m\n", filepath.Base(path))
		}
	}
}

// statFile returns the size and mtime of path. When the file cannot be
// stat'd (e.g. temporarily deleted), it returns -1,-1 so a subsequent
// successful stat will trigger a re-render once the file reappears.
func statFile(path string) (int64, int64) {
	info, err := os.Stat(path)
	if err != nil {
		return -1, -1
	}
	return info.Size(), info.ModTime().UnixNano()
}

// readFromFile loads the last-stdin snapshot (model, context window) and
// resolves the transcript path so the gather stage can parse tools/agents/todos.
// The snapshot is written on every live statusline invocation, so it reflects
// the most recent state from the active Claude Code session.
//
// Transcript path priority:
//  1. positional argument (first non-flag arg)
//  2. CLAUDE_TRANSCRIPT_PATH env var
//  3. snapshot's own TranscriptPath
//  4. auto-discover: most recently modified .jsonl in ~/.claude/projects/<cwd-slug>/
func readFromFile() (*model.StdinData, error) {
	// Start from the persisted snapshot when available. If missing, fall back
	// to an empty StdinData — dump still works, just without model/context.
	data, err := stdin.LoadSnapshot()
	if err != nil {
		data = &model.StdinData{}
	}

	// Resolve transcript path, allowing explicit overrides.
	path := flag.Arg(0)
	if path == "" {
		path = os.Getenv("CLAUDE_TRANSCRIPT_PATH")
	}
	if path == "" {
		path = data.TranscriptPath
	}
	if path == "" {
		path, err = findCurrentTranscript()
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("--dump-current: %w", err)
	}

	data.TranscriptPath = path
	if data.Cwd == "" {
		data.Cwd = mustCwd()
	}

	return data, nil
}

// findCurrentTranscript auto-discovers the most recently modified .jsonl file
// in ~/.claude/projects/<cwd-slug>/. The cwd-slug is computed from the current
// working directory using Claude Code's path encoding scheme.
func findCurrentTranscript() (string, error) {
	projectDir, err := currentProjectDir()
	if err != nil {
		return "", fmt.Errorf("--dump-current: resolve project dir: %w", err)
	}

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", fmt.Errorf("--dump-current: no transcript found (could not read %s): %w", projectDir, err)
	}

	var newest string
	var newestTime int64
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".jsonl") {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		if mt := info.ModTime().UnixNano(); mt > newestTime {
			newestTime = mt
			newest = filepath.Join(projectDir, de.Name())
		}
	}

	if newest == "" {
		return "", fmt.Errorf("--dump-current: no .jsonl transcript found in %s", projectDir)
	}
	return newest, nil
}

// currentProjectDir returns ~/.claude/projects/<encoded-cwd>. Symlinks in the
// cwd are resolved so the encoded path matches what Claude Code produces on
// disk (e.g. macOS /tmp -> /private/tmp).
func currentProjectDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Resolve symlinks so the encoded path matches Claude Code's on-disk output.
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}

	encoded := encodePath(cwd)
	return filepath.Join(home, ".claude", "projects", encoded), nil
}

// encodePath encodes an absolute filesystem path into a Claude Code project
// directory name. Three characters are replaced with "-": path separators (/),
// dots (.), and underscores (_). Ported from tail-claude's parser/session.go
// and verified empirically across 273 project directories.
func encodePath(absPath string) string {
	r := strings.NewReplacer(
		string(filepath.Separator), "-",
		".", "-",
		"_", "-",
	)
	return r.Replace(absPath)
}

// mustCwd returns the current working directory, resolving symlinks to match
// Claude Code's on-disk encoding (e.g. macOS /tmp -> /private/tmp).
func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	return cwd
}

// bgCacheFile is where the detected background mode is cached between invocations.
// The statusline runs every ~300ms; querying the terminal each time causes race
// conditions (overlapping OSC 11 responses). A short TTL prevents races while
// still picking up theme changes within seconds.
var bgCacheFile = filepath.Join(model.PluginDir(), "bg-mode")

const bgCacheTTL = 5 * time.Second

// detectLightBackground returns true if the terminal has a light background.
// Results are cached to disk so the expensive /dev/tty query only runs once
// per TTL period, avoiding OSC 11 race conditions when invoked every ~300ms.
//
// When the terminal query fails (common under piped I/O), the cached value is
// preserved rather than defaulting to dark. This prevents flip-flopping between
// light and dark when queries intermittently timeout.
func detectLightBackground() bool {
	cachedLight := false
	hasCached := false

	// Read cache first.
	if data, err := os.ReadFile(bgCacheFile); err == nil {
		cachedLight = strings.TrimSpace(string(data)) == "light"
		hasCached = true

		if info, err := os.Stat(bgCacheFile); err == nil {
			if time.Since(info.ModTime()) < bgCacheTTL {
				return cachedLight
			}
		}
	}

	// Cache miss or stale — detect via BackgroundColor which exposes errors.
	// On failure, preserve the cached value. No cache + failed query = dark.
	light, ok := queryLightBackground()
	if !ok {
		return hasCached && cachedLight
	}

	mode := "dark"
	if light {
		mode = "light"
	}
	_ = os.MkdirAll(filepath.Dir(bgCacheFile), 0o755)
	_ = os.WriteFile(bgCacheFile, []byte(mode+"\n"), 0o644)
	return light
}

// queryLightBackground queries the terminal background via /dev/tty,
// falling back to stderr. Returns (isLight, ok). When ok is false the
// query failed and the caller should preserve the cached value.
func queryLightBackground() (light bool, ok bool) {
	var bg color.Color
	var err error

	if tty, ttyErr := os.OpenFile("/dev/tty", os.O_RDWR, 0); ttyErr == nil {
		defer tty.Close()
		bg, err = lipgloss.BackgroundColor(tty, tty)
	} else {
		bg, err = lipgloss.BackgroundColor(os.Stdin, os.Stderr)
	}

	if err != nil || bg == nil {
		return false, false
	}
	return isLightColor(bg), true
}

// isLightColor returns true when a color's HSL lightness is >= 0.5.
// Mirrors lipgloss's unexported isDarkColor with the inverse condition.
func isLightColor(c color.Color) bool {
	col, ok := colorful.MakeColor(c)
	if !ok {
		return false // can't determine — assume dark (safe default)
	}
	_, _, l := col.Hsl()
	return l >= 0.5
}

// printRaw writes rendered output with ANSI escape sequences made visible as
// printable text. Each ESC byte (0x1b) is replaced with the literal string
// "\x1b" so colors and cursor codes appear inline rather than being
// interpreted by the terminal. Useful for verifying that threshold colors,
// powerline backgrounds, and other ANSI styling are actually present.
func printRaw(w io.Writer, data []byte) {
	for i := 0; i < len(data); i++ {
		if data[i] == 0x1b {
			fmt.Fprint(w, `\x1b`)
		} else {
			w.Write(data[i : i+1]) //nolint:errcheck
		}
	}
}

// hasPowerlineLines returns true if any configured line uses powerline mode.
func hasPowerlineLines(cfg *config.Config) bool {
	for _, line := range cfg.Lines {
		if line.Mode == "powerline" {
			return true
		}
	}
	return false
}

// runHook dispatches hook subcommands. Always exits 0 — a hook failure
// must not block Claude Code.
func runHook() {
	if len(os.Args) < 3 {
		return
	}
	var err error
	switch os.Args[2] {
	case "permission-request":
		err = hook.HandlePermissionRequest(os.Stdin)
	case "cleanup":
		err = hook.HandleCleanup(os.Stdin)
	}
	if err != nil {
		_ = err // log to debug file if available, but never fail
	}
}

// runUpdate installs the latest version via go install.
func runUpdate() {
	current := version.String()
	fmt.Printf("Current version: %s\n", current)
	fmt.Printf("Installing latest from %s...\n", installPath)

	cmd := exec.Command("go", "install", installPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}

	// Run the newly installed binary to get its version.
	out, err := exec.Command("tail-claude-hud", "version").Output()
	if err != nil {
		fmt.Println("Updated successfully.")
		return
	}
	newVersion := strings.TrimSpace(string(out))
	if newVersion == current {
		fmt.Printf("Already up to date (%s).\n", current)
	} else {
		fmt.Printf("Updated %s -> %s\n", current, newVersion)
	}
}

// usage prints help with -- prefixed flags (Go's flag package defaults to single -).
func usage() {
	fmt.Fprintf(os.Stderr, "Usage: tail-claude-hud [command] [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  hook <event>    handle a Claude Code hook event\n")
	fmt.Fprintf(os.Stderr, "  update          install the latest version via go install\n")
	fmt.Fprintf(os.Stderr, "  version         print the current version\n")
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	flag.VisitAll(func(f *flag.Flag) {
		name := f.Name
		typeName, usage := flag.UnquoteUsage(f)
		if typeName != "" {
			fmt.Fprintf(os.Stderr, "  --%s %s\n    \t%s\n", name, typeName, usage)
		} else {
			fmt.Fprintf(os.Stderr, "  --%s\n    \t%s\n", name, usage)
		}
	})
}
