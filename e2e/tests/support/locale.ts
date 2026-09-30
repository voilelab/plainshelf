import type { Page } from '@playwright/test';

// Mirrors LOCALE_STORAGE_KEY in frontend/src/i18n/index.ts.
const LOCALE_STORAGE_KEY = 'plainshelf.locale';

/**
 * Forces the app into a locale before it boots.
 *
 * Seeding storage rather than driving the language switcher is deliberate. The
 * switcher is a reka-ui Select, so `selectOption()` does not work on it, its
 * options are portaled to the body, and its accessible name ("Language")
 * collides with the book-metadata form's own Language combobox — see the note
 * in metadata-edit.spec.ts. Seeding also exercises the persistence path that
 * a returning user actually takes.
 *
 * Playwright contexts start with empty storage and navigator.languages is
 * en-US, so every other spec boots in English without doing anything.
 *
 * Call before `goto`: addInitScript only applies to subsequent navigations.
 */
export async function useLocale(page: Page, locale: 'en' | 'zh-Hant'): Promise<void> {
  await page.addInitScript(
    ([key, value]) => {
      window.localStorage.setItem(key, value);
    },
    [LOCALE_STORAGE_KEY, locale] as const
  );
}

// The pattern the missing-key crawl matches on lives with the catalog it is
// derived from, so a new catalog section widens the guard on its own. Imported
// by relative path: the module and the catalog it reads have no imports of
// their own, so neither needs the frontend's `@/` alias to resolve here.
// Its own unit test is frontend/src/i18n/missingKeyPattern.test.ts.
export { MISSING_KEY_PATTERN } from '../../../frontend/src/i18n/missingKeyPattern';
