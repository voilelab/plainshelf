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

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM ?? '/opt/pw-browsers/chromium' });
const page = await browser.newPage(); // a fresh context: OPFS starts empty
page.on('console', (m) => { if (process.env.VERBOSE) console.log('[page]', m.text()); });
page.on('pageerror', (e) => console.log('[pageerror]', e.message));

let failed = false;
function check(name, ok, detail = '') {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${detail ? ` — ${detail}` : ''}`);
  if (!ok) failed = true;
}

// Seeds one book only into an empty shelf, the way a demo would.
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
      await serve('/api/shelves/demo/books', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: '在瀏覽器裡建立的書', folder: [] }),
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
const openApp = async (reload) => {
  const t0 = Date.now();
  await (reload ? page.reload() : page.goto(`${origin}/`));
  await page.getByText('Total Books').first().waitFor({ timeout: 30000 });
  return Date.now() - t0;
};

const firstMs = await openApp(false);
const first = await listBooks();
check('first load seeds one book', first.length === 1 && await page.evaluate(() => window.seedRan), `${firstMs} ms`);
await page.waitForTimeout(1000); // no explicit flush: the debounced write must land on its own

const secondMs = await openApp(true);
const second = await listBooks();
check('reload keeps the book', JSON.stringify(second) === JSON.stringify(first), `${secondMs} ms, ids ${second.join(',')}`);
check('reload does not seed again', !(await page.evaluate(() => window.seedRan)));
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
    const r = await window.plainshelfFetch(method, url, headers, enc);
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

await browser.close();
srv.close();
process.exit(failed ? 1 : 0);
