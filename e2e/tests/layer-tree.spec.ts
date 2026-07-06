import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { helloFixturePath, importBookFromPath } from './support/books';
import {
  addLayer,
  emptyDataTransfer,
  layerRow,
  layersQueryRegex,
  openLayerContextMenu,
  selectAllBooks,
  selectLayer,
  switchToCardView
} from './support/layers';

test('creates a nested layer in one submission and filters books by exact layer', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    await addLayer(page, 'novels/scifi');

    await expect(layerRow(page, 'novels')).toBeVisible();
    await expect(layerRow(page, 'scifi')).toBeVisible();

    await selectLayer(page, 'scifi');
    await expect(page).toHaveURL(layersQueryRegex('novels/scifi'));

    await importBookFromPath(page, helloFixturePath);
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();

    await selectAllBooks(page);
    await expect(page).not.toHaveURL(/[?&]layers=/);
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();

    // "novels" itself has no directly-attached books (only its "scifi" child does).
    await selectLayer(page, 'novels');
    await expect(page.getByText('No books in novels.')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('expands and collapses a layer that has children', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await addLayer(page, 'novels/scifi');

    const novelsRow = layerRow(page, 'novels');
    const scifiRow = layerRow(page, 'scifi');
    await expect(scifiRow).toBeVisible();

    await novelsRow.getByRole('button', { name: 'Collapse layer', exact: true }).click();
    await expect(scifiRow).toHaveCount(0);

    await novelsRow.getByRole('button', { name: 'Expand layer', exact: true }).click();
    await expect(scifiRow).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('renames a layer, updating the tree and the active URL filter', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await addLayer(page, 'temp');
    await selectLayer(page, 'temp');
    await expect(page).toHaveURL(layersQueryRegex('temp'));

    await openLayerContextMenu(page, 'temp');
    await page.getByRole('menuitem', { name: 'Rename', exact: true }).click();
    const renameDialog = page.getByRole('dialog', { name: 'Rename layer' });
    await expect(renameDialog).toBeVisible();
    await renameDialog.getByLabel('Layer name').fill('renamed');
    await renameDialog.getByRole('button', { name: 'Rename', exact: true }).click();

    await expect(renameDialog).not.toBeVisible();
    await expect(layerRow(page, 'temp')).toHaveCount(0);
    await expect(layerRow(page, 'renamed')).toBeVisible();
    await expect(page).toHaveURL(layersQueryRegex('renamed'));
  } finally {
    await server.dispose();
  }
});

test('only offers Delete for empty layers, and deleting removes it from the tree', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);

    await addLayer(page, 'withbook');
    await selectLayer(page, 'withbook');
    await importBookFromPath(page, helloFixturePath);

    await openLayerContextMenu(page, 'withbook');
    await expect(page.getByRole('menuitem', { name: 'Rename', exact: true })).toBeVisible();
    await expect(page.getByRole('menuitem', { name: 'Delete', exact: true })).toHaveCount(0);
    await page.keyboard.press('Escape');

    await addLayer(page, 'removable');
    await openLayerContextMenu(page, 'removable');
    await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();

    const deleteDialog = page.getByRole('dialog', { name: 'Delete layer' });
    await expect(deleteDialog).toBeVisible();
    await deleteDialog.getByRole('button', { name: 'Delete', exact: true }).click();

    await expect(deleteDialog).not.toBeVisible();
    await expect(layerRow(page, 'removable')).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});

test('moves a book into a layer via drag and drop', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);
    await expect(
      page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();

    await addLayer(page, 'reading');
    await selectLayer(page, 'reading');
    await expect(page.getByText('No books in reading.')).toBeVisible();

    await selectAllBooks(page);
    await switchToCardView(page);

    const bookCard = page.locator('.book-card-view').filter({
      has: page.getByRole('heading', { name: 'hello', exact: true })
    });
    await expect(bookCard).toBeVisible();

    const dataTransfer = await emptyDataTransfer(page);
    try {
      await bookCard.dispatchEvent('dragstart', { dataTransfer });
      const readingRow = layerRow(page, 'reading');
      await readingRow.dispatchEvent('dragenter', { dataTransfer });
      await readingRow.dispatchEvent('dragover', { dataTransfer });
      await readingRow.dispatchEvent('drop', { dataTransfer });
    } finally {
      await dataTransfer.dispose();
    }

    await selectLayer(page, 'reading');
    await expect(
      page.locator('.book-card-view').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();
    await expect(page.getByText('1 books')).toBeVisible();

    await selectAllBooks(page);
    await expect(
      page.locator('.book-card-view').getByRole('heading', { name: 'hello', exact: true })
    ).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('moves a top-level layer under another layer via drag and drop', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await addLayer(page, 'A');
    await addLayer(page, 'B');

    const bRowBefore = layerRow(page, 'B');
    await expect(bRowBefore).toBeVisible();
    await expect(bRowBefore).toHaveCSS('padding-left', '8px');

    const dataTransfer = await emptyDataTransfer(page);
    try {
      await bRowBefore.dispatchEvent('dragstart', { dataTransfer });
      const aRow = layerRow(page, 'A');
      await aRow.dispatchEvent('dragenter', { dataTransfer });
      await aRow.dispatchEvent('dragover', { dataTransfer });
      await aRow.dispatchEvent('drop', { dataTransfer });
    } finally {
      await dataTransfer.dispose();
    }

    const bRowAfter = layerRow(page, 'B');
    await expect(bRowAfter).toHaveCSS('padding-left', '22px');

    await bRowAfter.getByRole('button', { name: 'B', exact: true }).click();
    await expect(page).toHaveURL(layersQueryRegex('A/B'));
  } finally {
    await server.dispose();
  }
});
