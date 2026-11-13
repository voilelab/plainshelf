import { promises as fs } from 'node:fs';
import path from 'node:path';
import { crc32, deflateSync } from 'node:zlib';
import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { tinyPngBase64 } from './support/books';

const getServer = useServer();
const sourceID = '20260402-090000';

function portraitPng(): Buffer {
  const chunk = (type: string, data: Buffer) => {
    const body = Buffer.concat([Buffer.from(type), data]);
    const out = Buffer.alloc(body.length + 8);
    out.writeUInt32BE(data.length, 0);
    body.copy(out, 4);
    out.writeUInt32BE(crc32(body), body.length + 4);
    return out;
  };
  const header = Buffer.alloc(13);
  header.writeUInt32BE(1, 0);
  header.writeUInt32BE(2, 4);
  header.set([8, 0, 0, 0, 0], 8);
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', header), chunk('IDAT', deflateSync(Buffer.from([0, 255, 0, 128]))), chunk('IEND', Buffer.alloc(0))
  ]);
}

async function writeComic(shelfDir: string, id: string, direction: string, listed: string[], files: string[], image = Buffer.from(tinyPngBase64, 'base64')): Promise<string> {
  const bookDir = path.join(shelfDir, 'books', `${id}.bookpkg`);
  const sourceDir = path.join(bookDir, 'sources', sourceID);
  await fs.mkdir(path.join(sourceDir, 'assets'), { recursive: true });
  await fs.writeFile(path.join(bookDir, 'book.json'), JSON.stringify({
    schema_version: 1, id, title: id, format: 'images', current_source: sourceID
  }));
  await fs.writeFile(path.join(sourceDir, 'meta.json'), JSON.stringify({
    schema_version: 1, id: sourceID, format: 'images', page_direction: direction,
    md5_hash: 'abc', created_at: '2026-04-02T09:00:00Z'
  }));
  await fs.writeFile(path.join(sourceDir, 'source.txt'), listed.map((name) => `${name}\n`).join(''));
  for (const name of files) {
    await fs.writeFile(path.join(sourceDir, 'assets', name), image);
  }
  return bookDir;
}

