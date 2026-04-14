// stubs.js — minimal browser-like surface for executing AWS WAF challenge.js
// inside a sobek (ES2022) runtime. Host-side bridges (__goFetch,
// __goCryptoRandom, __goDigest, __goSetTimeout, __goClearTimeout,
// __goNow, __goLog) are installed by stubs.go before this file runs.
(function () {
  "use strict";
  var g = globalThis;
  var startTime = (g.__goNow && g.__goNow()) || Date.now();

  // ----- window / self / top / parent / frames -----
  var win = g;
  g.window = win;
  g.self = win;
  g.top = win;
  g.parent = win;
  g.frames = win;
  g.closed = false;
  g.length = 0;
  g.name = "";
  g.origin = g.__goOrigin || "https://example.com";
  g.location = {
    href: g.origin + "/",
    protocol: "https:",
    host: (g.origin || "https://example.com").replace(/^https?:\/\//, ""),
    hostname: (g.origin || "https://example.com").replace(/^https?:\/\//, ""),
    pathname: "/",
    search: "",
    hash: "",
    origin: g.origin,
    port: "",
    assign: function () {},
    replace: function () {},
    reload: function () {},
    toString: function () { return this.href; }
  };
  g.history = {
    length: 1, state: null, scrollRestoration: "auto",
    back: function () {}, forward: function () {}, go: function () {},
    pushState: function () {}, replaceState: function () {}
  };

  // ----- navigator -----
  g.navigator = {
    userAgent: g.__goUserAgent || "Mozilla/5.0",
    appName: "Netscape",
    appVersion: "5.0",
    platform: "MacIntel",
    vendor: "Google Inc.",
    product: "Gecko",
    productSub: "20030107",
    language: "en-US",
    languages: ["en-US", "en"],
    hardwareConcurrency: 8,
    deviceMemory: 8,
    maxTouchPoints: 0,
    webdriver: false,
    onLine: true,
    cookieEnabled: true,
    doNotTrack: null,
    plugins: { length: 0, item: function () { return null; }, namedItem: function () { return null; } },
    mimeTypes: { length: 0, item: function () { return null; }, namedItem: function () { return null; } },
    connection: { effectiveType: "4g", rtt: 50, downlink: 10, saveData: false, type: "wifi" },
    permissions: { query: function () { return Promise.resolve({ state: "granted" }); } },
    sendBeacon: function () { return true; }
  };

  // ----- screen -----
  g.screen = { width: 1920, height: 1080, availWidth: 1920, availHeight: 1040, colorDepth: 24, pixelDepth: 24, orientation: { type: "landscape-primary", angle: 0 } };
  g.innerWidth = 1920; g.innerHeight = 1040;
  g.outerWidth = 1920; g.outerHeight = 1080;
  g.devicePixelRatio = 2; g.scrollX = 0; g.scrollY = 0;

  // ----- performance -----
  g.performance = {
    now: function () { return (g.__goNow ? g.__goNow() : Date.now()) - startTime; },
    timeOrigin: startTime,
    timing: {}, navigation: { type: 0, redirectCount: 0 },
    getEntries: function () { return []; },
    getEntriesByName: function () { return []; },
    getEntriesByType: function () { return []; },
    mark: function () {}, measure: function () {},
    clearMarks: function () {}, clearMeasures: function () {}, clearResourceTimings: function () {}
  };

  // ----- document -----
  function makeElement(tag) {
    var el = {
      nodeType: 1, tagName: (tag || "div").toUpperCase(), nodeName: (tag || "div").toUpperCase(),
      children: [], childNodes: [], attributes: {}, style: {}, dataset: {},
      innerHTML: "", outerHTML: "", textContent: "", className: "", id: "",
      parentNode: null, firstChild: null, lastChild: null, nextSibling: null, previousSibling: null,
      ownerDocument: null,
      getAttribute: function (k) { return this.attributes[k] == null ? null : this.attributes[k]; },
      setAttribute: function (k, v) { this.attributes[k] = String(v); },
      removeAttribute: function (k) { delete this.attributes[k]; },
      hasAttribute: function (k) { return k in this.attributes; },
      appendChild: function (n) { this.children.push(n); this.childNodes.push(n); n.parentNode = this; return n; },
      removeChild: function (n) { var i = this.childNodes.indexOf(n); if (i >= 0) this.childNodes.splice(i, 1); i = this.children.indexOf(n); if (i >= 0) this.children.splice(i, 1); return n; },
      insertBefore: function (n) { this.appendChild(n); return n; },
      cloneNode: function () { return makeElement(tag); },
      addEventListener: function () {}, removeEventListener: function () {}, dispatchEvent: function () { return true; },
      querySelector: function () { return null; }, querySelectorAll: function () { return []; },
      getElementsByTagName: function () { return []; }, getElementsByClassName: function () { return []; },
      getBoundingClientRect: function () { return { x: 0, y: 0, top: 0, left: 0, right: 0, bottom: 0, width: 0, height: 0 }; },
      focus: function () {}, blur: function () {}, click: function () {},
      contains: function () { return false; }
    };
    // ----- canvas getContext("2d") stub for fingerprinting -----
    if ((tag || "").toLowerCase() === "canvas") {
      el.width = 300; el.height = 150;
      el.getContext = function(type) {
        return {
          fillStyle: "", strokeStyle: "", font: "10px sans-serif",
          globalAlpha: 1, globalCompositeOperation: "source-over",
          fillRect: function(){}, strokeRect: function(){}, clearRect: function(){},
          fillText: function(){}, strokeText: function(){}, measureText: function(t){ return {width: t.length * 6}; },
          beginPath: function(){}, closePath: function(){}, moveTo: function(){}, lineTo: function(){},
          arc: function(){}, arcTo: function(){}, rect: function(){}, ellipse: function(){},
          fill: function(){}, stroke: function(){}, clip: function(){},
          drawImage: function(){}, createImageData: function(w,h){ return {data: new Uint8ClampedArray(w*h*4), width:w, height:h}; },
          getImageData: function(x,y,w,h){ return {data: new Uint8ClampedArray(w*h*4), width:w, height:h}; },
          putImageData: function(){},
          createLinearGradient: function(){ return {addColorStop: function(){}}; },
          createRadialGradient: function(){ return {addColorStop: function(){}}; },
          createPattern: function(){ return {}; },
          save: function(){}, restore: function(){},
          translate: function(){}, rotate: function(){}, scale: function(){},
          transform: function(){}, setTransform: function(){}, resetTransform: function(){},
          canvas: el
        };
      };
      el.toDataURL = function(){ return "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVQI12NgAAIABQABNjN9GQA="; };
      el.toBlob = function(cb){ if(cb) cb(new Blob([""])); };
    }
    return el;
  }
  var docCookieJar = Object.create(null);
  var docHead = makeElement("head"), docBody = makeElement("body"), docHTML = makeElement("html");
  docHTML.appendChild(docHead); docHTML.appendChild(docBody);
  g.document = {
    nodeType: 9, readyState: "complete", visibilityState: "visible", hidden: false,
    URL: g.location.href, documentURI: g.location.href, baseURI: g.location.href,
    title: "", referrer: "", domain: g.location.hostname, characterSet: "UTF-8", contentType: "text/html",
    documentElement: docHTML, head: docHead, body: docBody, defaultView: g, activeElement: null,
    createElement: function (t) { return makeElement(t); },
    createElementNS: function (_ns, t) { return makeElement(t); },
    createTextNode: function (d) { return { nodeType: 3, nodeValue: String(d), textContent: String(d) }; },
    createDocumentFragment: function () { return makeElement("#fragment"); },
    createEvent: function () { return { initEvent: function () {}, preventDefault: function () {}, stopPropagation: function () {} }; },
    getElementById: function () { return null; }, getElementsByTagName: function () { return []; },
    getElementsByName: function () { return []; }, getElementsByClassName: function () { return []; },
    querySelector: function () { return null; }, querySelectorAll: function () { return []; },
    addEventListener: function () {}, removeEventListener: function () {}, dispatchEvent: function () { return true; },
    write: function () {}, writeln: function () {}, open: function () {}, close: function () {},
    hasFocus: function () { return true; },
    get cookie() {
      var out = [];
      for (var k in docCookieJar) out.push(k + "=" + docCookieJar[k]);
      return out.join("; ");
    },
    set cookie(v) {
      var s = String(v), semi = s.indexOf(";"), pair = semi >= 0 ? s.substring(0, semi) : s;
      var eq = pair.indexOf("="); if (eq < 0) return;
      var name = pair.substring(0, eq).trim(), val = pair.substring(eq + 1).trim();
      if (!name) return;
      if (/expires=/i.test(s) && /1970|Thu, 01 Jan 1970/.test(s)) { delete docCookieJar[name]; return; }
      docCookieJar[name] = val;
      if (g.__goSetCookie) g.__goSetCookie(name, val, s);
    }
  };
  g.__docCookieJar = docCookieJar;

  // ----- storage -----
  function makeStorage() {
    var m = Object.create(null);
    return {
      get length() { return Object.keys(m).length; },
      key: function (i) { return Object.keys(m)[i] || null; },
      getItem: function (k) { return k in m ? m[k] : null; },
      setItem: function (k, v) { m[k] = String(v); },
      removeItem: function (k) { delete m[k]; },
      clear: function () { for (var k in m) delete m[k]; }
    };
  }
  g.localStorage = makeStorage(); g.sessionStorage = makeStorage();

  // ----- crypto -----
  g.crypto = {
    getRandomValues: function (buf) { g.__goCryptoRandom(buf); return buf; },
    randomUUID: function () {
      var b = new Uint8Array(16); g.__goCryptoRandom(b);
      b[6] = (b[6] & 0x0f) | 0x40; b[8] = (b[8] & 0x3f) | 0x80;
      var h = []; for (var i = 0; i < 16; i++) h.push((b[i] + 0x100).toString(16).substring(1));
      return h[0]+h[1]+h[2]+h[3]+"-"+h[4]+h[5]+"-"+h[6]+h[7]+"-"+h[8]+h[9]+"-"+h[10]+h[11]+h[12]+h[13]+h[14]+h[15];
    },
    subtle: {
      digest: function (alg, data) {
        return new Promise(function (resolve, reject) {
          try {
            var name = typeof alg === "string" ? alg : (alg && alg.name) || "";
            var out = g.__goDigest(String(name).toUpperCase(), data);
            resolve(out.buffer ? out.buffer : out);
          } catch (e) { reject(e); }
        });
      },
      encrypt: function () { return Promise.reject(new Error("subtle.encrypt not available")); },
      decrypt: function () { return Promise.reject(new Error("subtle.decrypt not available")); },
      sign: function () { return Promise.reject(new Error("subtle.sign not available")); },
      verify: function () { return Promise.reject(new Error("subtle.verify not available")); },
      deriveBits: function () { return Promise.reject(new Error("subtle.deriveBits not available")); },
      deriveKey: function () { return Promise.reject(new Error("subtle.deriveKey not available")); },
      importKey: function () { return Promise.reject(new Error("subtle.importKey not available")); },
      exportKey: function () { return Promise.reject(new Error("subtle.exportKey not available")); },
      generateKey: function () { return Promise.reject(new Error("subtle.generateKey not available")); },
      wrapKey: function () { return Promise.reject(new Error("subtle.wrapKey not available")); },
      unwrapKey: function () { return Promise.reject(new Error("subtle.unwrapKey not available")); }
    }
  };

  // ----- timers -----
  g.setTimeout = function (cb, ms) { return g.__goSetTimeout(cb, +ms || 0, false); };
  g.setInterval = function (cb, ms) { return g.__goSetTimeout(cb, +ms || 0, true); };
  g.clearTimeout = function (id) { g.__goClearTimeout(id); };
  g.clearInterval = function (id) { g.__goClearTimeout(id); };
  g.requestAnimationFrame = function (cb) { return g.__goSetTimeout(function () { cb(g.performance.now()); }, 16, false); };
  g.cancelAnimationFrame = function (id) { g.__goClearTimeout(id); };
  g.queueMicrotask = function (cb) { Promise.resolve().then(cb); };

  // ----- events -----
  function EventCtor(type, init) { this.type = String(type); this.bubbles = !!(init && init.bubbles); this.cancelable = !!(init && init.cancelable); this.defaultPrevented = false; this.timeStamp = g.performance.now(); }
  EventCtor.prototype.preventDefault = function () { this.defaultPrevented = true; };
  EventCtor.prototype.stopPropagation = function () {};
  EventCtor.prototype.stopImmediatePropagation = function () {};
  g.Event = EventCtor;
  g.CustomEvent = function (type, init) { EventCtor.call(this, type, init); this.detail = init ? init.detail : null; };
  g.CustomEvent.prototype = Object.create(EventCtor.prototype);
  g.MessageEvent = function (type, init) { EventCtor.call(this, type, init); this.data = init ? init.data : null; };
  g.MessageEvent.prototype = Object.create(EventCtor.prototype);

  // ----- observers (silent no-ops) -----
  function NoopObserver() {}
  NoopObserver.prototype.observe = function () {}; NoopObserver.prototype.disconnect = function () {};
  NoopObserver.prototype.unobserve = function () {}; NoopObserver.prototype.takeRecords = function () { return []; };
  g.MutationObserver = NoopObserver; g.IntersectionObserver = NoopObserver;
  g.ResizeObserver = NoopObserver; g.PerformanceObserver = NoopObserver;

  // ----- not-available stubs -----
  function notAvailable(name) { return function () { throw new Error(name + " is not available"); }; }
  g.WebSocket = notAvailable("WebSocket");
  g.Worker = notAvailable("Worker");
  g.SharedWorker = notAvailable("SharedWorker");
  g.ServiceWorker = notAvailable("ServiceWorker");
  g.navigator.serviceWorker = { register: function () { return Promise.reject(new Error("serviceWorker unavailable")); }, ready: new Promise(function () {}) };

  // ----- window methods -----
  g.alert = function () {}; g.confirm = function () { return true; }; g.prompt = function () { return null; };
  g.addEventListener = function () {}; g.removeEventListener = function () {}; g.dispatchEvent = function () { return true; };
  g.getComputedStyle = function () { return { getPropertyValue: function () { return ""; } }; };
  g.matchMedia = function () { return { matches: false, addListener: function () {}, removeListener: function () {}, addEventListener: function () {}, removeEventListener: function () {} }; };
  g.scroll = function () {}; g.scrollTo = function () {}; g.scrollBy = function () {};
  g.postMessage = function () {}; g.open = function () { return null; }; g.close = function () {}; g.print = function () {};
  g.atob = g.atob || function (s) {
    var chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=", out = "", i = 0;
    s = String(s).replace(/[^A-Za-z0-9+/=]/g, "");
    while (i < s.length) {
      var e1 = chars.indexOf(s.charAt(i++)), e2 = chars.indexOf(s.charAt(i++));
      var e3 = chars.indexOf(s.charAt(i++)), e4 = chars.indexOf(s.charAt(i++));
      var c1 = (e1 << 2) | (e2 >> 4), c2 = ((e2 & 15) << 4) | (e3 >> 2), c3 = ((e3 & 3) << 6) | e4;
      out += String.fromCharCode(c1); if (e3 !== 64) out += String.fromCharCode(c2); if (e4 !== 64) out += String.fromCharCode(c3);
    }
    return out;
  };
  g.btoa = g.btoa || function (s) {
    var chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=", out = "", i = 0;
    s = String(s);
    while (i < s.length) {
      var c1 = s.charCodeAt(i++), c2 = s.charCodeAt(i++), c3 = s.charCodeAt(i++);
      var e1 = c1 >> 2, e2 = ((c1 & 3) << 4) | (c2 >> 4);
      var e3 = isNaN(c2) ? 64 : (((c2 & 15) << 2) | (c3 >> 6));
      var e4 = isNaN(c3) ? 64 : (c3 & 63);
      out += chars.charAt(e1) + chars.charAt(e2) + chars.charAt(e3) + chars.charAt(e4);
    }
    return out;
  };

  // ----- console -----
  g.console = {
    log: g.__goLog || function(){},
    warn: g.__goLog || function(){},
    error: g.__goLog || function(){},
    info: g.__goLog || function(){},
    debug: g.__goLog || function(){},
    trace: function(){}, dir: function(){}, table: function(){},
    assert: function(){}, time: function(){}, timeEnd: function(){},
    group: function(){}, groupEnd: function(){}, groupCollapsed: function(){}
  };

  // ----- TextEncoder / TextDecoder -----
  g.TextEncoder = function(){};
  g.TextEncoder.prototype.encode = function(s) {
    var a = new Uint8Array(s.length);
    for (var i = 0; i < s.length; i++) a[i] = s.charCodeAt(i) & 0xff;
    return a;
  };
  g.TextDecoder = function(enc){ this.encoding = enc || "utf-8"; };
  g.TextDecoder.prototype.decode = function(buf) {
    var a = new Uint8Array(buf.buffer || buf);
    var s = "";
    for (var i = 0; i < a.length; i++) s += String.fromCharCode(a[i]);
    return s;
  };

  // ----- AbortController / AbortSignal -----
  g.AbortController = function() {
    var ctrl = this;
    ctrl.signal = { aborted: false, reason: undefined, addEventListener: function(){}, removeEventListener: function(){} };
    ctrl.abort = function(reason) { ctrl.signal.aborted = true; ctrl.signal.reason = reason; };
  };

  // ----- URL / URLSearchParams -----
  if (typeof g.URL === "undefined") {
    g.URL = function(url, base) {
      if (base && url.startsWith("/")) url = base.replace(/\/[^/]*$/, "") + url;
      var m = url.match(/^(https?:)\/\/([^/:]+)(:\d+)?(\/[^?#]*)?(\?[^#]*)?(#.*)?$/);
      this.protocol = m ? m[1] : ""; this.hostname = m ? m[2] : "";
      this.port = m ? (m[3] || "").replace(":", "") : "";
      this.pathname = m ? (m[4] || "/") : "/";
      this.search = m ? (m[5] || "") : ""; this.hash = m ? (m[6] || "") : "";
      this.host = this.hostname + (this.port ? ":" + this.port : "");
      this.origin = this.protocol + "//" + this.host;
      this.href = url;
      this.searchParams = new g.URLSearchParams(this.search.replace(/^\?/, ""));
      this.toString = function(){ return this.href; };
    };
  }
  if (typeof g.URLSearchParams === "undefined") {
    g.URLSearchParams = function(init) {
      this._entries = [];
      if (typeof init === "string") {
        var pairs = init.split("&");
        for (var i = 0; i < pairs.length; i++) {
          var kv = pairs[i].split("=");
          if (kv[0]) this._entries.push([decodeURIComponent(kv[0]), decodeURIComponent(kv[1] || "")]);
        }
      }
    };
    g.URLSearchParams.prototype.get = function(k) { for (var i=0;i<this._entries.length;i++) if(this._entries[i][0]===k) return this._entries[i][1]; return null; };
    g.URLSearchParams.prototype.set = function(k,v) { for (var i=0;i<this._entries.length;i++) if(this._entries[i][0]===k){this._entries[i][1]=v;return;} this._entries.push([k,v]); };
    g.URLSearchParams.prototype.append = function(k,v) { this._entries.push([k,v]); };
    g.URLSearchParams.prototype.has = function(k) { for (var i=0;i<this._entries.length;i++) if(this._entries[i][0]===k) return true; return false; };
    g.URLSearchParams.prototype.delete = function(k) { this._entries = this._entries.filter(function(e){return e[0]!==k;}); };
    g.URLSearchParams.prototype.toString = function() { return this._entries.map(function(e){return encodeURIComponent(e[0])+"="+encodeURIComponent(e[1]);}).join("&"); };
  }

  // ----- fetch / XHR bridge -----
  function buildResponse(raw) {
    var bodyBytes = raw.body || new Uint8Array(0);
    var headers = raw.headers || {};
    var hObj = {
      get: function (k) { k = String(k).toLowerCase(); for (var hk in headers) if (hk.toLowerCase() === k) return headers[hk]; return null; },
      has: function (k) { return this.get(k) !== null; },
      forEach: function (fn) { for (var hk in headers) fn(headers[hk], hk); }
    };
    var used = false;
    function consume() {
      if (used) return Promise.reject(new TypeError("body stream already read"));
      used = true; return Promise.resolve(bodyBytes);
    }
    return {
      ok: raw.status >= 200 && raw.status < 300,
      status: raw.status, statusText: raw.statusText || "", url: raw.url || "",
      redirected: !!raw.redirected, type: "basic", headers: hObj,
      clone: function () { return buildResponse(raw); },
      arrayBuffer: function () { return consume().then(function (b) { return b.buffer; }); },
      text: function () { return consume().then(function (b) { var s = ""; for (var i = 0; i < b.length; i++) s += String.fromCharCode(b[i]); return s; }); },
      json: function () { return this.text().then(function (t) { return JSON.parse(t); }); },
      blob: function () { return consume(); },
      bytes: function () { return consume(); }
    };
  }
  g.__buildResponse = buildResponse;
  g.fetch = function (input, init) {
    return new Promise(function (resolve, reject) {
      try {
        var url = typeof input === "string" ? input : (input && input.url) || String(input);
        g.__goFetch(url, init || {}, function (err, raw) {
          if (err) reject(new TypeError(String(err))); else resolve(buildResponse(raw));
        });
      } catch (e) { reject(e); }
    });
  };
  g.XMLHttpRequest = function () {
    var xhr = this;
    xhr.readyState = 0; xhr.status = 0; xhr.responseText = ""; xhr.response = null; xhr.responseType = "";
    xhr._headers = {}; xhr._method = "GET"; xhr._url = ""; xhr._async = true;
    xhr.open = function (m, u, a) { xhr._method = m; xhr._url = u; xhr._async = a !== false; xhr.readyState = 1; if (xhr.onreadystatechange) xhr.onreadystatechange(); };
    xhr.setRequestHeader = function (k, v) { xhr._headers[k] = v; };
    xhr.getResponseHeader = function () { return null; };
    xhr.getAllResponseHeaders = function () { return ""; };
    xhr.abort = function () {};
    xhr.send = function (body) {
      g.__goFetch(xhr._url, { method: xhr._method, headers: xhr._headers, body: body }, function (err, raw) {
        if (err) { xhr.readyState = 4; xhr.status = 0; if (xhr.onerror) xhr.onerror(); if (xhr.onreadystatechange) xhr.onreadystatechange(); return; }
        xhr.readyState = 4; xhr.status = raw.status;
        var b = raw.body || new Uint8Array(0), s = "";
        for (var i = 0; i < b.length; i++) s += String.fromCharCode(b[i]);
        xhr.responseText = s; xhr.response = xhr.responseType === "arraybuffer" ? b.buffer : s;
        if (xhr.onload) xhr.onload(); if (xhr.onreadystatechange) xhr.onreadystatechange();
      });
    };
  };
})();
