import { expect, test, type Page } from '@playwright/test';
import { useServer } from './support/server';

const getServer = useServer();

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
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
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
});

test('rail icons navigate and reflect the active route', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  await page.getByRole('button', { name: 'Collapse sidebar' }).click();

  const trashLink = railNav(page).getByRole('link', { name: 'Trash' });
  await trashLink.click();
  await expect(page).toHaveURL(/\/trash$/);
  await expect(trashLink).toHaveClass(/active/);
});

test('rail mode persists across a reload', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  await page.getByRole('button', { name: 'Collapse sidebar' }).click();
  await expect(railNav(page)).toBeVisible();

  await page.reload();
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  await expect(railNav(page)).toBeVisible();
  await expectSidebarWidth(page, (width) => Math.abs(width - railWidth) <= 2);
});

test('restores the dragged expanded width after railing and reloading', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  const handle = page.locator('.reka-resize-handle');
  const handleBox = await handle.boundingBox();
  expect(handleBox).not.toBeNull();
  await page.mouse.move(handleBox!.x + handleBox!.width / 2, handleBox!.y + handleBox!.height / 2);
  await page.mouse.down();
  await page.mouse.move(handleBox!.x + 45, handleBox!.y + handleBox!.height / 2, { steps: 10 });
  await page.mouse.up();

  await expectSidebarWidth(page, (width) => width > 260);
  const draggedWidth = (await sidebar(page).boundingBox())!.width;

  await page.getByRole('button', { name: 'Collapse sidebar' }).click();
  await expect(railNav(page)).toBeVisible();

  await page.reload();
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();
  await expect(railNav(page)).toBeVisible();

  await page.getByRole('button', { name: 'Expand sidebar' }).click();
  await expectSidebarWidth(page, (width) => Math.abs(width - draggedWidth) <= 2);
});

test('narrow-viewport drawer shows the full sidebar even in rail mode', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  await page.getByRole('button', { name: 'Collapse sidebar' }).click();
  await expect(railNav(page)).toBeVisible();

  await page.setViewportSize({ width: 700, height: 800 });
  await expect(railNav(page)).toHaveCount(0);

  await page.getByRole('button', { name: 'Open menu' }).click();
  await expect(sidebar(page).getByRole('button', { name: 'Toggle sidebar folders', exact: true })).toBeVisible();

  await page.setViewportSize({ width: 1280, height: 800 });
  await expect(railNav(page)).toBeVisible();
});
