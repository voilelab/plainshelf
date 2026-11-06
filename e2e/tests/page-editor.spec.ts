import { promises as fs } from 'node:fs';
import path from 'node:path';
import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { tinyPngBase64 } from './support/books';

const getServer = useServer();
const sourceID = '20260402-090000';

test('reorders, adds and deletes comic pages, rewriting only source.txt and one image', async ({ page }) => {
  const { baseUrl, shelfDir } = getServer();
  const bookDir = path.join(shelfDir, 'books', 'pagedcomic.bookpkg');
  const sourceDir = path.join(bookDir, 'sources', sourceID);
  const assetsDir = path.join(sourceDir, 'assets');
  const png = Buffer.from(tinyPngBase64, 'base64');
  await fs.mkdir(assetsDir, { recursive: true });
  await fs.writeFile(path.join(bookDir, 'book.json'), JSON.stringify({
    schema_version: 1, id: 'pagedcomic', title: 'pagedcomic', format: 'images', current_source: sourceID
  }));
  await fs.writeFile(path.join(sourceDir, 'meta.json'), JSON.stringify({
    schema_version: 1, id: sourceID, format: 'images', md5_hash: 'abc', created_at: '2026-04-02T09:00:00Z'
  }));
  await fs.writeFile(path.join(sourceDir, 'source.txt'), 'a.png\nb.png\nc.png\n');
  for (const name of ['a.png', 'b.png', 'c.png']) await fs.writeFile(path.join(assetsDir, name), png);
  const readList = async () => fs.readFile(path.join(sourceDir, 'source.txt'), 'utf8');
  const assetNames = async () => (await fs.readdir(assetsDir)).sort();

  await page.goto(`${baseUrl}/books`);
  await page.getByRole('button', { name: 'Update book list' }).click();
  await expect(page.locator('.book-list-row').getByRole('heading', { name: 'pagedcomic', exact: true })).toBeVisible();

  await page.goto(`${baseUrl}/books/pagedcomic/sources`);
  const cells = page.getByTestId('page-grid').locator('> li');
  await expect(cells).toHaveCount(3);
  const before = await Promise.all((await assetNames()).map(async (name) => [name, (await fs.stat(path.join(assetsDir, name))).mtimeMs]));

  await cells.nth(2).dragTo(cells.nth(0));
  await expect.poll(readList).toBe('c.png\na.png\nb.png\n');
  await expect(cells.nth(0)).toHaveAttribute('data-page-name', 'c.png');
  const after = await Promise.all((await assetNames()).map(async (name) => [name, (await fs.stat(path.join(assetsDir, name))).mtimeMs]));
  expect(after).toEqual(before);

  await page.getByRole('button', { name: 'Page 1', exact: true }).click();
  await page.getByTestId('page-upload').setInputFiles({ name: 'new.png', mimeType: 'image/png', buffer: png });
  await expect(cells).toHaveCount(4);
  await expect.poll(async () => (await readList()).split('\n').length).toBe(5);
  const added = (await readList()).split('\n')[1];
  expect(added).toMatch(/^[A-Z2-7]+\.png$/);
  expect(await assetNames()).toEqual(['a.png', 'b.png', 'c.png', added].sort());

  await page.locator('li[data-page-name="a.png"]').getByRole('button', { name: 'Delete page' }).click();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  await expect(cells).toHaveCount(3);
  await expect.poll(readList).toBe(`c.png\n${added}\nb.png\n`);
  expect(await assetNames()).toEqual(['b.png', 'c.png', added].sort());
});
