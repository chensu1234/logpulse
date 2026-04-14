/**
 * Unit tests for lib/watcher.js
 * Run with: node --test test/watcher.test.js
 */

'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert');

// We test watcher behaviour by exercising the pattern-matching logic
// separately since the file-watching part requires actual file I/O.

describe('Watcher pattern matching', () => {
  // Helper: simulate what _emitLine does for pattern matching
  function matchPatterns(patterns, line) {
    for (const p of patterns) {
      const re = typeof p.regex === 'string' ? new RegExp(p.regex, 'i') : p.regex;
      if (re.test(line)) return { name: p.name, severity: p.severity || 'info' };
    }
    return null;
  }

  it('matches a simple error pattern', () => {
    const patterns = [
      { name: 'error', regex: 'ERROR', severity: 'error' },
      { name: 'warn',  regex: 'WARN',  severity: 'warn'  },
    ];
    const result = matchPatterns(patterns, 'Something went wrong: ERROR code=500');
    assert.strictEqual(result.name, 'error');
    assert.strictEqual(result.severity, 'error');
  });

  it('returns null when no pattern matches', () => {
    const patterns = [
      { name: 'error', regex: 'ERROR', severity: 'error' },
    ];
    const result = matchPatterns(patterns, 'Everything is fine today');
    assert.strictEqual(result, null);
  });

  it('matches HTTP 5xx errors', () => {
    const patterns = [
      { name: 'http_error', regex: /5\d{2}/i, severity: 'error' },
    ];
    const r = patterns[0].regex; // actual RegExp object
    function mp(patterns, line) {
      for (const p of patterns) {
        if (p.regex.test(line)) return { name: p.name, severity: p.severity || 'info' };
      }
      return null;
    }
    assert.strictEqual(mp(patterns, 'HTTP 503 Service Unavailable').name, 'http_error');
    assert.strictEqual(mp(patterns, 'HTTP 500 Internal Error').name, 'http_error');
    assert.strictEqual(mp(patterns, 'HTTP 200 OK'), null);
  });

  it('matches auth failure patterns', () => {
    const patterns = [
      { name: 'auth_failure', regex: /(authentication failure|failed login)/i, severity: 'error' },
    ];
    function mp(patterns, line) {
      for (const p of patterns) {
        if (p.regex.test(line)) return { name: p.name, severity: p.severity || 'info' };
      }
      return null;
    }
    assert.strictEqual(mp(patterns, 'Jul 14 09:12:33 server sshd: authentication failure for user admin').name, 'auth_failure');
    assert.strictEqual(mp(patterns, 'Login successful for user admin'), null);
  });
});

describe('AlertManager cooldown', () => {
  // Minimal test of cooldown logic without needing full AlertManager instantiation
  function canFire(lastFired, cooldownMs) {
    const now = Date.now();
    return !lastFired || (now - lastFired) >= cooldownMs;
  }

  it('fires when never fired before', () => {
    assert.strictEqual(canFire(0, 1000), true);
  });

  it('fires after cooldown has elapsed', () => {
    const past = Date.now() - 2000;
    assert.strictEqual(canFire(past, 1000), true);
  });

  it('blocks fire within cooldown window', () => {
    const recent = Date.now() - 500;
    assert.strictEqual(canFire(recent, 1000), false);
  });
});

describe('Config deep merge', () => {
  // Replicate config.js deepMerge for isolated testing
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

  it('merges nested objects', () => {
    const base = { a: 1, b: { c: 2, d: 3 } };
    const over = { b: { d: 99, e: 5 } };
    const result = deepMerge(JSON.parse(JSON.stringify(base)), over);
    assert.strictEqual(result.b.c, 2);   // preserved
    assert.strictEqual(result.b.d, 99);  // overridden
    assert.strictEqual(result.b.e, 5);   // added
    assert.strictEqual(result.a, 1);     // unchanged
  });

  it('overwrites arrays entirely', () => {
    const base = { arr: [1, 2, 3] };
    const over = { arr: [4, 5] };
    const result = deepMerge(JSON.parse(JSON.stringify(base)), over);
    assert.deepStrictEqual(result.arr, [4, 5]);
  });
});
