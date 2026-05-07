# logpulse

> **Intelligent real-time log monitoring and alerting tool**

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](#)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey.svg)](#)

---

**logpulse** is a fast, zero-dependency log monitoring tool that tails one or more log files in real time, matches lines against configurable regex rules, and fires alerts through multiple output channels — terminal, file, webhooks, and Slack.

## ✨ Features

- **🔍 Real-time log tailing** — Follow multiple log files simultaneously with automatic log rotation detection
- **📐 Rule-based alerting** — Powerful regex-based pattern matching with severity levels (`info`, `warning`, `error`, `critical`)
- **⚡ Rate limiting & deduplication** — Built-in cooldown and burst detection to prevent alert fatigue
- **📊 Live TUI dashboard** — Beautiful terminal dashboard showing stats, top rules, and recent alerts
- **🔔 Multiple output channels** — stdout, file (JSON/text), webhooks, and Slack integration
- **🧩 Stateful filtering** — Per-rule thresholds, rate limits, and tag-based filtering
- **⚙️ YAML configuration** — Clean, declarative config file with sensible defaults
- **🚫 Zero runtime dependencies** — Single static binary, no interpreter or runtime required
- **🏗️ Cross-platform** — Builds for Linux, macOS, and Windows out of the box

## 🏃 Quick Start

### Installation

**Option 1: Download a binary** (fastest)

```bash
# Linux
curl -sL https://github.com/chensu22/logpulse/releases/latest/download/logpulse-linux-amd64 -o logpulse
chmod +x logpulse
sudo mv logpulse /usr/local/bin/

# macOS (Intel)
curl -sL https://github.com/chensu22/logpulse/releases/latest/download/logpulse-darwin-amd64 -o logpulse
chmod +x logpulse
sudo mv logpulse /usr/local/bin/

# macOS (Apple Silicon)
curl -sL https://github.com/chensu22/logpulse/releases/latest/download/logpulse-darwin-arm64 -o logpulse
chmod +x logpulse
sudo mv logpulse /usr/local/bin/
```

**Option 2: Build from source** (requires Go 1.21+)

```bash
git clone https://github.com/chensu22/logpulse.git
cd logpulse
make build
./bin/logpulse --version
```

### Usage

**Basic usage — tail a single log file:**

```bash
logpulse /var/log/syslog
```

**With a configuration file:**

```bash
logpulse -config config/config.yaml
```

**Monitor multiple files and show plain text (no TUI):**

```bash
logpulse -no-tui /var/log/nginx/access.log /var/log/nginx/error.log
```

**Test your configuration:**

```bash
logpulse -test-config -config config/config.yaml
```

**Generate some sample log output to test:**

```bash
# In one terminal, generate fake logs:
while true; do
  echo "[$(date)] ERROR: database connection failed" >> /tmp/test.log
  echo "[$(date)] WARN: request latency 2500ms" >> /tmp/test.log
  echo "[$(date)] INFO: server started on port 8080" >> /tmp/test.log
  sleep 1
done

# In another terminal:
logpulse /tmp/test.log
```

## ⚙️ Configuration

All configuration lives in `config/config.yaml`. Every field has a sensible default — you only need to configure what you need.

### Minimal config

```yaml
targets:
  - /var/log/syslog

rules:
  - name: error-keyword
    pattern: '(?i)error|exception|fatal'
    severity: error
    enabled: true

outputs:
  - type: stdout
    format: text
```

### Full config reference

```yaml
# ─────────────────────────────────────────
# targets — files to monitor
# ─────────────────────────────────────────
targets:
  - /var/log/syslog
  - /var/log/nginx/access.log

# ─────────────────────────────────────────
# rules — alerting rules (regex-based)
# ─────────────────────────────────────────
rules:
  - name: http-errors              # Unique rule name
    pattern: 'HTTP/[12]"? 5\d{2}'   # Regex pattern (PCRE-compatible)
    description: 'HTTP 5xx errors'  # Human-readable description
    severity: error                 # info | warning | error | critical
    tags: [http, server-errors]    # Optional tags for filtering
    threshold: 5                    # Fire after N matches (0 = every match)
    rate: 30                        # Max alerts per minute (0 = unlimited)
    enabled: true                   # Set false to disable

# ─────────────────────────────────────────
# outputs — alert destinations
# ─────────────────────────────────────────
outputs:
  - type: stdout                    # stdout | file | webhook | slack
    name: terminal
    format: text                    # text | json
    level: info                     # Minimum severity level to output

  - type: file
    name: alert-log
    path: log/alerts.log
    format: json
    level: warning

  - type: webhook
    name: generic-webhook
    url: https://your-webhook-endpoint.com/alerts
    level: error

  - type: slack
    name: slack-alerts
    url: https://hooks.slack.com/services/xxx/yyy/zzz
    level: warning
    fields:
      channel: "#alerts"

# ─────────────────────────────────────────
# general settings
# ─────────────────────────────────────────
verbose: false
filter_mode: all    # all = fire on every match | first = fire on first match per rule
```

## 📋 Command-Line Options

| Flag | Description | Default |
|------|-------------|---------|
| `-config <path>` | Path to YAML configuration file | _(none)_ |
| `-no-color` | Disable colored terminal output | `false` |
| `-no-tui` | Disable TUI dashboard (plain text mode) | `false` |
| `-v` | Enable verbose output | `false` |
| `-test-config` | Validate configuration and exit | `false` |
| `-version` | Print version information | `false` |
| `<files...>` | One or more log files to monitor | _(required)_ |

## 📁 Project Structure

```
logpulse/
├── cmd/
│   └── logpulse/
│       └── main.go              # Application entry point
├── pkg/
│   ├── alert/
│   │   └── alert.go             # Alert engine (rate limiting, deduplication)
│   ├── config/
│   │   └── config.go            # Configuration loading and validation
│   ├── filter/
│   │   └── filter.go            # Regex-based line filtering
│   ├── input/
│   │   └── filetail.go          # Log file tailing (follow + rotation)
│   └── output/
│       └── output.go            # Output handlers (stdout, file, webhook, slack)
├── internal/
│   └── tui/
│       └── dashboard.go         # Terminal UI dashboard
├── config/
│   └── config.yaml              # Example configuration
├── log/                         # Output log directory
│   └── .gitkeep
├── bin/                         # Build output directory
├── Makefile                     # Build automation
├── .gitignore
├── go.mod
├── README.md
├── CHANGELOG.md
└── LICENSE
```

## 📝 Rule Pattern Examples

```yaml
# Match error keywords (case-insensitive)
- pattern: '(?i)(error|exception|fatal|critical)'

# Match HTTP 5xx responses
- pattern: 'HTTP/[12]"? 5\d{2}'

# Match slow requests (>2 seconds)
- pattern: 'latency [2-9]\d{3,}ms|latency \d{5,}ms'

# Match specific IP addresses
- pattern: '192\.168\.\d+\.\d+|10\.\d+\.\d+\.\d+'

# Match JSON log entries with certain fields
- pattern: '"level":"error"|"severity":"error"'

# Match Go panic stack traces
- pattern: 'panic: |goroutine \d+ \[running\]'

# Match failed authentication
- pattern: '(?i)(auth.*fail|unauthorized|invalid token|jwt.*expired)'

# Match disk/memory pressure
- pattern: '(?i)(out of memory|no space left|disk quota)'
```

## 📝 CHANGELOG

See [CHANGELOG.md](CHANGELOG.md) for the full version history.

### v1.0.0 (2026-05-07)
- Initial release
- Real-time log file tailing with rotation detection
- Regex-based alerting rules with severity levels
- Multiple output channels (stdout, file, webhook, Slack)
- Rate limiting and deduplication engine
- Beautiful terminal TUI dashboard
- YAML-based configuration
- Zero external runtime dependencies

## 📄 License

logpulse is released under the MIT License. See [LICENSE](LICENSE) for details.

---

_Built with ❤️ for developers and DevOps engineers who need to keep an eye on their logs._
