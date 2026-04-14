/**
 * File watcher – tails one or more log files, emits line events,
 * and detects anomalies (rate spikes & error bursts).
 * @module lib/watcher
 */

'use strict';

const fs   = require('fs');
const path = require('path');
const chokidar = require('chokidar');
const { EventEmitter } = require('events');
const logger = require('./logger');

/** Severity levels */
const SEV = { INFO: 'info', WARN: 'warn', ERROR: 'error', CRITICAL: 'critical' };

class Watcher extends EventEmitter {
  /**
   * @param {object} options
   * @param {string[]} options.files – absolute paths to watch
   * @param {boolean} [options.tail] – read last N lines before watching
   * @param {object[]} options.patterns – pattern definitions from config
   * @param {object} options.stats – shared stats object
   */
  constructor({ files, tail = false, patterns = [], stats }) {
    super();
    this.files   = files;
    this.tail    = tail;
    this.patterns = patterns.map(p => ({
      ...p,
      regex: typeof p.regex === 'string' ? new RegExp(p.regex, 'i') : p.regex,
    }));
    this.stats   = stats;

    // Per-file state
    this._handles   = {};   // filename → { pos, fh, lineBuf }
    this._watchers  = {};   // filename → chokidar watcher
    this._rateCount = 0;    // lines in current rate window
    this._rateTimer = null;
    this._anomalyState = { errors: 0, total: 0 };
    this._closed    = false;

    // Start rate counter
    this._startRateCounter();
  }

  // ─── Public API ─────────────────────────────────────────────────────────────

  /** Begin watching all files. Resolves when initial tail (if any) is done. */
  async start() {
    await Promise.all(this.files.map(f => this._open(f)));
  }

  /** Stop all watchers and close all file handles. */
  async close() {
    this._closed = true;
    clearInterval(this._rateTimer);

    await Promise.all(
      Object.values(this._watchers).map(w => w.close())
    );
    for (const { fh } of Object.values(this._handles)) {
      if (fh) fh.release();
    }
    this._handles = {};
    logger.debug('all watchers closed');
  }

  // ─── Internals ──────────────────────────────────────────────────────────────

  /** Open a file: initial tail read + start chokidar watch. */
  async _open(filePath) {
    const absPath = path.resolve(filePath);

    // Ensure directory exists (for chokidar watch events on new files)
    const dir = path.dirname(absPath);
    if (!fs.existsSync(dir)) {
      logger.warn({ dir }, 'directory does not exist, skipping');
      return;
    }

    // Open file handle for persistent tail
    let fh;
    try {
      fh = fs.openSync(absPath, 'r');
    } catch (err) {
      logger.error({ err, file: absPath }, 'cannot open file');
      return;
    }

    const stat = fs.fstatSync(fh);
    const state = { pos: stat.size, fh, lineBuf: '' };
    this._handles[absPath] = state;

    // Tail existing content
    if (this.tail) {
      await this._tailFrom(absPath, state, stat.size);
    }

    // Start watching
    const watcher = chokidar.watch(absPath, {
      persistent: true,
      usePolling: false,
      ignoreInitial: true,
    });

    watcher.on('change', () => this._onChange(absPath));
    watcher.on('unlink', () => logger.info({ file: absPath }, 'file deleted'));

    this._watchers[absPath] = watcher;
    logger.debug({ file: absPath }, 'watching file');
  }

  /**
   * Read the last N lines of a file starting from `fromPos`.
   * Used for initial tail before starting live watch.
   */
  async _tailFrom(filePath, state, fromPos) {
    const BUF_SIZE = 8192;
    const lines = [];
    let pos = fromPos;

    while (pos > 0 && lines.length < 50) {
      const chunk = Math.min(BUF_SIZE, pos);
      pos -= chunk;
      const buf = Buffer.alloc(chunk);
      const bytes = fs.readSync(state.fh, buf, 0, chunk, pos);
      if (bytes === 0) break;
      const str = buf.toString('utf8', 0, bytes);
      const blockLines = str.split('\n');
      // First fragment may be incomplete line
      if (pos > 0) lines.unshift(...blockLines.slice(1));
      else lines.unshift(...blockLines);
    }

    // Emit without anomaly scoring for historical tail
    for (const line of lines.slice(-50)) {
      if (line) this._emitLine(filePath, line, false);
    }
    logger.debug({ file: filePath, linesRead: Math.min(lines.length, 50) }, 'tailed historical lines');
  }

