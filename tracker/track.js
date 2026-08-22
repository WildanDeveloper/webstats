











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

  function getSessionId() {
    try {
      var id = localStorage.getItem(STORAGE);
      if (id) return id;
      id = randomId();
      localStorage.setItem(STORAGE, id);
      return id;
    } catch (e) {
      return randomId();
    }
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

  function send(payload) {
    if (optedOut()) return;
    try {
      var blob = new Blob([JSON.stringify(payload)], { type: 'application/json' });
      if (navigator.sendBeacon) {
        navigator.sendBeacon(collect, blob);
        return;
      }
      fetch(collect, { method: 'POST', body: blob, keepalive: true });
    } catch (e) {
      
      try {
        var q = JSON.parse(localStorage.getItem(QUEUE) || '[]');
        q.push(payload);
        localStorage.setItem(QUEUE, JSON.stringify(q.slice(-20)));
      } catch (e2) {}
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
      path: location.pathname + location.search,
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

  function onRoute() {
    var p = routeKey();
    if (p === lastPath) return;
    lastPath = p;
    pageview();
  }

  function event(name, props) {
    if (!name || optedOut()) return;
    try {
      var body = {
        kind: 'event',
        site_id: SITE,
        session_id: getSessionId(),
        event_name: name,
        props: props || {},
        url: location.pathname + location.search,
        ua: navigator.userAgent,
        ts: Date.now()
      };
      fetch(eventUrl, {
        method: 'POST',
        body: JSON.stringify(body),
        keepalive: true
      });
    } catch (e) {}
  }

  if (auto) {
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
    if (!wantOutbound && !wantDownload && !wantScroll) return;

    if (wantOutbound || wantDownload) {
      var DL_RE = /\.(pdf|zip|rar|7z|tar|gz|tgz|docx?|xlsx?|pptx?|csv|mp[34]|m4a|wav|ogg|webm|avi|mov|dmg|exe|apk)$/i;
      document.addEventListener('click', function (e) {
        var t = e.target;
        var a = t && t.closest ? t.closest('a[href]') : null;
        if (!a) return;
        var href = a.getAttribute('href') || '';
        if (href.indexOf('http') !== 0) return;
        var clean = href.split('?')[0].split('#')[0];
        if (wantDownload && DL_RE.test(clean)) { event('download', { url: href }); return; }
        if (wantOutbound && a.hostname && a.hostname !== location.hostname) event('outbound', { url: href });
      }, true);
    }

    if (wantScroll) {
      var seen = {};
      window.addEventListener('scroll', function () {
        var d = document.documentElement;
        var max = d.scrollHeight - window.innerHeight;
        var pct = max <= 0 ? 100 : Math.round(((window.scrollY || d.scrollTop) / max) * 100);
        [25, 50, 75, 100].forEach(function (t) {
          if (pct >= t && !seen[t]) { seen[t] = 1; event('scroll', { depth: t }); }
        });
      }, { passive: true });
    }
  }

  window.webstats = { event: event, pageview: pageview, setOptout: function (v) { try { localStorage.setItem(OPTOUT, v ? '1' : '0'); } catch (e) {} } };
})();
