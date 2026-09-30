import { expect, test } from '@playwright/test';
import { useServer } from './support/server';
import { helloFixturePath, importBookAs } from './support/books';
import { MISSING_KEY_PATTERN, useLocale } from './support/locale';

const getServer = useServer();

// t() returns the key itself on a miss — no throw, no warning — so a typo'd or
// not-yet-added key reaches the screen as a literal dotted path. That failure
// is invisible to the catalog tests, because the catalogs are fine; it is the
// reference that is wrong.
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
