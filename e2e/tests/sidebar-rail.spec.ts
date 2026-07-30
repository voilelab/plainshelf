import { expect, test, type Page } from '@playwright/test';
import { startServer } from './support/server';

const railWidth = 48;

function sidebar(page: Page) {
  return page.locator('aside.sidebar');
}

function railNav(page: Page) {
  return page.getByRole('navigation', { name: 'Sidebar navigation' });
}

async function expectSidebarWidth(page: Page, predicate: (width: number) => boolean): Promise<void> {
  await expect(async () => {
    const box = await sidebar(page).boundingBox();
    expect(box).not.toBeNull();
    expect(predicate(box!.width)).toBe(true);
  }).toPass();
}

test('sidebar toggles between expanded mode and a 48px icon rail', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    await expect(railNav(page)).toHaveCount(0);
    await expectSidebarWidth(page, (width) => width >= 200);

    await page.getByRole('button', { name: 'Collapse sidebar' }).click();

    await expect(railNav(page)).toBeVisible();
    await expect(railNav(page).getByRole('link')).toHaveCount(5);
    await expect(page.locator('.sidebar-inner')).toHaveCount(0);
    await expectSidebarWidth(page, (width) => Math.abs(width - railWidth) <= 2);

    await page.getByRole('button', { name: 'Expand sidebar' }).click();

    await expect(railNav(page)).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Toggle sidebar folders', exact: true })).toBeVisible();
    await expectSidebarWidth(page, (width) => width >= 200);
  } finally {
    await server.dispose();
  }
});

test('rail icons navigate and reflect the active route', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    await page.getByRole('button', { name: 'Collapse sidebar' }).click();

    const trashLink = railNav(page).getByRole('link', { name: 'Trash' });
    await trashLink.click();
    await expect(page).toHaveURL(/\/trash$/);
    await expect(trashLink).toHaveClass(/active/);
  } finally {
    await server.dispose();
  }
});

test('rail mode persists across a reload', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    await page.getByRole('button', { name: 'Collapse sidebar' }).click();
    await expect(railNav(page)).toBeVisible();

    await page.reload();
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
    await expect(railNav(page)).toBeVisible();
    await expectSidebarWidth(page, (width) => Math.abs(width - railWidth) <= 2);
  } finally {
    await server.dispose();
  }
});

test('narrow-viewport drawer shows the full sidebar even in rail mode', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    await page.getByRole('button', { name: 'Collapse sidebar' }).click();
    await expect(railNav(page)).toBeVisible();

    await page.setViewportSize({ width: 700, height: 800 });
    await expect(railNav(page)).toHaveCount(0);

    await page.getByRole('button', { name: 'Open menu' }).click();
    await expect(sidebar(page).getByRole('button', { name: 'Toggle sidebar folders', exact: true })).toBeVisible();

    await page.setViewportSize({ width: 1280, height: 800 });
    await expect(railNav(page)).toBeVisible();
  } finally {
    await server.dispose();
  }
});
