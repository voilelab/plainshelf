(() => {
  const base = document.currentScript.src.replace(/[^/]*$/, '');

  const route = new URLSearchParams(location.search).get('demo-route');
  if (route !== null) history.replaceState(null, '', new URL('..', base).pathname + route);
  const RESEND_MS = 1000;
  const channel = new BroadcastChannel('plainshelf-demo');
  const tabId = crypto.randomUUID();

  const started = new Promise((resolve) => { window.plainshelfReady = resolve; });

  window.plainshelfFlush = () => window.plainshelfStorage?.flush() ?? Promise.resolve();

  const callLocal = (r) => (r.url === '/_demo/storage'
    ? Promise.resolve({
      status: 200,
      headers: { 'Content-Type': 'application/json' },
      body: new TextEncoder().encode(JSON.stringify({ persistent: window.plainshelfStorage?.persistent === true })),
    })
    : window.plainshelfFetch(r.method, r.url, r.headers, r.body));

  let serving = null;
  const pending = new Map();
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

  function send(id, entry) {
    clearTimeout(entry.timer);
    channel.postMessage({ type: 'request', id, r: entry.r });
    entry.timer = setTimeout(() => {
      if (!entry.acked && pending.has(id) && !serving) send(id, entry);
    }, RESEND_MS);
  }

  channel.onmessage = async ({ data }) => {
    if (data.type === 'persist') {
      dispatchEvent(new CustomEvent(window.plainshelfDemo.persistEvent));
      return;
    }
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

      for (const [id, entry] of pending) send(id, entry);
    }
  };

  async function runServer() {
    const go = new Go();
    const wasm = fetch(base + 'plainshelf.wasm');

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

      for (const [id, entry] of pending) {
        pending.delete(id);
        clearTimeout(entry.timer);
        callLocal(entry.r).then(entry.resolve);
      }
    }, (err) => console.error('plainshelf wasm failed to start', err));
    return serving;
  }

  if (navigator.locks) {

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

  window.plainshelfRequest = call;

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

  window.plainshelfReloadAll = () => {
    channel.postMessage({ type: 'reload' });
    return reloadSelf();
  };

  const PERSIST_EVENT = 'plainshelf-demo-persist';
  window.plainshelfDemo = {
    persistEvent: PERSIST_EVENT,

    async hasStorage() {
      const res = await call({ method: 'GET', url: '/_demo/storage', headers: {} });
      return JSON.parse(new TextDecoder().decode(res.body)).persistent === true;
    },
    canPersist: () => typeof navigator.storage?.persist === 'function',
    persisted: async () => (await navigator.storage?.persisted?.()) === true,
    async persist() {
      const granted = (await navigator.storage.persist()) === true;

      channel.postMessage({ type: 'persist' });
      dispatchEvent(new CustomEvent(PERSIST_EVENT));
      return granted;
    },
  };

  const nativeFetch = window.fetch.bind(window);
  const relayReady = registerRelay();
  const fetchViaServer = asFetch(call);
  window.fetch = async (input, init) => {
    const url = new URL(new Request(input, init).url);
    if (url.origin !== location.origin || !/^\/(api\/|health$)/.test(url.pathname)) {
      return nativeFetch(input, init);
    }
    await relayReady;
    return fetchViaServer(input, init);
  };

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

    const taken = new Promise((resolve) => {
      sw.addEventListener('controllerchange', resolve, { once: true });
      setTimeout(resolve, 3000);
    });
    // Firefox can bypass an active worker on hard reload; explicitly request
    // clients.claim() (https://github.com/voilelab/plainshelf/pull/700).
    (await sw.ready).active?.postMessage({ type: 'plainshelf-claim' });
    await taken;
  }
})();
