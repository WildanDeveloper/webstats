











(function () {
  'use strict';

  var script = document.currentScript || document.querySelector('script[data-site]');
  if (!script) return;

  var SITE = script.getAttribute('data-site');
  if (!SITE) return;

  
  var host = script.getAttribute('data-host') || (script.src ? script.src.replace(/\/track\.js.*$/, '') : '');
  var auto = script.getAttribute('data-auto') !== 'false';
  var collect = host + '/api/collect';
  var eventUrl = host + '/api/event';

  var STORAGE = '_wst_sid';
  var OPTOUT = '_wst_optout';
  var QUEUE = '_wst_queue';

  var session = null;
  var SESSION_TIMEOUT = 30 * 60 * 1000;

  function getSessionId() {
    var now = Date.now();
    try {
      var stored = JSON.parse(localStorage.getItem(STORAGE));
      if (stored && typeof stored.id === 'string' && typeof stored.last === 'number' &&
          stored.last <= now && now - stored.last < SESSION_TIMEOUT) session = stored;
    } catch (e) {}
    if (!session || session.last > now || now - session.last >= SESSION_TIMEOUT) {
      session = { id: randomId(), last: now };
    }
    session.last = now;
    try { localStorage.setItem(STORAGE, JSON.stringify(session)); } catch (e) {}
    return session.id;
  }

  function randomId() {
    if (window.crypto && crypto.randomUUID) return crypto.randomUUID();
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
      var r = (Math.random() * 16) | 0;
      return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16);
    });
  }

  function optedOut() {
    try {
      if (localStorage.getItem(OPTOUT) === '1') return true;
    } catch (e) {}
    try {
      if (document.cookie.indexOf('webstats_optout') !== -1) return true;
    } catch (e) {}
    return false;
  }

  var MAX_TRIES = 5;

  function enqueue(payload) {
    try {
      var q = JSON.parse(localStorage.getItem(QUEUE) || '[]');
      payload._tries = (payload._tries || 0) + 1;
      if (payload._tries > MAX_TRIES) return;
      q.push(payload);
      localStorage.setItem(QUEUE, JSON.stringify(q.slice(-20)));
    } catch (e) {}
  }

  function send(payload) {
    if (optedOut()) return;
    var url = payload.kind === 'event' ? eventUrl : collect;
    try {
      var body = JSON.stringify(payload);
      if (navigator.sendBeacon) {
        var blob = new Blob([body], { type: 'application/json' });
        if (navigator.sendBeacon(url, blob)) return;
      }
      fetch(url, {
        method: 'POST',
        body: body,
        headers: { 'Content-Type': 'application/json' },
        keepalive: true
      }).then(function (res) {
        if (!res.ok) enqueue(payload);
      }).catch(function () { enqueue(payload); });
    } catch (e) {
      enqueue(payload);
    }
  }

  function flushQueue() {
    try {
      var q = JSON.parse(localStorage.getItem(QUEUE) || '[]');
      if (!q.length) return;
      localStorage.setItem(QUEUE, '[]');
      q.forEach(function (p) { try { send(p); } catch (e) {} });
    } catch (e) {}
  }

  function utmParams() {
    var out = { utm_source: '', utm_medium: '', utm_campaign: '', utm_content: '', utm_term: '' };
    try {
      var q = new URLSearchParams(location.search);
      for (var k in out) {
        if (q.has(k)) out[k] = (q.get(k) || '').slice(0, 200);
      }
    } catch (e) {}
    return out;
  }

  function pageview() {
    var ref = document.referrer;
    if (ref.indexOf(location.origin) === 0) ref = '';
    var u = utmParams();
    var payload = {
      kind: 'pageview',
      site_id: SITE,
      session_id: getSessionId(),
      id: randomId(),
      path: pagePath(),
      title: document.title || '',
      referrer: ref,
      screen: screen.width + 'x' + screen.height,
      lang: (navigator.language || '').slice(0, 5),
      ua: navigator.userAgent,
      ts: Date.now()
    };
    for (var k in u) {
      if (u[k]) payload[k] = u[k];
    }
    send(payload);
  }

   
  var lastPath = '';
  function hook() {
    var orig = history.pushState;
    history.pushState = function () {
      var r = orig.apply(this, arguments);
      onRoute();
      return r;
    };
    var origReplace = history.replaceState;
    history.replaceState = function () {
      var r = origReplace.apply(this, arguments);
      onRoute();
      return r;
    };
    window.addEventListener('popstate', onRoute);
    window.addEventListener('hashchange', onRoute);
  }

  function routeKey() {
    return location.pathname + location.search + location.hash;
  }

  function pagePath() {
    return location.pathname + location.search + location.hash;
  }

  function onRoute() {
    var p = routeKey();
    if (p === lastPath) return;
    lastPath = p;
    pageview();
  }

  function event(name, props) {
    if (!name || optedOut()) return;
    var payload = {
      kind: 'event',
      site_id: SITE,
      session_id: getSessionId(),
      id: randomId(),
      event_name: name,
      props: props || {},
      url: pagePath(),
      ua: navigator.userAgent,
      ts: Date.now()
    };
    send(payload);
  }

  if (auto) {
    lastPath = routeKey();
    if (document.readyState === 'complete' || document.readyState === 'interactive') {
      pageview();
    } else {
      window.addEventListener('load', pageview);
    }
    hook();
    flushQueue();
    setupAutoEvents();
  }

  function setupAutoEvents() {
    var wantOutbound = script.getAttribute('data-outbound') != null;
    var wantDownload = script.getAttribute('data-download') != null;
    var wantScroll = script.getAttribute('data-scroll') != null;
    var wantVitals = script.getAttribute('data-vitals') != null;
    if (wantVitals) setupVitals();
    if (!wantOutbound && !wantDownload && !wantScroll) return;

    if (wantOutbound || wantDownload) {
      var DL_RE = /\.(pdf|zip|rar|7z|tar|gz|tgz|docx?|xlsx?|pptx?|csv|mp[34]|m4a|wav|ogg|webm|avi|mov|dmg|exe|apk)$/i;
      document.addEventListener('click', function (e) {
        var t = e.target;
        var a = t && t.closest ? t.closest('a[href]') : null;
        if (!a) return;
        var href = a.href;
        if (!href) return;
        var u;
        try { u = new URL(href, location.href); } catch (err) { return; }
        if (u.protocol !== 'http:' && u.protocol !== 'https:') return;
        var clean = (u.pathname + u.search).split('?')[0].split('#')[0];
        if (wantDownload && DL_RE.test(clean)) { event('download', { url: href }); return; }
        if (wantOutbound && u.hostname && u.hostname !== location.hostname) event('outbound', { url: href });
      }, true);
    }

    if (wantScroll) {
      var seen = {};
      var seenRoute = '';
      window.addEventListener('scroll', function () {
        if (seenRoute !== routeKey()) { seen = {}; seenRoute = routeKey(); }
        var d = document.documentElement;
        var max = d.scrollHeight - window.innerHeight;
        var pct = max <= 0 ? 100 : Math.round(((window.scrollY || d.scrollTop) / max) * 100);
        [25, 50, 75, 100].forEach(function (t) {
          if (pct >= t && !seen[t]) { seen[t] = 1; event('scroll', { depth: t }); }
        });
      }, { passive: true });
    }
  }

  // Web Vitals (opt-in via data-vitals): LCP, CLS, INP, FCP, TTFB are sent
  // as web_vitals events with { metric, value }. FCP/TTFB are known early;
  // LCP/CLS/INP are flushed once stable and again on page hide with the
  // final value (each metric reports once per page).
  function setupVitals() {
    if (!window.PerformanceObserver) return;
    var sent = {};
    var vals = { lcp: null, cls: 0, inp: 0, fcp: null, ttfb: null };
    var deb = {};
    function report(m) {
      if (sent[m]) return;
      var v = vals[m];
      if (v == null) return;
      sent[m] = 1;
      if (deb[m]) { clearTimeout(deb[m]); deb[m] = null; }
      event('web_vitals', { metric: m, value: m === 'cls' ? Math.round(v * 1000) / 1000 : Math.round(v) });
    }
    function schedule(m, ms) {
      if (sent[m]) return;
      if (deb[m]) clearTimeout(deb[m]);
      deb[m] = setTimeout(function () { report(m); }, ms || 3000);
    }
    try {
      new PerformanceObserver(function (l) {
        var es = l.getEntries();
        for (var i = 0; i < es.length; i++) {
          if (es[i].name === 'first-contentful-paint') { vals.fcp = es[i].startTime; schedule('fcp', 1000); }
        }
      }).observe({ type: 'paint', buffered: true });
    } catch (e) {}
    try {
      new PerformanceObserver(function (l) {
        var es = l.getEntries();
        if (es.length) { vals.lcp = es[es.length - 1].startTime; schedule('lcp'); }
      }).observe({ type: 'largest-contentful-paint', buffered: true });
    } catch (e) {}
    try {
      new PerformanceObserver(function (l) {
        var es = l.getEntries();
        for (var i = 0; i < es.length; i++) {
          if (!es[i].hadRecentInput) { vals.cls += es[i].value; schedule('cls'); }
        }
      }).observe({ type: 'layout-shift', buffered: true });
    } catch (e) {}
    try {
      new PerformanceObserver(function (l) {
        var es = l.getEntries();
        for (var i = 0; i < es.length; i++) {
          if (es[i].duration > vals.inp) { vals.inp = es[i].duration; schedule('inp'); }
        }
      }).observe({ type: 'event', buffered: true, durationThreshold: 40 });
    } catch (e) {}
    try {
      var nav = performance.getEntriesByType && performance.getEntriesByType('navigation')[0];
      if (nav && nav.responseStart) { vals.ttfb = nav.responseStart; schedule('ttfb', 1000); }
    } catch (e) {}
    function flushAll() {
      ['fcp', 'ttfb', 'lcp', 'cls', 'inp'].forEach(report);
    }
    document.addEventListener('visibilitychange', function () {
      if (document.visibilityState === 'hidden') flushAll();
    });
    window.addEventListener('pagehide', flushAll);
  }

  window.webstats = { event: event, pageview: pageview, setOptout: function (v) { try { localStorage.setItem(OPTOUT, v ? '1' : '0'); } catch (e) {} } };
})();
