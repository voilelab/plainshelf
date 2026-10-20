self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()));

self.addEventListener('message', (event) => {
  if (event.data?.type === 'plainshelf-claim') event.waitUntil(self.clients.claim());
});

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);
  if (url.origin !== location.origin || !url.pathname.startsWith('/api/')) return;
  event.respondWith(relay(event));
});

async function relay(event) {
  const req = event.request;

  const client = event.clientId && await self.clients.get(event.clientId);
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
