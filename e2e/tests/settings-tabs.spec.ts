import { expect, test, type Page } from '@playwright/test';
import { startServer } from './support/server';

const tabNames = ['Cover', 'Reading history', 'Reader', 'Import', 'About', 'Shelves'];

/**
 * Vertical distance between the bottom of the tab list and the top of the
 * active panel. Reka keeps inactive panels mounted, so a panel that does not
 * collapse leaves an empty row whose grid gap shows up here.
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
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/settings`);
    await expect(page.getByRole('heading', { name: 'Settings', level: 2 })).toBeVisible();

    const offsets: number[] = [];
    for (const name of tabNames) {
      await page.getByRole('tab', { name, exact: true }).click();
      await expect(page.locator('.settings-tab-content[data-state="active"] .panel').first()).toBeVisible();
      offsets.push(await panelOffset(page));
    }

    expect(offsets).toEqual(tabNames.map(() => offsets[0]));
  } finally {
    await server.dispose();
  }
});
