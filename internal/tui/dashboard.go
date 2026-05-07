// Package tui implements a terminal user interface dashboard for logpulse.
package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/chensu22/logpulse/pkg/alert"
	"github.com/mattn/go-runewidth"
)

// DashboardConfig contains configuration for the dashboard.
type DashboardConfig struct {
	Width      int
	NoColor    bool
	UpdateRate time.Duration
}

// Dashboard displays a real-time log monitoring dashboard in the terminal.
type Dashboard struct {
	config  DashboardConfig
	events  []alert.Event
	stats   DashboardStats
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// ANSI color codes.
	colors map[string]string
}

// DashboardStats holds dashboard statistics.
type DashboardStats struct {
	LinesRead   uint64
	AlertsFired uint64
	StartTime   time.Time
	RuleStats   map[string]uint64
}

// NewDashboard creates a new dashboard instance.
func NewDashboard(cfg DashboardConfig) *Dashboard {
	if cfg.Width == 0 {
		cfg.Width = 120
	}
	if cfg.UpdateRate == 0 {
		cfg.UpdateRate = 250 * time.Millisecond
	}

	d := &Dashboard{
		config: cfg,
		stopCh: make(chan struct{}),
		stats: DashboardStats{
			StartTime: time.Now(),
			RuleStats: make(map[string]uint64),
		},
		colors: map[string]string{
			"reset":    "\033[0m",
			"bold":     "\033[1m",
			"dim":      "\033[2m",
			"critical": "\033[38;5;9m",   // Red
			"error":    "\033[38;5;203m", // Bright red
			"warning":  "\033[38;5;220m", // Yellow
			"info":     "\033[38;5;75m",  // Cyan
			"header":   "\033[38;5;39m",  // Blue
			"green":    "\033[38;5;82m",  // Green
			"gray":     "\033[38;5;240m", // Gray
		},
	}

	return d
}

// Start begins the dashboard render loop.
func (d *Dashboard) Start() error {
	d.mu.Lock()
	d.running = true
	d.mu.Unlock()

	// Clear the screen and hide the cursor.
	fmt.Print("\033[2J\033[?25l")

	d.wg.Add(1)
	go d.renderLoop()

	return nil
}

// Record adds a new alert event to the dashboard.
func (d *Dashboard) Record(evt alert.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.events = append(d.events, evt)
	d.stats.AlertsFired++

	// Update per-rule stats.
	d.stats.RuleStats[evt.Rule]++

	// Keep only the most recent 100 events for display.
	if len(d.events) > 100 {
		d.events = d.events[len(d.events)-100:]
	}
}

// UpdateLines is called periodically to refresh the lines counter.
func (d *Dashboard) UpdateLines(n uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stats.LinesRead = n
}

// renderLoop periodically redraws the dashboard.
func (d *Dashboard) renderLoop() {
	defer d.wg.Done()
	ticker := time.NewTicker(d.config.UpdateRate)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.mu.RLock()
			if d.running {
				d.draw()
			}
			d.mu.RUnlock()
		}
	}
}

// draw renders the complete dashboard.
func (d *Dashboard) draw() {
	stats := d.stats
	uptime := time.Since(stats.StartTime)
	width := d.config.Width

	// Move cursor to top-left.
	fmt.Print("\033[H")

	var c func(key string) string
	if d.config.NoColor {
		c = func(key string) string { return "" }
	} else {
		c = func(key string) string { return d.colors[key] }
	}

	// Header.
	sep := strings.Repeat("─", width)
	header := fmt.Sprintf(" %slogpulse%s — real-time log monitor ", c("bold")+c("header"), c("reset"))
	fmt.Println(sep)
	printCentered(header, width)
	fmt.Println(sep)

	// Stats bar.
	statsLine := fmt.Sprintf(" %sUptime:%s %s  %sLines:%s %d  %sAlerts:%s %d  %sEvents:%s %d ",
		c("dim"), c("reset"),
		formatDuration(uptime),
		c("dim"), c("reset"),
		stats.LinesRead,
		c("dim"), c("reset"),
		stats.AlertsFired,
		c("dim"), c("reset"),
		len(d.events),
	)
	fmt.Println(statsLine)
	fmt.Println(sep)

	// Rule statistics.
	if len(stats.RuleStats) > 0 {
		fmt.Printf(" %sTop Rules%s\n", c("bold")+c("header"), c("reset"))
		fmt.Println(sep)

		type ruleStat struct {
			name  string
			count uint64
		}
		rules := make([]ruleStat, 0, len(stats.RuleStats))
		for name, count := range stats.RuleStats {
			rules = append(rules, ruleStat{name, count})
		}

		// Simple sort for top 5.
		for i := 0; i < len(rules); i++ {
			for j := i + 1; j < len(rules); j++ {
				if rules[j].count > rules[i].count {
					rules[i], rules[j] = rules[j], rules[i]
				}
			}
		}

		for i := 0; i < len(rules) && i < 5; i++ {
			barLen := minInt(int(rules[i].count), 50)
			bar := strings.Repeat("█", barLen)
			sev := d.colors["green"]
			fmt.Printf(" %s%-20s %s%-6d %s%s\n",
				sev, rules[i].name, c("gray"), rules[i].count,
				c("green"), bar)
		}
		fmt.Println(sep)
	}

	// Recent events.
	if len(d.events) > 0 {
		displayCount := minInt(len(d.events), 10)
		fmt.Printf(" %sRecent Alerts%s (last %d)\n", c("bold")+c("header"), c("reset"), displayCount)
		fmt.Println(sep)

		start := len(d.events) - displayCount
		for _, evt := range d.events[start:] {
			d.printEvent(evt)
		}
	} else {
		fmt.Printf(" %s%sWaiting for matching log lines...%s\n", c("dim"), c("gray"), c("reset"))
	}

	// Footer.
	fmt.Println(sep)
	fmt.Printf(" Press %sCtrl+C%s to stop\n", c("bold"), c("reset"))
}

// printEvent renders a single alert event.
func (d *Dashboard) printEvent(evt alert.Event) {
	ts := evt.Timestamp.Format("15:04:05")
	sev := evt.Severity
	if sev == "" {
		sev = "info"
	}

	var color string
	switch sev {
	case "critical":
		color = d.colors["critical"]
	case "error":
		color = d.colors["error"]
	case "warning":
		color = d.colors["warning"]
	default:
		color = d.colors["info"]
	}

	// Truncate long messages.
	msg := evt.Message
	if utf8.RuneCountInString(msg) > 60 {
		runes := []rune(msg)
		msg = string(runes[:57]) + "..."
	}

	sevStr := fmt.Sprintf("%-8s", sev)
	fmt.Printf(" %s[%s]%s %s[%s]%s %s%s%s %s\n",
		d.colors["dim"], ts, d.colors["reset"],
		color, sevStr, d.colors["reset"],
		color, evt.Rule, d.colors["reset"],
		msg,
	)
}

// Stop halts the dashboard and restores the terminal.
func (d *Dashboard) Stop() {
	d.mu.Lock()
	d.running = false
	d.mu.Unlock()

	close(d.stopCh)
	d.wg.Wait()

	// Restore cursor and clear screen.
	fmt.Print("\033[?25h\033[2J\033[H")
}

// printCentered prints a string centered within a given width.
func printCentered(s string, width int) {
	w := runewidth.StringWidth(s)
	pad := (width - w) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Printf("%s%s%s\n", strings.Repeat(" ", pad), s, strings.Repeat(" ", width-pad-w))
}

// formatDuration formats a duration as HH:MM:SS.
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
