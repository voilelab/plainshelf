import { expect, test } from '@playwright/test';
import { startServer, useServer } from './support/server';
import { helloFixturePath, importBookAs, importBookFromPath } from './support/books';
import { MISSING_KEY_PATTERN, useLocale } from './support/locale';

const getServer = useServer();

// Every other spec runs in English, so nothing until now proved the UI actually
// reads from the zh-Hant catalog. locales.test.ts proves the two catalogs agree
// with each other, which is a different claim: a screen with the string welded
// into the template passes it, and so does a t() call naming a key that does
// not exist.
test('the shared chrome renders from the zh-Hant catalog', async ({ page }) => {
  const { baseUrl } = getServer();

  // Import first, in English: the import dialog is not translated yet, so
  // driving it in zh-Hant would couple this spec to work that has not landed.
  // The shelf is shared across this file's tests, so each imports under its own
  // unique title.
  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'locale-chrome');

  // addInitScript only affects later navigations, hence the reload.
  await useLocale(page, 'zh-Hant');
  await page.reload();

  await expect(page.locator('html')).toHaveAttribute('lang', 'zh-Hant');

  // FolderTree: the nav's accessible name and the root button both used to be
  // welded in as English.
  const foldersNav = page.getByRole('navigation', { name: '資料夾', exact: true });
  await expect(foldersNav).toBeVisible();
  await expect(foldersNav.getByRole('button', { name: '所有書籍', exact: true })).toBeVisible();

  // Pagination's edge buttons are icon-only, so their names are the only
  // thing a screen reader gets.
  await expect(page.getByRole('button', { name: '第一頁', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '最後一頁', exact: true })).toBeVisible();

  // The former "missing author" maintenance page is now a book-list filter:
  // an old /books/maintenance/missing-author link redirects to the library
  // filtered by ?author=none, whose toolbar renders from the zh-Hant catalog.
  await page.goto(`${baseUrl}/books/maintenance/missing-author`);
  // author=none can sit anywhere in the query once LibraryPage normalizes
  // page/sort/order in, so match it position-independently.
  await expect(page).toHaveURL(/\/books\?[^#]*author=none/);
  await expect(page.getByRole('button', { name: '搜尋', exact: true })).toBeVisible();
});

// The batch-trash dialog omits cancel-text and close-label, so both come from
// ConfirmModal's own defaults. Those defaults were English literals in
// withDefaults, which is evaluated once where the component is defined — a t()
// call there would have frozen the English string rather than following the
// locale, so this asserts the computed fallback, not just the translation.
test('ConfirmModal falls back to translated defaults', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'locale-confirm');

  await useLocale(page, 'zh-Hant');
  await page.reload();

  await page.getByLabel('選取「locale-confirm」', { exact: true }).check({ force: true });
  const selectionToolbar = page.getByRole('toolbar', { name: '已選書籍操作' });
  await selectionToolbar.getByRole('button', { name: '移到垃圾桶', exact: true }).click();

  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('button', { name: '取消', exact: true })).toBeVisible();
  await expect(dialog.getByRole('button', { name: '關閉確認對話框', exact: true })).toBeVisible();
});

// The book-language labels were hardcoded Traditional Chinese, so they showed
// Chinese in the English UI and could not follow a switch in either direction.
// metadata-edit.spec.ts covers the English side; this covers zh-Hant.
test('book language labels follow the locale', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'locale-lang');

  await useLocale(page, 'zh-Hant');
  await page.reload();

  await page.locator('.book-list-row').getByRole('heading', { name: 'locale-lang', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  // Straight to the edit route rather than through the More menu: that menu
  // belongs to a later pass, so driving it would couple this test to strings
  // that have not moved yet.
  await page.goto(`${page.url()}/edit`);

  // By role, not by label: the field's accessible name now comes from the
  // translated label, and getByLabel matches substrings — it was quietly
  // matching "language tag" inside the help text below the field instead.
  const languageTrigger = page.locator('.edit-form').getByRole('combobox');
  await languageTrigger.click();
  await page.getByRole('option', { name: '英文', exact: true }).click();
  await expect(languageTrigger).toHaveText('英文');
});

