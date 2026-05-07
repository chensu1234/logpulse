// Package input provides log file tailing capabilities.
package input

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"
)

// FileTailConfig contains configuration for the file tailer.
type FileTailConfig struct {
	// Paths to files to tail.
	Paths []string

	// ReopenDelay is the time to wait before re-opening a missing file.
	ReopenDelay time.Duration

	// BufSize is the buffer size for reading lines.
	BufSize int

	// FollowNewFiles enables watching for new files matching the same pattern.
	FollowNewFiles bool
}

// FileTail monitors one or more log files in real time.
type FileTail struct {
	config  FileTailConfig
	lines   chan string
	errs    chan error
	done    chan struct{}
	wg      sync.WaitGroup
	mu      sync.RWMutex
	files   map[string]*os.File // filename → file handle
	offsets map[string]int64    // filename → last read position
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewFileTail creates a new file tailer.
func NewFileTail(cfg FileTailConfig) *FileTail {
	if cfg.ReopenDelay == 0 {
		cfg.ReopenDelay = 1 * time.Second
	}
	if cfg.BufSize == 0 {
		cfg.BufSize = 64 * 1024
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &FileTail{
		config:  cfg,
		lines:   make(chan string, 1000),
		errs:    make(chan error, 10),
		done:    make(chan struct{}),
		files:   make(map[string]*os.File),
		offsets: make(map[string]int64),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start begins tailing all configured files. It returns two channels:
// lines (matched log lines) and errors.
func (t *FileTail) Start() (chan string, chan error) {
	// Open all initial files.
	for _, path := range t.config.Paths {
		t.openFile(path)
	}

	// Start a goroutine to monitor each file.
	for _, path := range t.config.Paths {
		path := path // capture loop variable
		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			t.tailFile(path)
		}()
	}

	// Start a periodic re-check for missing files.
	if t.config.FollowNewFiles {
		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			t.watchForNewFiles()
		}()
	}

	return t.lines, t.errs
}

// openFile opens a file and seeks to the end if it exists.
func (t *FileTail) openFile(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Resolve the absolute path for consistent map keys.
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Check if already open.
	if _, ok := t.files[absPath]; ok {
		return
	}

	file, err := os.Open(absPath)
	if err != nil {
		// File doesn't exist yet — that's okay, we'll retry.
		return
	}

	// Seek to end so we only read new lines.
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return
	}

	t.files[absPath] = file
	t.offsets[absPath] = stat.Size()
}

// tailFile continuously reads new lines from a single file.
func (t *FileTail) tailFile(path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-time.After(t.config.ReopenDelay):
		}

		t.mu.RLock()
		file, ok := t.files[absPath]
		if !ok {
			t.mu.RUnlock()
			t.openFile(path)
			t.mu.RLock()
			file, ok = t.files[absPath]
			if !ok {
				t.mu.RUnlock()
				continue
			}
		}
		offset := t.offsets[absPath]
		t.mu.RUnlock()

		// Seek to last known position.
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			t.errs <- fmt.Errorf("seek error on %s: %w", path, err)
			continue
		}

		// Read new lines.
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, t.config.BufSize), t.config.BufSize)

		for scanner.Scan() {
			line := scanner.Text()
			select {
			case t.lines <- line:
			case <-t.ctx.Done():
				return
			}

			// Update offset.
			pos, _ := file.Seek(0, io.SeekCurrent)
			t.mu.Lock()
			t.offsets[absPath] = pos
			t.mu.Unlock()
		}

		if err := scanner.Err(); err != nil {
			t.errs <- fmt.Errorf("read error on %s: %w", path, err)
		}

		// Check if file was truncated (log rotation).
		t.mu.RLock()
		curOffset := t.offsets[absPath]
		t.mu.RUnlock()

		stat, err := file.Stat()
		if err != nil {
			continue
		}

		if stat.Size() < curOffset {
			// File was truncated — reset offset.
			t.mu.Lock()
			t.offsets[absPath] = 0
			t.mu.Unlock()

			// Reopen and seek to start.
			file.Close()
			t.mu.Lock()
			delete(t.files, absPath)
			t.mu.Unlock()

			t.openFile(path)
		}
	}
}

// watchForNewFiles periodically checks for new files matching target patterns.
func (t *FileTail) watchForNewFiles() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			for _, path := range t.config.Paths {
				// Expand glob patterns.
				matches, err := filepath.Glob(path)
				if err != nil {
					continue
				}
				for _, match := range matches {
					abs, _ := filepath.Abs(match)
					t.mu.RLock()
					_, exists := t.files[abs]
					t.mu.RUnlock()
					if !exists {
						t.openFile(match)
					}
				}
			}
		}
	}
}

// Stop halts all file monitoring.
func (t *FileTail) Stop() {
	t.cancel()

	// Close all open files.
	t.mu.Lock()
	for _, f := range t.files {
		f.Close()
	}
	t.files = nil
	t.offsets = nil
	t.mu.Unlock()

	// Wait for goroutines.
	t.wg.Wait()
	close(t.lines)
	close(t.errs)
}

// Done returns a channel that's closed when the tailer has stopped.
func (t *FileTail) Done() <-chan struct{} {
	return t.done
}

// Stats holds file tailing statistics.
type Stats struct {
	FilesOpened int
	LinesRead   uint64
	BytesRead   uint64
	Uptime      time.Duration
	Errors      int
}

// ComputeStats calculates current statistics.
func (t *FileTail) ComputeStats() Stats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s := Stats{
		FilesOpened: len(t.files),
		LinesRead:   uint64(len(t.lines)), // Approximate.
	}

	for _, f := range t.files {
		if fi, err := f.Stat(); err == nil {
			s.BytesRead += uint64(fi.Size())
		}
	}

	return s
}

// RuneCount returns the approximate rune count for a string,
// matching the behavior of utf8.RuneCountInString.
func RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}

// minInt returns the smaller of two integers.
func minInt(a, b int) int {
	return int(math.Min(float64(a), float64(b)))
}
