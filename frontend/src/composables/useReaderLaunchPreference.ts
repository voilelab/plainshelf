import { ref, watch } from 'vue';

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
export const READER_LAUNCH_MODES = ['new-reader', 'in-window'] as const;

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

// Module-level state, mirroring useAppZoom.ts: a single shared preference for
// the whole app rather than per-component state, so the settings panel and the
// goRead action below always agree without threading a ref between them.
const launchMode = ref<ReaderLaunchMode>(DEFAULT_READER_LAUNCH_MODE);

if (typeof window !== 'undefined') {
  launchMode.value = parseReaderLaunchMode(window.localStorage.getItem(STORAGE_KEY));
}

watch(
  launchMode,
  (value) => {
    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem(STORAGE_KEY, parseReaderLaunchMode(value));
  },
  { immediate: false }
);

/**
 * The current preference, read fresh from the shared state. `goRead` calls this
 * at click time so it always honours the latest choice — including one made in
 * the settings tab during the same session.
 */
export function getReaderLaunchMode(): ReaderLaunchMode {
  return launchMode.value;
}

export function setReaderLaunchMode(mode: ReaderLaunchMode): void {
  launchMode.value = parseReaderLaunchMode(mode);
}

/**
 * Reactive access for the settings panel: the shared `mode` ref plus a setter
 * that validates and persists it. Every consumer shares one ref, so a change in
 * the panel is visible to `getReaderLaunchMode` immediately.
 */
export function useReaderLaunchPreference() {
  return {
    mode: launchMode,
    setMode: setReaderLaunchMode
  };
}
