# logpulse

**A smart, real-time log monitoring and pattern analysis CLI tool.**

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v0.1.0-blue)](https://github.com/chensu/logpulse/releases)
[![Build](https://img.shields.io/badge/Build-passing-brightgreen)](#)

logpulse tails log files or stdin, colorizes output by log level, extracts patterns (IPs, URLs, error signatures), tracks live statistics, and fires configurable alerts.

---

## ✨ Features

- **Real-time tailing** — follow files (`tail -f` style) or pipe stdin
- **Colorized output** — ANSI colors by level: DEBUG (cyan), INFO (green), WARN (yellow), ERROR (red), FATAL (magenta)
- **Pattern extraction** — auto-detects IPv4 addresses, HTTP URLs, and error keywords
- **Live statistics** — per-level counts, lines/sec rate, spike detection
- **Configurable alerting** — threshold-based rules (count or rate) with cooldown
- **Multiple output modes** — normal (colored), JSON, quiet (stats only)
- **Level filtering** — show only DEBUG, INFO, WARN, ERROR, or FATAL lines
- **Regex pattern matching** — highlight lines matching custom patterns
- **YAML configuration** — all options configurable via config file
- **Zero dependencies** — pure Go, single binary

---

## 🏃 Quick Start

### Install

**Via `go install`:**
```bash
go install github.com/chensu/logpulse@latest
```

**Via binary download** (macOS/Linux):
```bash
# macOS x86_64
curl -L https://github.com/chensu/logpulse/releases/latest/download/logpulse-darwin-amd64 -o logpulse
chmod +x logpulse
sudo mv logpulse /usr/local/bin/

# Linux x86_64
curl -L https://github.com/chensu/logpulse/releases/latest/download/logpulse-linux-amd64 -o logpulse
chmod +x logpulse
sudo mv logpulse /usr/local/bin/
```

**From source:**
```bash
git clone https://github.com/chensu/logpulse.git
cd logpulse
go build -o bin/logpulse ./cmd/logpulse/
./bin/logpulse --help
```

### Usage Examples

```bash
# Tail a log file with colorized output
logpulse --file /var/log/app.log

# Watch stdin (pipe from another command)
tail -f /var/log/app.log | logpulse

# Show only ERROR and FATAL lines
logpulse --file /var/log/app.log --level error

# Highlight lines matching a regex pattern
logpulse --file /var/log/app.log --pattern "failed|timeout"

# Show live statistics panel
logpulse --file /var/log/app.log --stats

# JSON output for downstream tools
logpulse --file /var/log/app.log --json

# Quiet mode (stats only, no line output)
logpulse --file /var/log/app.log --quiet --stats

# With custom alert rules
logpulse --file /var/log/app.log --alert ./alerts.yaml

# With YAML config file
logpulse --config ./logpulse.yaml --file /var/log/app.log

# Disable colors (useful in CI/CD)
logpulse --file /var/log/app.log --no-color
```

---

## ⚙️ Configuration

Create a `logpulse.yaml` config file:

```yaml
display:
  colorScheme: "default"       # default | dark | light
  showTimestamp: true
  showStats: true
  statsPosition: "bottom"       # bottom | right
  statsIntervalSeconds: 1
  noColor: false

parser:
  detectLevel: true            # auto-detect log level
  extractPatterns: true         # extract IPs, URLs, error keywords

monitor:
  tailMode: true
  maxBufferLines: 10000
  follow: true
  bufferSize: 100

alerts:
  enabled: true
  rules:
    - name: highErrorRate
      level: ERROR
      condition: rate            # count | rate
      threshold: 5
      windowSeconds: 60          # look back window
      message: "ERROR rate exceeded 5/min"
```

### Alert Rules

Each rule supports:
- `name` — unique identifier
- `level` — DEBUG, INFO, WARN, ERROR, FATAL
- `condition` — `count` (absolute) or `rate` (per second × window)
- `threshold` — numeric trigger value
- `windowSeconds` — time window for rate calculation
- `message` — custom alert message

---

## 📋 CLI Options

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--file` | `-f` | "" | Log file to tail (empty = stdin) |
| `--level` | | "" | Show only this level (DEBUG/INFO/WARN/ERROR/FATAL) |
| `--pattern` | | [] | Highlight lines matching regex (repeatable) |
| `--stats` | | false | Show live stats panel |
| `--json` | | false | Output JSON instead of colored text |
| `--quiet` | `-q` | false | No line output, stats only |
| `--config` | | "" | Path to YAML config file |
| `--alert` | | "" | Path to alert rules YAML file |
| `--no-color` | | false | Disable ANSI colors |
| `--follow` | | true | Follow file (tail -f behavior) |
| `--buffer` | | 10000 | Max lines in buffer |

---

## 📁 Project Structure

```
logpulse/
├── README.md
├── SPEC.md
├── LICENSE
├── CHANGELOG.md
├── go.mod
├── go.sum
├── bin/
│   └── logpulse              (compiled binary)
├── cmd/
│   └── logpulse/
│       └── main.go           (CLI entry point)
├── internal/
│   ├── config/
│   │   └── config.go         (YAML config loader)
│   ├── monitor/
│   │   ├── monitor.go        (core tail + pipeline)
│   │   └── stats.go          (statistics tracker)
│   ├── parser/
│   │   └── parser.go         (level detection, pattern extraction)
│   ├── output/
│   │   └── output.go         (colorized + JSON rendering)
│   └── alert/
│       └── alert.go          (threshold alerting engine)
├── config/
│   └── logpulse.yaml         (default config)
└── docs/
    └── screenshot.png
```

---

## 📝 CHANGELOG

### v0.1.0 (Initial Release)

- ✅ Real-time log tailing (file or stdin)
- ✅ Auto level detection (DEBUG/INFO/WARN/ERROR/FATAL)
- ✅ ANSI colorized output
- ✅ Pattern extraction (IP, URL, error keywords)
- ✅ Live statistics (counts, rate, spike detection)
- ✅ Configurable alert rules
- ✅ Normal / JSON / Quiet output modes
- ✅ Level filtering and regex pattern matching
- ✅ YAML configuration file support

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.
