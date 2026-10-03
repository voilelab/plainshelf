// Usage: node smoke.mjs <site dir from build.sh> [screenshot.png]
// Opens the real frontend on the wasm server in chromium, checks that the
// shelf survives a reload through OPFS, then drives the raw API page.
import path from 'node:path';
import { createRequire } from 'node:module';
import { serveStatic } from './serve.mjs';

const require = createRequire(new URL('../../../e2e/package.json', import.meta.url));
const { chromium } = require('playwright');

const dir = path.resolve(process.argv[2] ?? '.');
const srv = await serveStatic(dir);
const origin = `http://127.0.0.1:${srv.address().port}`;

// CHROMIUM points at a preinstalled browser; unset, Playwright uses its own.
const browser = await chromium.launch({ executablePath: process.env.CHROMIUM || undefined });
const context = await browser.newContext(); // fresh: OPFS starts empty
const page = await context.newPage();
page.on('console', (m) => { if (process.env.VERBOSE) console.log('[page]', m.text()); });
page.on('pageerror', (e) => console.log('[pageerror]', e.message));

let failed = false;
function check(name, ok, detail = '') {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${detail ? ` — ${detail}` : ''}`);
  if (!ok) failed = true;
}

// Seeds one book with a cover, only into an empty shelf, the way a demo would.
await page.addInitScript(() => {
  window.plainshelfSeed = async (serve) => {
    window.seedRan = false;
    for (let i = 0; i < 50; i++) {
      const r = await serve('/api/shelves/demo/books');
      if (r.status === 503) {
        await new Promise((ok) => setTimeout(ok, 100));
        continue;
      }
      if ((await r.json()).length > 0) return;
      const created = await serve('/api/shelves/demo/books', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: '在瀏覽器裡建立的書', folder: [] }),
      });
      const canvas = new OffscreenCanvas(120, 180);
      const ctx = canvas.getContext('2d');
      ctx.fillStyle = '#3b6ea5';
      ctx.fillRect(0, 0, 120, 180);
      const png = await canvas.convertToBlob({ type: 'image/png' });
      const { meta } = await created.json();
      await serve(`/api/shelves/demo/books/${meta.id}/cover`, {
        method: 'PUT', headers: { 'Content-Type': 'image/png' }, body: png,
      });
      window.seedRan = true;
      return;
    }
  };
});

const listBooks = () => page.evaluate(async () => {
  const r = await fetch('/api/shelves/demo/books');
  return (await r.json()).map((b) => b.meta.id);
});
const bookCaches = () => page.evaluate(() => new Promise((resolve) => {
  fs.readdir('/plainshelf/shelf/app', (err, names) => resolve(err ? [] : names.filter((n) => n.startsWith('book-cache-'))));
}));
// Covers load through <img src>, which only the service worker relay can answer.
const coverStatus = async (tab = page) => {
  const img = tab.locator('img[src*="/cover"]').first();
  try {
    await img.waitFor({ timeout: 10000 });
    await tab.waitForFunction((el) => el.complete, await img.elementHandle(), { timeout: 10000 });
  } catch {
    return 'no cover <img>';
  }
  return img.evaluate((el) => `${el.naturalWidth}x${el.naturalHeight}`);
};
const openApp = async (reload) => {
  const t0 = Date.now();
  await (reload ? page.reload() : page.goto(`${origin}/`));
  await page.getByText('Total Books').first().waitFor({ timeout: 30000 });
  return Date.now() - t0;
};

const firstMs = await openApp(false);
const first = await listBooks();
check('first load seeds one book', first.length === 1 && await page.evaluate(() => window.seedRan), `${firstMs} ms`);
const firstCover = await coverStatus();
check('cover renders on first visit', /^[1-9]\d*x[1-9]/.test(firstCover), firstCover);
await page.waitForTimeout(1000); // no explicit flush: the debounced write must land on its own

const secondMs = await openApp(true);
const second = await listBooks();
check('reload keeps the book', JSON.stringify(second) === JSON.stringify(first), `${secondMs} ms, ids ${second.join(',')}`);
check('reload does not seed again', !(await page.evaluate(() => window.seedRan)));
const secondCover = await coverStatus();
check('cover renders after reload', /^[1-9]\d*x[1-9]/.test(secondCover), secondCover);
await page.waitForTimeout(500); // let the startup scan export its cache
const caches = await bookCaches();
check('one book cache across loads', caches.length === 1, caches.join(','));
console.log('app text:', (await page.innerText('body')).replace(/\s+/g, ' ').slice(0, 200));
if (process.argv[3]) await page.screenshot({ path: process.argv[3] });

// The raw API page, on the same persisted shelf.
await page.goto(`${origin}/wasm/index.html`);
await page.waitForFunction(() => document.getElementById('status').textContent !== 'loading…', null, { timeout: 60000 });
const apiCall = (method, url, body) => page.evaluate(async ([method, url, body]) => {
  const enc = body === undefined ? undefined : new TextEncoder().encode(JSON.stringify(body));
  const headers = enc ? { 'Content-Type': 'application/json' } : {};
  // A read during the initial scan answers 503; retry briefly.
  for (let i = 0; ; i++) {
    const r = await window.plainshelfRequest({ method, url, headers, body: enc });
    if (r.status !== 503 || i > 50) {
      const text = new TextDecoder().decode(r.body);
      return { status: r.status, body: text ? JSON.parse(text) : null };
    }
    await new Promise((ok) => setTimeout(ok, 100));
  }
}, [method, url, body]);
const created = await apiCall('POST', '/api/shelves/demo/books', { title: '測試書', folder: [] });
const books = await apiCall('GET', '/api/shelves/demo/books');
check('API page creates a second book', created.status === 201 && books.body.length === 2, `status ${created.status}`);

// A move and a trash are renames in memfs; the old OPFS entries must go.
const moved = await apiCall('PATCH', `/api/shelves/demo/books/${first[0]}`, { folder: ['收藏'] });
const trashed = await apiCall('DELETE', `/api/shelves/demo/books/${created.body.meta.id}`);
check('move and trash succeed', moved.status === 200 && trashed.status < 300, `${moved.status}, ${trashed.status}`);
await page.evaluate(() => window.plainshelfFlush());
await page.reload();
await page.waitForFunction(() => document.getElementById('status').textContent === 'ready', null, { timeout: 60000 });
const after = (await apiCall('GET', '/api/shelves/demo/books')).body;
const trash = (await apiCall('GET', '/api/shelves/demo/trash/books')).body;
check('move survives reload', after.length === 1 && after[0].folder.join('/') === '收藏', JSON.stringify(after.map((x) => x.folder)));
check('trash survives reload', trash.length === 1, `${trash.length} in trash`);
const top = await page.evaluate(() => new Promise((ok) => fs.readdir('/plainshelf/shelf/books', (e, l) => ok(l))));
check('no stale book directories', JSON.stringify(top) === '["收藏"]', JSON.stringify(top));

// Two tabs: one runs the server, the other forwards to it, and the second takes
// over from OPFS when the first closes. Neither may lose the other's writes.
const tabA = page;
const tabB = await context.newPage();
await tabB.goto(`${origin}/wasm/index.html`);
await tabB.waitForFunction(() => document.getElementById('status').textContent === 'ready', null, { timeout: 60000 });
const callIn = (tab, method, url, body) => tab.evaluate(async ([method, url, body]) => {
  const enc = body === undefined ? undefined : new TextEncoder().encode(JSON.stringify(body));
  const r = await window.plainshelfRequest({ method, url, headers: enc ? { 'Content-Type': 'application/json' } : {}, body: enc });
  const text = new TextDecoder().decode(r.body);
  return { status: r.status, body: text ? JSON.parse(text) : null };
}, [method, url, body]);
const titles = async (tab) => (await callIn(tab, 'GET', '/api/shelves/demo/books')).body.map((x) => x.meta.title).sort();
const runsServer = (tab) => tab.evaluate(() => typeof window.plainshelfFetch === 'function');
check('only the first tab runs the server', await runsServer(tabA) && !(await runsServer(tabB)));
const fromB = await callIn(tabB, 'POST', '/api/shelves/demo/books', { title: '分頁B的書', folder: [] });
const fromA = await callIn(tabA, 'POST', '/api/shelves/demo/books', { title: '分頁A的書', folder: [] });
const seenA = await titles(tabA);
const seenB = await titles(tabB);
check('both tabs see both writes', fromA.status === 201 && fromB.status === 201 &&
  JSON.stringify(seenA) === JSON.stringify(seenB) && seenA.length === 3, seenB.join(','));
await tabA.waitForTimeout(500); // the serving tab's debounced write
await tabA.close();
await tabB.waitForFunction(() => typeof window.plainshelfFetch === 'function', null, { timeout: 30000 });
check('the other tab takes over when it closes', true);
const fromB2 = await callIn(tabB, 'POST', '/api/shelves/demo/books', { title: '接手後的書', folder: [] });
const afterTakeover = await titles(tabB);
check('takeover keeps every write', fromB2.status === 201 && afterTakeover.length === 4, afterTakeover.join(','));
await tabB.evaluate(() => window.plainshelfFlush());
await tabB.reload();
await tabB.waitForFunction(() => document.getElementById('status').textContent === 'ready', null, { timeout: 60000 });
const afterReload = await titles(tabB);
check('and they survive a reload', JSON.stringify(afterReload) === JSON.stringify(afterTakeover), afterReload.join(','));

// A tab without the server still shows covers: sw.js -> that tab -> the serving tab.
const tabC = await context.newPage();
await tabC.goto(`${origin}/`);
await tabC.getByText('Total Books').first().waitFor({ timeout: 30000 });
const forwardedCover = await coverStatus(tabC);
check('cover renders in a tab that forwards', !(await runsServer(tabC)) && /^[1-9]\d*x[1-9]/.test(forwardedCover), forwardedCover);

await browser.close();
srv.close();
process.exit(failed ? 1 : 0);
