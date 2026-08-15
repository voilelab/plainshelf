import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { importHelloBook } from './support/books';
import {
  addMobileShelf,
  connectMobile,
  openMobileShelfEditor,
  reopenMobileAt,
  waitForMobileApp
} from './support/mobile';

// The device keeps its own list of shelves, each with its own source type —
// PlainShelf servers and pCloud folders side by side. These tests cover the
// list itself (features/mobile/pages/MobileShelvesPage.vue) and the sidebar
// picker that switches between entries (composables/useShelfPicker.ts), driven
// through `?mobile-shell-preview=1` like the other mobile specs.
//
// Only server entries appear here: a pCloud entry needs a real pCloud account
// and an OAuth approval in the system browser, so its persistence and cache
// scoping are covered by unit tests instead (providers/mobileConfig.test.ts).

test('keeps several shelves on the device and switches between them', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    // The first shelf goes through the empty-list form; the second through the
    // list page's own "Add a shelf" button.
    await connectMobile(page, server.baseUrl);
    await addMobileShelf(page, server.baseUrl, { name: 'Second shelf', shelfName: 'Default Shelf' });

    await reopenMobileAt(page, server.baseUrl, '/connect');
    const rows = page.locator('.mobile-shelves-item');
    await expect(rows).toHaveCount(2);
    // The entry just saved became the active one.
    await expect(rows.filter({ hasText: 'Second shelf' })).toContainText('In use');

    // Each entry shows the source it is read through, which is the whole point
    // of one list holding more than one kind.
    await expect(rows.first()).toContainText('PlainShelf server');

    // Switching back through the list restarts the app on the first shelf.
    await rows.first().getByRole('button', { name: 'Use this shelf' }).click();
    await expect(page).toHaveURL(/\/books(\?|$)/);
    await waitForMobileApp(page);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    await reopenMobileAt(page, server.baseUrl, '/connect');
    await expect(
      page.locator('.mobile-shelves-item').filter({ hasText: 'Second shelf' })
    ).not.toContainText('In use');
  } finally {
    await server.dispose();
  }
});

test('lists every saved shelf in the sidebar picker', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await connectMobile(page, server.baseUrl);
    await addMobileShelf(page, server.baseUrl, { name: 'Second shelf', shelfName: 'Default Shelf' });

    // The sidebar dropdown lists the device's shelves, not the server's — a
    // server cannot enumerate the pCloud folders or the other servers on it.
    await reopenMobileAt(page, server.baseUrl, '/books');
    await page.locator('.sidebar-shelf-select').click();
    await expect(page.getByRole('option', { name: /Second shelf/ })).toBeVisible();
    await expect(page.getByRole('option')).toHaveCount(2);
  } finally {
    await server.dispose();
  }
});

test('removes a shelf from the device', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await connectMobile(page, server.baseUrl);
    await addMobileShelf(page, server.baseUrl, { name: 'Second shelf', shelfName: 'Default Shelf' });

    await reopenMobileAt(page, server.baseUrl, '/connect');
    const second = page.locator('.mobile-shelves-item').filter({ hasText: 'Second shelf' });
    await second.getByRole('button', { name: 'Remove' }).click();

    // Removing takes the downloaded books with it, so it asks first.
    const confirm = page.locator('.mobile-shelves-confirm');
    await expect(confirm).toContainText('Second shelf');
    await confirm.getByRole('button', { name: 'Remove' }).click();

    // It was the active shelf, so the app restarts on the one that is left.
    await expect(page).toHaveURL(/\/books(\?|$)/);
    await waitForMobileApp(page);

    await reopenMobileAt(page, server.baseUrl, '/connect');
    await expect(page.locator('.mobile-shelves-item')).toHaveCount(1);
  } finally {
    await server.dispose();
  }
});

test('edits a saved shelf without adding a second one', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importHelloBook(page);

    await connectMobile(page, server.baseUrl);
    await openMobileShelfEditor(page, server.baseUrl);

    await page.getByLabel('Name').fill('Renamed shelf');
    await page.getByRole('button', { name: 'Save and continue' }).click();
    await expect(page).toHaveURL(/\/books(\?|$)/);
    await waitForMobileApp(page);

    await reopenMobileAt(page, server.baseUrl, '/connect');
    const rows = page.locator('.mobile-shelves-item');
    await expect(rows).toHaveCount(1);
    await expect(rows.first()).toContainText('Renamed shelf');
  } finally {
    await server.dispose();
  }
});
