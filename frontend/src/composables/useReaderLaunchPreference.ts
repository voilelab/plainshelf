import { ref } from 'vue';

const STORAGE_KEY = 'reader-launch-mode';

/**
 * How pressing "Read" opens a book.
 *
 * - `new-reader`: open a fresh reader — a new browser tab on the web build, the
 *   standalone PlainShelfReader.app on the desktop build. This is the default.
 * - `in-window`: navigate the current window in place to `/reader/:id`.
 *
 * The preference is device-local (localStorage), not a shelf setting: the same
 * shelf opened on the web and on the desktop means different things by "new
 * window" — a new in-app tab versus a separate native app — so it cannot be a
 * server value shared across clients.
 */
const READER_LAUNCH_MODES = ['new-reader', 'in-window'] as const;

export type ReaderLaunchMode = (typeof READER_LAUNCH_MODES)[number];

export const DEFAULT_READER_LAUNCH_MODE: ReaderLaunchMode = 'new-reader';

export function parseReaderLaunchMode(rawValue: string | null): ReaderLaunchMode {
  if (rawValue && (READER_LAUNCH_MODES as readonly string[]).includes(rawValue)) {
    return rawValue as ReaderLaunchMode;
  }
  // Never set, or some unknown value written by a hand-edit or an older build:
  // fall back to opening a new reader.
  return DEFAULT_READER_LAUNCH_MODE;
}

// One shared preference for the whole app rather than per-component state, so
// the settings panel stays in sync with the launch path.
const launchMode = ref<ReaderLaunchMode>(DEFAULT_READER_LAUNCH_MODE);

if (typeof window !== 'undefined') {
  launchMode.value = parseReaderLaunchMode(window.localStorage.getItem(STORAGE_KEY));
  // `storage` fires in *other* same-origin tabs, so a change made in one reaches
  // every open tab's ref instead of sticking at the value read once at load.
  // The writing tab updates its own ref in setReaderLaunchMode.
  window.addEventListener('storage', (event) => {
    if (event.key === STORAGE_KEY) {
      launchMode.value = parseReaderLaunchMode(event.newValue);
    }
  });
}

/**
 * Read at click time straight from localStorage rather than from the ref, so a
 * change made in another tab is honoured before its `storage` event has
 * necessarily been processed here. The ref is kept in step for the panel.
 */
export function getReaderLaunchMode(): ReaderLaunchMode {
  if (typeof window === 'undefined') {
    return launchMode.value;
  }
  const stored = parseReaderLaunchMode(window.localStorage.getItem(STORAGE_KEY));
  launchMode.value = stored;
  return stored;
}

export function setReaderLaunchMode(mode: ReaderLaunchMode): void {
  // Persisted synchronously, so a getReaderLaunchMode() right after a change
  // reads the new value with no watcher flush in between.
  const next = parseReaderLaunchMode(mode);
  launchMode.value = next;
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(STORAGE_KEY, next);
  }
}

/** Reactive access for the settings panel. */
export function useReaderLaunchPreference() {
  return {
    mode: launchMode,
    setMode: setReaderLaunchMode
  };
}
