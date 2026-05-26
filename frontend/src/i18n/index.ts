import { ref } from 'vue';
import en from './locales/en';
import zhHant from './locales/zh-Hant';

interface Messages {
  [key: string]: string | Messages;
}

export const SUPPORTED_LOCALES = ['en', 'zh-Hant'] as const;
export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number];

const FALLBACK_LOCALE: SupportedLocale = 'en';
const LOCALE_STORAGE_KEY = 'plainshelf.locale';

const MESSAGES: Record<SupportedLocale, Messages> = {
  en,
  'zh-Hant': zhHant
};

function isSupportedLocale(value: string): value is SupportedLocale {
  return SUPPORTED_LOCALES.includes(value as SupportedLocale);
}

function normalizeLocale(value: unknown): SupportedLocale | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }

  const normalized = value.trim();
  if (!normalized) {
    return undefined;
  }

  if (isSupportedLocale(normalized)) {
    return normalized;
  }

  const lower = normalized.toLowerCase();

  if (lower.startsWith('en')) {
    return 'en';
  }

  if (
    lower.startsWith('zh-hant') ||
    lower.startsWith('zh-tw') ||
    lower.startsWith('zh-hk') ||
    lower.startsWith('zh-mo')
  ) {
    return 'zh-Hant';
  }

  return undefined;
}

function getByPath(messages: Messages, path: string): string | undefined {
  const parts = path.split('.');
  let cursor: unknown = messages;

  for (const part of parts) {
    if (!cursor || typeof cursor !== 'object' || !(part in cursor)) {
      return undefined;
    }
    cursor = (cursor as Record<string, unknown>)[part];
  }

  return typeof cursor === 'string' ? cursor : undefined;
}

function interpolate(template: string, params?: Record<string, string | number>): string {
  if (!params) {
    return template;
  }

  return template.replace(/\{(\w+)\}/g, (_, key: string) => {
    const value = params[key];
    return value === undefined ? `{${key}}` : String(value);
  });
}

function getStoredLocale(): SupportedLocale | undefined {
  if (typeof window === 'undefined') {
    return undefined;
  }

  try {
    return normalizeLocale(window.localStorage.getItem(LOCALE_STORAGE_KEY));
  } catch {
    return undefined;
  }
}

function getBrowserLocale(): SupportedLocale | undefined {
  if (typeof navigator === 'undefined') {
    return undefined;
  }

  for (const candidate of navigator.languages) {
    const normalized = normalizeLocale(candidate);
    if (normalized) {
      return normalized;
    }
  }

  return normalizeLocale(navigator.language);
}

function resolveInitialLocale(): SupportedLocale {
  return getStoredLocale() ?? getBrowserLocale() ?? FALLBACK_LOCALE;
}

const localeRef = ref<SupportedLocale>(resolveInitialLocale());

function persistLocale(locale: SupportedLocale): void {
  if (typeof window === 'undefined') {
    return;
  }

  try {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  } catch {
    // Ignore storage errors.
  }
}

export function setLocale(locale: SupportedLocale): void {
  if (localeRef.value === locale) {
    return;
  }

  localeRef.value = locale;
  persistLocale(locale);
}

export function t(key: string, params?: Record<string, string | number>): string {
  const value = getByPath(MESSAGES[localeRef.value], key) ?? getByPath(MESSAGES[FALLBACK_LOCALE], key) ?? key;
  return interpolate(value, params);
}

export function useI18n() {
  return {
    locale: localeRef,
    setLocale,
    t,
    supportedLocales: SUPPORTED_LOCALES
  };
}