// A validation message shown on screen has to follow a locale switch like
// everything around it. This one is derived from a flag rather than stored as
// text, and this drives the switcher in place — seeding storage would reload
// and clear the error before it could be observed.
test('a shown validation error follows an in-place locale switch', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'locale-validation');

  await page.locator('.book-list-row').getByRole('heading', { name: 'locale-validation', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  await page.goto(`${page.url()}/edit`);

  const languageTrigger = page.locator('.edit-form').getByRole('combobox');
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
});

// The whole source editor bypassed the catalog — the densest cluster of
// hardcoded strings in the app, and the one a zh-Hant reader would notice
// first, since editing sources is where the English used to be unavoidable.
test('the source editor renders from the zh-Hant catalog', async ({ page }) => {
  const { baseUrl } = getServer();

  await page.goto(`${baseUrl}/books`);
  await importBookAs(page, helloFixturePath, 'locale-source');

  await page.locator('.book-list-row').getByRole('heading', { name: 'locale-source', exact: true }).click();
  await expect(page).toHaveURL(/\/books\/[^/]+$/);
  // Straight to the route: the More menu is part of a later pass.
  await page.goto(`${page.url()}/sources`);
  await expect(page.getByText('No pending changes').first()).toBeVisible();

  await useLocale(page, 'zh-Hant');
  await page.reload();

  // SourceList, the editor status line, and the find/replace group each came
  // from a different file.
  await expect(page.getByRole('heading', { name: '來源', exact: true })).toBeVisible();
  await expect(page.getByText('沒有待儲存的變更').first()).toBeVisible();
  await expect(page.getByRole('group', { name: '尋找與取代' })).toBeVisible();
  await expect(page.getByRole('button', { name: '全部取代', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '儲存', exact: true })).toBeVisible();

  // The find status is built from a state descriptor rather than a stored
  // sentence, so it has to come out translated too.
  await page.getByRole('textbox', { name: '尋找' }).fill('zzz-no-such-text');
  await page.getByRole('button', { name: '下一個', exact: true }).click();
  await expect(page.getByText('沒有符合的結果。')).toBeVisible();
});

// The library's own forms were the last cluster: the metadata editor, the
// import dialog and the duplicate-content page. The import dialog also used to
// render its per-file status as the raw enum token.
// per-test server: the duplicate-content page asserts the translated
// no-duplicates empty state, a whole-library claim. A shared shelf accumulates
// this file's other hello.txt imports, which are byte-identical and so register
// as duplicate content — the empty state would never show. A pristine shelf per
// run keeps that assertion meaningful.
test('the library forms render from the zh-Hant catalog', async ({ page }) => {
  const server = await startServer();

  try {
    await page.goto(`${server.baseUrl}/books`);
    await importBookFromPath(page, helloFixturePath);

    await useLocale(page, 'zh-Hant');
    await page.reload();

    await page.locator('.book-list-row').getByRole('heading', { name: 'hello', exact: true }).click();
    await expect(page).toHaveURL(/\/books\/[^/]+$/);
    await page.goto(`${page.url()}/edit`);

    await expect(page.getByRole('heading', { name: '編輯中繼資料' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: '書名' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: '儲存中繼資料' })).toBeVisible();
    await expect(page.getByRole('button', { name: '新增識別碼' })).toBeVisible();

    await page.goto(`${server.baseUrl}/duplicates`);
    await expect(page.getByRole('heading', { name: '重複內容' })).toBeVisible();
    await expect(page.getByText('沒有發現重複內容。')).toBeVisible();
  } finally {
    await server.dispose();
  }
});

// The guard below is only as good as this pattern, and an earlier version of it
// required two dots — which silently excluded every two-segment key, half of
// what it was written to catch. Pinning both directions keeps that from
// happening again without anyone noticing.
test('the missing-key pattern matches keys and not ordinary content', () => {
  for (const key of [
    'pagination.firstPage',
    'common.confirm',
    'layout.foldersNavLabel',
    'settings.shelves.idColumn',
    'notFound.title'
  ]) {
    expect(key, `${key} should look like a missing key`).toMatch(MISSING_KEY_PATTERN);
  }

  // A book library renders filenames as titles; those are content, not bugs.
  for (const text of ['hello.txt', 'notes.md', 'book.epub', 'Settings', 'No books yet.']) {
    expect(text, `${text} should not look like a missing key`).not.toMatch(MISSING_KEY_PATTERN);
  }
});

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
