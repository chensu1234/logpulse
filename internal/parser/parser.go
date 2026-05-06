// Package parser provides log line parsing and pattern extraction.
package parser

import (
	"regexp"
	"strings"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	// DEBUG is the least severe log level.
	DEBUG Level = iota
	// INFO is for informational messages.
	INFO
	// WARN is for warning messages.
	WARN
	// ERROR is for error messages.
	ERROR
	// FATAL is for fatal/critical messages.
	FATAL
	// UNKNOWN means the level could not be determined.
	UNKNOWN
)

// String returns the string representation of a Level.
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel attempts to detect the log level from a line of text.
// It looks for common level markers and returns the corresponding Level.
func ParseLevel(line string) Level {
	upper := strings.ToUpper(line)

	switch {
	case strings.Contains(upper, "FATAL") || strings.Contains(upper, "CRITICAL"):
		return FATAL
	case strings.Contains(upper, "ERROR") || strings.Contains(upper, "[ERR]"):
		return ERROR
	case strings.Contains(upper, "WARN") || strings.Contains(upper, "WARNING"):
		return WARN
	case strings.Contains(upper, "INFO") || strings.Contains(upper, "[I]"):
		return INFO
	case strings.Contains(upper, "DEBUG") || strings.Contains(upper, "[DBG]"):
		return DEBUG
	default:
		return UNKNOWN
	}
}

// Patterns holds compiled regex patterns for extraction.
type Patterns struct {
	IP   *regexp.Regexp
	URL  *regexp.Regexp
	Error *regexp.Regexp
}

// NewPatterns compiles and returns a Patterns struct with built-in patterns.
func NewPatterns() *Patterns {
	return &Patterns{
		IP:    regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`),
		URL:   regexp.MustCompile(`https?://[^\s]+`),
		Error: regexp.MustCompile(`(?i)(error|exception|failed|failure|stacktrace)`),
	}
}

// MatchResult holds the results of parsing a single log line.
type MatchResult struct {
	Timestamp time.Time
	Level     Level
	Raw       string
	Message   string
	IPs       []string
	URLs      []string
	HasError  bool
	Matched   bool // true if line matched user-specified pattern
}

// Parser handles parsing of log lines.
type Parser struct {
	patterns  *Patterns
	levelFilter Level
	userPatterns []*regexp.Regexp
}

// NewParser creates a new Parser with optional user-defined regex patterns.
func NewParser(levelFilter string, userPatterns []string) (*Parser, error) {
	p := &Parser{
		patterns: NewPatterns(),
		levelFilter: UNKNOWN,
		userPatterns: make([]*regexp.Regexp, 0),
	}

	if levelFilter != "" {
		switch strings.ToUpper(levelFilter) {
		case "DEBUG":
			p.levelFilter = DEBUG
		case "INFO":
			p.levelFilter = INFO
		case "WARN", "WARNING":
			p.levelFilter = WARN
		case "ERROR":
			p.levelFilter = ERROR
		case "FATAL":
			p.levelFilter = FATAL
		}
	}

	for _, pattern := range userPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		p.userPatterns = append(p.userPatterns, re)
	}

	return p, nil
}

// Parse processes a single log line and returns a MatchResult.
func (p *Parser) Parse(line string) *MatchResult {
	result := &MatchResult{
		Timestamp: time.Now(),
		Raw:       line,
		Message:   line,
	}

	// Detect level
	result.Level = ParseLevel(line)

	// Check level filter
	if p.levelFilter != UNKNOWN && result.Level != p.levelFilter {
		return nil
	}

	// Extract patterns
	if p.patterns != nil {
		result.IPs = p.patterns.IP.FindAllString(line, -1)
		result.URLs = p.patterns.URL.FindAllString(line, -1)
		result.HasError = p.patterns.Error.MatchString(line)
	}

	// Check user patterns
	result.Matched = len(p.userPatterns) == 0
	for _, re := range p.userPatterns {
		if re.MatchString(line) {
			result.Matched = true
			break
		}
	}

	return result
}

// ParseLine is a convenience wrapper for parsing a single line with defaults.
func ParseLine(line string) *MatchResult {
	p, _ := NewParser("", nil)
	return p.Parse(line)
}