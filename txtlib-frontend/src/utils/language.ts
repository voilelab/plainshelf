export type LanguageOption = {
  value: string;
  label: string;
};

export const LANGUAGE_OPTIONS: LanguageOption[] = [
  { value: '', label: '未指定' },
  { value: 'zh-Hant', label: '中文（繁體）' },
  { value: 'zh-Hans', label: '中文（簡體）' },
  { value: 'ja', label: '日文' },
  { value: 'ko', label: '韓文' },
  { value: 'en', label: '英文' }
];

export const CUSTOM_LANGUAGE_VALUE = 'custom';

export const LANGUAGE_SELECT_OPTIONS: LanguageOption[] = [
  ...LANGUAGE_OPTIONS,
  { value: CUSTOM_LANGUAGE_VALUE, label: '自訂...' }
];

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

const languageLabelMap = new Map(
  LANGUAGE_OPTIONS.filter((option) => option.value !== '').map((option) => [option.value, option.label])
);

export function formatLanguage(input?: string): string {
  const value = normalizeLanguage(input ?? '');

  if (!value) {
    return '未指定';
  }

  return languageLabelMap.get(value) ?? value;
}

const languageTagRE = /^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$/;

export function validateLanguageTag(input: string): string | null {
  const value = input.trim();

  if (value === '') {
    return null;
  }

  if (!languageTagRE.test(value)) {
    return '語言格式不正確，請使用 en、ja、zh-Hant、zh-TW 這類格式。';
  }

  return null;
}