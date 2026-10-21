import type { Page } from '@playwright/test';

const LOCALE_STORAGE_KEY = 'plainshelf.locale';

// Playwright 1.62.1 addInitScript runs on subsequent navigations after document creation and before page scripts (types/types.d.ts).
export async function useLocale(page: Page, locale: 'en' | 'zh-Hant'): Promise<void> {
  await page.addInitScript(
    ([key, value]) => {
      window.localStorage.setItem(key, value);
    },
    [LOCALE_STORAGE_KEY, locale] as const
  );
}

export { MISSING_KEY_PATTERN } from '../../../frontend/src/i18n/missingKeyPattern';
