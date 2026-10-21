import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { addMobileShelf, connectMobile, reopenMobileAt, waitForMobileApp } from './support/mobile';


const getServer = useServer();

test('keeps several shelves on the device and switches between them', async ({ page }) => {
  const { baseUrl } = getServer();

  await connectMobile(page, baseUrl);
  await addMobileShelf(page, baseUrl, { name: 'Second shelf', shelfName: 'Default Shelf' });

  await reopenMobileAt(page, baseUrl, '/connect');
  const rows = page.locator('.mobile-shelves-item');
  await expect(rows).toHaveCount(2);
  await expect(rows.filter({ hasText: 'Second shelf' })).toContainText('In use');

  await expect(rows.first()).toContainText('PlainShelf server');

  await rows.first().getByRole('button', { name: 'Use this shelf' }).click();
  await expect(page).toHaveURL(/\/books(\?|$)/);
  await waitForMobileApp(page);
  await expect(page.getByRole('heading', { name: 'All books' })).toBeVisible();

  await reopenMobileAt(page, baseUrl, '/connect');
  await expect(
    page.locator('.mobile-shelves-item').filter({ hasText: 'Second shelf' })
  ).not.toContainText('In use');
});
