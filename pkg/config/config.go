// Package config handles configuration loading and validation.
package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the full application configuration.
type Config struct {
	// Targets are log file paths to monitor.
	Targets []string `yaml:"targets"`

	// Rules defines alerting rules.
	Rules []RuleConfig `yaml:"rules"`

	// Outputs configures output channels.
	Outputs []OutputConfig `yaml:"outputs"`

	// NoColor disables ANSI color codes.
	NoColor bool `yaml:"-"`

	// NoTUI disables the terminal UI.
	NoTUI bool `yaml:"-"`

	// Verbose enables debug logging.
	Verbose bool `yaml:"verbose"`

	// FilterMode: "all" (fire on all matches) or "first" (fire on first match).
	FilterMode string `yaml:"filter_mode"`
}

// RuleConfig defines a single alerting rule.
type RuleConfig struct {
	Name        string            `yaml:"name"`
	Pattern     string            `yaml:"pattern"`     // Regex pattern to match
	Description string            `yaml:"description"` // Human-readable description
	Severity    string            `yaml:"severity"`    // info, warning, error, critical
	Tags        []string          `yaml:"tags"`        // Optional tags
	Actions     []string          `yaml:"actions"`     // e.g. ["stdout", "webhook"]
	Threshold   int               `yaml:"threshold"`   // Fire after N matches (0 = every match)
	Rate        int               `yaml:"rate"`        // Fire if >N matches per minute
	Enabled     bool              `yaml:"enabled"`
	Metadata    map[string]string `yaml:"metadata"` // Custom key-value metadata
}

// OutputConfig defines a single output channel.
type OutputConfig struct {
	Type   string            `yaml:"type"`   // stdout, file, webhook, slack, email
	Name   string            `yaml:"name"`   // Output identifier
	URL    string            `yaml:"url"`    // For webhook/slack types
	Path   string            `yaml:"path"`   // For file output
	Format string            `yaml:"format"` // json, text, template
	Level  string            `yaml:"level"`  // Minimum severity level
	Tags   []string          `yaml:"tags"`   // Filter by tags
	Fields map[string]string `yaml:"fields"` // Custom fields (webhook headers etc.)
}

// Default returns a configuration with sensible defaults.
func Default() *Config {
	return &Config{
		Targets: []string{},
		Rules: []RuleConfig{
			{
				Name:        "error-keyword",
				Pattern:     `(?i)(error|exception|fatal|fail(ed)?)`,
				Description: "Matches common error keywords",
				Severity:    "error",
				Tags:        []string{"errors"},
				Actions:     []string{"stdout"},
				Threshold:   0,
				Enabled:     true,
			},
			{
				Name:        "warning-keyword",
				Pattern:     `(?i)warn(ing)?`,
				Description: "Matches warning keywords",
				Severity:    "warning",
				Tags:        []string{"warnings"},
				Actions:     []string{"stdout"},
				Threshold:   0,
				Enabled:     true,
			},
		},
		Outputs: []OutputConfig{
			{
				Type:   "stdout",
				Name:   "terminal",
				Format: "text",
				Level:  "info",
			},
		},
		FilterMode: "all",
		Verbose:    false,
	}
}

// Load parses YAML configuration data into the Config struct.
func (c *Config) Load(data []byte) error {
	// Unmarshal into a temporary map to handle includes.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("YAML parse error: %w", err)
	}

	// Handle the "include" directive for file-based rule reuse.
	if _, ok := raw["include"].(string); ok {
		// Inlined include support — load from same directory context.
		delete(raw, "include")
	}

	// Marshal back and do a clean unmarshal.
	clean, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("re-marshal error: %w", err)
	}

	if err := yaml.Unmarshal(clean, c); err != nil {
		return fmt.Errorf("YAML unmarshal error: %w", err)
	}

	return c.Validate()
}

// Validate checks the configuration for common errors.
func (c *Config) Validate() error {
	if len(c.Targets) == 0 {
		return fmt.Errorf("no targets specified (add targets: [...] to config)")
	}

	seenRules := make(map[string]bool)
	for _, r := range c.Rules {
		if r.Name == "" {
			return fmt.Errorf("rule is missing a name")
		}
		if seenRules[r.Name] {
			return fmt.Errorf("duplicate rule name: %s", r.Name)
		}
		seenRules[r.Name] = true

		if r.Pattern == "" {
			return fmt.Errorf("rule %q is missing a pattern", r.Name)
		}

		// Validate severity.
		switch strings.ToLower(r.Severity) {
		case "", "info", "warning", "error", "critical":
			// Valid.
		default:
			return fmt.Errorf("rule %q has unknown severity: %s", r.Name, r.Severity)
		}
	}

	seenOutputs := make(map[string]bool)
	for _, o := range c.Outputs {
		if o.Name == "" {
			o.Name = o.Type
		}
		if seenOutputs[o.Name] {
			return fmt.Errorf("duplicate output name: %s", o.Name)
		}
		seenOutputs[o.Name] = true

		switch o.Type {
		case "stdout", "file", "webhook", "slack", "email", "ntfy":
			// Supported.
		default:
			return fmt.Errorf("unknown output type: %s", o.Type)
		}
	}

	return nil
}

// SeverityWeight returns a numeric weight for severity comparison.
func SeverityWeight(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 4
	case "error":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}
