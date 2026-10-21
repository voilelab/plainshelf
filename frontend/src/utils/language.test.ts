import { beforeEach, describe, expect, it } from 'vitest';
import { setLocale } from '@/i18n';
import {
  CUSTOM_LANGUAGE_VALUE,
  LANGUAGE_VALUES,
  formatLanguage,
  isValidLanguageTag,
  languageSelectOptions,
  normalizeLanguage
} from './language';

describe('book language labels', () => {
  beforeEach(() => {
    setLocale('en');
  });

  it('labels the presets in the active locale', () => {
    expect(formatLanguage('en')).toBe('English');
    expect(formatLanguage('ja')).toBe('Japanese');
    expect(formatLanguage('')).toBe('Unspecified');

    setLocale('zh-Hant');
    expect(formatLanguage('en')).toBe('英文');
    expect(formatLanguage('ja')).toBe('日文');
    expect(formatLanguage('')).toBe('未指定');
  });

  it('normalizes regional tags before labelling them', () => {
    expect(formatLanguage('zh-TW')).toBe('Chinese (Traditional)');
    expect(formatLanguage('zh-CN')).toBe('Chinese (Simplified)');
  });

  it('passes through a tag that is not a preset', () => {
    expect(formatLanguage('fr')).toBe('fr');
    expect(formatLanguage(CUSTOM_LANGUAGE_VALUE)).toBe(CUSTOM_LANGUAGE_VALUE);
  });

  it('offers every preset plus the custom entry, labelled', () => {
    const options = languageSelectOptions();

    expect(options.map((option) => option.value)).toEqual([
      ...LANGUAGE_VALUES,
      CUSTOM_LANGUAGE_VALUE
    ]);
    expect(options[options.length - 1].label).toBe('Custom...');

    setLocale('zh-Hant');
    const zhOptions = languageSelectOptions();
    expect(zhOptions[zhOptions.length - 1].label).toBe('自訂...');
  });

  it('judges tag validity without producing a message', () => {
    expect(isValidLanguageTag('zh-Hant')).toBe(true);
    expect(isValidLanguageTag('fr')).toBe(true);
    expect(isValidLanguageTag('')).toBe(true);
    expect(isValidLanguageTag('  ')).toBe(true);

    expect(isValidLanguageTag('not a tag')).toBe(false);
    expect(isValidLanguageTag('zh_Hant')).toBe(false);

    setLocale('zh-Hant');
    expect(isValidLanguageTag('not a tag')).toBe(false);
  });

  it('leaves tag normalization alone', () => {
    expect(normalizeLanguage(' zh-tw ')).toBe('zh-Hant');
    expect(normalizeLanguage('fr')).toBe('fr');
  });
});
