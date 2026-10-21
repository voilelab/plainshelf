import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { helloFixturePath, importBookAs } from './support/books';
import { MISSING_KEY_PATTERN, useLocale } from './support/locale';

const getServer = useServer();

test('no missing-key paths leak into the rendered page', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'locale-nomiss');

  await useLocale(page, 'zh-Hant');

  for (const route of ['/books', '/home', '/read-history', '/settings', '/trash']) {
    await page.goto(`${baseUrl}${route}`);
    const text = await page.locator('main, .page-area').first().innerText();
    expect(text, `missing i18n key rendered on ${route}`).not.toMatch(MISSING_KEY_PATTERN);
  }
});
