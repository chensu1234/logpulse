// Package output implements alert output channels.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/chensu22/logpulse/pkg/alert"
	"github.com/chensu22/logpulse/pkg/config"
)

// Output is the interface for all output handlers.
type Output interface {
	Send(alert.Event)
	Close()
	Name() string
}

// StdoutOutput writes alerts to standard output.
type StdoutOutput struct {
	name   string
	mu     sync.Mutex
	level  int
	format string
	colors map[string]string
}

// NewStdoutOutput creates a stdout output handler.
func NewStdoutOutput(cfg config.OutputConfig) *StdoutOutput {
	return &StdoutOutput{
		name:   cfg.Name,
		format: cfg.Format,
		level:  levelFromString(cfg.Level),
		colors: map[string]string{
			"critical": "\033[31m", // Red
			"error":    "\033[91m", // Bright red
			"warning":  "\033[33m", // Yellow
			"info":     "\033[36m", // Cyan
			"reset":    "\033[0m",
		},
	}
}

func (o *StdoutOutput) Name() string { return o.name }

func (o *StdoutOutput) Send(evt alert.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	severity := evt.Severity
	if severity == "" {
		severity = "info"
	}

	color := o.colors[severity]
	reset := o.colors["reset"]

	ts := evt.Timestamp.Format("15:04:05")
	rule := evt.Rule

	switch o.format {
	case "json":
		data, _ := json.MarshalIndent(map[string]interface{}{
			"timestamp": ts,
			"severity":  severity,
			"rule":      rule,
			"message":   evt.Message,
			"line":      evt.LineNum,
		}, "", "  ")
		fmt.Println(string(data))
	default:
		// Text format.
		fmt.Printf("%s[%s]%s %s[%s]%s %s%s%s %s\n",
			color, ts, reset,
			color, severity, reset,
			color, rule, reset,
			evt.Message,
		)
	}
}

func (o *StdoutOutput) Close() {}

// FileOutput writes alerts to a log file.
type FileOutput struct {
	name    string
	path    string
	file    *os.File
	mu      sync.Mutex
	format  string
	maxSize int64
	rotated int
}

// NewFileOutput creates a file output handler.
func NewFileOutput(cfg config.OutputConfig) (*FileOutput, error) {
	f, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open output file: %w", err)
	}

	return &FileOutput{
		name:   cfg.Name,
		path:   cfg.Path,
		file:   f,
		format: cfg.Format,
	}, nil
}

func (o *FileOutput) Name() string { return o.name }

func (o *FileOutput) Send(evt alert.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	var line string
	switch o.format {
	case "json":
		data, _ := json.Marshal(map[string]interface{}{
			"timestamp": evt.Timestamp.Format(time.RFC3339),
			"severity":  evt.Severity,
			"rule":      evt.Rule,
			"message":   evt.Message,
			"line":      evt.LineNum,
		})
		line = string(data) + "\n"
	default:
		ts := evt.Timestamp.Format("2006-01-02 15:04:05")
		line = fmt.Sprintf("[%s] [%s] %s: %s\n", ts, evt.Severity, evt.Rule, evt.Message)
	}

	o.file.WriteString(line)
}

func (o *FileOutput) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.file != nil {
		o.file.Close()
	}
}

// WebhookOutput sends alerts to a webhook endpoint.
type WebhookOutput struct {
	name    string
	url     string
	method  string
	headers map[string]string
	tmpl    *template.Template
	mu      sync.Mutex
	client  *http.Client
}

// NewWebhookOutput creates a webhook output handler.
func NewWebhookOutput(cfg config.OutputConfig) (*WebhookOutput, error) {
	tmpl, err := template.New("webhook").Parse(`{"text": "[{{.Severity}}] {{.Rule}}: {{.Message}}"}`)
	if err != nil {
		return nil, fmt.Errorf("parse webhook template: %w", err)
	}

	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"
	for k, v := range cfg.Fields {
		headers[k] = v
	}

	return &WebhookOutput{
		name:    cfg.Name,
		url:     cfg.URL,
		method:  "POST",
		headers: headers,
		tmpl:    tmpl,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (o *WebhookOutput) Name() string { return o.name }

func (o *WebhookOutput) Send(evt alert.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	var buf bytes.Buffer
	if err := o.tmpl.Execute(&buf, evt); err != nil {
		fmt.Fprintf(os.Stderr, "webhook template error: %v\n", err)
		return
	}

	req, err := http.NewRequest(o.method, o.url, &buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "webhook request error: %v\n", err)
		return
	}

	for k, v := range o.headers {
		req.Header.Set(k, v)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "webhook delivery error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "webhook returned error status: %d\n", resp.StatusCode)
	}
}

func (o *WebhookOutput) Close() {}

// SlackOutput sends formatted alerts to a Slack incoming webhook.
type SlackOutput struct {
	*WebhookOutput
	channel string
}

// NewSlackOutput creates a Slack output handler.
func NewSlackOutput(cfg config.OutputConfig) (*SlackOutput, error) {
	// Slack expects a specific JSON format.
	tmpl, err := template.New("slack").Parse(`{
		"channel": "{{.Tags}}",
		"username": "logpulse",
		"icon_emoji": ":warning:",
		"text": "[{{.Severity | printf \"%-8s\"}}] *{{.Rule}}*\n>{{.Message}}"
	}`)
	if err != nil {
		return nil, fmt.Errorf("parse slack template: %w", err)
	}

	return &SlackOutput{
		WebhookOutput: &WebhookOutput{
			name:   cfg.Name,
			url:    cfg.URL,
			method: "POST",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			tmpl:   tmpl,
			client: &http.Client{Timeout: 10 * time.Second},
		},
		channel: cfg.Fields["channel"],
	}, nil
}

// levelFromString converts a severity string to a numeric level.
func levelFromString(level string) int {
	switch level {
	case "critical":
		return 4
	case "error":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 1
	}
}

// BuildOutputs creates output handlers from configuration.
func BuildOutputs(cfg []config.OutputConfig) []Output {
	outputs := make([]Output, 0, len(cfg))
	for _, c := range cfg {
		var out Output
		var err error

		switch c.Type {
		case "stdout":
			out = NewStdoutOutput(c)
		case "file":
			out, err = NewFileOutput(c)
		case "webhook":
			out, err = NewWebhookOutput(c)
		case "slack":
			out, err = NewSlackOutput(c)
		default:
			fmt.Fprintf(os.Stderr, "Unknown output type: %s\n", c.Type)
			continue
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create output %s: %v\n", c.Name, err)
			continue
		}

		outputs = append(outputs, out)
	}
	return outputs
}
