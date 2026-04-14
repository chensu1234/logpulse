#!/usr/bin/env node
/**
 * logpulse – Real-time log file watcher with pattern detection & anomaly alerting
 * Entry point
 */

'use strict';

const fs   = require('fs');
const path = require('path');
const { program } = require('commander');
const cfg  = require('../lib/config');
const logger = require('../lib/logger');
const Watcher = require('../lib/watcher');
const AlertManager = require('../lib/alert');
const Dashboard = require('../lib/dashboard');

// ─── CLI Options ──────────────────────────────────────────────────────────────
program
  .name('logpulse')
  .description('Real-time log file watcher with pattern detection & anomaly alerting')
  .version('1.0.0')
  .option('-c, --config <path>',   'Config file path',                    'config/logpulse.yml')
  .option('-f, --files <paths>',   'Log files to watch (comma-separated)','config/logpulse.yml')
  .option('-p, --port <number>',    'Dashboard HTTP port',                '3000')
  .option('-l, --log-level <level>','Log level (trace|debug|info|warn|error)','info')
  .option('--no-dashboard',        'Disable built-in web dashboard')
  .option('--tail',                'Start by reading the last 50 lines of each file before watching')
  .parse(process.argv);

const opts = program.opts();

// ─── Bootstrap ────────────────────────────────────────────────────────────────
async function main() {
  // Load configuration
  const configPath = path.resolve(opts.config);
  const config = cfg.load(configPath);

  // Merge CLI overrides
  if (opts.port)           config.dashboard = config.dashboard || {};
  if (opts.port)           config.dashboard.port = parseInt(opts.port, 10);
  if (opts.dashboard === false) config.dashboard = { ...config.dashboard, enabled: false };
  if (opts.logLevel)       config.logLevel = opts.logLevel;

  // Resolve files list
  const files = opts.files
    ? opts.files.split(',').map(f => path.resolve(f.trim()))
    : config.files || [];

  if (files.length === 0) {
    logger.error('No log files specified. Provide --files or define "files" in config.');
    process.exit(1);
  }

  logger.info({ files, port: config.dashboard?.port }, 'logpulse starting…');

  // ─── Alert manager ───────────────────────────────────────────────────────
  const alertManager = new AlertManager(config.alerts || []);

  // ─── File watcher ────────────────────────────────────────────────────────
  const watcher = new Watcher({
    files,
    tail: opts.tail,
    patterns: config.patterns || [],
    stats: { bytesRead: 0, linesMatched: 0, anomalies: 0 },
  });

  // Forward live events to alert manager & dashboard
  watcher.on('line', (event) => {
    alertManager.process(event);
    dashboard?.broadcast(event);
  });

  watcher.on('anomaly', (event) => {
    alertManager.processAnomaly(event);
    dashboard?.broadcastAnomaly(event);
  });

  watcher.on('error', (err) => logger.error({ err }, 'watcher error'));

  // ─── Web dashboard ──────────────────────────────────────────────────────
  let dashboard = null;
  if (config.dashboard?.enabled !== false) {
    dashboard = new Dashboard(config.dashboard || {});
    dashboard.start();
    watcher.on('line',    (e) => dashboard.pushLine(e));
    watcher.on('anomaly', (e) => dashboard.pushAnomaly(e));
    logger.info({ port: dashboard.port }, 'dashboard started');
  }

  // ─── Graceful shutdown ───────────────────────────────────────────────────
  const shutdown = async (signal) => {
    logger.info({ signal }, 'shutting down…');
    await watcher.close();
    dashboard?.stop();
    process.exit(0);
  };

  process.on('SIGINT',  () => shutdown('SIGINT'));
  process.on('SIGTERM', () => shutdown('SIGTERM'));

  // ─── Start watching ──────────────────────────────────────────────────────
  await watcher.start();

  logger.info({ watching: files.length, files }, 'watching log files');
}

main().catch((err) => {
  logger.fatal({ err }, 'fatal error');
  process.exit(1);
});
