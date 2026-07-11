import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { addLayer, layersNav, layersSectionToggle } from './support/layers';

const foldableSections = [
  { name: 'LAYERS', controlledId: 'sidebar-section-layers' },
  { name: 'READING', controlledId: 'sidebar-section-reading' },
  { name: 'MAINTENANCE', controlledId: 'sidebar-section-maintenance' },
  { name: 'ADMIN', controlledId: 'sidebar-section-admin' }
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

test('layer helpers still work after the foldable Layers section is toggled', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

    await layersSectionToggle(page).click();
    await expect(layersNav(page)).toBeHidden();

    await addLayer(page, 'foldable');
    await expect(layersNav(page).getByRole('button', { name: 'foldable', exact: true })).toBeVisible();
  } finally {
    await server.dispose();
  }
});
