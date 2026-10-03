// Usage: node smoke.mjs <site dir from build.sh> [screenshot.png]
// Opens the real frontend on the wasm server in a browser, checks that the
// shelf survives a reload through OPFS, then drives the raw API page.
import path from 'node:path';
import { createRequire } from 'node:module';
import { serveStatic } from './serve.mjs';

const require = createRequire(new URL('../../../e2e/package.json', import.meta.url));
const playwright = require('playwright');

// SMOKE_BROWSER picks the engine: chromium (the default and the PR gate),
// firefox or webkit (nightly, since the public demo has visitors on all three).
const engine = process.env.SMOKE_BROWSER || 'chromium';
if (!['chromium', 'firefox', 'webkit'].includes(engine)) {
  throw new Error(`SMOKE_BROWSER: unknown browser "${engine}"; expected chromium, firefox or webkit`);
}

const dir = path.resolve(process.argv[2] ?? '.');
const srv = await serveStatic(dir);
const origin = `http://127.0.0.1:${srv.address().port}`;

// CHROMIUM points at a preinstalled chromium; unset, Playwright uses its own.
const executablePath = engine === 'chromium' ? process.env.CHROMIUM || undefined : undefined;
const browser = await playwright[engine].launch({ executablePath });
console.log(`browser: ${engine} ${browser.version()}`);
const context = await browser.newContext(); // fresh: OPFS starts empty
// Every tab's console, printed when a check fails: the other engines only fail on CI.
const consoleLog = [];
let nextTab = 0;
context.on('page', (tab) => {
  const label = `tab${nextTab++}`;
  tab.on('console', (m) => {
    consoleLog.push(`[${label} ${m.type()}] ${m.text()}`);
    if (process.env.VERBOSE) console.log(`[${label}]`, m.text());
  });
  tab.on('pageerror', (e) => consoleLog.push(`[${label} pageerror] ${e.message}`));
});
const page = await context.newPage();
process.on('exit', (code) => {
  if (code !== 0) console.log(`last console lines:\n${consoleLog.slice(-40).join('\n')}`);
});

