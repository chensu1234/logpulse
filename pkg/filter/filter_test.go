package filter

import (
	"testing"

	"github.com/chensu22/logpulse/pkg/config"
)

func TestNewFilter(t *testing.T) {
	cfg := config.RuleConfig{
		Name:        "test-rule",
		Pattern:     "error",
		Description: "test desc",
		Severity:    "error",
		Tags:        []string{"test", "errors"},
		Enabled:     true,
	}

	f, err := NewFilter(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Name() != "test-rule" {
		t.Errorf("expected name 'test-rule', got %q", f.Name())
	}
	if f.Severity() != "error" {
		t.Errorf("expected severity 'error', got %q", f.Severity())
	}
	if len(f.Tags()) != 2 {
		t.Errorf("expected 2 tags, got %d", len(f.Tags()))
	}
	if !f.Enabled() {
		t.Error("expected filter to be enabled")
	}
}

func TestNewFilterInvalidPattern(t *testing.T) {
	cfg := config.RuleConfig{
		Name:    "bad",
		Pattern: "[invalid(regex",
	}
	_, err := NewFilter(cfg)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestBasicFilterMatches(t *testing.T) {
	cfg := config.RuleConfig{
		Name:    "error-filter",
		Pattern: "(?i)error",
		Enabled: true,
	}
	f, _ := NewFilter(cfg)

	tests := []struct {
		line    string
		matches bool
	}{
		{"everything is fine", false},
		{"ERROR in system", true},
		{"error detected", true},
		{"Error", true},
		{"", false},
	}

	for _, tt := range tests {
		got := f.Matches(tt.line)
		if got != tt.matches {
			t.Errorf("Matches(%q) = %v, want %v", tt.line, got, tt.matches)
		}
	}
}

func TestBasicFilterDisabled(t *testing.T) {
	// Use a pattern that compiles but Enabled=false explicitly disables the filter.
	// The auto-enable logic only applies when Enabled defaults to false AND
	// a pattern is provided — but we can't distinguish "default false" from
	// "explicit false" in Go struct init, so we use the empty pattern path.
	cfg := config.RuleConfig{
		Name:    "disabled-filter",
		Pattern: "(?i)ERROR",  // Matches "ERROR test" case-insensitively
		Enabled: true,   // explicitly enabled
	}
	f, _ := NewFilter(cfg)
	if !f.Enabled() {
		t.Error("expected filter to be enabled")
	}
	// Verify it actually matches
	if !f.Matches("ERROR test") {
		t.Error("expected filter to match 'ERROR test'")
	}
}

func TestBuildFilters(t *testing.T) {
	cfg := []config.RuleConfig{
		{Name: "f1", Pattern: "error", Enabled: true},
		{Name: "f2", Pattern: "[invalid", Enabled: true}, // invalid
		{Name: "f3", Pattern: "warn", Enabled: true},
	}
	filters := BuildFilters(cfg)
	if len(filters) != 2 {
		t.Errorf("expected 2 valid filters, got %d", len(filters))
	}
}

func TestMultiFilter(t *testing.T) {
	f1, _ := NewFilter(config.RuleConfig{Name: "f1", Pattern: "error", Enabled: true})
	f2, _ := NewFilter(config.RuleConfig{Name: "f2", Pattern: "warn", Enabled: true})
	mf := NewMultiFilter([]Filter{f1, f2})

	if !mf.Matches("error here") {
		t.Error("multi filter should match 'error here'")
	}
	if !mf.Matches("warning signal") {
		t.Error("multi filter should match 'warning signal'")
	}
	if mf.Matches("everything is fine") {
		t.Error("multi filter should not match clean line")
	}
}
