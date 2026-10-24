import fs from 'node:fs/promises';
import path from 'node:path';
import { createRequire } from 'node:module';
import { serveStatic } from './serve.mjs';

const require = createRequire(new URL('../../../e2e/package.json', import.meta.url));
const playwright = require('playwright');

const engine = process.env.SMOKE_BROWSER || 'chromium';
if (!['chromium', 'firefox', 'webkit'].includes(engine)) {
  throw new Error(`SMOKE_BROWSER: unknown browser "${engine}"; expected chromium, firefox or webkit`);
}

const dir = path.resolve(process.argv[2] ?? '.');
const srv = await serveStatic(dir);
const origin = `http://127.0.0.1:${srv.address().port}`;

const base = process.env.SMOKE_BASE || '/';
const app = origin + base;

const executablePath = engine === 'chromium' ? process.env.CHROMIUM || undefined : undefined;
const browser = await playwright[engine].launch({ executablePath });
console.log(`browser: ${engine} ${browser.version()}`);
const context = await browser.newContext();

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

function zipEntryNames(buf) {
  const end = buf.lastIndexOf(Buffer.from([0x50, 0x4b, 0x05, 0x06]));
  if (end < 0) return [];
  const names = [];
  let at = buf.readUInt32LE(end + 16);
  for (let i = 0; i < buf.readUInt16LE(end + 10); i++) {
    const nameLen = buf.readUInt16LE(at + 28);
    names.push(buf.subarray(at + 46, at + 46 + nameLen).toString('utf8'));
    at += 46 + nameLen + buf.readUInt16LE(at + 30) + buf.readUInt16LE(at + 32);
  }
  return names;
}

