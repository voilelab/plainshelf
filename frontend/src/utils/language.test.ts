import { beforeEach, describe, expect, it } from 'vitest';
import { setLocale } from '@/i18n';
import {
  CUSTOM_LANGUAGE_VALUE,
  LANGUAGE_VALUES,
  formatLanguage,
  languageSelectOptions,
  normalizeLanguage,
  validateLanguageTag
} from './language';

// These labels used to be hardcoded Traditional Chinese, so an English UI
// showed Chinese for a book's language.
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
    // The custom sentinel is a Select choice, never a stored tag: a book whose
    // language literally reads "custom" keeps showing that rather than picking
    // up the "Custom..." option label.
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

  it('translates the invalid-tag message and stays silent otherwise', () => {
    expect(validateLanguageTag('zh-Hant')).toBeNull();
    expect(validateLanguageTag('')).toBeNull();

    expect(validateLanguageTag('not a tag')).toBe(
      'That is not a valid language tag. Use a form like en, ja, zh-Hant or zh-TW.'
    );

    setLocale('zh-Hant');
    expect(validateLanguageTag('not a tag')).toBe(
      '語言格式不正確，請使用 en、ja、zh-Hant、zh-TW 這類格式。'
    );
  });

  it('leaves tag normalization alone', () => {
    expect(normalizeLanguage(' zh-tw ')).toBe('zh-Hant');
    expect(normalizeLanguage('fr')).toBe('fr');
  });
});
