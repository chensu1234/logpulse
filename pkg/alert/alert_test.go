package alert

import (
	"sync"
	"testing"
	"time"
)

// countingHandler counts events for testing.
type countingHandler struct {
	mu      sync.Mutex
	count   int
	events  []Event
}

func (h *countingHandler) Send(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.events = append(h.events, e)
}

func (h *countingHandler) Close() {}

func TestEngineCooldown(t *testing.T) {
	engine := NewEngine(EngineConfig{
		Cooldown:     1 * time.Second,
		MaxPerMinute: 100,
	})
	h := &countingHandler{}
	engine.RegisterHandler(h)

	evt := Event{Rule: "test-rule", Message: "test"}

	// First event should pass.
	engine.Handle(evt)
	if h.count != 1 {
		t.Errorf("expected 1, got %d", h.count)
	}

	// Immediate second event should be blocked by cooldown.
	engine.Handle(evt)
	if h.count != 1 {
		t.Errorf("expected 1 (cooldown), got %d", h.count)
	}

	// After cooldown, should pass again.
	time.Sleep(1100 * time.Millisecond)
	engine.Handle(evt)
	if h.count != 2 {
		t.Errorf("expected 2 after cooldown, got %d", h.count)
	}
}

func TestEngineRateLimit(t *testing.T) {
	engine := NewEngine(EngineConfig{
		Cooldown:     1 * time.Millisecond, // Very small cooldown
		MaxPerMinute: 5,
		BurstWindow:  1 * time.Minute,
	})
	h := &countingHandler{}
	engine.RegisterHandler(h)

	evt := Event{Rule: "test-rule", Message: "test"}

	// Should allow up to MaxPerMinute with tiny cooldown between events.
	for i := 0; i < 5; i++ {
		engine.Handle(evt)
		time.Sleep(2 * time.Millisecond) // tiny delay
	}
	if h.count != 5 {
		t.Errorf("expected 5, got %d", h.count)
	}

	// Wait for cooldown to fully expire, then 6th should pass cooldown
	// but be blocked by rate limit.
	time.Sleep(50 * time.Millisecond) // cooldown fully expired
	engine.Handle(evt)
	if h.count != 5 {
		t.Errorf("expected 5 (rate limited), got %d", h.count)
	}
}

func TestEngineGlobalLimit(t *testing.T) {
	engine := NewEngine(EngineConfig{
		Cooldown:     1 * time.Millisecond,
		MaxPerMinute: 3,
	})
	h := &countingHandler{}
	engine.RegisterHandler(h)

	// Different rules but same global limit applies.
	for i := 0; i < 3; i++ {
		engine.Handle(Event{Rule: "rule-1", Message: "test"})
		time.Sleep(2 * time.Millisecond)
	}
	// Wait for cooldown
	time.Sleep(50 * time.Millisecond)
	// 4th event (any rule) should be global rate limited.
	engine.Handle(Event{Rule: "rule-2", Message: "test"})
	if h.count != 3 {
		t.Errorf("expected 3 (global rate limit), got %d", h.count)
	}
}

func TestEngineNoOutputs(t *testing.T) {
	// Should not panic when no handlers registered.
	engine := NewEngine(EngineConfig{})
	engine.Handle(Event{Rule: "test", Message: "test"})
}

func TestNewEngineDefaults(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	if engine.config.Cooldown != 30*time.Second {
		t.Errorf("expected 30s cooldown default")
	}
	if engine.config.MaxPerMinute != 60 {
		t.Errorf("expected 60/min default")
	}
}

func TestEventFields(t *testing.T) {
	now := time.Now()
	evt := Event{
		Timestamp: now,
		Rule:      "test-rule",
		Message:   "test message",
		LineNum:   42,
		Severity:  "error",
		Tags:      []string{"test"},
		Count:     1,
	}

	if evt.Rule != "test-rule" {
		t.Errorf("expected rule 'test-rule', got %q", evt.Rule)
	}
	if evt.LineNum != 42 {
		t.Errorf("expected line 42, got %d", evt.LineNum)
	}
}
