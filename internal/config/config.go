// Package config handles loading and merging configuration from YAML files and CLI flags.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level logpulse configuration.
type Config struct {
	Display DisplayConfig `yaml:"display"`
	Parser  ParserConfig  `yaml:"parser"`
	Monitor MonitorConfig  `yaml:"monitor"`
	Alerts  AlertsConfig  `yaml:"alerts"`
}

// DisplayConfig controls rendering and output behavior.
type DisplayConfig struct {
	ColorScheme          string `yaml:"colorScheme"`          // default|dark|light
	ShowTimestamp        bool   `yaml:"showTimestamp"`
	ShowStats            bool   `yaml:"showStats"`
	StatsPosition        string `yaml:"statsPosition"`        // bottom|right
	StatsIntervalSeconds int    `yaml:"statsIntervalSeconds"`
	NoColor              bool   `yaml:"noColor"`
}

// ParserConfig controls log line parsing.
type ParserConfig struct {
	DetectLevel    bool `yaml:"detectLevel"`
	ExtractPatterns bool `yaml:"extractPatterns"`
}

// MonitorConfig controls input monitoring.
type MonitorConfig struct {
	TailMode         bool `yaml:"tailMode"`
	MaxBufferLines   int  `yaml:"maxBufferLines"`
	Follow           bool `yaml:"follow"`
	BufferSize       int  `yaml:"bufferSize"`
}

// AlertsConfig controls alerting behavior.
type AlertsConfig struct {
	Enabled bool     `yaml:"enabled"`
	Rules   []Rule   `yaml:"rules"`
}

// Rule defines a single alerting rule.
type Rule struct {
	Name            string `yaml:"name"`
	Level           string `yaml:"level"`
	Condition       string `yaml:"condition"`   // count | rate
	Threshold       int    `yaml:"threshold"`
	WindowSeconds   int    `yaml:"windowSeconds"`
	Message         string `yaml:"message"`
}

// CLIConfig holds command-line flag values that override config file settings.
type CLIConfig struct {
	File          string
	Level         string
	Pattern       []string
	Stats         bool
	JSON          bool
	Quiet         bool
	ConfigPath    string
	AlertPath     string
	NoColor       bool
	Follow        bool
	Buffer        int
}

// Merge combines CLI flags with config file settings. CLI values take precedence.
func (c *CLIConfig) Merge(cfg *Config) {
	if c.NoColor {
		cfg.Display.NoColor = true
	}
	if c.Stats {
		cfg.Display.ShowStats = true
	}
	if c.Level != "" {
		cfg.Parser.DetectLevel = true
	}
	if c.Follow == false {
		cfg.Monitor.Follow = false
	}
	if c.Buffer > 0 {
		cfg.Monitor.MaxBufferLines = c.Buffer
	}
}

// Load reads a YAML configuration file and returns a Config struct.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return &cfg, nil
}

// LoadAlertRules reads a separate YAML file containing only alert rules.
func LoadAlertRules(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read alert rules file %s: %w", path, err)
	}

	var alerts AlertsConfig
	if err := yaml.Unmarshal(data, &alerts); err != nil {
		return nil, fmt.Errorf("failed to parse alert rules file %s: %w", path, err)
	}

	return alerts.Rules, nil
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Display: DisplayConfig{
			ColorScheme:          "default",
			ShowTimestamp:        true,
			ShowStats:            true,
			StatsPosition:        "bottom",
			StatsIntervalSeconds: 1,
			NoColor:              false,
		},
		Parser: ParserConfig{
			DetectLevel:     true,
			ExtractPatterns: true,
		},
		Monitor: MonitorConfig{
			TailMode:       true,
			MaxBufferLines: 10000,
			Follow:         true,
			BufferSize:     100,
		},
		Alerts: AlertsConfig{
			Enabled: true,
			Rules:   nil,
		},
	}
}

// Validate checks the config for invalid values and returns an error if found.
func (c *Config) Validate() error {
	validColorSchemes := map[string]bool{"default": true, "dark": true, "light": true}
	if !validColorSchemes[c.Display.ColorScheme] {
		return fmt.Errorf("invalid colorScheme: %s (must be default|dark|light)", c.Display.ColorScheme)
	}

	validPositions := map[string]bool{"bottom": true, "right": true}
	if !validPositions[c.Display.StatsPosition] {
		return fmt.Errorf("invalid statsPosition: %s (must be bottom|right)", c.Display.StatsPosition)
	}

	validConditions := map[string]bool{"count": true, "rate": true}
	for _, r := range c.Alerts.Rules {
		if !validConditions[r.Condition] {
			return fmt.Errorf("invalid alert condition %q for rule %q: must be count|rate", r.Condition, r.Name)
		}
		upperLevel := strings.ToUpper(r.Level)
		if upperLevel != "DEBUG" && upperLevel != "INFO" && upperLevel != "WARN" && upperLevel != "ERROR" && upperLevel != "FATAL" {
			return fmt.Errorf("invalid alert level %q for rule %q", r.Level, r.Name)
		}
	}

	return nil
}