import { ref } from 'vue';

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

const launchMode = ref<ReaderLaunchMode>(DEFAULT_READER_LAUNCH_MODE);

if (typeof window !== 'undefined') {
  launchMode.value = parseReaderLaunchMode(window.localStorage.getItem(STORAGE_KEY));
  // HTML Web Storage sends storage events to other documents, excluding the document that performed the write.
  window.addEventListener('storage', (event) => {
    if (event.key === STORAGE_KEY) {
      launchMode.value = parseReaderLaunchMode(event.newValue);
    }
  });
}

export function getReaderLaunchMode(): ReaderLaunchMode {
  if (typeof window === 'undefined') {
    return launchMode.value;
  }
  const stored = parseReaderLaunchMode(window.localStorage.getItem(STORAGE_KEY));
  launchMode.value = stored;
  return stored;
}

export function setReaderLaunchMode(mode: ReaderLaunchMode): void {
  const next = parseReaderLaunchMode(mode);
  launchMode.value = next;
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(STORAGE_KEY, next);
  }
}

export function useReaderLaunchPreference() {
  return {
    mode: launchMode,
    setMode: setReaderLaunchMode
  };
}
