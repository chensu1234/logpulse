package config

import (
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if len(cfg.Targets) != 0 {
		t.Errorf("expected no default targets, got %d", len(cfg.Targets))
	}
	if len(cfg.Rules) == 0 {
		t.Error("expected default rules")
	}
	if cfg.FilterMode != "all" {
		t.Errorf("expected filter_mode=all, got %s", cfg.FilterMode)
	}
}

func TestSeverityWeight(t *testing.T) {
	tests := []struct {
		severity string
		weight   int
	}{
		{"critical", 4},
		{"error", 3},
		{"warning", 2},
		{"info", 1},
		{"INFO", 1},
		{"ERROR", 3},
		{"", 0},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			got := SeverityWeight(tt.severity)
			if got != tt.weight {
				t.Errorf("SeverityWeight(%q) = %d, want %d", tt.severity, got, tt.weight)
			}
		})
	}
}

func TestConfigLoadValid(t *testing.T) {
	cfg := Default()
	data := []byte(`
targets:
  - /var/log/test.log
rules:
  - name: test-error
    pattern: 'error'
    severity: error
    enabled: true
`)
	if err := cfg.Load(data); err != nil {
		t.Errorf("unexpected load error: %v", err)
	}
	if len(cfg.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(cfg.Targets))
	}
}

func TestConfigLoadInvalidYAML(t *testing.T) {
	cfg := Default()
	data := []byte(`targets: [not valid yaml`)
	if err := cfg.Load(data); err == nil {
		t.Error("expected YAML parse error")
	}
}

func TestConfigValidateEmptyTargets(t *testing.T) {
	cfg := Default()
	cfg.Targets = nil
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty targets")
	}
}

func TestConfigValidateDuplicateRuleName(t *testing.T) {
	cfg := Default()
	cfg.Rules = []RuleConfig{
		{Name: "dup", Pattern: "a", Enabled: true},
		{Name: "dup", Pattern: "b", Enabled: true},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for duplicate rule name")
	}
}

func TestConfigValidateUnknownSeverity(t *testing.T) {
	cfg := Default()
	cfg.Rules = []RuleConfig{
		{Name: "bad-sev", Pattern: "test", Severity: "unknown-severity", Enabled: true},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for unknown severity")
	}
}

func TestConfigValidateUnknownOutputType(t *testing.T) {
	cfg := Default()
	cfg.Outputs = []OutputConfig{
		{Type: "unknown-type", Name: "bad"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for unknown output type")
	}
}