let failed = false;
function check(name, ok, detail = '') {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${detail ? ` — ${detail}` : ''}`);
  if (!ok) failed = true;
}

// Without OPFS the demo runs in memory and says so; that is its documented
// behavior, so those engines check it instead of persistence. Only an engine
// declared to lack OPFS (SMOKE_EXPECT_MEMORY=1) may skip persistence, so a
// regression elsewhere cannot hide behind the fallback.
const expectMemory = process.env.SMOKE_EXPECT_MEMORY === '1';
let persistent = true;
function persisted(name, ok, detail = '') {
  if (persistent) check(name, ok, detail);
  else console.log(`skip ${name} — this browser has no OPFS, so nothing persists`);
}

// Seeds one book with a cover, only into an empty shelf, the way a demo would.
// On the context, so whichever tab ends up serving seeds what it starts with.
await context.addInitScript(() => {
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
    // The app swaps a cover that fails to load for its placeholder, so say what is there.
    const seen = await tab.evaluate(() => ({
      controlled: Boolean(navigator.serviceWorker?.controller),
      imgs: [...document.images].map((i) => i.getAttribute('src')).slice(0, 5),
    }));
    return `no cover <img>; controlled=${seen.controlled}, imgs=${JSON.stringify(seen.imgs)}`;
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
persistent = await page.evaluate(() => window.plainshelfStorage?.persistent === true);
check(expectMemory ? 'runs in memory, as expected without OPFS' : 'persists to OPFS', persistent !== expectMemory,
  `persistent=${persistent}`);
const firstCover = await coverStatus();
check('cover renders on first visit', /^[1-9]\d*x[1-9]/.test(firstCover), firstCover);
await page.waitForTimeout(1000); // no explicit flush: the debounced write must land on its own

const secondMs = await openApp(true);
const second = await listBooks();
persisted('reload keeps the book', JSON.stringify(second) === JSON.stringify(first), `${secondMs} ms, ids ${second.join(',')}`);
persisted('reload does not seed again', !(await page.evaluate(() => window.seedRan)));
if (!persistent) {
  check('without OPFS it warns and a reload starts over',
    consoleLog.some((l) => l.includes('the demo will not survive a reload')) && second.length === 1 &&
      second[0] !== first[0] && await page.evaluate(() => window.seedRan),
    `ids ${first.join(',')} -> ${second.join(',')}`);
}
const secondCover = await coverStatus();
check('cover renders after reload', /^[1-9]\d*x[1-9]/.test(secondCover), secondCover);
if (engine === 'chromium') {
  // A hard reload loads the page past the service worker, which then has to be
  // asked to take it over. Only chromium exposes a cache-bypassing reload here.
  const cdp = await context.newCDPSession(page);
  const loaded = page.waitForEvent('load');
  await cdp.send('Page.reload', { ignoreCache: true });
  await loaded;
  await page.getByText('Total Books').first().waitFor({ timeout: 30000 });
  const hardCover = await coverStatus();
  check('cover renders after a hard reload', /^[1-9]\d*x[1-9]/.test(hardCover), hardCover);
}
await page.waitForTimeout(500); // let the startup scan export its cache
const caches = await bookCaches();
check('one book cache across loads', caches.length === 1, caches.join(','));
console.log('app text:', (await page.innerText('body')).replace(/\s+/g, ' ').slice(0, 200));
if (process.argv[3]) await page.screenshot({ path: process.argv[3] });

// The raw API page, on the same shelf (persisted, or reseeded without OPFS).
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
// Settings live in the store, which the browser build keeps in a file under OPFS.
const nsfwBefore = await apiCall('GET', '/api/setting/show_nsfw');
const nsfwSet = await apiCall('POST', '/api/setting/show_nsfw', true); // the body is a bare true or false
const created = await apiCall('POST', '/api/shelves/demo/books', { title: '測試書', folder: [] });
const books = await apiCall('GET', '/api/shelves/demo/books');
check('API page creates a second book', created.status === 201 && books.body.length === 2, `status ${created.status}`);

// A move and a trash are renames in memfs; the old OPFS entries must go.
const seeded = books.body.find((b) => b.meta.id !== created.body.meta.id).meta.id;
const moved = await apiCall('PATCH', `/api/shelves/demo/books/${seeded}`, { folder: ['收藏'] });
const trashed = await apiCall('DELETE', `/api/shelves/demo/books/${created.body.meta.id}`);
check('move and trash succeed', moved.status === 200 && trashed.status < 300, `${moved.status}, ${trashed.status}`);
await page.evaluate(() => window.plainshelfFlush());
await page.reload();
await page.waitForFunction(() => document.getElementById('status').textContent === 'ready', null, { timeout: 60000 });
const after = (await apiCall('GET', '/api/shelves/demo/books')).body;
const trash = (await apiCall('GET', '/api/shelves/demo/trash/books')).body;
persisted('move survives reload', after.length === 1 && after[0].folder.join('/') === '收藏', JSON.stringify(after.map((x) => x.folder)));
persisted('trash survives reload', trash.length === 1, `${trash.length} in trash`);
const nsfwAfter = await apiCall('GET', '/api/setting/show_nsfw');
persisted('a saved setting survives reload', nsfwBefore.body?.value === false && nsfwSet.status < 300 && nsfwAfter.body?.value === true,
  `${nsfwBefore.body?.value} -> ${nsfwSet.status} -> ${nsfwAfter.body?.value}`);
const top = await page.evaluate(() => new Promise((ok) => fs.readdir('/plainshelf/shelf/books', (e, l) => ok(l))));
persisted('no stale book directories', JSON.stringify(top) === '["收藏"]', JSON.stringify(top));

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
persisted('takeover keeps every write', fromB2.status === 201 && afterTakeover.length === 4, afterTakeover.join(','));
await tabB.evaluate(() => window.plainshelfFlush());
await tabB.reload();
await tabB.waitForFunction(() => document.getElementById('status').textContent === 'ready', null, { timeout: 60000 });
const afterReload = await titles(tabB);
persisted('and they survive a reload', JSON.stringify(afterReload) === JSON.stringify(afterTakeover), afterReload.join(','));

// A tab without the server still shows covers: sw.js -> that tab -> the serving tab.
const tabC = await context.newPage();
await tabC.goto(`${origin}/`);
await tabC.getByText('Total Books').first().waitFor({ timeout: 30000 });
const forwardedCover = await coverStatus(tabC);
check('cover renders in a tab that forwards', !(await runsServer(tabC)) && /^[1-9]\d*x[1-9]/.test(forwardedCover), forwardedCover);

await browser.close();
srv.close();
process.exit(failed ? 1 : 0);
