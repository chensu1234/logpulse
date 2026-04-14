# logpulse

> Real-time log file watcher with pattern detection, anomaly alerting, and a live web dashboard.

![Node.js](https://img.shields.io/badge/node-%3E%3D18-green)
![License](https://img.shields.io/badge/license-MIT-blue)
![Version](https://img.shields.io/badge/version-1.0.0-black)

**logpulse** watches one or more log files in real time. As new lines appear it:
- Matches them against configurable regex **patterns** and assigns severity levels
- Detects **anomalies** — rate spikes and error bursts — automatically
- Fires **alerts** via log, webhook, or arbitrary shell commands
- Streams live events to a **built-in web dashboard** via WebSocket

---

## ✨ Features

| Feature | Description |
|---------|-------------|
| 🔍 **Multi-file tail** | Watch multiple log files simultaneously with automatic log-rotation detection |
| 🎨 **Pattern matching** | Define named regex patterns with severity escalation |
| 🚨 **Anomaly detection** | Rate-spike and error-ratio burst detection out of the box |
| 📡 **Web dashboard** | Live-updating UI via WebSocket — filter, search, and scroll |
| 📨 **Flexible alerts** | Log, webhook (HTTP POST), or shell command execution |
| ⏱️ **Cooldown engine** | Per-alert cooldown to prevent alert storms |
| 📄 **Tail history** | Read the last N lines of each file before watching begins |
| ⚙️ **YAML config** | Full configuration via a single YAML file, with CLI overrides |
| 🔄 **Log rotation safe** | Detects truncation/truncation and resets position automatically |

---

## 🏃 Quick Start

### Installation

```bash
# Clone
git clone https://github.com/YOUR_HANDLE/logpulse.git
cd logpulse

# Install dependencies
npm install

# Make executable
chmod +x bin/logpulse.js
```

### Start watching

```bash
# With the default config (edit config/logpulse.yml first)
npm start

# Override files via CLI
./bin/logpulse.js --files /var/log/syslog,/var/log/nginx/access.log

# Tail historical lines before watching
./bin/logpulse.js --files /var/log/app.log --tail

# Start without the web dashboard
./bin/logpulse.js --files /var/log/app.log --no-dashboard

# Use a custom config path
./bin/logpulse.js --config /etc/logpulse/prod.yml --port 8080
```

### Docker

```bash
docker run -v /var/log:/var/log:ro \
           -p 3000:3000 \
           logpulse --files /var/log/syslog
```

---

## ⚙️ Configuration

All settings live in `config/logpulse.yml`. CLI flags override config values.

```yaml
# Files to watch (absolute paths recommended)
files:
  - /var/log/syslog
  - /var/log/nginx/access.log

# Dashboard
dashboard:
  enabled: true
  port: 3000

# Pattern definitions
patterns:
  - name: http_error
    regex: '(5\d{2}|ERROR|CRITICAL)'
    severity: error

  - name: warning
    regex: '\b(WARN|WARNING)\b'
    severity: warn

# Anomaly detection
anomaly:
  enabled: true
  rateThreshold: 100    # lines/sec before triggering spike alert
  rateWindowMs: 5000
  errorRatioThreshold: 0.5  # fire if >50% of window lines are errors

# Alerts
alerts:
  - name: critical_alert
    channel: webhook
    webhook: 'https://your-webhook.example.com/alert'
    condition:
      severity: critical
    cooldownMs: 300000
```

---

## 📋 Command-Line Options

| Short | Long | Default | Description |
|-------|------|---------|-------------|
| `-c` | `--config <path>` | `config/logpulse.yml` | Path to YAML config file |
| `-f` | `--files <paths>` | from config | Comma-separated list of log files to watch |
| `-p` | `--port <number>` | `3000` | Dashboard HTTP port |
| `-l` | `--log-level <level>` | `info` | Log level: `trace`, `debug`, `info`, `warn`, `error` |
| — | `--no-dashboard` | — | Disable the built-in web dashboard |
| — | `--tail` | — | Read last 50 lines of each file before watching |
| `-V` | `--version` | — | Print version and exit |
| `-h` | `--help` | — | Print help and exit |

---

## 📁 Project Structure

```
logpulse/
├── bin/
│   └── logpulse.js          # Entry point & CLI parser
├── config/
│   └── logpulse.yml         # Sample configuration
├── lib/
│   ├── config.js            # YAML config loader + defaults
│   ├── logger.js           # Pino structured logger
│   ├── watcher.js          # File watcher + pattern matching + anomaly detection
│   ├── alert.js            # Alert dispatcher (log/webhook/exec)
│   └── dashboard.js         # Express + WebSocket dashboard server
├── web/
│   └── ui/
│       └── index.html      # Self-contained live dashboard UI
├── test/
│   └── watcher.test.js      # Unit tests
├── package.json
├── README.md
└── LICENSE
```

---

## 🎨 Dashboard

Open `http://localhost:3000` in your browser.

Features:
- **Live stream** of log lines with severity color-coding
- **Regex filter** text box — filters as you type
- **Severity dropdown** — show only errors/warnings/...
- **Pattern badge** — each matched pattern is labeled
- **Anomaly panel** — slide-up toast for detected anomalies
- **Auto-scroll** toggle — pause live stream to inspect

---

## 📝 CHANGELOG

### [1.0.0] – 2026-04-14

- Initial release
- Multi-file log tail with rotation detection
- Pattern matching with named tags and severity escalation
- Rate-spike and error-ratio anomaly detection
- Alert channels: log, HTTP webhook, shell exec
- Per-alert cooldown engine
- Built-in web dashboard (Express + WebSocket)
- YAML configuration with CLI override support

---

## 📄 License

MIT © 2026
