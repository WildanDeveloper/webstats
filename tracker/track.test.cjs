const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const test = require('node:test');

function tracker(file, blocked = false) {
  let now = 1700000000000;
  let sequence = 0;
  const sent = [];
  const stored = new Map();
  const listeners = {};
  const observers = {};
  const context = {
    URL, URLSearchParams, Blob,
    Date: { now: () => now },
    document: {
      currentScript: { getAttribute: key => ({ 'data-site': 'site', 'data-vitals': '', 'data-host': 'https://stats.example' })[key] ?? null },
      readyState: 'complete', referrer: '', title: 'Test', cookie: '', visibilityState: 'visible',
      addEventListener: (name, fn) => { listeners[name] = fn; },
    },
    location: { pathname: '/', search: '', hash: '', origin: 'https://example.com' },
    screen: { width: 100, height: 100 },
    history: { pushState() {}, replaceState() {} },
    localStorage: {
      getItem(key) { if (blocked) throw Error('storage disabled'); return stored.get(key) ?? null; },
      setItem(key, value) { if (blocked) throw Error('storage disabled'); stored.set(key, value); },
    },
    navigator: { language: 'en', userAgent: 'Test' },
    crypto: { randomUUID: () => `id-${++sequence}` },
    fetch: (url, options) => { sent.push(JSON.parse(options.body)); return Promise.resolve({ ok: true }); },
    setTimeout: () => 1, clearTimeout() {},
    performance: { getEntriesByType: () => [] },
    PerformanceObserver: class {
      constructor(callback) { this.callback = callback; }
      observe({ type }) { observers[type] = this.callback; }
      takeRecords() { return []; }
      disconnect() {}
    },
    addEventListener: (name, fn) => { listeners[name] = fn; },
  };
  context.window = context;
  vm.runInNewContext(fs.readFileSync(file, 'utf8'), context);
  return { sent, context, listeners, observers, advance: ms => { now += ms; } };
}

for (const file of [__dirname + '/track.js', __dirname + '/../backend/internal/static/track.min.js']) {
  test(file + ': session persists in memory when storage is unavailable', () => {
    const t = tracker(file, true);
    t.context.webstats.event('test');
    assert.equal(t.sent[0].session_id, t.sent[1].session_id);
  });
  test(file + ': session expires after inactivity', () => {
    const t = tracker(file);
    const first = t.sent[0].session_id;
    t.advance(29 * 60000);
    t.context.webstats.event('active');
    assert.equal(t.sent.at(-1).session_id, first);
    t.advance(29 * 60000);
    t.context.webstats.event('active');
    assert.equal(t.sent.at(-1).session_id, first);
    t.advance(31 * 60000);
    t.context.webstats.event('new visit');
    assert.notEqual(t.sent.at(-1).session_id, first);
  });
}
