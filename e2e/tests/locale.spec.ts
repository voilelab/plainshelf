import { expect, test } from '@playwright/test';
import { startServer } from './support/server';
import { helloFixturePath, importBookFromPath } from './support/books';
import { MISSING_KEY_PATTERN, useLocale } from './support/locale';

// Every other spec runs in English, so nothing until now proved the UI actually
// reads from the zh-Hant catalog. locales.test.ts proves the two catalogs agree
// with each other, which is a different claim: a screen with the string welded
// into the template passes it, and so does a t() call naming a key that does
// not exist.
test('the shared chrome renders from the zh-Hant catalog', async ({ page }) => {
  const server = await startServer();

  try {
    // Import first, in English: the import dialog is not translated yet, so
    // driving it in zh-Hant would couple this spec to work that has not landed.
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);

    // addInitScript only affects later navigations, hence the reload.
    await useLocale(page, 'zh-Hant');
    await page.reload();

    await expect(page.locator('html')).toHaveAttribute('lang', 'zh-Hant');

    // LayerTree: the nav's accessible name and the root button both used to be
    // welded in as English.
    const layersNav = page.getByRole('navigation', { name: '資料夾', exact: true });
    await expect(layersNav).toBeVisible();
    await expect(layersNav.getByRole('button', { name: '所有書籍', exact: true })).toBeVisible();

    // Pagination's edge buttons are icon-only, so their names are the only
    // thing a screen reader gets.
    await expect(page.getByRole('button', { name: '第一頁', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: '最後一頁', exact: true })).toBeVisible();

    // BookListView's per-row Edit action. Only the maintenance pages pass
    // show-edit-action, so the library list never renders it — the fixture has
    // no author, which is what puts it on this page.
    await page.goto(`${server.baseUrl}/books/maintenance/missing-author`);
    await expect(page.getByRole('button', { name: '編輯', exact: true })).toBeVisible();
  } finally {
    await server.dispose();
  }
});

// The batch-trash dialog omits cancel-text and close-label, so both come from
// ConfirmModal's own defaults. Those defaults were English literals in
// withDefaults, which is evaluated once where the component is defined — a t()
// call there would have frozen the English string rather than following the
// locale, so this asserts the computed fallback, not just the translation.
test('ConfirmModal falls back to translated defaults', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);

    await useLocale(page, 'zh-Hant');
    await page.reload();

    await page.getByLabel('選取「hello」', { exact: true }).check({ force: true });
    const selectionToolbar = page.getByRole('toolbar', { name: '已選書籍操作' });
    await selectionToolbar.getByRole('button', { name: '移到垃圾桶', exact: true }).click();

    const dialog = page.getByRole('dialog');
    await expect(dialog.getByRole('button', { name: '取消', exact: true })).toBeVisible();
    await expect(dialog.getByRole('button', { name: '關閉確認對話框', exact: true })).toBeVisible();
  } finally {
    await server.dispose();
  }
});

// The book-language labels were hardcoded Traditional Chinese, so they showed
// Chinese in the English UI and could not follow a switch in either direction.
// metadata-edit.spec.ts covers the English side; this covers zh-Hant.
test('book language labels follow the locale', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);

    await useLocale(page, 'zh-Hant');
    await page.reload();

    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    // Straight to the edit route rather than through the More menu: that menu
    // belongs to a later pass, so driving it would couple this test to strings
    // that have not moved yet.
    await page.goto(`${page.url()}/edit`);

    const languageTrigger = page.locator('.edit-form').getByLabel('Language');
    await languageTrigger.click();
    await page.getByRole('option', { name: '英文', exact: true }).click();
    await expect(languageTrigger).toHaveText('英文');
  } finally {
    await server.dispose();
  }
});

// A validation message shown on screen has to follow a locale switch like
// everything around it. This one is derived from a flag rather than stored as
// text, and this drives the switcher in place — seeding storage would reload
// and clear the error before it could be observed.
test('a shown validation error follows an in-place locale switch', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);

    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.goto(`${page.url()}/edit`);

    const languageTrigger = page.locator('.edit-form').getByLabel('Language');
    await languageTrigger.click();
    await page.getByRole('option', { name: 'Custom...', exact: true }).click();
    await page.getByPlaceholder('e.g. zh-TW, zh-HK, fr, de').fill('not a tag');
    await page.getByRole('button', { name: 'Save metadata' }).click();

    const invalidEn = 'That is not a valid language tag. Use a form like en, ja, zh-Hant or zh-TW.';
    await expect(page.getByText(invalidEn)).toBeVisible();

    // The UI-language switcher lives in the topbar; scope past the edit form's
    // own Language combobox. The option label is an endonym, so it reads the
    // same whichever locale you start from.
    await page.locator('.language-select').getByRole('combobox').click();
    await page.getByRole('option', { name: '繁體中文', exact: true }).click();

    await expect(page.getByText('語言格式不正確，請使用 en、ja、zh-Hant、zh-TW 這類格式。')).toBeVisible();
    await expect(page.getByText(invalidEn)).toHaveCount(0);
  } finally {
    await server.dispose();
  }
});

// t() returns the key itself on a miss — no throw, no warning — so a typo'd or
// not-yet-added key reaches the screen as a literal dotted path. That failure
// is invisible to the catalog tests, because the catalogs are fine; it is the
// reference that is wrong.
test('no missing-key paths leak into the rendered page', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);

    await useLocale(page, 'zh-Hant');

    for (const route of ['/books', '/dashboard', '/read-history', '/settings', '/trash']) {
      await page.goto(`${server.baseUrl}${route}`);
      const text = await page.locator('main, .page-area').first().innerText();
      expect(text, `missing i18n key rendered on ${route}`).not.toMatch(MISSING_KEY_PATTERN);
    }
  } finally {
    await server.dispose();
  }
});
