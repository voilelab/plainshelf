// Answers the page's /api and /health requests, plus those sw.js relays from
// elements that bypass fetch, with plainshelf.wasm. One tab per origin runs the
// server, chosen by a Web Lock; the others forward their requests to it over a
// BroadcastChannel, so tabs share one shelf instead of writing over each other.
// When the serving tab closes, a waiting tab takes the lock and restores from OPFS.
// Load after memfs.js, opfs.js and wasm_exec.js, before the app bundle. An optional
// window.plainshelfSeed(serve) runs in the serving tab before it answers anyone.
(() => {
  const base = document.currentScript.src.replace(/[^/]*$/, '');
  const RESEND_MS = 1000;
  const channel = new BroadcastChannel('plainshelf-demo');
  const tabId = crypto.randomUUID();

  // Go calls plainshelfReady once it has registered plainshelfFetch.
  const started = new Promise((resolve) => { window.plainshelfReady = resolve; });
  // Writes changes to storage now instead of after the debounce.
  window.plainshelfFlush = () => window.plainshelfStorage?.flush() ?? Promise.resolve();

  // A request is { method, url, headers, body? } and a response { status, headers, body }.
  // /_demo/storage reports whether the serving tab persists, which only it knows.
  const callLocal = (r) => (r.url === '/_demo/storage'
    ? Promise.resolve({
      status: 200,
      headers: { 'Content-Type': 'application/json' },
      body: new TextEncoder().encode(JSON.stringify({ persistent: window.plainshelfStorage?.persistent === true })),
    })
    : window.plainshelfFetch(r.method, r.url, r.headers, r.body));

  let serving = null; // set when this tab takes the server role; resolves once it answers
  const pending = new Map(); // id -> { r, resolve, acked, timer }, requests sent to the serving tab
  let nextId = 0;

  function call(r) {
    if (serving) return serving.then(() => callLocal(r));
    return new Promise((resolve) => {
      const id = `${tabId}:${nextId++}`;
      const entry = { r, resolve, acked: false };
      pending.set(id, entry);
      send(id, entry);
    });
  }

  // Resent until a serving tab acknowledges it, which also covers the startup race.
  function send(id, entry) {
    clearTimeout(entry.timer);
    channel.postMessage({ type: 'request', id, r: entry.r });
    entry.timer = setTimeout(() => {
      if (!entry.acked && pending.has(id) && !serving) send(id, entry);
    }, RESEND_MS);
  }

  channel.onmessage = async ({ data }) => {
    if (data.type === 'request') {
      if (!serving) return;
      channel.postMessage({ type: 'ack', id: data.id });
      let res;
      try {
        await serving;
        res = await callLocal(data.r);
      } catch (err) {
        console.error('plainshelf: forwarded request failed', data.r.url, err);
        res = { status: 500, headers: {}, body: new Uint8Array() };
      }
      channel.postMessage({ type: 'response', id: data.id, res });
    } else if (data.type === 'ack') {
      const entry = pending.get(data.id);
      if (entry) entry.acked = true;
    } else if (data.type === 'response') {
      const entry = pending.get(data.id);
      if (!entry) return;
      pending.delete(data.id);
      clearTimeout(entry.timer);
      entry.resolve(data.res);
    } else if (data.type === 'reload') {
      reloadSelf();
    } else if (data.type === 'server-down') {
      for (const resolve of serverDownWaiters.splice(0)) resolve();
    } else if (data.type === 'server-up' && !serving) {
      // A new serving tab: whatever the last one acknowledged but never answered goes again.
      for (const [id, entry] of pending) send(id, entry);
    }
  };

  async function runServer() {
    const go = new Go();
    const wasm = fetch(base + 'plainshelf.wasm');
    // Restore the persisted shelf before Go reads it; on failure run in memory.
    await window.plainshelfStorage?.load().catch((err) => console.error('plainshelf: restore failed', err));
    const { instance } = await WebAssembly.instantiate(await (await wasm).arrayBuffer(), go.importObject);
    go.run(instance);
    await started;
    await window.plainshelfSeed?.(asFetch(callLocal));
  }

  function takeServerRole() {
    serving = runServer();
    serving.then(() => {
      channel.postMessage({ type: 'server-up' });
      // Requests this tab forwarded before it took over are now its own.
      for (const [id, entry] of pending) {
        pending.delete(id);
        clearTimeout(entry.timer);
        callLocal(entry.r).then(entry.resolve);
      }
    }, (err) => console.error('plainshelf wasm failed to start', err));
    return serving;
  }

  if (navigator.locks) {
    // Held until the tab goes away; a failed start releases it for the next tab.
    navigator.locks.request('plainshelf-demo-server', () => takeServerRole().then(() => new Promise(() => {})));
  } else {
    takeServerRole();
  }

  async function toRequest(input, init) {
    const req = new Request(input, init);
    const url = new URL(req.url);
    const body = ['GET', 'HEAD'].includes(req.method) ? undefined : new Uint8Array(await req.arrayBuffer());
    return { method: req.method, url: url.pathname + url.search, headers: Object.fromEntries(req.headers.entries()), body };
  }
  const hasNullBody = (res, method) => [101, 204, 205, 304].includes(res.status) || method === 'HEAD';
  const asFetch = (fn) => async (input, init) => {
    const r = await toRequest(input, init);
    const res = await fn(r);
    return new Response(hasNullBody(res, r.method) ? null : res.body, { status: res.status, headers: res.headers });
  };

  // For pages and tests that talk to the server without going through fetch.
  window.plainshelfRequest = call;

  // The serving tab writes its changes out, then says it is going; the others
  // wait for that, or they would reload into the old server and its stale view.
  const serverDownWaiters = [];
  async function reloadSelf() {
    if (serving) {
      await window.plainshelfFlush().catch(() => {});
      channel.postMessage({ type: 'server-down' });
    } else {
      await new Promise((resolve) => {
        serverDownWaiters.push(resolve);
        setTimeout(resolve, 5000);
      });
    }
    location.reload();
  }
  // Reloads every tab of the demo, after a change the running server should not
  // reconcile in memory (an imported shelf).
  window.plainshelfReloadAll = () => {
    channel.postMessage({ type: 'reload' });
    return reloadSelf();
  };

  const nativeFetch = window.fetch.bind(window);
  const relayReady = registerRelay();
  const fetchViaServer = asFetch(call);
  window.fetch = async (input, init) => {
    const url = new URL(new Request(input, init).url);
    if (url.origin !== location.origin || !/^\/(api\/|health$)/.test(url.pathname)) {
      return nativeFetch(input, init);
    }
    await relayReady; // so the covers of the first render reach the relay too
    return fetchViaServer(input, init);
  };

  // Covers and assets load through <img src>, not fetch; sw.js relays those requests here.
  async function registerRelay() {
    const sw = navigator.serviceWorker;
    if (!sw) return;
    sw.addEventListener('message', async (e) => {
      if (e.data?.type !== 'plainshelf-fetch') return;
      const { method, url, headers, body } = e.data;
      let reply;
      try {
        const res = await call({ method, url, headers, body: body ? new Uint8Array(body) : undefined });
        reply = { status: res.status, headers: res.headers, body: hasNullBody(res, method) ? null : res.body };
      } catch (err) {
        reply = { status: 500, headers: {}, body: null };
        console.error('plainshelf: relayed request failed', url, err);
      }
      e.ports[0].postMessage(reply);
    });
    sw.startMessages();
    try {
      await sw.register(new URL('../plainshelf-sw.js', base));
    } catch (err) {
      console.warn('plainshelf: no service worker; covers will not load', err);
      return;
    }
    if (sw.controller) return;
    // sw.js claims open pages when it activates. A page loaded past an already
    // active worker, as a hard reload is, has to ask; either way give it a
    // moment before the app renders.
    const taken = new Promise((resolve) => {
      sw.addEventListener('controllerchange', resolve, { once: true });
      setTimeout(resolve, 3000);
    });
    (await sw.ready).active?.postMessage({ type: 'plainshelf-claim' });
    await taken;
  }
})();
