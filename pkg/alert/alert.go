// Package alert implements the alerting engine for logpulse.
package alert

import (
	"sync"
	"sync/atomic"
	"time"
)

// Event represents a single alerting event generated from a matched log line.
type Event struct {
	Timestamp time.Time
	Rule      string
	Message   string
	LineNum   uint64
	Severity  string
	Tags      []string
	Count     int // Number of matches for this rule
}

// EventHandler is the interface for components that handle alerts.
type EventHandler interface {
	Send(Event)
	Close()
}

// EngineConfig contains configuration for the alert engine.
type EngineConfig struct {
	// Cooldown is the minimum interval between alerts for the same rule.
	Cooldown time.Duration

	// MaxPerMinute limits the maximum number of alerts per rule per minute.
	MaxPerMinute int

	// BurstWindow is the time window for burst detection.
	BurstWindow time.Duration
}

// Engine processes alerting events, applies rate limiting and deduplication,
// and dispatches to registered handlers.
type Engine struct {
	config   EngineConfig
	handlers []EventHandler

	// Per-rule state.
	mu         sync.RWMutex
	lastAlert  map[string]time.Time // rule → last alert time
	ruleCount  map[string]*int64    // rule → count within window
	windowSeen map[string]time.Time // rule → start of current window

	// Global rate limiting.
	globalMu        sync.Mutex
	globalCount     int64
	globalWindow    time.Time
	globalMaxPerMin int64
}

// NewEngine creates a new alert engine.
func NewEngine(cfg EngineConfig) *Engine {
	if cfg.Cooldown == 0 {
		cfg.Cooldown = 30 * time.Second
	}
	if cfg.MaxPerMinute == 0 {
		cfg.MaxPerMinute = 60
	}
	if cfg.BurstWindow == 0 {
		cfg.BurstWindow = 1 * time.Minute
	}

	return &Engine{
		config:          cfg,
		handlers:        make([]EventHandler, 0),
		lastAlert:       make(map[string]time.Time),
		ruleCount:       make(map[string]*int64),
		windowSeen:      make(map[string]time.Time),
		globalMaxPerMin: int64(cfg.MaxPerMinute),
	}
}

// RegisterHandler adds a handler to receive alert events.
func (e *Engine) RegisterHandler(h EventHandler) {
	e.handlers = append(e.handlers, h)
}

// Handle processes an incoming alert event and dispatches to handlers
// if it passes rate limiting and deduplication checks.
func (e *Engine) Handle(evt Event) {
	// Apply global rate limit.
	if !e.checkGlobalLimit() {
		return
	}

	// Check per-rule cooldown.
	if !e.checkCooldown(evt.Rule) {
		return
	}

	// Check per-rule rate limit (burst detection).
	if !e.checkRateLimit(evt.Rule) {
		return
	}

	// All checks passed — dispatch to handlers.
	e.dispatch(evt)
}

// checkCooldown returns true if enough time has passed since the last alert
// for this rule, or if there has never been an alert for this rule.
func (e *Engine) checkCooldown(rule string) bool {
	// Zero cooldown means no rate limiting by time.
	if e.config.Cooldown == 0 {
		return true
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	last, ok := e.lastAlert[rule]
	if !ok {
		// First alert for this rule — always pass.
		e.lastAlert[rule] = time.Now()
		return true
	}

	if time.Since(last) >= e.config.Cooldown {
		e.lastAlert[rule] = time.Now()
		return true
	}

	return false
}

// checkRateLimit returns true if the alert is within the rate limit for this rule.
func (e *Engine) checkRateLimit(rule string) bool {
	if e.config.MaxPerMinute <= 0 {
		return true
	}

	burstWindow := e.config.BurstWindow
	if burstWindow == 0 {
		burstWindow = 1 * time.Minute
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	windowStart, ok := e.windowSeen[rule]
	if !ok {
		windowStart = now
		e.windowSeen[rule] = windowStart
		e.ruleCount[rule] = new(int64)
	}

	// Reset counter if we've moved to a new window.
	if now.Sub(windowStart) >= burstWindow {
		e.windowSeen[rule] = now
		e.ruleCount[rule] = new(int64)
	}

	count := atomic.LoadInt64(e.ruleCount[rule])
	if count >= int64(e.config.MaxPerMinute) {
		return false
	}

	atomic.AddInt64(e.ruleCount[rule], 1)
	return true
}

// checkGlobalLimit applies a global rate limit across all rules.
func (e *Engine) checkGlobalLimit() bool {
	if e.config.MaxPerMinute <= 0 {
		return true
	}

	now := time.Now()
	e.globalMu.Lock()
	defer e.globalMu.Unlock()

	// Reset window every minute.
	if now.Sub(e.globalWindow) >= time.Minute {
		e.globalWindow = now
		atomic.StoreInt64(&e.globalCount, 0)
	}

	count := atomic.AddInt64(&e.globalCount, 1)
	return count <= e.globalMaxPerMin
}

// dispatch sends the event to all registered handlers.
func (e *Engine) dispatch(evt Event) {
	for _, h := range e.handlers {
		h.Send(evt)
	}
}

// Close shuts down the alert engine and all handlers.
func (e *Engine) Close() {
	for _, h := range e.handlers {
		h.Close()
	}
}

// Stats returns current engine statistics.
type EngineStats struct {
	TotalProcessed  uint64
	TotalDispatched uint64
	RulesTracked    int
	GlobalRateUsed  float64
}

// GetStats returns a snapshot of engine statistics.
func (e *Engine) GetStats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Sum up all per-rule counts.
	var total uint64
	for _, countPtr := range e.ruleCount {
		total += uint64(atomic.LoadInt64(countPtr))
	}

	// Compute global rate.
	globalUsed := atomic.LoadInt64(&e.globalCount)

	return EngineStats{
		TotalDispatched: uint64(len(e.handlers)), // Simplified.
		RulesTracked:    len(e.lastAlert),
		GlobalRateUsed:  float64(globalUsed) / float64(e.globalMaxPerMin),
	}
}
