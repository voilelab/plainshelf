import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
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

test('lists every saved shelf in the sidebar picker', async ({ page }) => {
  const { baseUrl } = getServer();

  await connectMobile(page, baseUrl);
  await addMobileShelf(page, baseUrl, { name: 'Second shelf', shelfName: 'Default Shelf' });

  // The sidebar dropdown lists the device's shelves, not the server's — a
  // server cannot enumerate the pCloud folders or the other servers on it.
  await reopenMobileAt(page, baseUrl, '/books');
  await page.locator('.sidebar-shelf-select').click();
  await expect(page.getByRole('option', { name: /Second shelf/ })).toBeVisible();
  await expect(page.getByRole('option')).toHaveCount(2);
});

test('removes a shelf from the device', async ({ page }) => {
  const { baseUrl } = getServer();

  await connectMobile(page, baseUrl);
  await addMobileShelf(page, baseUrl, { name: 'Second shelf', shelfName: 'Default Shelf' });

  await reopenMobileAt(page, baseUrl, '/connect');
  const second = page.locator('.mobile-shelves-item').filter({ hasText: 'Second shelf' });
  await second.getByRole('button', { name: 'Remove' }).click();

  // Removing takes the downloaded books with it, so it asks first.
  const confirm = page.locator('.mobile-shelves-confirm');
  await expect(confirm).toContainText('Second shelf');
  await confirm.getByRole('button', { name: 'Remove' }).click();

  // It was the active shelf, so the app restarts on the one that is left.
  await expect(page).toHaveURL(/\/books(\?|$)/);
  await waitForMobileApp(page);

  await reopenMobileAt(page, baseUrl, '/connect');
  await expect(page.locator('.mobile-shelves-item')).toHaveCount(1);
});

test('edits a saved shelf without adding a second one', async ({ page }) => {
  const { baseUrl } = getServer();

  await connectMobile(page, baseUrl);
  await openMobileShelfEditor(page, baseUrl);

  await page.getByLabel('Name').fill('Renamed shelf');
  await page.getByRole('button', { name: 'Save and continue' }).click();
  await expect(page).toHaveURL(/\/books(\?|$)/);
  await waitForMobileApp(page);

  await reopenMobileAt(page, baseUrl, '/connect');
  const rows = page.locator('.mobile-shelves-item');
  await expect(rows).toHaveCount(1);
  await expect(rows.first()).toContainText('Renamed shelf');
});

// The setup routes sit outside MainLayout, so its .main-scroll wrapper is not
// there to absorb a form taller than the screen: the page root has to scroll
// itself (features/mobile/styles/mobile-connect.css). It did not, and the
// global `html, body { overflow: hidden }` simply cut the form off at the
// bottom of a phone screen — Save included, with no way to reach it.
test('scrolls the shelf editor when it is taller than the screen', async ({ page }) => {
  const { baseUrl } = getServer();

  await connectMobile(page, baseUrl);

  // Phone-sized, so the form genuinely overflows. The pCloud variant is the
  // tallest one, but it needs a real OAuth approval; the server form
  // overflows too at this height, which is all this asserts about.
  await page.setViewportSize({ width: 360, height: 480 });
  await openMobileShelfEditor(page, baseUrl);

  const shell = page.locator('.mobile-connect');
  await expect
    .poll(async () => shell.evaluate((el) => el.scrollHeight - el.clientHeight))
    .toBeGreaterThan(0);

  // The Save button is what the user was actually locked out of.
  const save = page.getByRole('button', { name: 'Save and continue' });
  await save.scrollIntoViewIfNeeded();
  await expect(save).toBeInViewport();
});
