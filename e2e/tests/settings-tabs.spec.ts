import { expect, test, type Page } from '@playwright/test';
import { useServer } from './support/server';

const getServer = useServer();

const tabNames = ['Cover', 'Reading history', 'Import', 'About', 'Shelves'];

/**
 * Vertical distance between the bottom of the tab list and the top of the
 * active panel. Reka keeps every tab panel's wrapper in the DOM, so a panel
 * that does not collapse leaves a row whose grid gap shows up here.
 */
async function panelOffset(page: Page): Promise<number> {
  return page.evaluate(() => {
    const list = document.querySelector('.settings-tabs-list');
    const panel = document.querySelector('.settings-tab-content[data-state="active"]');
    if (!list || !panel) {
      throw new Error('Settings tabs are not rendered.');
    }
    return Math.round(panel.getBoundingClientRect().top - list.getBoundingClientRect().bottom);
  });
}

test('every settings tab starts at the same offset below the tab list', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/settings`);
  await expect(page.getByRole('heading', { name: 'Settings', level: 2 })).toBeVisible();

  const offsets: number[] = [];
  for (const name of tabNames) {
    await page.getByRole('tab', { name, exact: true }).click();
    await expect(page.locator('.settings-tab-content[data-state="active"] .panel').first()).toBeVisible();
    offsets.push(await panelOffset(page));
  }

  expect(offsets).toEqual(tabNames.map(() => offsets[0]));
});

/**
 * Each tab body is its own component, and several of them own state that has to
 * outlive a tab switch: a book-cache export in flight, the fetched shelf list,
 * the font-license cache. Reka unmounts an inactive tab's content unless the
 * root opts out, which would silently reset that state and refetch on every
 * visit, so the panels have to stay mounted behind the tab list.
 */
test('tab panels stay mounted and do not refetch when the tab changes', async ({ page }) => {
  const { baseUrl } = getServer();

  const requestCounts = new Map<string, number>();
  page.on('request', (request) => {
    const path = new URL(request.url()).pathname;
    if (path === '/api/version' || path === '/api/shelves') {
      requestCounts.set(path, (requestCounts.get(path) ?? 0) + 1);
    }
  });

  await page.goto(`${baseUrl}/settings`);
  await expect(page.getByRole('heading', { name: 'Settings', level: 2 })).toBeVisible();
  await expect(page.locator('.settings-tab-content[data-state="active"] .panel').first()).toBeVisible();

  // An inactive panel keeps its content; it is only hidden. Counted rather
  // than checked for visibility, since the panels collapse to display: none.
  const inactivePanels = page.locator('.settings-tab-content[data-state="inactive"] .panel');
  expect(await inactivePanels.count()).toBeGreaterThan(0);

  for (const name of ['About', 'Shelves', 'About', 'Shelves']) {
    await page.getByRole('tab', { name, exact: true }).click();
    await expect(page.locator('.settings-tab-content[data-state="active"] .panel').first()).toBeVisible();
  }

  // Both loads belong to the page, not to a tab visit.
  expect(requestCounts.get('/api/version')).toBe(1);
  expect(requestCounts.get('/api/shelves')).toBe(1);
});
