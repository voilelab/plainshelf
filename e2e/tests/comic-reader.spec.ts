import { promises as fs } from 'node:fs';
import path from 'node:path';
import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { tinyPngBase64 } from './support/books';

const getServer = useServer();
const sourceID = '20260402-090000';

test('reads ltr and rtl comics, restores page progress, and edits pages touching only source.txt and one image', { tag: '@smoke' }, async ({ page }) => {
  const { baseUrl, shelfDir } = getServer();
  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  for (const direction of ['ltr', 'rtl']) {
    const id = `comic${direction}`;
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
    await fs.writeFile(path.join(sourceDir, 'source.txt'), 'random-z.png\nlost.png\nrandom-a.png\n');
    for (const name of ['random-z.png', 'random-a.png']) {
      await fs.writeFile(path.join(sourceDir, 'assets', name), Buffer.from(tinyPngBase64, 'base64'));
    }
  }
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

  await cells.nth(2).dragTo(cells.nth(0));
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
