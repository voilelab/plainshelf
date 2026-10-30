import { expect, type Page } from '@playwright/test';

// HTML Standard Web Storage is shared by same-origin pages even when noopener severs the opener relationship.
export async function openReaderTab(
  page: Page,
  open: () => Promise<void>,
  urlPattern: RegExp = /\/reader\/[^/]+$/
): Promise<Page> {
  const [reader] = await Promise.all([page.context().waitForEvent('page'), open()]);
  await reader.waitForLoadState('domcontentloaded');
  await expect(reader).toHaveURL(urlPattern);
  return reader;
}
