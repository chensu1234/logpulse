// Package output provides colored and formatted log output rendering.
package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/chensu/logpulse/internal/alert"
	"github.com/chensu/logpulse/internal/config"
	"github.com/chensu/logpulse/internal/parser"
)

// Color codes for ANSI terminal output.
const (
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"

	// Level colors (default theme)
	ColorDebug  = "\033[36m" // Cyan
	ColorInfo   = "\033[32m" // Green
	ColorWarn   = "\033[33m" // Yellow
	ColorError  = "\033[31m" // Red
	ColorFatal  = "\033[35m" // Magenta
	ColorDim    = "\033[90m" // Dim gray

	// Dark theme variants
	DarkColorDebug = "\033[96m"
	DarkColorInfo  = "\033[92m"
	DarkColorWarn  = "\033[93m"
	DarkColorError = "\033[91m"
	DarkColorFatal = "\033[95m"
)

// Renderer handles formatting and colorizing log output.
type Renderer struct {
	cfg        *config.DisplayConfig
	colorCodes map[parser.Level]string
	noColor    bool
}

// NewRenderer creates a new Renderer with the given display config.
func NewRenderer(cfg *config.DisplayConfig) *Renderer {
	colorCodes := map[parser.Level]string{
		parser.DEBUG:   ColorDebug,
		parser.INFO:    ColorInfo,
		parser.WARN:    ColorWarn,
		parser.ERROR:   ColorError,
		parser.FATAL:   ColorFatal,
		parser.UNKNOWN: ColorReset,
	}

	if cfg != nil && cfg.ColorScheme == "dark" {
		colorCodes = map[parser.Level]string{
			parser.DEBUG:   DarkColorDebug,
			parser.INFO:    DarkColorInfo,
			parser.WARN:    DarkColorWarn,
			parser.ERROR:   DarkColorError,
			parser.FATAL:   DarkColorFatal,
			parser.UNKNOWN: ColorReset,
		}
	}

	noColor := cfg != nil && cfg.NoColor

	return &Renderer{
		cfg:        cfg,
		colorCodes: colorCodes,
		noColor:    noColor,
	}
}

// Render returns a colorized string for a log line.
func (r *Renderer) Render(level parser.Level, line string) string {
	if r.noColor {
		return line
	}
	color := r.colorCodes[level]
	if color == "" {
		color = ColorReset
	}
	return color + line + ColorReset
}

// RenderWithTimestamp adds a timestamp prefix before colorizing.
func (r *Renderer) RenderWithTimestamp(level parser.Level, line string, ts time.Time) string {
	tsStr := ts.Format("15:04:05")
	dim := ColorDim
	if r.noColor {
		dim = ""
	}
	prefix := fmt.Sprintf("%s[%s]%s ", dim, tsStr, ColorReset)
	return prefix + r.Render(level, line)
}

// RenderJSON returns a JSON-formatted string for a log entry.
func (r *Renderer) RenderJSON(result *parser.MatchResult) string {
	var sb strings.Builder
	sb.WriteString("{")
	sb.WriteString(fmt.Sprintf(`"timestamp":"%s",`, result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf(`"level":"%s",`, result.Level))
	sb.WriteString(fmt.Sprintf(`"message":%q,`, result.Raw))

	if len(result.IPs) > 0 {
		sb.WriteString(`"ips":[`)
		for i, ip := range result.IPs {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("%q", ip))
		}
		sb.WriteString("],")
	}
	if len(result.URLs) > 0 {
		sb.WriteString(`"urls":[`)
		for i, url := range result.URLs {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("%q", url))
		}
		sb.WriteString("],")
	}
	if result.HasError {
		sb.WriteString(`"has_error":true`)
	}
	sb.WriteString("}")
	return sb.String()
}

// StatsPanel renders a live statistics panel as a string.
func (r *Renderer) StatsPanel(snapshot alert.StatsSnapshot) string {
	if r.noColor {
		return renderPlainStatsPanel(snapshot)
	}
	return renderColoredStatsPanel(snapshot, r.colorCodes)
}

func renderPlainStatsPanel(snapshot alert.StatsSnapshot) string {
	spike := ""
	if snapshot.SpikeDetected {
		spike = " ▲spike"
	}
	return fmt.Sprintf("DEBUG:%d  INFO:%d  WARN:%d  ERROR:%d  FATAL:%d  | %.1f lines/s%s",
		snapshot.Counts[parser.DEBUG],
		snapshot.Counts[parser.INFO],
		snapshot.Counts[parser.WARN],
		snapshot.Counts[parser.ERROR],
		snapshot.Counts[parser.FATAL],
		snapshot.Rate,
		spike,
	)
}

func renderColoredStatsPanel(snapshot alert.StatsSnapshot, colors map[parser.Level]string) string {
	d := colors[parser.DEBUG]
	i := colors[parser.INFO]
	w := colors[parser.WARN]
	e := colors[parser.ERROR]
	f := colors[parser.FATAL]
	rst := ColorReset
	dim := ColorDim

	spikeStr := ""
	if snapshot.SpikeDetected {
		spikeStr = " \033[91m▲\033[0m spike"
	}

	return fmt.Sprintf("%s[DEBUG:%s%d%s] %s[INFO:%s%d%s] %s[WARN:%s%d%s] %s[ERROR:%s%d%s] %s[FATAL:%s%d%s] %s| %.1f lines/s%s%s",
		dim, d, snapshot.Counts[parser.DEBUG], rst,
		dim, i, snapshot.Counts[parser.INFO], rst,
		dim, w, snapshot.Counts[parser.WARN], rst,
		dim, e, snapshot.Counts[parser.ERROR], rst,
		dim, f, snapshot.Counts[parser.FATAL], rst,
		dim, snapshot.Rate, rst,
		spikeStr,
	)
}