test('reads ltr and rtl comics, restores page progress, sizes pages, and edits pages touching only source.txt and one image', { tag: '@smoke' }, async ({ page }) => {
  const { baseUrl, shelfDir } = getServer();
  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  for (const direction of ['ltr', 'rtl']) {
    await writeComic(shelfDir, `comic${direction}`, direction, ['random-z.png', 'lost.png', 'random-a.png'], ['random-z.png', 'random-a.png']);
  }
  const fitBookDir = await writeComic(shelfDir, 'comicfit', 'rtl', ['a.png', 'b.png'], ['a.png', 'b.png'], portraitPng());
  const fitShelfFiles = async () => Promise.all(['book.json', `sources/${sourceID}/meta.json`].map((name) => fs.readFile(path.join(fitBookDir, name), 'utf8')));
  const fitShelfBefore = await fitShelfFiles();
  await page.getByRole('button', { name: 'Update book list' }).click();
  await expect(page.locator('.book-list-row').getByRole('heading', { name: 'comicrtl', exact: true })).toBeVisible();

  for (const direction of ['ltr', 'rtl']) {
    await page.setViewportSize(direction === 'ltr' ? { width: 1280, height: 720 } : { width: 390, height: 844 });
    await page.goto(`${baseUrl}/reader/comic${direction}`);
    const image = page.locator('.comic-page-image:visible img');
    const counter = page.locator('.comic-count');
    await expect(counter).toHaveText('1 / 3');
    await expect(image).toHaveAttribute('alt', 'Page 1');
    await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);
    const imageBox = await image.boundingBox();
    expect(imageBox!.height).toBeLessThan(direction === 'ltr' ? 720 : 844);
    const viewport = page.locator('.comic-viewport');
    const box = await viewport.boundingBox();
    await viewport.click({ position: { x: box!.width * (direction === 'ltr' ? 0.9 : 0.1), y: box!.height / 2 } });
    await expect(counter).toHaveText('2 / 3');
    await expect(page.getByText('Page 2 is unavailable. You can continue to the next page.')).toBeVisible();
    await page.keyboard.press(direction === 'ltr' ? 'ArrowRight' : 'ArrowLeft');
    await expect(counter).toHaveText('3 / 3');
    await expect(image).toHaveAttribute('alt', 'Page 3');
    await expect(page.locator('.comic-progress')).toContainText('100%');
    await page.getByRole('button', { name: 'Back', exact: true }).click();
    await page.goto(`${baseUrl}/books/comic${direction}`);
    await expect(page.locator('.progress-percent')).toHaveText('100%');
    await expect(page.getByRole('button', { name: 'Read again', exact: true })).toBeVisible();
    await page.goto(`${baseUrl}/reader/comic${direction}`);
    await expect(counter).toHaveText('3 / 3');
    await page.keyboard.press(direction === 'ltr' ? 'ArrowLeft' : 'ArrowRight');
    await expect(counter).toHaveText('2 / 3');
  }

  const fitViewport = page.locator('.comic-viewport');
  const fitImage = page.locator('.comic-page-image:visible img');
  const fitCounter = page.locator('.comic-count');
  const scroll = () => fitViewport.evaluate((el) => ({
    top: el.scrollTop, maxTop: el.scrollHeight - el.clientHeight, width: el.clientWidth
  }));
  for (const size of [{ width: 1280, height: 720 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(size);
    await page.goto(`${baseUrl}/reader/comicfit`);
    await expect(fitCounter).toHaveText(/^\d \/ 2$/);
    await expect.poll(() => fitImage.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);
    if (size.width === 1280) {
      await expect(page.getByRole('combobox', { name: 'Page size' })).toHaveValue('fit-height');
      expect((await scroll()).maxTop).toBe(0);
      await page.keyboard.press('+');
      await expect(page.locator('.comic-zoom')).toHaveText('125%');
      await page.keyboard.press('0');
      await expect(page.locator('.comic-zoom')).toHaveText('100%');
      await page.getByRole('combobox', { name: 'Page size' }).selectOption('fit-width');
    }
    await expect(page.getByRole('combobox', { name: 'Page size' })).toHaveValue('fit-width');
    await expect.poll(async () => Math.round((await fitImage.boundingBox())!.width)).toBe((await scroll()).width);
    const start = await scroll();
    expect(start.maxTop).toBeGreaterThan(0);
    expect(start.top).toBe(0);
    await fitViewport.hover();
    await page.mouse.wheel(0, start.maxTop + 500);
    await expect.poll(async () => (await scroll()).top).toBe(start.maxTop);
    await expect(fitCounter).toHaveText(size.width === 1280 ? '1 / 2' : '2 / 2');
    if (size.width === 1280) {
      await page.keyboard.press('ArrowLeft');
      await expect(fitCounter).toHaveText('2 / 2');
      await expect.poll(async () => (await scroll()).top).toBe(0);
    }
  }
  expect(await fitShelfFiles()).toEqual(fitShelfBefore);

  await page.setViewportSize({ width: 1280, height: 720 });
  const sourceDir = path.join(shelfDir, 'books', 'comicltr.bookpkg', 'sources', sourceID);
  const assetsDir = path.join(sourceDir, 'assets');
  const readList = async () => fs.readFile(path.join(sourceDir, 'source.txt'), 'utf8');
  const assetStats = async () => Promise.all((await fs.readdir(assetsDir)).sort()
    .map(async (name) => [name, (await fs.stat(path.join(assetsDir, name))).mtimeMs]));
  await page.goto(`${baseUrl}/books/comicltr/sources`);
  const cells = page.getByTestId('page-grid').locator('> li');
  await expect(cells).toHaveCount(3);
  await expect(cells.nth(1)).toContainText('Missing page');
  const before = await assetStats();

  await cells.nth(2).dragTo(cells.nth(0), { targetPosition: { x: 8, y: 40 } });
  await expect.poll(readList).toBe('random-a.png\nrandom-z.png\nlost.png\n');
  const rightHalf = async (index: number) => {
    const box = await cells.nth(index).boundingBox();
    return { x: (box?.width ?? 16) - 8, y: 40 };
  };
  await cells.nth(0).dragTo(cells.nth(1), { targetPosition: await rightHalf(1) });
  await expect.poll(readList).toBe('random-z.png\nrandom-a.png\nlost.png\n');
  await cells.nth(1).dragTo(cells.nth(0), { targetPosition: { x: 8, y: 40 } });
  await expect.poll(readList).toBe('random-a.png\nrandom-z.png\nlost.png\n');
  expect(await assetStats()).toEqual(before);

  await page.getByRole('button', { name: 'Page 1', exact: true }).click();
  await page.getByTestId('page-upload').setInputFiles({ name: 'new.png', mimeType: 'image/png', buffer: Buffer.from(tinyPngBase64, 'base64') });
  await expect(cells).toHaveCount(4);
  await expect.poll(async () => (await readList()).split('\n').length).toBe(5);
  const added = (await readList()).split('\n')[1];
  expect(added).toMatch(/^[A-Z2-7]+\.png$/);
  expect((await fs.readdir(assetsDir)).sort()).toEqual(['random-a.png', 'random-z.png', added].sort());

  await page.locator('li[data-page-name="random-z.png"]').getByRole('button', { name: 'Delete page' }).click();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  await expect(cells).toHaveCount(3);
  await expect.poll(readList).toBe(`random-a.png\n${added}\nlost.png\n`);
  expect((await fs.readdir(assetsDir)).sort()).toEqual(['random-a.png', added].sort());

  await page.goto(`${baseUrl}/reader/comicltr`);
  await expect(page.locator('.comic-count')).toHaveText(/\/ 3$/);
});
