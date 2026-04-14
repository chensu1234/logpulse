/**
 * Pino logger wrapper – structured JSON logging to stdout.
 * Pretty-printed to console when NODE_ENV=development.
 * @module lib/logger
 */

'use strict';

const pino = require('pino');

const logLevel = process.env.LOG_LEVEL || 'info';

const transport = process.env.NODE_ENV === 'development'
  ? {
      target: 'pino-pretty',
      options: {
        colorize: true,
        translateTime: 'SYS:standard',
        ignore: 'pid,hostname',
      },
    }
  : undefined;

const logger = pino({
  level: logLevel,
  base: { pid: process.pid },
  timestamp: pino.stdTimeFunctions.isoDate,
}, transport);

module.exports = logger;
