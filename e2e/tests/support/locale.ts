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

// The top-level sections of frontend/src/i18n/locales/en.ts. Anchoring on these
// is what lets the pattern below accept a two-segment key without also matching
// every filename on screen — this is a book library, so `notes.md` as a title is
// ordinary content, not a bug.
//
// Add a section here when one is added to the catalog. Forgetting only narrows
// the guard for that section; it cannot produce a false failure.
const CATALOG_SECTIONS = [
  'app',
  'language',
  'common',
  'layout',
  'dashboard',
  'settings',
  'adminLogs',
  'maintenance',
  'library',
  'bookDetail',
  'bookCollection',
  'pagination',
  'deleteModal',
  'readHistory',
  'trash',
  'downloads',
  'reader',
  'mobileConnect',
  'mobileShelves',
  'notFound'
];

/**
 * Matches the dotted path a missing key renders as. `t()` returns the key
 * itself when a lookup misses (i18n/index.ts) — it does not throw and does not
 * warn — so a typo'd key reaches the screen as literal `pagination.firstPag`.
 * Unit tests cannot see this: the catalogs are consistent, the *reference* is
 * wrong.
 *
 * Two segments is the floor, not three: plenty of real keys are
 * `common.confirm` shaped, and an earlier version of this pattern required two
 * dots and so missed half the keys it was written to guard.
 */
export const MISSING_KEY_PATTERN = new RegExp(
  `\\b(${CATALOG_SECTIONS.join('|')})(\\.[a-zA-Z][a-zA-Z0-9]*)+\\b`
);
