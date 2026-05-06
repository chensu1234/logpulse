# logpulse Specification

## Overview

**logpulse** is a CLI tool for real-time log monitoring and pattern analysis. It tails log files or stdin, colorizes output by log level, detects patterns (IPs, URLs, error signatures), computes live statistics, and fires alerts when thresholds are exceeded.

## Architecture

```
stdin/file → Parser → Monitor (stats + filter) → Output (colorized)
                              ↓
                         Alert Engine
```

### Components

| Component | Responsibility |
|-----------|---------------|
| `config` | Load YAML config, CLI flags, merge with defaults |
| `parser` | Parse log lines: detect level, extract IPs/URLs, regex match |
| `monitor` | Tail input (file or stdin), feed lines through pipeline, compute stats |
| `stats` | Counters per level, lines/sec rate, spike detection |
| `output` | Render colorized lines to terminal, support normal/JSON/quiet modes |
| `alert` | Evaluate rules against stats, fire alerts on threshold breach |

### Concurrency Model

- **main goroutine**: orchestrates shutdown
- **tail goroutine**: reads lines from file/stdin, sends on channel
- **processor goroutine(s)**: parse + filter + output each line
- **stats goroutine**: receives stats updates, computes rates, triggers alerts
- **alert goroutine**: evaluates rules on interval, fires notifications

Channels:
- `logCh`: raw lines from tail → processor
- `statCh`: stats deltas from processor → stats engine

## Log Level Detection

The parser inspects the line for known level markers (case-insensitive):

| Level | Markers |
|-------|---------|
| DEBUG | `DEBUG`, `[DEBUG]`, `[DBG]` |
| INFO | `INFO`, `[INFO]`, `[I]` |
| WARN | `WARN`, `WARNING`, `[WARN]`, `[WARN]` |
| ERROR | `ERROR`, `[ERROR]`, `[ERR]` |
| FATAL | `FATAL`, `CRITICAL`, `[FATAL]`, `[CRIT]` |
| UNKNOWN | fallback |

Level can also be forced via `--level` CLI flag (filters out non-matching lines).

## Pattern Matching

### Built-in Patterns
- **IP**: `\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b` — IPv4 addresses
- **URL**: `https?://[^\s]+` — HTTP/HTTPS URLs
- **Error signature**: `(?i)(error|exception|failed|failure|stacktrace)` — error keywords

### Custom Patterns
Regex patterns passed via `--pattern` flag (multiple allowed). Lines matching any pattern are highlighted.

## Statistics

Tracked per-level:
- **Count**: total lines seen per level
- **Rate**: lines/sec rolling average (1s window)
- **Spike**: count exceeds `spikeThreshold` × previous average → spike flag

## Alerting

Alerts are defined in YAML config. Each rule has:
- `name`: identifier
- `condition`: `count` (absolute) or `rate` (per second)
- `level`: which log level to watch
- `threshold`: numeric threshold
- `windowSeconds`: time window for rate calculation
- `message`: alert message template

Example rule:
```yaml
alerts:
  - name: highErrorRate
    level: ERROR
    condition: rate
    threshold: 5
    windowSeconds: 60
    message: "ERROR rate exceeded 5/min"
```

## Output Modes

| Mode | Description |
|------|-------------|
| `normal` | Colorized lines printed to stdout |
| `json` | Each line as JSON: `{"time","level","message","patterns"}` |
| `quiet` | No line output, stats panel only |

Stats panel (bottom or side):
```
[10:23:45] DEBUG: 120  INFO: 450  WARN: 30  ERROR: 12  FATAL: 0  | 45.2 lines/s | ▲ spike
```

## Configuration

YAML config file (`--config` flag):
```yaml
display:
  colorScheme: "default"   # default | dark | light
  showTimestamp: true
  showStats: true
  statsPosition: "bottom"  # bottom | right
  statsIntervalSeconds: 1

parser:
  detectLevel: true
  extractPatterns: true

monitor:
  tailMode: true           # keep reading new lines
  maxBufferLines: 10000

alerts:
  enabled: true
  rules:
    - name: highError
      level: ERROR
      condition: count
      threshold: 10
      windowSeconds: 60
      message: "High ERROR count detected"
```

CLI flags override config file values.

## CLI Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file` | string | "" | Log file to tail (empty = stdin) |
| `--level` | string | "" | Filter: show only this level (DEBUG/INFO/WARN/ERROR/FATAL) |
| `--pattern` | string | "" | Highlight lines matching regex (repeatable) |
| `--stats` | bool | false | Show live stats panel |
| `--json` | bool | false | Output JSON instead of colored text |
| `--quiet` | bool | false | No line output, stats only |
| `--config` | string | "" | Path to YAML config file |
| `--alert` | string | "" | Path to alert rules YAML |
| `--no-color` | bool | false | Disable ANSI colors |
| `--follow` | bool | true | Follow file (tail -f behavior) |
| `--buffer` | int | 10000 | Max lines in buffer |

## File Structure

```
logpulse/
├── README.md
├── SPEC.md
├── LICENSE
├── CHANGELOG.md
├── go.mod
├── go.sum
├── bin/
│   └── logpulse
├── cmd/
│   └── logpulse/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── monitor/
│   │   ├── monitor.go
│   │   └── stats.go
│   ├── parser/
│   │   └── parser.go
│   ├── output/
│   │   └── output.go
│   └── alert/
│       └── alert.go
├── config/
│   └── logpulse.yaml
└── docs/
    └── screenshot.png
```

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing
- `github.com/hpcloud/tail` — reliable file tailing

## Release v0.1.0

- Initial release
- Core: tail stdin/file, level detection, color output
- Stats: per-level counts, rate, spike detection
- Pattern matching: regex, IP, URL, error signatures
- Alerting: configurable threshold rules
- Output modes: normal, JSON, quiet