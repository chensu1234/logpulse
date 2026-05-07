// Package main is the entry point for logpulse.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chensu22/logpulse/internal/tui"
	"github.com/chensu22/logpulse/pkg/alert"
	"github.com/chensu22/logpulse/pkg/config"
	"github.com/chensu22/logpulse/pkg/filter"
	"github.com/chensu22/logpulse/pkg/input"
	"github.com/chensu22/logpulse/pkg/output"
)

const (
	version     = "1.0.0"
	programName = "logpulse"
)

func main() {
	// Parse command-line flags.
	cfgPath := flag.String("config", "", "Path to configuration file")
	noColor := flag.Bool("no-color", false, "Disable colored output")
	noTUI := flag.Bool("no-tui", false, "Disable TUI dashboard (plain text mode)")
	verbose := flag.Bool("v", false, "Enable verbose output")
	showVersion := flag.Bool("version", false, "Show version information")
	testConfig := flag.Bool("test-config", false, "Test configuration and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s version %s\n", programName, version)
		os.Exit(0)
	}

	// Load configuration.
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	cfg.NoColor = *noColor
	cfg.NoTUI = *noTUI
	cfg.Verbose = *verbose

	if *testConfig {
		fmt.Println("✓ Configuration is valid")
		os.Exit(0)
	}

	// Set up cancellation context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Trap signals for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Build the log monitoring pipeline.
	pipe := buildPipeline(ctx, cfg)

	// Start all components.
	if err := pipe.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: failed to start pipeline: %v\n", err)
		os.Exit(1)
	}

	// Wait for termination signal.
	<-sigCh
	fmt.Println("\nShutting down...")

	// Gracefully stop the pipeline.
	stopCtx, stopCancel := context.WithTimeout(ctx, 5*time.Second)
	defer stopCancel()
	pipe.Stop(stopCtx)
}

// pipeline ties all components together and manages their lifecycle.
type pipeline struct {
	input    *input.FileTail
	filters  []filter.Filter
	alerter  *alert.Engine
	outputs  []output.Output
	ui       *tui.Dashboard
	lines    uint64
	duration time.Duration
}

func buildPipeline(ctx context.Context, cfg *config.Config) *pipeline {
	pipe := &pipeline{}

	// Initialize file tail input.
	pipe.input = input.NewFileTail(input.FileTailConfig{
		Paths:       cfg.Targets,
		ReopenDelay: 1 * time.Second,
		BufSize:     64 * 1024,
	})

	// Initialize filters.
	pipe.filters = filter.BuildFilters(cfg.Rules)

	// Initialize alert engine.
	pipe.alerter = alert.NewEngine(alert.EngineConfig{
		Cooldown:     30 * time.Second,
		MaxPerMinute: 60,
	})

	// Initialize outputs.
	pipe.outputs = output.BuildOutputs(cfg.Outputs)

	// Initialize TUI if not disabled.
	if !cfg.NoTUI {
		pipe.ui = tui.NewDashboard(tui.DashboardConfig{
			Width:      120,
			NoColor:    cfg.NoColor,
			UpdateRate: 250 * time.Millisecond,
		})
	}

	return pipe
}

func (p *pipeline) Start() error {
	start := time.Now()

	// Register outputs with the alerter.
	for _, out := range p.outputs {
		p.alerter.RegisterHandler(out)
	}

	// Start the TUI dashboard.
	if p.ui != nil {
		if err := p.ui.Start(); err != nil {
			return fmt.Errorf("failed to start TUI: %w", err)
		}
	}

	// Start the file tail.
	linesCh, errCh := p.input.Start()

	// Process lines asynchronously.
	go func() {
		for {
			select {
			case line, ok := <-linesCh:
				if !ok {
					return
				}
				p.processLine(line)
			case err := <-errCh:
				if err != nil {
					fmt.Fprintf(os.Stderr, "Input error: %v\n", err)
				}
				return
			case <-p.input.Done():
				return
			}
		}
	}()

	p.duration = time.Since(start)
	return nil
}

func (p *pipeline) processLine(line string) {
	p.lines++
	timestamp := time.Now()

	// Apply all filters.
	for _, f := range p.filters {
		if !f.Matches(line) {
			continue
		}

		// Filter matched — create alert event.
		evt := alert.Event{
			Timestamp: timestamp,
			Rule:      f.Name(),
			Message:   line,
			LineNum:   p.lines,
		}

		// Send to alerter.
		p.alerter.Handle(evt)

		// Forward to all outputs.
		for _, out := range p.outputs {
			out.Send(evt)
		}

		// Update TUI.
		if p.ui != nil {
			p.ui.Record(evt)
		}
	}
}

func (p *pipeline) Stop(ctx context.Context) {
	// Stop input first.
	p.input.Stop()

	// Stop all outputs.
	for _, out := range p.outputs {
		out.Close()
	}

	// Stop TUI.
	if p.ui != nil {
		p.ui.Stop()
	}
}

// loadConfig loads configuration from file or defaults.
func loadConfig(path string) (*config.Config, error) {
	cfg := config.Default()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		if err := cfg.Load(data); err != nil {
			return nil, fmt.Errorf("parsing config: %w", err)
		}
	}

	// Override targets from command-line arguments.
	if flag.NArg() > 0 {
		cfg.Targets = flag.Args()
	}

	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("no log files specified (use -config or provide file arguments)")
	}

	return cfg, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `%s — intelligent log monitoring and alerting

Usage: %s [options] [log-file ...]

Options:
  -config <path>       Path to YAML configuration file
  -no-color            Disable colored output
  -no-tui              Disable TUI dashboard (plain text mode)
  -v                   Enable verbose output
  -test-config         Validate configuration and exit
  -version             Print version information

Examples:
  %s /var/log/syslog
  %s -config config.yaml /var/log/nginx/access.log
  %s -no-tui /var/log/app.log

For full documentation, visit: https://github.com/chensu22/logpulse
`, programName, programName, programName, programName, programName)
}
