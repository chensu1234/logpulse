// Package monitor provides log tailing, processing, and statistics aggregation.
package monitor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chensu/logpulse/internal/alert"
	"github.com/chensu/logpulse/internal/config"
	"github.com/chensu/logpulse/internal/parser"
)

// Monitor is the core log monitoring engine.
type Monitor struct {
	cfg                *config.Config
	cliCfg             *config.CLIConfig
	parser             *parser.Parser
	stats              *Stats
	alerter            *alert.Alerter
	outputCh           chan string
	statCh             chan parser.Level
	skipOutputRenderer bool
}

// NewMonitor creates a new Monitor instance.
func NewMonitor(cfg *config.Config, cliCfg *config.CLIConfig, alrt *alert.Alerter) (*Monitor, error) {
	p, err := parser.NewParser(cliCfg.Level, cliCfg.Pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	return &Monitor{
		cfg:      cfg,
		cliCfg:   cliCfg,
		parser:   p,
		stats:    NewStats(3.0),
		alerter:  alrt,
		outputCh: make(chan string, 1000),
		statCh:   make(chan parser.Level, 1000),
	}, nil
}

// Run starts the monitoring pipeline and blocks until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	wg.Add(1)
	go m.runStatAggregator(ctx, &wg)

	if m.alerter != nil {
		wg.Add(1)
		go m.runAlertEvaluator(ctx, &wg)
	}

	if !m.skipOutputRenderer {
		wg.Add(1)
		go m.runOutputRenderer(ctx, &wg)
	}

	var err error
	if m.cliCfg.File != "" {
		err = m.tailFile(ctx)
	} else {
		err = m.tailStdin(ctx)
	}

	// Signal producers done; let aggregator flush then close statCh
	// Consumers close their channels when ctx is cancelled
	wg.Wait()
	return err
}

func (m *Monitor) tailFile(ctx context.Context) error {
	file, err := os.Open(m.cliCfg.File)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", m.cliCfg.File, err)
	}
	defer file.Close()

	if m.cfg.Monitor.Follow {
		info, err := file.Stat()
		if err == nil {
			file.Seek(info.Size(), 0)
		}
	}

	reader := bufio.NewReader(file)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				if m.cfg.Monitor.Follow {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return nil
			}
			if err != nil {
				return fmt.Errorf("read error: %w", err)
			}
			line = strings.TrimRight(line, "\r\n")
			if line != "" {
				m.processLine(line)
			}
		}
	}
}

func (m *Monitor) tailStdin(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("stdin read error: %w", err)
			}
			line = strings.TrimRight(line, "\r\n")
			if line != "" {
				m.processLine(line)
			}
		}
	}
}

func (m *Monitor) processLine(line string) {
	result := m.parser.Parse(line)
	if result == nil {
		return
	}

	select {
	case m.statCh <- result.Level:
	default:
	}

	if !m.cliCfg.Quiet {
		select {
		case m.outputCh <- line:
		default:
		}
	}
}

func (m *Monitor) runStatAggregator(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case level, ok := <-m.statCh:
			if !ok {
				return
			}
			m.stats.Record(level)
		}
	}
}

func (m *Monitor) runAlertEvaluator(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot := m.stats.Snapshot()
			m.alerter.Evaluate(snapshot)
		}
	}
}

func (m *Monitor) runOutputRenderer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-m.outputCh:
			if !ok {
				return
			}
			fmt.Println(line)
		}
	}
}

// Stats returns the current Stats object for external access.
func (m *Monitor) Stats() *Stats {
	return m.stats
}

// OutputChannel returns the output channel for external consumers.
func (m *Monitor) OutputChannel() chan string {
	return m.outputCh
}

// SkipOutputRenderer disables the internal output renderer goroutine.
func (m *Monitor) SkipOutputRenderer() {
	m.skipOutputRenderer = true
}
