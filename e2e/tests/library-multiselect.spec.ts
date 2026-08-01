import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { anotherFixturePath, helloFixturePath, importBookFromPath } from './support/books';
import { addLayer, emptyDataTransfer, layerRow, selectAllBooks, selectLayer } from './support/layers';

test('selects the current page and moves the selected books through a task chain', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);
    await importBookFromPath(page, anotherFixturePath);
    await addLayer(page, 'selected-target');
    await selectAllBooks(page);

    await page.getByLabel('Select hello', { exact: true }).check({ force: true });
    await page.getByLabel('Select another', { exact: true }).check({ force: true });
    const selectionToolbar = page.getByRole('toolbar', { name: 'Selected books actions' });
    await expect(selectionToolbar.getByText('2 selected')).toBeVisible();

    await selectionToolbar.getByRole('button', { name: 'Move', exact: true }).click();
    const moveDialog = page.getByRole('dialog', { name: 'Move selected books' });
    await moveDialog.getByLabel('Destination').selectOption('selected-target');
    await moveDialog.getByRole('button', { name: 'Move 2 books', exact: true }).click();

    const progressDialog = page.getByRole('dialog', { name: 'Move selected books' });
    await expect(progressDialog.getByText('2 books completed.')).toBeVisible();
    await progressDialog.getByRole('button', { name: 'Close', exact: true }).click();
    await expect(selectionToolbar).not.toBeVisible();

    await selectLayer(page, 'selected-target');
    await expect(page.getByRole('heading', { name: 'hello', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'another', exact: true })).toBeVisible();
  } finally {
    await server.dispose();
  }
});

test('keeps selection across view modes and clears it with Escape', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);
    await importBookFromPath(page, anotherFixturePath);
    await page.locator('.book-list-row', { hasText: 'hello' }).click({ modifiers: ['Meta'] });
    await page.locator('.book-list-row', { hasText: 'another' }).click({ modifiers: ['Shift'] });
    await expect(page.getByRole('toolbar', { name: 'Selected books actions' }).getByText('2 selected')).toBeVisible();

    await page.getByRole('button', { name: 'List', exact: true }).click();
    await page.getByRole('menuitemradio', { name: 'Card', exact: true }).click();
    await expect(page.locator('.book-card-view[aria-selected="true"]')).toHaveCount(2);

    await page.keyboard.press('Escape');
    await expect(page.getByRole('toolbar', { name: 'Selected books actions' })).not.toBeVisible();
    await expect(page).toHaveURL(/\/books/);
  } finally {
    await server.dispose();
  }
});

test('drags an unselected book alone and a selected group through the batch worker', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);
    await importBookFromPath(page, anotherFixturePath);
    await addLayer(page, 'drag-target');
    await selectAllBooks(page);

    await page.getByLabel('Select hello', { exact: true }).check({ force: true });
    const selectionToolbar = page.getByRole('toolbar', { name: 'Selected books actions' });
    const targetRow = layerRow(page, 'drag-target');
    const anotherRow = page.locator('.book-list-row', { hasText: 'another' });

    const singleTransfer = await emptyDataTransfer(page);
    try {
      await anotherRow.dispatchEvent('dragstart', { dataTransfer: singleTransfer });
      await targetRow.dispatchEvent('dragenter', { dataTransfer: singleTransfer });
      await targetRow.dispatchEvent('dragover', { dataTransfer: singleTransfer });
      await targetRow.dispatchEvent('drop', { dataTransfer: singleTransfer });
    } finally {
      await singleTransfer.dispose();
    }

    await expect(targetRow.locator('.sidebar-nav-count')).toHaveText('1');
    await expect(selectionToolbar.getByText('1 selected')).toBeVisible();

    await page.getByLabel('Select another', { exact: true }).check({ force: true });
    const helloRow = page.locator('.book-list-row', { hasText: 'hello' });
    const groupTransfer = await emptyDataTransfer(page);
    try {
      await helloRow.dispatchEvent('dragstart', { dataTransfer: groupTransfer });
      await targetRow.dispatchEvent('dragenter', { dataTransfer: groupTransfer });
      await targetRow.dispatchEvent('dragover', { dataTransfer: groupTransfer });
      await targetRow.dispatchEvent('drop', { dataTransfer: groupTransfer });
    } finally {
      await groupTransfer.dispose();
    }

    const progressDialog = page.getByRole('dialog', { name: 'Move selected books' });
    await expect(progressDialog.getByText('2 books completed.')).toBeVisible();
    await progressDialog.getByRole('button', { name: 'Close', exact: true }).click();
    await expect(selectionToolbar).not.toBeVisible();
    await expect(targetRow.locator('.sidebar-nav-count')).toHaveText('2');
  } finally {
    await server.dispose();
  }
});
