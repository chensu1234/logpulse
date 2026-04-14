/**
 * Alert manager – processes line events and anomalies, dispatches
 * notifications via configured channels (log, webhook, exec).
 * @module lib/alert
 */

'use strict';

const { execSync } = require('child_process');
const logger = require('./logger');

const COOLDOWN_MS = 30_000; // minimum interval between repeated alerts of same name

class AlertManager {
  /**
   * @param {object[]} alertConfigs – array of alert definitions from config
   */
  constructor(alertConfigs = []) {
    /** @type {Map<string, number>} last fire timestamp per alert name */
    this._lastFired = new Map();
    this.alerts = alertConfigs.map(a => ({
      name:     a.name || 'unnamed',
      enabled:  a.enabled !== false,
      channel:  a.channel || 'log',    // 'log' | 'webhook' | 'exec'
      condition: a.condition || {},    // { pattern, severity }
      cooldownMs: a.cooldownMs || COOLDOWN_MS,
      // channel-specific
      webhook:  a.webhook,
      command:   a.command,
      ...a,
    }));
  }

  /**
   * Process a line event – check if any alert conditions are met.
   * @param {object} event
   */
  process(event) {
    for (const alert of this.alerts) {
      if (!alert.enabled) continue;
      if (!this._matches(alert.condition, event)) continue;
      if (!this._canFire(alert.name, alert.cooldownMs)) continue;

      this._dispatch(alert, event);
    }
  }

  /**
   * Process an anomaly event – always notify on anomalies.
   * @param {object} event
   */
  processAnomaly(event) {
    for (const alert of this.alerts) {
      if (!alert.enabled) continue;
      if (!this._canFire(alert.name, alert.cooldownMs)) continue;

      // Anomaly alerts are triggered by type or severity
      if (alert.condition.type && alert.condition.type !== event.type) continue;
      if (alert.condition.severity && alert.condition.severity !== event.severity) continue;

      this._dispatch(alert, event);
    }
  }

  // ─── Private ──────────────────────────────────────────────────────────────

  /** Check if an event matches alert condition filters. */
  _matches(condition, event) {
    if (condition.pattern && !new RegExp(condition.pattern, 'i').test(event.line)) {
      return false;
    }
    if (condition.severity && condition.severity !== event.severity) {
      return false;
    }
    if (condition.file && !new RegExp(condition.file, 'i').test(event.file)) {
      return false;
    }
    return true;
  }

  /** Enforce per-alert cooldown to prevent alert storms. */
  _canFire(name, cooldownMs) {
    const last = this._lastFired.get(name) || 0;
    if (Date.now() - last < cooldownMs) return false;
    this._lastFired.set(name, Date.now());
    return true;
  }

  /** Dispatch alert through the configured channel. */
  _dispatch(alert, event) {
    try {
      switch (alert.channel) {
        case 'webhook':
          this._sendWebhook(alert, event);
          break;
        case 'exec':
          this._runCommand(alert, event);
          break;
        default:
          this._logAlert(alert, event);
      }
    } catch (err) {
      logger.error({ err, alert: alert.name }, 'alert dispatch failed');
    }
  }

  _logAlert(alert, event) {
    const msg = `[ALERT:${alert.name}] ${event.message || event.line}`;
    switch (event.severity) {
      case 'error':
      case 'critical':
        logger.error({ alert: alert.name, event }, msg);
        break;
      case 'warn':
        logger.warn({ alert: alert.name, event }, msg);
        break;
      default:
        logger.info({ alert: alert.name, event }, msg);
    }
  }

  _sendWebhook(alert, event) {
    if (!alert.webhook) return;
    const payload = JSON.stringify({
      alert: alert.name,
      event,
      time: new Date().toISOString(),
    });

    try {
      // Use exec curl for simplicity (no external HTTP library needed)
      const cmd = `curl -s -X POST ${alert.webhook} ` +
        `-H "Content-Type: application/json" ` +
        `-d ${JSON.stringify(payload)}`;
      execSync(cmd, { timeout: 5000 });
      logger.info({ alert: alert.name, webhook: alert.webhook }, 'webhook alert sent');
    } catch (err) {
      logger.warn({ err, alert: alert.name }, 'webhook delivery failed');
    }
  }

  _runCommand(alert, event) {
    if (!alert.command) return;
    try {
      const expanded = alert.command
        .replace('{{line}}',  event.line)
        .replace('{{file}}',  event.file)
        .replace('{{severity}}', event.severity)
        .replace('{{pattern}}', event.pattern || '');
      execSync(expanded, { timeout: 10000, shell: '/bin/sh' });
      logger.info({ alert: alert.name, command: alert.command }, 'exec alert triggered');
    } catch (err) {
      logger.warn({ err, alert: alert.name }, 'exec alert failed');
    }
  }
}

module.exports = AlertManager;
