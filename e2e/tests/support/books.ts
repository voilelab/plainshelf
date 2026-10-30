import { expect, type Page } from '@playwright/test';
import { promises as fs } from 'node:fs';
import os from 'node:os';
import path from 'node:path';

export const helloFixturePath = path.resolve(__dirname, '..', '..', 'fixtures', 'hello.txt');
export const anotherFixturePath = path.resolve(__dirname, '..', '..', 'fixtures', 'another.txt');
export const helloMarkdownFixturePath = path.resolve(__dirname, '..', '..', 'fixtures', 'hello.md');
export const chaptersMarkdownFixturePath = path.resolve(
  __dirname,
  '..',
  '..',
  'fixtures',
  'chapters.md'
);
export const safeHtmlMarkdownFixturePath = path.resolve(
  __dirname,
  '..',
  '..',
  'fixtures',
  'safe-html.md'
);

export const tinyPngBase64 =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9s3FoXcAAAAASUVORK5CYII=';

export async function importBookFromPath(page: Page, filePath: string): Promise<void> {
  const filename = path.basename(filePath);

  await page.getByRole('button', { name: /^Import/ }).click();
  await page.getByRole('menuitem', { name: 'Import from files' }).click();

  const dialog = page.getByRole('dialog', { name: 'Import Book' });
  await expect(dialog).toBeVisible();
  await dialog.locator('input[type="file"]').setInputFiles(filePath);
  await expect(dialog.getByText(filename)).toBeVisible();

  const importButton = dialog.getByRole('button', { name: 'Import', exact: true });
  await importButton.click();
  await expect(importButton).toBeDisabled();

  await dialog.getByRole('button', { name: 'Cancel' }).click();
  await expect(dialog).not.toBeVisible();
}

export async function importHelloBook(page: Page): Promise<void> {
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  await expect(page.getByText('No books yet.')).toBeVisible();

  await importBookFromPath(page, helloFixturePath);

  await expect(page.getByText('1 books')).toBeVisible();
  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
  ).toBeVisible();
}

export async function importBookAs(
  page: Page,
  fixturePath: string,
  title: string
): Promise<string> {
  const ext = path.extname(fixturePath);
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'plainshelf-e2e-import-'));
  const tempPath = path.join(tempDir, `${title}${ext}`);
  await fs.copyFile(fixturePath, tempPath);

  try {
    await importBookFromPath(page, tempPath);
  } finally {
    await fs.rm(tempDir, { recursive: true, force: true });
  }

  await expect(
    page.locator('.book-list-row').getByRole('heading', { name: title, exact: true })
  ).toBeVisible();
  return title;
}

export async function createCoverDataTransfer(page: Page) {
  return await page.evaluateHandle((bytes) => {
    const dataTransfer = new DataTransfer();
    const binary = Uint8Array.from(atob(bytes), (char) => char.charCodeAt(0));
    dataTransfer.items.add(new File([binary], 'cover.png', { type: 'image/png' }));
    return dataTransfer;
  }, tinyPngBase64);
}
