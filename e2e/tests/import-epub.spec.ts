import { expect, test, type Locator, type Page } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { startServer } from './support/server';
import {
  epubFixtureChapters,
  epubFixtureDescription,
  epubFixtureTitle,
  writeEpubFixture
} from './support/epub';

let fixtureDir = '';
let epubFixturePath = '';

test.beforeAll(() => {
  fixtureDir = fs.mkdtempSync(path.join(os.tmpdir(), 'plainshelf-epub-'));
  epubFixturePath = writeEpubFixture(fixtureDir);
});

test.afterAll(() => {
  if (fixtureDir) {
    fs.rmSync(fixtureDir, { recursive: true, force: true });
  }
});

/**
 * Opens the import modal and selects the EPUB without importing yet, so the
 * caller can inspect or change the conversion options first.
 */
async function openImportWithEpub(page: Page): Promise<Locator> {
  await page.getByRole('button', { name: /^Import/ }).click();
  await page.getByRole('menuitem', { name: 'Import from files' }).click();

  const dialog = page.getByRole('dialog', { name: 'Import Book' });
  await expect(dialog).toBeVisible();
  await dialog.locator('input[type="file"]').setInputFiles(epubFixturePath);
  await expect(dialog.getByText('hello.epub')).toBeVisible();

  return dialog;
}

async function runImport(dialog: Locator): Promise<void> {
  const importButton = dialog.getByRole('button', { name: 'Import', exact: true });
  await importButton.click();
  await expect(importButton).toBeDisabled();

  await dialog.getByRole('button', { name: 'Cancel' }).click();
  await expect(dialog).not.toBeVisible();
}

async function openImportedBookInReader(page: Page): Promise<void> {
  const bookTitle = page
    .locator('.book-list-row')
    .getByRole('heading', { name: epubFixtureTitle, exact: true });
  await expect(bookTitle).toBeVisible();
  await bookTitle.click();

  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  await page.getByRole('button', { name: 'Start reading' }).click();
  await expect(page).toHaveURL(/\/reader\/[^/]+$/);
}

test('should import an EPUB and name its sections after the table of contents', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
    await expect(page.getByText('No books yet.')).toBeVisible();

    const dialog = await openImportWithEpub(page);

    // The conversion options only appear once an EPUB is selected.
    await expect(dialog.getByText('EPUB Conversion')).toBeVisible();

    await runImport(dialog);

    await expect(page.getByText('1 books')).toBeVisible();
    // The book's own dc:title wins over the filename.
    await openImportedBookInReader(page);

    const reader = page.getByRole('article');

    // A header section plus one section per chapter.
    await expect(reader.getByText('1 / 3')).toBeVisible();
    await expect(reader.getByText(epubFixtureDescription)).toBeVisible();

    // The default markdown preset produces a regex split, so sections are named
    // after the table of contents rather than "Part N", and the "##" heading
    // marker must not leak into the name.
    const sectionTitle = reader.locator('.reader-chapter-title');

    await reader.getByRole('button', { name: 'Next' }).click();
    await expect(reader.getByText('2 / 3')).toBeVisible();
    await expect(sectionTitle).toHaveText(epubFixtureChapters[0]);
    await expect(reader.getByText(`## ${epubFixtureChapters[0]}`)).not.toBeVisible();
    await expect(reader.getByText('This text came from a real uploaded EPUB file.')).toBeVisible();

    await reader.getByRole('button', { name: 'Next' }).click();
    await expect(reader.getByText('3 / 3')).toBeVisible();
    await expect(sectionTitle).toHaveText(epubFixtureChapters[1]);
  } finally {
    await server.dispose();
  }
});

test('should honour the plain text conversion option', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    const dialog = await openImportWithEpub(page);

    await dialog.getByRole('combobox').click();
    await page.getByRole('option', { name: 'Plain text' }).click();
    await dialog.getByText('Put the book description at the start of the text').click();

    await runImport(dialog);

    await expect(page.getByText('1 books')).toBeVisible();
    await openImportedBookInReader(page);

    const reader = page.getByRole('article');

    // include_description was turned off, so the description stays in the book
    // metadata and out of the text.
    await expect(reader.getByText('1 / 3')).toBeVisible();
    await expect(reader.getByText(epubFixtureDescription)).not.toBeVisible();

    await reader.getByRole('button', { name: 'Next' }).click();
    await expect(reader.getByText('This text came from a real uploaded EPUB file.')).toBeVisible();
    // Plain titles carry no marker to anchor a regex on, so the split falls back
    // to boundaries and the reader labels sections positionally.
    await expect(reader.locator('.reader-chapter-title')).toHaveText('Part 2');
    // The chapter name is still in the text even though it is not the label.
    await expect(reader.getByText(epubFixtureChapters[0], { exact: true }).first()).toBeVisible();
  } finally {
    await server.dispose();
  }
});
