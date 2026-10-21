import { t } from '@/i18n';

type LanguageOption = {
  value: string;
  label: string;
};

export const LANGUAGE_VALUES = ['', 'zh-Hant', 'zh-Hans', 'ja', 'ko', 'en'] as const;

export const CUSTOM_LANGUAGE_VALUE = 'custom';

const LABEL_KEYS: Record<string, string> = {
  '': 'language.book.unspecified',
  'zh-Hant': 'language.book.zhHant',
  'zh-Hans': 'language.book.zhHans',
  ja: 'language.book.ja',
  ko: 'language.book.ko',
  en: 'language.book.en',
  [CUSTOM_LANGUAGE_VALUE]: 'language.book.custom'
};

function labelFor(value: string): string {
  const key = LABEL_KEYS[value];
  return key ? t(key) : value;
}

function languageOptions(): LanguageOption[] {
  return LANGUAGE_VALUES.map((value) => ({ value, label: labelFor(value) }));
}

export function languageSelectOptions(): LanguageOption[] {
  return [
    ...languageOptions(),
    { value: CUSTOM_LANGUAGE_VALUE, label: labelFor(CUSTOM_LANGUAGE_VALUE) }
  ];
}

export function normalizeLanguage(input: string): string {
  const value = input.trim();

  const map: Record<string, string> = {
    'zh-tw': 'zh-Hant',
    'zh-hk': 'zh-Hant',
    'zh-mo': 'zh-Hant',
    'zh-hant': 'zh-Hant',
    'zh-cn': 'zh-Hans',
    'zh-sg': 'zh-Hans',
    'zh-hans': 'zh-Hans'
  };

  return map[value.toLowerCase()] ?? value;
}

export function formatLanguage(input?: string): string {
  const value = normalizeLanguage(input ?? '');

  if (!value) {
    return t('language.book.unspecified');
  }

  return (LANGUAGE_VALUES as readonly string[]).includes(value) ? labelFor(value) : value;
}

const languageTagRE = /^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$/;

export function isValidLanguageTag(input: string): boolean {
  const value = input.trim();
  return value === '' || languageTagRE.test(value);
}
