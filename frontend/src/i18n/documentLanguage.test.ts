// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import { setLocale, useI18n } from './index';

describe('document language', () => {
  it('is applied for the initial locale at import time', () => {
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
