// Package alert provides threshold-based alerting for log statistics.
package alert

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/chensu/logpulse/internal/config"
	"github.com/chensu/logpulse/internal/parser"
)

// StatsReader is an interface for accessing log statistics.
// Implemented by monitor.Stats (via StatsSnapshot).
type StatsReader interface {
	Snapshot() StatsSnapshot
}

// StatsSnapshot holds a point-in-time view of log statistics.
type StatsSnapshot struct {
	Counts       map[parser.Level]int
	Rate         float64
	SpikeDetected bool
}

// Total returns the sum of all level counts.
func (ss StatsSnapshot) Total() int {
	total := 0
	for _, v := range ss.Counts {
		total += v
	}
	return total
}

// Alerter evaluates alert rules against live statistics and fires notifications.
type Alerter struct {
	rules    []config.Rule
	fired    map[string]time.Time
	mu       sync.RWMutex
	cooldown time.Duration
	onAlert  func(string, string) // callback: rule name, message
}

// NewAlerter creates a new Alerter with the given rules.
func NewAlerter(rules []config.Rule) *Alerter {
	return &Alerter{
		rules:    rules,
		fired:    make(map[string]time.Time),
		cooldown: 30 * time.Second, // don't re-fire same rule within 30s
	}
}

// SetAlertCallback sets a callback function invoked when an alert fires.
func (a *Alerter) SetAlertCallback(fn func(name, msg string)) {
	a.onAlert = fn
}

// Evaluate checks all rules against the current stats snapshot and fires alerts as needed.
func (a *Alerter) Evaluate(snapshot StatsSnapshot) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, rule := range a.rules {
		level := parser.UNKNOWN
		switch strings.ToUpper(rule.Level) {
		case "DEBUG":
			level = parser.DEBUG
		case "INFO":
			level = parser.INFO
		case "WARN", "WARNING":
			level = parser.WARN
		case "ERROR":
			level = parser.ERROR
		case "FATAL":
			level = parser.FATAL
		}

		count := snapshot.Counts[level]
		var value float64
		switch rule.Condition {
		case "count":
			value = float64(count)
		case "rate":
			if rule.WindowSeconds > 0 {
				value = snapshot.Rate * float64(rule.WindowSeconds)
			} else {
				value = snapshot.Rate
			}
		}

		if value > float64(rule.Threshold) {
			if a.canFire(rule.Name) {
				a.fire(rule.Name, rule.Message)
			}
		}
	}
}

// canFire returns true if the rule hasn't fired recently (within cooldown period).
func (a *Alerter) canFire(name string) bool {
	lastFired, ok := a.fired[name]
	if !ok {
		return true
	}
	return time.Since(lastFired) > a.cooldown
}

// fire triggers an alert for the given rule.
func (a *Alerter) fire(name, message string) {
	a.fired[name] = time.Now()

	if message == "" {
		message = fmt.Sprintf("Alert triggered: %s", name)
	}

	if a.onAlert != nil {
		a.onAlert(name, message)
	} else {
		log.Printf("\033[91m[ALERT] %s\033[0m %s", name, message)
	}
}

// SetCooldown sets the minimum time between fires of the same rule.
func (a *Alerter) SetCooldown(d time.Duration) {
	a.cooldown = d
}

// Rules returns a copy of the current rules.
func (a *Alerter) Rules() []config.Rule {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]config.Rule, len(a.rules))
	copy(result, a.rules)
	return result
}
