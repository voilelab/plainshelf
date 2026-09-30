import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { addMobileShelf, connectMobile, reopenMobileAt, waitForMobileApp } from './support/mobile';

// The device keeps its own list of shelves, each with its own source type —
// PlainShelf servers and pCloud folders side by side. These tests cover the
// list itself (features/mobile/pages/MobileShelvesPage.vue) and the sidebar
// picker that switches between entries (composables/useShelfPicker.ts), driven
// through `?mobile-shell-preview=1` like the other mobile specs.
//
// Only server entries appear here: a pCloud entry needs a real pCloud account
// and an OAuth approval in the system browser, so its persistence and cache
// scoping are covered by unit tests instead (providers/mobileConfig.test.ts).
//
// These tests assert only on the device's own shelf list (per-test browser
// storage) and never on library books, so the shared server shelf can stay
// empty — no book import is needed.

const getServer = useServer();

test('keeps several shelves on the device and switches between them', async ({ page }) => {
  const { baseUrl } = getServer();

  // The first shelf goes through the empty-list form; the second through the
  // list page's own "Add a shelf" button.
  await connectMobile(page, baseUrl);
  await addMobileShelf(page, baseUrl, { name: 'Second shelf', shelfName: 'Default Shelf' });

  await reopenMobileAt(page, baseUrl, '/connect');
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

  await reopenMobileAt(page, baseUrl, '/connect');
  await expect(
    page.locator('.mobile-shelves-item').filter({ hasText: 'Second shelf' })
  ).not.toContainText('In use');
});
