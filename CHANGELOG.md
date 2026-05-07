# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] — 2026-05-07

### Added
- **Initial release**
- Real-time log file tailing with automatic log rotation detection
- Regex-based alerting rules with configurable severity levels (`info`, `warning`, `error`, `critical`)
- Rate limiting engine with per-rule cooldown and burst detection
- Multiple output channels:
  - `stdout` — colored terminal output
  - `file` — structured log files (JSON or plain text)
  - `webhook` — generic HTTP webhook with custom headers and templates
  - `slack` — Slack incoming webhook integration
- Beautiful terminal TUI dashboard showing:
  - System uptime and line/alert counters
  - Top matching rules by frequency
  - Real-time scrolling recent alerts
- YAML-based configuration with sensible defaults
- Configuration validation via `-test-config`
- Zero external runtime dependencies — single static binary
- Cross-platform builds for Linux, macOS (Intel & ARM), and Windows
- Comprehensive Makefile with `build`, `test`, `release`, `install` targets
- Well-structured Go project layout following standard conventions

### Known Limitations
- No built-in encryption for webhook payloads (use HTTPS endpoints)
- No persistent alert history or database backend
- Rate limiting is in-memory only (resets on restart)
