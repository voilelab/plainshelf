import { ref } from 'vue';
import { getPreference, setPreference, subscribePreference } from '@/storage/preferences';

const STORAGE_KEY = 'app-theme';

const APP_THEMES = ['system', 'light', 'dark'] as const;

export type AppTheme = (typeof APP_THEMES)[number];

export function parseAppTheme(rawValue: string | null): AppTheme {
  return (APP_THEMES as readonly string[]).includes(rawValue ?? '') ? (rawValue as AppTheme) : 'system';
}

function applyAppTheme(theme: AppTheme): void {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  if (theme === 'system') {
    root.removeAttribute('data-theme');
  } else {
    root.setAttribute('data-theme', theme);
  }
}

const appTheme = ref<AppTheme>('system');

function setAppTheme(theme: AppTheme): void {
  appTheme.value = theme;
  applyAppTheme(theme);
  setPreference(STORAGE_KEY, theme === 'system' ? null : theme);
}

export function initAppTheme(): void {
  appTheme.value = parseAppTheme(getPreference(STORAGE_KEY));
  applyAppTheme(appTheme.value);
  subscribePreference(STORAGE_KEY, (value) => {
    appTheme.value = parseAppTheme(value);
    applyAppTheme(appTheme.value);
  });
}

export function useAppTheme() {
  return { theme: appTheme, setTheme: setAppTheme };
}