let failed = false;
function check(name, ok, detail = '') {
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${detail ? ` — ${detail}` : ''}`);
  if (!ok) failed = true;
}

const settleStartupWrites = (tab) => tab.waitForTimeout(1000);
const settleWriteBehind = (tab) => tab.waitForTimeout(500);
const waitForImportRefusal = (tab) => tab.waitForTimeout(500);

const expectMemory = process.env.SMOKE_EXPECT_MEMORY === '1';
let persistent = true;
function persisted(name, ok, detail = '') {
  if (persistent) check(name, ok, detail);
  else console.log(`skip ${name} — this browser has no OPFS, so nothing persists`);
}

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

const coverStatus = async (tab = page) => {
  const img = tab.locator('img[src*="/cover"]').first();
  try {
    await img.waitFor({ timeout: 10000 });
    await tab.waitForFunction((el) => el.complete, await img.elementHandle(), { timeout: 10000 });
  } catch {

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
  await (reload ? page.reload() : page.goto(app));
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
await settleStartupWrites(page);

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
  const cdp = await context.newCDPSession(page);
  const loaded = page.waitForEvent('load');
  await cdp.send('Page.reload', { ignoreCache: true });
  await loaded;
  await page.getByText('Total Books').first().waitFor({ timeout: 30000 });
  const hardCover = await coverStatus();
  check('cover renders after a hard reload', /^[1-9]\d*x[1-9]/.test(hardCover), hardCover);
}
await settleWriteBehind(page);
const caches = await bookCaches();
check('one book cache across loads', caches.length === 1, caches.join(','));
console.log('app text:', (await page.innerText('body')).replace(/\s+/g, ' ').slice(0, 200));
if (process.argv[3]) await page.screenshot({ path: process.argv[3] });

await page.goto(`${app}wasm/index.html`);
await page.waitForFunction(() => document.getElementById('status').textContent !== 'loading…', null, { timeout: 60000 });
const apiCall = (method, url, body) => page.evaluate(async ([method, url, body]) => {
  const enc = body === undefined ? undefined : new TextEncoder().encode(JSON.stringify(body));
  const headers = enc ? { 'Content-Type': 'application/json' } : {};

  for (let i = 0; ; i++) {
    const r = await window.plainshelfRequest({ method, url, headers, body: enc });
    if (r.status !== 503 || i > 50) {
      const text = new TextDecoder().decode(r.body);
      return { status: r.status, body: text ? JSON.parse(text) : null };
    }
    await new Promise((ok) => setTimeout(ok, 100));
  }
}, [method, url, body]);

const nsfwBefore = await apiCall('GET', '/api/setting/show_nsfw');
const nsfwSet = await apiCall('POST', '/api/setting/show_nsfw', true);
const created = await apiCall('POST', '/api/shelves/demo/books', { title: '測試書', folder: [] });
const books = await apiCall('GET', '/api/shelves/demo/books');
check('API page creates a second book', created.status === 201 && books.body.length === 2, `status ${created.status}`);

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

const tabA = page;
const tabB = await context.newPage();
await tabB.goto(`${app}wasm/index.html`);
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
await settleWriteBehind(tabA);
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

const tabC = await context.newPage();
await tabC.goto(app);
await tabC.getByText('Total Books').first().waitFor({ timeout: 30000 });
const forwardedCover = await coverStatus(tabC);
check('cover renders in a tab that forwards', !(await runsServer(tabC)) && /^[1-9]\d*x[1-9]/.test(forwardedCover), forwardedCover);

check('the demo bar is shown', await tabC.locator('#plainshelf-demo-bar').isVisible());

const noteWanted = persistent ? 'may clear it' : 'a reload starts over';
const note = await tabC.locator('#plainshelf-demo-note').filter({ hasText: noteWanted })
  .waitFor({ timeout: 5000 }).then(() => true, () => false);
check('the demo bar warns where the shelf is kept', note,
  await tabC.locator('#plainshelf-demo-note').innerText().catch(() => 'missing'));

const persistPrompt = tabC.locator('#plainshelf-demo-persist');
const offered = await persistPrompt.isVisible();
check(persistent ? 'the demo bar offers to keep the shelf' : 'without OPFS it offers nothing to keep', offered === persistent);
let kept = false;
if (persistent && engine === 'chromium') {
  await tabC.getByRole('button', { name: 'Keep this shelf' }).click();
  kept = await tabC.evaluate(() => navigator.storage.persisted());
  const answer = kept ? 'has agreed not to clear it' : 'The browser declined';
  const shown = await tabC.locator('#plainshelf-demo-bar').filter({ hasText: answer })
    .waitFor({ timeout: 5000 }).then(() => true, () => false);
  check('the demo bar shows the browser\'s answer', shown, `persisted=${kept}`);
}

const settingsTab = await context.newPage();
await settingsTab.goto(new URL('settings?tab=demo-storage', app).href);
const statusWanted = !persistent ? 'Not saved' : kept ? 'Kept:' : 'Not kept';
const status = await settingsTab.getByTestId('demo-storage-status').filter({ hasText: statusWanted })
  .waitFor({ timeout: 30000 }).then(() => true, () => false);
check('settings report the browser storage state', status,
  await settingsTab.getByTestId('demo-storage-status').innerText().catch(() => 'missing'));
check('settings offer to ask only while the shelf is not kept',
  await settingsTab.getByRole('button', { name: 'Ask to keep it' }).isVisible() === (persistent && !kept));
if (engine === 'chromium') {
  check('an answered prompt does not return in another tab', !(await settingsTab.locator('#plainshelf-demo-persist').isVisible()));
}
await settingsTab.close();
const [download] = await Promise.all([
  tabC.waitForEvent('download'),
  tabC.getByRole('button', { name: 'Export shelf' }).click(),
]);
const zipBytes = await fs.readFile(await download.path());
const exportedTitles = await titles(tabB);
const zipNames = zipEntryNames(zipBytes);

check('export is a shelf zip', zipNames.length > 0 && zipNames.every((n) => /^(books|trash)\//.test(n)) &&
  exportedTitles.every((t) => zipNames.some((n) => n.endsWith(`${t.toLowerCase()}.bookpkg/book.json`))),
  `${zipNames.length} entries, ${exportedTitles.length} books`);
const extra = await callIn(tabB, 'POST', '/api/shelves/demo/books', { title: '匯入後應消失的書', folder: [] });
const dialogs = [];
tabC.on('dialog', (d) => {
  dialogs.push(d.message());
  d.accept();
});
const [chooser] = await Promise.all([
  tabC.waitForEvent('filechooser'),
  tabC.getByRole('button', { name: 'Import shelf…' }).click(),
]);
if (persistent) {
  const reloaded = tabC.waitForEvent('load');
  await chooser.setFiles({ name: 'shelf.zip', mimeType: 'application/zip', buffer: zipBytes });
  await reloaded;
  await tabC.getByText('Total Books').first().waitFor({ timeout: 30000 });
  const importedTitles = await titles(tabC);
  check('import restores the exported shelf in every tab', extra.status === 201 &&
    JSON.stringify(importedTitles) === JSON.stringify(exportedTitles), importedTitles.join(','));
} else {
  await chooser.setFiles({ name: 'shelf.zip', mimeType: 'application/zip', buffer: zipBytes });
  await waitForImportRefusal(tabC);
  const after = await titles(tabC);
  check('without OPFS an import is refused, not lost', dialogs.some((m) => m.includes('cannot keep')) &&
    after.includes('匯入後應消失的書'), dialogs.join(' | '));
}
const badImport = await tabC.evaluate(async () => (await window.plainshelfRequest({
  method: 'PUT', url: '/_demo/shelf.zip', headers: {}, body: new TextEncoder().encode('not a zip'),
})).status);
check('a file that is not a shelf zip is refused', badImport === 400, `status ${badImport}`);

const tabD = await context.newPage();
const linked = await tabD.goto(`${app}trash`);
await tabD.waitForFunction(() => document.title.startsWith('Trash'), null, { timeout: 30000 });
const landed = await tabD.evaluate(() => location.pathname + location.search);
check('a deep link opens its route', landed === `${base}trash`, `status ${linked.status()}, at ${landed}`);

await browser.close();
srv.close();
process.exit(failed ? 1 : 0);
