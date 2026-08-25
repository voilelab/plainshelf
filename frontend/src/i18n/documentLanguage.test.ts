// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import { setLocale, useI18n } from './index';

// index.html declares lang="en" statically, so the document would otherwise
// stay declared as English while rendering Chinese.
describe('document language', () => {
  it('is applied for the initial locale at import time', () => {
    // No setLocale call has run yet in this module's lifetime. setLocale
    // early-returns when the value is unchanged, so importing the module has to
    // be what puts the resolved locale on the document.
    const { locale } = useI18n();
    expect(document.documentElement.lang).toBe(locale.value);
  });

  it('follows setLocale in both directions', () => {
    setLocale('zh-Hant');
    expect(document.documentElement.lang).toBe('zh-Hant');

    setLocale('en');
    expect(document.documentElement.lang).toBe('en');
  });
});
