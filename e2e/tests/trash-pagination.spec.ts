import { expect, test, type Page } from '@playwright/test';
import { startServer } from './support/server';

const TRASHED_COUNT = 25;
const PAGE_SIZE = 10;

function trashedBooks(count: number) {
  return Array.from({ length: count }, (_, index) => {
    const label = String(index + 1).padStart(2, '0');
    return {
      id: `trashed-${label}`,
      title: `Trashed ${label}`,
      authors: ['E2E'],
      original_layer: [],
      original_path: `books/trashed-${label}`,
      deleted_at: '2026-01-01T00:00:00Z'
    };
  });
}

function titlesOn(page: Page) {
  return page.locator('.trash-table tbody tr td:first-child');
}

test('should paginate the trash listing and keep the page in the URL', async ({ page }) => {
  const server = await startServer();

  try {
    // The listing has no server-side pagination and the shelf would need 25 real
    // imports to fill three pages, so stub the response instead of seeding it.
    await page.route('**/api/shelves/*/trash/books**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(trashedBooks(TRASHED_COUNT))
      });
    });

    // Trash shares the library's per-page setting; pin it so the page count is
    // independent of the default.
    await page.addInitScript((size) => {
      window.localStorage.setItem('plainshelf.books.pageSize', String(size));
    }, PAGE_SIZE);

    await page.goto(`${server.baseUrl}/trash`);
    await expect(page.getByRole('heading', { name: 'Trash' })).toBeVisible();

    // First page: only the page window is rendered, the summary counts all items.
    await expect(titlesOn(page)).toHaveCount(PAGE_SIZE);
    await expect(titlesOn(page).first()).toHaveText('Trashed 01');
    await expect(titlesOn(page).last()).toHaveText('Trashed 10');
    await expect(page.getByText('Page 1 / 3')).toBeVisible();

    // Paging forward moves the window and records the page in the URL.
    await page.getByRole('button', { name: 'Next Page' }).click();
    await expect(page).toHaveURL(/\/trash\?(.*&)?page=2/);
    await expect(titlesOn(page).first()).toHaveText('Trashed 11');
    await expect(titlesOn(page).last()).toHaveText('Trashed 20');

    // A deep-linked page survives a fresh load: the listing arrives after mount,
    // so the page must not be clamped against the still-empty list.
    await page.goto(`${server.baseUrl}/trash?page=3`);
    await expect(page.getByText('Page 3 / 3')).toBeVisible();
    await expect(titlesOn(page)).toHaveCount(TRASHED_COUNT - 2 * PAGE_SIZE);
    await expect(titlesOn(page).first()).toHaveText('Trashed 21');
  } finally {
    await server.dispose();
  }
});
