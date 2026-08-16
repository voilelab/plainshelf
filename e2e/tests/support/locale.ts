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

/**
 * Matches the dotted path a missing key renders as. `t()` returns the key
 * itself when a lookup misses (i18n/index.ts) — it does not throw and does not
 * warn — so a typo'd key reaches the screen as literal `library.charCount.mn`.
 * Unit tests cannot see this: the catalogs are consistent, the *reference* is
 * wrong.
 */
export const MISSING_KEY_PATTERN = /\b[a-z][a-zA-Z]*(\.[a-z][a-zA-Z]*){2,}\b/;
