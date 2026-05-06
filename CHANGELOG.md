# Changelog

All notable changes to logpulse will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.1.0] - 2024-01-01

### Added
- Real-time log tailing from file or stdin
- Auto-detection of log levels: DEBUG, INFO, WARN, ERROR, FATAL
- ANSI colorized terminal output (default + dark theme)
- Built-in pattern extraction: IPv4 addresses, HTTP URLs, error keywords
- Live statistics: per-level counts, lines/sec rate, spike detection
- Configurable alerting with threshold rules (count or rate)
- Multiple output modes: normal (colored), JSON, quiet (stats only)
- Level filtering via `--level` flag
- Regex pattern matching via `--pattern` flag (repeatable)
- YAML configuration file support
- Spike detection: flags when rate exceeds 3× rolling average
- Alert cooldown: prevents duplicate alerts within 30s window

### Fixed
- N/A

### Changed
- N/A
