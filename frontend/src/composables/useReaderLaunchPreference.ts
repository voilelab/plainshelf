import { ref } from 'vue';
import { getPreference, setPreference, subscribePreference } from '@/storage/preferences';
import { isDesktopHost } from '@/providers/runtime';

const STORAGE_KEY = 'reader-launch-mode';

const READER_LAUNCH_MODES = ['new-window', 'new-reader', 'in-window'] as const;

export type ReaderLaunchMode = (typeof READER_LAUNCH_MODES)[number];

export function readerLaunchModes(): readonly ReaderLaunchMode[] {
  return isDesktopHost() ? READER_LAUNCH_MODES : READER_LAUNCH_MODES.filter((mode) => mode !== 'new-window');
}

function defaultReaderLaunchMode(): ReaderLaunchMode {
  return readerLaunchModes()[0];
}

export function parseReaderLaunchMode(rawValue: string | null): ReaderLaunchMode {
  const mode = readerLaunchModes().find((candidate) => candidate === rawValue);
  return mode ?? defaultReaderLaunchMode();
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
