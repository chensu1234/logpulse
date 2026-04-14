/**
 * Built-in web dashboard – Express + WebSocket server that streams
 * live log events to connected browsers.
 * @module lib/dashboard
 */

'use strict';

const http = require('http');
const path = require('path');
const express = require('express');
const { WebSocketServer } = require('ws');
const logger = require('./logger');

const PORT_MIN = 1024;
const PORT_MAX = 65535;
const MAX_LINES = 500; // keep last N lines in memory for new clients

class Dashboard {
  constructor(options = {}) {
    this.port     = options.port || 3000;
    this.enabled  = options.enabled !== false;
    this._server  = null;
    this._wss     = null;
    this._clients = new Set();

    /** Ring buffer of recent lines */
    this._lineBuffer = [];
    /** Ring buffer of recent anomalies */
    this._anomalyBuffer = [];

    this.app = express();

    // ── Routes ────────────────────────────────────────────────────────────
    this.app.use(express.static(path.join(__dirname, '../web/ui')));

    this.app.get('/api/status', (req, res) => {
      res.json({
        uptime: process.uptime(),
        clients: this._clients.size,
        linesBuffered: this._lineBuffer.length,
        version: '1.0.0',
      });
    });

    this.app.get('/api/lines', (req, res) => {
      // Return last MAX_LINES lines
      res.json(this._lineBuffer.slice(-200));
    });

    this.app.get('/api/anomalies', (req, res) => {
      res.json(this._anomalyBuffer.slice(-50));
    });
  }

  /** Start the HTTP + WebSocket server. */
  start() {
    if (!this.enabled) return;

    this._server = http.createServer(this.app);
    this._wss = new WebSocketServer({ server: this._server });

    this._wss.on('connection', (ws) => {
      this._clients.add(ws);
      logger.debug({ clients: this._clients.size }, 'dashboard client connected');

      // Send current buffer to new client
      if (this._lineBuffer.length > 0) {
        ws.send(JSON.stringify({ type: 'snapshot', lines: this._lineBuffer.slice(-100) }));
      }

      ws.on('close', () => this._clients.delete(ws));
      ws.on('error', () => this._clients.delete(ws));
    });

    this._server.listen(this.port, () => {
      logger.info({ port: this.port }, 'dashboard listening');
    });
  }

  /** Stop the server. */
  stop() {
    for (const ws of this._clients) ws.close();
    this._clients.clear();
    this._server?.close();
    logger.debug('dashboard stopped');
  }

  // ─── Event handlers (called by watcher) ──────────────────────────────────

  /** Push a new line event to all connected clients. */
  pushLine(event) {
    this._append(this._lineBuffer, MAX_LINES, event);
    this._broadcast(JSON.stringify({ type: 'line', ...event }));
  }

  /** Push a new anomaly event to all connected clients. */
  pushAnomaly(event) {
    this._append(this._anomalyBuffer, 100, event);
    this._broadcast(JSON.stringify({ type: 'anomaly', ...event }));
  }

  /** Broadcast raw event to all WS clients. */
  broadcast(event) {
    this._broadcast(JSON.stringify(event));
  }

  broadcastAnomaly(event) {
    this._broadcast(JSON.stringify({ type: 'anomaly', ...event }));
  }

  // ─── Helpers ─────────────────────────────────────────────────────────────

  _broadcast(data) {
    for (const ws of this._clients) {
      if (ws.readyState === 1 /* OPEN */) {
        try { ws.send(data); } catch (_) { /* ignore */ }
      }
    }
  }

  /** Append item to ring buffer, evicting oldest if over maxSize. */
  _append(buf, maxSize, item) {
    buf.push(item);
    if (buf.length > maxSize) buf.shift();
  }
}

module.exports = Dashboard;
