// Relays /api requests the page's fetch patch never sees (<img src> covers,
// asset links) to the page that runs the wasm server. Served from the site root
// so its scope covers the whole app.
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()));

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);
  if (url.origin !== location.origin || !url.pathname.startsWith('/api/')) return;
  event.respondWith(relay(event));
});

async function relay(event) {
  const req = event.request;
  // Any window of this origin can answer: each one forwards to the serving tab.
  // The requesting page comes first, the others cover a clientId that does not resolve.
  const client = (event.clientId && await self.clients.get(event.clientId)) ||
    (await self.clients.matchAll({ type: 'window' }))[0];
  // A navigation with no page open has no server behind it; let the static host answer.
  if (!client) return fetch(req);

  const body = ['GET', 'HEAD'].includes(req.method) ? undefined : await req.arrayBuffer();
  const { port1, port2 } = new MessageChannel();
  const reply = new Promise((resolve) => { port1.onmessage = (e) => resolve(e.data); });
  const url = new URL(req.url);
  client.postMessage({
    type: 'plainshelf-fetch',
    method: req.method,
    url: url.pathname + url.search,
    headers: Object.fromEntries(req.headers.entries()),
    body,
  }, [port2]);
  const res = await reply;
  return new Response(res.body, { status: res.status, headers: res.headers });
}
