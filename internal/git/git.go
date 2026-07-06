// Package git provides git repository status information for the working directory.
package git

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/logging"
	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

const timeout = time.Second
const cacheTTL = time.Second

// cache holds the last result per working directory.
type cacheEntry struct {
	status    *model.GitStatus
	timestamp time.Time
}

var (
	cacheMu sync.Mutex
	cache   = map[string]cacheEntry{}
)

// GetStatus returns the git status for the given directory, or nil if the
// directory is not a git repo or any command fails. Callers must guard against
// nil before dereferencing the result.
//
// Results are cached per directory for up to 1 second so rapid successive
// calls (e.g. multiple render ticks) avoid redundant subprocess spawns.
func GetStatus(cwd string) *model.GitStatus {
	cacheMu.Lock()
	if entry, ok := cache[cwd]; ok && time.Since(entry.timestamp) < cacheTTL {
		cacheMu.Unlock()
		return entry.status
	}
	cacheMu.Unlock()

	status := fetchStatus(cwd)

	cacheMu.Lock()
	cache[cwd] = cacheEntry{status: status, timestamp: time.Now()}
	cacheMu.Unlock()

	return status
}

// fetchStatus runs a single `git status --branch --porcelain=v2` subprocess and
// parses all fields needed by model.GitStatus. Returns nil when the directory is
// not a git repo or the command fails.
func fetchStatus(cwd string) *model.GitStatus {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "status", "--branch", "--porcelain=v2")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		// Non-zero exit means we're outside a git repo or git is unavailable.
		logging.Debug("git: porcelain=v2 failed in %s: %v", cwd, err)
		return nil
	}

	status := parsePorcelainV2(string(out))
	if status != nil && status.IsDirty() {
		status.LinesAdded, status.LinesRemoved = fetchLineStats(ctx, cwd)
	}
	return status
}

// fetchLineStats runs `git diff HEAD --numstat` and returns the summed
// added/removed line counts of all uncommitted changes (staged + unstaged).
// Returns zeros on any failure (e.g. an unborn HEAD in a fresh repo) —
// line stats are decoration, never worth failing the whole status for.
func fetchLineStats(ctx context.Context, cwd string) (added, removed int) {
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD", "--numstat")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		logging.Debug("git: diff --numstat failed in %s: %v", cwd, err)
		return 0, 0
	}
	return parseNumstat(string(out))
}

// parseNumstat sums the per-file added/removed counts from `git diff --numstat`
// output. Each line is "<added>\t<removed>\t<path>"; binary files report "-"
// in both count columns and are skipped.
func parseNumstat(output string) (added, removed int) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		a, errA := strconv.Atoi(fields[0])
		r, errR := strconv.Atoi(fields[1])
		if errA != nil || errR != nil {
			continue // binary file ("-") or malformed line
		}
		added += a
		removed += r
	}
	return added, removed
}

// parsePorcelainV2 parses the output of `git status --branch --porcelain=v2`
// and returns a populated GitStatus. The format is documented in git-status(1).
//
// Header lines start with "# " and carry branch/upstream metadata:
//
//	# branch.head <name>
//	# branch.ab +<ahead> -<behind>
//
// Entry lines describe file changes:
//
//	1 <XY> ... (ordinary changed entry)
//	2 <XY> ... (renamed/copied entry)
//	? <path>  (untracked file)
//	u <XY> ... (unmerged entry)
func parsePorcelainV2(output string) *model.GitStatus {
	status := &model.GitStatus{}

	for _, line := range strings.Split(output, "\n") {
		if len(line) == 0 {
			continue
		}

		switch {
		case strings.HasPrefix(line, "# branch.head "):
			// Branch name; "(detached)" when HEAD is detached.
			status.Branch = strings.TrimPrefix(line, "# branch.head ")

		case strings.HasPrefix(line, "# branch.ab "):
			// Ahead/behind counts relative to upstream: "+N -M"
			parts := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			if len(parts) == 2 {
				if n, err := strconv.Atoi(strings.TrimPrefix(parts[0], "+")); err == nil {
					status.AheadBy = n
				}
				if n, err := strconv.Atoi(strings.TrimPrefix(parts[1], "-")); err == nil {
					status.BehindBy = n
				}
			}

		case line[0] == '?':
			// Untracked file.
			status.Untracked++
			status.Dirty = true

		case line[0] == '1' || line[0] == '2' || line[0] == 'u':
			// Ordinary change, rename/copy, or unmerged entry.
			// Field layout: <type> <XY> ...
			// XY is at position 2-3: X = index (staged) status, Y = worktree status.
			if len(line) < 4 {
				continue
			}
			status.Dirty = true
			x := line[2] // index column
			y := line[3] // worktree column

			// Staged: index column is not '.' (unchanged) or '?'
			if x != '.' && x != '?' {
				status.Staged++
			}

			// Modified in worktree: 'M' (modified) or 'D' (deleted)
			if y == 'M' || y == 'D' {
				status.Modified++
			}
		}
	}

	return status
}
