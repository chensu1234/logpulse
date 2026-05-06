// Package monitor provides log tailing and statistics tracking.
package monitor

import (
	"sync"
	"time"

	"github.com/chensu/logpulse/internal/alert"
	"github.com/chensu/logpulse/internal/parser"
)

// Stats tracks counts, rates, and spikes for each log level.
type Stats struct {
	mu sync.RWMutex

	// Per-level counters
	Counts map[parser.Level]int

	// Rate tracking (lines per second)
	Rate         float64
	windowSize   time.Duration
	lastUpdate   time.Time
	recentCounts []timeWindow

	// Spike detection
	SpikeDetected  bool
	spikeThreshold float64
}

type timeWindow struct {
	timestamp time.Time
	total     int
}

// NewStats creates a new Stats tracker with optional spike threshold multiplier.
func NewStats(spikeThreshold float64) *Stats {
	if spikeThreshold <= 0 {
		spikeThreshold = 3.0 // default: flag as spike if 3x normal rate
	}
	return &Stats{
		Counts:          make(map[parser.Level]int),
		windowSize:      time.Second,
		lastUpdate:      time.Now(),
		spikeThreshold: spikeThreshold,
		recentCounts:    make([]timeWindow, 0, 60),
	}
}

// Record adds a log level event to the statistics.
func (s *Stats) Record(level parser.Level) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Counts[level]++
	s.recentCounts = append(s.recentCounts, timeWindow{timestamp: time.Now(), total: 1})

	// Prune old windows beyond 60 seconds
	cutoff := time.Now().Add(-60 * time.Second)
	newRecent := make([]timeWindow, 0, len(s.recentCounts))
	for _, tw := range s.recentCounts {
		if tw.timestamp.After(cutoff) {
			newRecent = append(newRecent, tw)
		}
	}
	s.recentCounts = newRecent

	s.updateRate()
	s.detectSpike()
	s.lastUpdate = time.Now()
}

// updateRate computes the rolling lines/second rate over the recent window.
func (s *Stats) updateRate() {
	if len(s.recentCounts) == 0 {
		s.Rate = 0
		return
	}

	cutoff := time.Now().Add(-s.windowSize)
	var total int
	for _, tw := range s.recentCounts {
		if tw.timestamp.After(cutoff) {
			total += tw.total
		}
	}

	elapsed := time.Since(s.lastUpdate.Add(-s.windowSize))
	if elapsed <= 0 {
		elapsed = time.Second
	}
	s.Rate = float64(total) / elapsed.Seconds()
}

// detectSpike checks if the current rate exceeds spikeThreshold times the average.
func (s *Stats) detectSpike() {
	if len(s.recentCounts) < 5 {
		s.SpikeDetected = false
		return
	}

	var sum float64
	for _, tw := range s.recentCounts {
		sum += float64(tw.total)
	}
	avg := sum / float64(len(s.recentCounts))

	s.SpikeDetected = s.Rate > avg*s.spikeThreshold && avg > 0
}

// Snapshot returns a copy of the current stats as alert.StatsSnapshot.
func (s *Stats) Snapshot() alert.StatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[parser.Level]int)
	for k, v := range s.Counts {
		counts[k] = v
	}

	return alert.StatsSnapshot{
		Counts:        counts,
		Rate:          s.Rate,
		SpikeDetected: s.SpikeDetected,
	}
}

// Reset clears all counters.
func (s *Stats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Counts = make(map[parser.Level]int)
	s.Rate = 0
	s.SpikeDetected = false
	s.recentCounts = nil
}
