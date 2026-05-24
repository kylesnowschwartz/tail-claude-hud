// Package cachestate persists cache-hit-rate samples across statusline invocations
// so the cache widget can compute rolling averages over 5-minute and 1-hour windows.
//
// Each sample records the cache hit rate from a single API call. Samples are
// deduplicated: a new sample is only appended when the values differ from the
// most recent one, preventing duplicate entries on idle keypress ticks.
package cachestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kylesnowschwartz/tail-claude-hud/internal/model"
)

const (
	stateDir  = "tail-claude-hud"
	stateFile = "cache-samples.json"
)

// maxSamples is the maximum number of samples retained.
// At one sample per API call, 2000 covers heavy daily use.
const maxSamples = 2000

// addThreshold is the duration after which a new sample is always added, even
// when values haven't changed. This captures sessions where the user reads from
// the same cached state for a prolonged period.
const addThreshold = 5 * time.Minute

// State holds cache samples and handles persistence.
type State struct {
	Samples []model.CacheSample `json:"samples"`
	path    string
}

// Load reads cache samples from disk. Returns an empty state when the file is
// missing or corrupt.
func Load() *State {
	path := filepath.Join(stateDirPath(), stateFile)
	s := &State{
		Samples: []model.CacheSample{},
		path:    path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}

	var samples []model.CacheSample
	if err := json.Unmarshal(data, &samples); err != nil {
		return s
	}
	s.Samples = samples
	return s
}

// CacheHitRate computes the cache hit rate percentage: cacheRead / (cacheRead + cacheCreation) * 100.
// Returns 0 when there are no cacheable tokens. Uses int64 to avoid overflow.
func CacheHitRate(cacheRead, cacheCreation int) int {
	cacheable := int64(cacheRead) + int64(cacheCreation)
	if cacheable <= 0 {
		return 0
	}
	return int((int64(cacheRead) * 100) / cacheable)
}

// AppendIfChanged adds a new sample only when the cache values differ from the
// last recorded sample or when the last sample is older than addThreshold.
// Samples with zero cacheable tokens (cacheRead+cacheCreation == 0) are skipped.
func (s *State) AppendIfChanged(sample model.CacheSample) {
	if sample.CacheRead+sample.CacheCreation == 0 {
		return
	}

	now := time.Now()
	sample.Timestamp = now

	if len(s.Samples) > 0 {
		last := s.Samples[len(s.Samples)-1]
		if last.CacheRead == sample.CacheRead && last.CacheCreation == sample.CacheCreation {
			// Values haven't changed — only add a new sample if enough time passed.
			if now.Sub(last.Timestamp) < addThreshold {
				return
			}
		}
	}

	sample.CacheRate = CacheHitRate(sample.CacheRead, sample.CacheCreation)

	s.Samples = append(s.Samples, sample)
	if len(s.Samples) > maxSamples {
		s.Samples = s.Samples[len(s.Samples)-maxSamples:]
	}
}

// RollingAverage computes the average cache hit rate over the given window.
// Returns -1 when no samples fall within the window.
func RollingAverage(samples []model.CacheSample, window time.Duration) int {
	cutoff := time.Now().Add(-window)
	var windowed []model.CacheSample
	for _, s := range samples {
		if !s.Timestamp.Before(cutoff) {
			windowed = append(windowed, s)
		}
	}
	if len(windowed) == 0 {
		return -1
	}
	total := 0
	for _, s := range windowed {
		total += s.CacheRate
	}
	return total / len(windowed)
}

// Save persists samples to disk.
func (s *State) Save() error {
	return s.save()
}

func (s *State) save() error {
	if s.path == "" {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)

	data, err := json.Marshal(s.Samples)
	if err != nil {
		return fmt.Errorf("cachestate: marshal: %w", err)
	}
	return os.WriteFile(s.path, data, 0o644)
}

func stateDirPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(home, ".claude", "plugins", stateDir)
}
