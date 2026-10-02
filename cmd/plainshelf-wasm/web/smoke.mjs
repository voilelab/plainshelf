// Usage: node smoke.mjs <site dir from build.sh> [screenshot.png]
// Loads the wasm server in chromium, drives a create/read round trip, then
// opens the real frontend on top of it.
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire(new URL('../../../e2e/package.json', import.meta.url));
const { chromium } = require('playwright');

const dir = path.resolve(process.argv[2] ?? '.');
const types = {
  '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.wasm': 'application/wasm',
  '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.woff2': 'font/woff2',
};
const srv = http.createServer((req, res) => {
  let file = path.join(dir, decodeURIComponent(new URL(req.url, 'http://x').pathname));
  if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) file = path.join(dir, 'index.html'); // SPA fallback
  res.writeHead(200, { 'content-type': types[path.extname(file)] ?? 'application/octet-stream' });
  fs.createReadStream(file).pipe(res);
});
await new Promise((r) => srv.listen(0, '127.0.0.1', r));

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM ?? '/opt/pw-browsers/chromium' });
const page = await browser.newPage();
page.on('console', (m) => { if (process.env.VERBOSE) console.log('[page]', m.text()); });
page.on('pageerror', (e) => console.log('[pageerror]', e.message));
const t0 = Date.now();
const origin = `http://127.0.0.1:${srv.address().port}`;
await page.goto(`${origin}/wasm/index.html`);
await page.waitForFunction(() => document.getElementById('status').textContent !== 'loading…', null, { timeout: 60000 });
console.log('status:', await page.textContent('#status'), `(${Date.now() - t0} ms)`);

const result = await page.evaluate(async () => {
  const call = async (method, url, body) => {
    const enc = body === undefined ? undefined : new TextEncoder().encode(JSON.stringify(body));
    const headers = enc ? { 'Content-Type': 'application/json' } : {};
    // A read during the initial scan answers 503; retry briefly.
    for (let i = 0; ; i++) {
      const r = await window.plainshelfFetch(method, url, headers, enc);
      if (r.status !== 503 || i > 50) return { status: r.status, body: new TextDecoder().decode(r.body) };
      await new Promise((ok) => setTimeout(ok, 100));
    }
  };
  const out = {};
  out.shelves = await call('GET', '/api/shelves');
  out.create = await call('POST', '/api/shelves/demo/books', { title: '測試書', folder: [] });
  out.books = await call('GET', '/api/shelves/demo/books');
  out.files = (() => {
    const names = [];
    const walk = (p) => fs.readdir(p, (err, list) => {
      if (err) return;
      for (const n of list) { const c = p + '/' + n; names.push(c); walk(c); }
    });
    walk('/plainshelf/shelf');
    return names;
  })();
  return out;
});
for (const [k, v] of Object.entries(result)) console.log(k, JSON.stringify(v).slice(0, 400));

// The real frontend: its fetches to /api are answered by the wasm server.
await page.addInitScript(() => {
  window.plainshelfSeed = async (serve) => {
    for (let i = 0; i < 50; i++) {
      const r = await serve('/api/shelves/demo/books', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: '在瀏覽器裡建立的書', folder: [] }),
      });
      if (r.status !== 503) return;
      await new Promise((ok) => setTimeout(ok, 100));
    }
  };
});
await page.goto(`${origin}/`);
await page.getByText("Total Books").first().waitFor({ timeout: 30000 });
await page.waitForTimeout(500);
console.log('app title:', await page.title());
console.log('app text:', (await page.innerText('body')).replace(/\s+/g, ' ').slice(0, 300));
if (process.argv[3]) await page.screenshot({ path: process.argv[3] });
await browser.close();
srv.close();
