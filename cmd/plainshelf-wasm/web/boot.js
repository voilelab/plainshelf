// Starts plainshelf.wasm and routes the page's /api and /health fetches into it.
// Load after memfs.js, opfs.js and wasm_exec.js, before the app bundle. An optional
// window.plainshelfSeed(serve) runs once the server is up, before any app request.
(() => {
  const base = document.currentScript.src.replace(/[^/]*$/, '');
  window.plainshelfStarted = new Promise((resolve) => { window.plainshelfReady = resolve; });

  (async () => {
    const go = new Go();
    const wasm = fetch(base + 'plainshelf.wasm');
    // Restore the persisted shelf before Go reads it; on failure run in memory.
    await window.plainshelfStorage?.load().catch((err) => console.error('plainshelf: restore failed', err));
    const bytes = await (await wasm).arrayBuffer();
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance);
  })().catch((err) => console.error('plainshelf wasm failed to start', err));

  // Writes changes to storage now instead of after the debounce.
  window.plainshelfFlush = () => window.plainshelfStorage?.flush() ?? Promise.resolve();

  const nativeFetch = window.fetch.bind(window);
  let seeded;
  const whenSeeded = () => (seeded ??= window.plainshelfStarted.then(() => window.plainshelfSeed?.(serve)));

  async function serve(input, init) {
    const req = new Request(input, init);
    const url = new URL(req.url);
    const body = ['GET', 'HEAD'].includes(req.method) ? undefined : new Uint8Array(await req.arrayBuffer());
    const headers = Object.fromEntries(req.headers.entries());
    const res = await window.plainshelfFetch(req.method, url.pathname + url.search, headers, body);
    const nullBody = [101, 204, 205, 304].includes(res.status) || req.method === 'HEAD';
    return new Response(nullBody ? null : res.body, { status: res.status, headers: res.headers });
  }

  window.fetch = async (input, init) => {
    const url = new URL(new Request(input, init).url);
    if (url.origin !== location.origin || !/^\/(api\/|health$)/.test(url.pathname)) {
      return nativeFetch(input, init);
    }
    await whenSeeded();
    return serve(input, init);
  };
})();