  /** Called by chokidar when a watched file changes. */
  _onChange(filePath) {
    if (this._closed) return;
    const state = this._handles[filePath];
    if (!state) return;

    try {
      const stat = fs.fstatSync(state.fh);
      if (stat.size > state.pos) {
        this._read增量(filePath, state, stat.size);
        state.pos = stat.size;
      } else if (stat.size < state.pos) {
        // Log rotation: file was truncated, reset position
        logger.info({ file: filePath }, 'log rotation detected, resetting position');
        state.pos = 0;
        this._read增量(filePath, state, stat.size);
        state.pos = stat.size;
      }
    } catch (err) {
      logger.warn({ err, file: filePath }, 'error reading file after change');
    }
  }

  /** Read new bytes from `state.pos` to `endPos`. */
  _read增量(filePath, state, endPos) {
    const len = endPos - state.pos;
    if (len <= 0) return;

    const buf = Buffer.alloc(len);
    const bytes = fs.readSync(state.fh, buf, 0, len, state.pos);
    if (bytes === 0) return;

    const chunk = buf.toString('utf8', 0, bytes);
    state.lineBuf += chunk;

    const lines = state.lineBuf.split('\n');
    // Keep last fragment as unterminated line
    state.lineBuf = lines.pop();

    for (const line of lines) {
      this._emitLine(filePath, line, true);
    }
  }

  /**
   * Process a single log line: match patterns, update stats, emit events.
   * @param {string} filePath
   * @param {string} line
   * @param {boolean} scoreAnomaly – whether to count this line in anomaly detection
   */
  _emitLine(filePath, line, scoreAnomaly) {
    if (!line || line.length > 4096) return;

    const timestamp = new Date().toISOString();
    let severity = SEV.INFO;
    let matchedPattern = null;

    // ── Pattern matching ───────────────────────────────────────────────────
    for (const p of this.patterns) {
      if (p.regex.test(line)) {
        matchedPattern = p.name;
        severity = p.severity || SEV.INFO;
        break;
      }
    }

    const event = {
      id:        `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
      timestamp,
      file:      path.basename(filePath),
      filePath,
      line,
      severity,
      pattern:   matchedPattern,
    };

    // ── Stats ─────────────────────────────────────────────────────────────
    this.stats.bytesRead += Buffer.byteLength(line, 'utf8') + 1;
    if (matchedPattern) this.stats.linesMatched++;

    // ── Anomaly scoring ────────────────────────────────────────────────────
    if (scoreAnomaly) {
      this._anomalyState.total++;
      if (severity === SEV.ERROR || severity === SEV.CRITICAL) {
        this._anomalyState.errors++;
      }
      this._checkAnomaly();
    }

    this.emit('line', event);
  }

  /** Simple rate-based anomaly detection: fire when lines/sec exceeds threshold. */
  _checkAnomaly() {
    this._rateCount++;
  }

  /** Track lines/sec over a sliding window. */
  _startRateCounter() {
    const winMs = 5000;
    this._rateTimer = setInterval(() => {
      const rate = this._rateCount / (winMs / 1000);
      if (rate > 100) {
        this.emit('anomaly', {
          id:        `anomaly-${Date.now()}`,
          timestamp: new Date().toISOString(),
          type:      'rate_spike',
          rate,
          threshold: 100,
          message:   `High log rate: ${rate.toFixed(1)} lines/sec`,
          severity:  SEV.WARN,
        });
        this.stats.anomalies++;
      }
      this._rateCount = 0;

      // Check error ratio
      const { errors, total } = this._anomalyState;
      if (total > 20 && errors / total > 0.5) {
        this.emit('anomaly', {
          id:        `anomaly-${Date.now()}`,
          timestamp: new Date().toISOString(),
          type:      'error_burst',
          errorRatio: (errors / total).toFixed(3),
          totalErrors: errors,
          message:   `Error ratio spike: ${((errors/total)*100).toFixed(1)}% errors`,
          severity:  SEV.ERROR,
        });
        this._anomalyState = { errors: 0, total: 0 };
      }
    }, 5000);
  }
}

module.exports = Watcher;
