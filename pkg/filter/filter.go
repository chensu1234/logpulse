// Package filter implements pattern-based log line filtering.
package filter

import (
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/chensu22/logpulse/pkg/config"
)

// Filter is a single filtering rule that matches log lines.
type Filter interface {
	// Name returns the unique name of this filter.
	Name() string

	// Matches returns true if the line matches this filter's criteria.
	Matches(line string) bool

	// Description returns the human-readable description.
	Description() string

	// Severity returns the severity level.
	Severity() string

	// Tags returns associated tags.
	Tags() []string

	// Enabled reports whether the filter is active.
	Enabled() bool
}

// basicFilter is a regex-based filter implementation.
type basicFilter struct {
	name        string
	pattern     *regexp.Regexp
	description string
	severity    string
	tags        []string
	enabled     bool

	// Statistics.
	mu         sync.RWMutex
	matchCount uint64
}

var _ Filter = (*basicFilter)(nil)

// NewFilter creates a new regex-based filter from configuration.
func NewFilter(cfg config.RuleConfig) (Filter, error) {
	// Compile the regex pattern.
	pat, err := regexp.Compile(cfg.Pattern)
	if err != nil {
		return nil, err
	}

	// Normalize severity.
	sev := strings.ToLower(cfg.Severity)
	if sev == "" {
		sev = "info"
	}

	// Handle enabled flag.
	// If Enabled is not set (false by zero value), default to true for rules with a pattern.
	enabled := cfg.Enabled
	if !enabled && cfg.Pattern != "" {
		enabled = true
	}

	return &basicFilter{
		name:        cfg.Name,
		pattern:     pat,
		description: cfg.Description,
		severity:    sev,
		tags:        cfg.Tags,
		enabled:     enabled,
	}, nil
}

func (f *basicFilter) Name() string        { return f.name }
func (f *basicFilter) Description() string { return f.description }
func (f *basicFilter) Severity() string    { return f.severity }
func (f *basicFilter) Tags() []string      { f.mu.RLock(); defer f.mu.RUnlock(); return f.tags }
func (f *basicFilter) Enabled() bool       { f.mu.RLock(); defer f.mu.RUnlock(); return f.enabled }

func (f *basicFilter) Matches(line string) bool {
	if !f.enabled {
		return false
	}

	// Perform a quick length pre-check before running the regex.
	if len(line) == 0 || utf8.RuneCountInString(line) < 3 {
		return false
	}

	if f.pattern.MatchString(line) {
		f.mu.Lock()
		f.matchCount++
		f.mu.Unlock()
		return true
	}
	return false
}

// MatchCount returns the number of lines matched by this filter.
func (f *basicFilter) MatchCount() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.matchCount
}

// BuildFilters creates a list of filters from configuration.
func BuildFilters(cfg []config.RuleConfig) []Filter {
	filters := make([]Filter, 0, len(cfg))
	for _, c := range cfg {
		f, err := NewFilter(c)
		if err != nil {
			// Log the error but continue with other filters.
			continue
		}
		filters = append(filters, f)
	}
	return filters
}

// MultiFilter wraps multiple filters and checks all of them.
type MultiFilter struct {
	filters []Filter
}

var _ Filter = (*MultiFilter)(nil)

// NewMultiFilter creates a filter that combines multiple sub-filters.
func NewMultiFilter(filters []Filter) *MultiFilter {
	return &MultiFilter{filters: filters}
}

func (m *MultiFilter) Name() string { return "multi" }

func (m *MultiFilter) Matches(line string) bool {
	for _, f := range m.filters {
		if f.Matches(line) {
			return true
		}
	}
	return false
}

func (m *MultiFilter) Description() string { return "combined filter" }
func (m *MultiFilter) Severity() string    { return "info" }
func (m *MultiFilter) Tags() []string      { return nil }
func (m *MultiFilter) Enabled() bool       { return true }
