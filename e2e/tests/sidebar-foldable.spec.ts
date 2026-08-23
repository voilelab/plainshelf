import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { addFolder, foldersNav, foldersSectionToggle } from './support/folders';

const foldableSections = [
  { name: 'Toggle sidebar folders', controlledId: 'sidebar-section-folders' },
  { name: 'Toggle sidebar history', controlledId: 'sidebar-section-reading' },
  { name: 'Toggle sidebar maintenance', controlledId: 'sidebar-section-maintenance' },
  { name: 'Toggle sidebar administration', controlledId: 'sidebar-section-admin' }
] as const;

test('sidebar sections collapse and expand their contents', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    for (const section of foldableSections) {
      const toggle = page.getByRole('button', { name: section.name, exact: true });
      const content = page.locator(`#${section.controlledId}`);

      await expect(toggle).toHaveAttribute('aria-controls', section.controlledId);
      await expect(toggle).toHaveAttribute('aria-expanded', 'true');
      await expect(content).toBeVisible();

      await toggle.click();
      await expect(toggle).toHaveAttribute('aria-expanded', 'false');
      await expect(content).toBeHidden();

      await toggle.click();
      await expect(toggle).toHaveAttribute('aria-expanded', 'true');
      await expect(content).toBeVisible();
    }
  } finally {
    await server.dispose();
  }
});

test('folder helpers still work after the foldable Folders section is toggled', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    await foldersSectionToggle(page).click();
    await expect(foldersNav(page)).toBeHidden();

    await addFolder(page, 'foldable');
    await expect(foldersNav(page).getByRole('button', { name: 'foldable', exact: true })).toBeVisible();
  } finally {
    await server.dispose();
  }
});
