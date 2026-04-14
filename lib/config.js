/**
 * Configuration loader – reads YAML config and applies defaults.
 * @module lib/config
 */

'use strict';

const fs   = require('fs');
const path = require('path');
const yaml = require('yaml');

/** Default configuration */
const DEFAULTS = {
  logLevel: 'info',
  dashboard: {
    enabled: true,
    port: 3000,
    cors: true,
  },
  watcher: {
    tailLines: 50,      // lines to read on startup before watching
    debounceMs: 100,    // debounce rapid writes
    maxLineLength: 4096,
  },
  patterns: [],        // array of { name, regex, severity, message }
  alerts: [],          // array of alert configs
  anomaly: {
    enabled: true,
    /** Lines-per-second threshold for spike detection */
    rateThreshold: 100,
    /** Window in ms for rate calculation */
    rateWindowMs: 5000,
    /** Consecutive error ratio before triggering anomaly */
    errorRatioThreshold: 0.5,
  },
};

/**
 * Load and merge config from a YAML file.
 * @param {string} configPath
 * @returns {object}
 */
function load(configPath) {
  if (!fs.existsSync(configPath)) {
    // Return defaults if no config file exists
    return { ...DEFAULTS };
  }

  const raw = fs.readFileSync(configPath, 'utf8');
  const parsed = yaml.parse(raw) || {};

  // Deep merge
  return deepMerge({ ...DEFAULTS }, parsed);
}

/** Deep merge two objects (source overrides target) */
function deepMerge(target, source) {
  for (const key of Object.keys(source)) {
    if (
      source[key] !== null &&
      typeof source[key] === 'object' &&
      !Array.isArray(source[key]) &&
      typeof target[key] === 'object' &&
      !Array.isArray(target[key])
    ) {
      target[key] = deepMerge(target[key] || {}, source[key]);
    } else {
      target[key] = source[key];
    }
  }
  return target;
}

module.exports = { load, DEFAULTS };
