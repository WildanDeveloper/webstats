











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
      return localStorage.getItem(OPTOUT) === '1';
    } catch (e) {
      return false;
    }
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
  }

  function onRoute() {
    var p = location.pathname + location.search;
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
  }

  window.webstats = { event: event, pageview: pageview, setOptout: function (v) { try { localStorage.setItem(OPTOUT, v ? '1' : '0'); } catch (e) {} } };
})();
