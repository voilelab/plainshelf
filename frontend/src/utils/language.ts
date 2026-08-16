import { t } from '@/i18n';

export type LanguageOption = {
  value: string;
  label: string;
};

/**
 * The book languages offered as presets, values only.
 *
 * Values are data — they are written to book metadata and compared against it —
 * while labels are presentation. Keeping them apart is what lets the labels
 * follow the UI locale, and it lets callers derive value sets without pulling
 * translations in with them.
 *
 * `''` means "unspecified".
 */
export const LANGUAGE_VALUES = ['', 'zh-Hant', 'zh-Hans', 'ja', 'ko', 'en'] as const;

export const CUSTOM_LANGUAGE_VALUE = 'custom';

// An explicit map rather than a key built from the value: t() returns the key
// itself when a lookup misses, so a constructed key turns a typo into a dotted
// path rendered on screen instead of a compile error.
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

/**
 * A function rather than the module-level constant it replaces: a const array
 * would resolve its labels once at import and never follow a locale change.
 * Call it from a computed so the labels re-resolve on switch.
 */
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

/**
 * A display label for a stored language tag. Falls back to the tag itself for
 * anything outside the presets, which is what a custom BCP 47 tag hits.
 */
export function formatLanguage(input?: string): string {
  const value = normalizeLanguage(input ?? '');

  if (!value) {
    return t('language.book.unspecified');
  }

  // Presets only. The custom sentinel is a UI selection, never a stored tag, so
  // a book whose language literally reads "custom" keeps showing that — the
  // behaviour the previous label map had.
  return (LANGUAGE_VALUES as readonly string[]).includes(value) ? labelFor(value) : value;
}

const languageTagRE = /^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$/;

export function validateLanguageTag(input: string): string | null {
  const value = input.trim();

  if (value === '') {
    return null;
  }

  if (!languageTagRE.test(value)) {
    return t('language.book.invalidTag');
  }

  return null;
}
