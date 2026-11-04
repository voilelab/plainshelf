import { ref } from 'vue';
import { getPreference, setPreference, subscribePreference } from '@/storage/preferences';

const STORAGE_KEY = 'reader-launch-mode';

const READER_LAUNCH_MODES = ['new-reader', 'in-window'] as const;

export type ReaderLaunchMode = (typeof READER_LAUNCH_MODES)[number];

export const DEFAULT_READER_LAUNCH_MODE: ReaderLaunchMode = 'new-reader';

export function parseReaderLaunchMode(rawValue: string | null): ReaderLaunchMode {
  if (rawValue && (READER_LAUNCH_MODES as readonly string[]).includes(rawValue)) {
    return rawValue as ReaderLaunchMode;
  }
  return DEFAULT_READER_LAUNCH_MODE;
}

const launchMode = ref<ReaderLaunchMode>(parseReaderLaunchMode(getPreference(STORAGE_KEY)));

subscribePreference(STORAGE_KEY, (value) => {
  launchMode.value = parseReaderLaunchMode(value);
});

export function getReaderLaunchMode(): ReaderLaunchMode {
  const stored = parseReaderLaunchMode(getPreference(STORAGE_KEY));
  launchMode.value = stored;
  return stored;
}

export function setReaderLaunchMode(mode: ReaderLaunchMode): void {
  const next = parseReaderLaunchMode(mode);
  launchMode.value = next;
  setPreference(STORAGE_KEY, next);
}

export function useReaderLaunchPreference() {
  return {
    mode: launchMode,
    setMode: setReaderLaunchMode
  };
}
