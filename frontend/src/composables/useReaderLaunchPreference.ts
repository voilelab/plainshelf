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

// Module-level reactive state, mirroring useAppZoom.ts: a single shared
// preference for the whole app rather than per-component state, so the settings
// panel stays in sync with the goRead action below.
const launchMode = ref<ReaderLaunchMode>(DEFAULT_READER_LAUNCH_MODE);

if (typeof window !== 'undefined') {
  launchMode.value = parseReaderLaunchMode(window.localStorage.getItem(STORAGE_KEY));
  // A `storage` event fires in *other* same-origin tabs when this preference is
  // written, so a change made in one tab reaches every open tab's shared ref —
  // and thus its reactive settings panel — instead of being stuck at the value
  // read once at load. The writing tab updates its own ref in setReaderLaunchMode.
  window.addEventListener('storage', (event) => {
    if (event.key === STORAGE_KEY) {
      launchMode.value = parseReaderLaunchMode(event.newValue);
    }
  });
}

/**
 * The current preference. `goRead` calls this at click time, so it reads the
 * persisted value straight from localStorage rather than the once-seeded ref:
 * that way a change made in another tab is honoured immediately, before that
 * tab's `storage` event has necessarily been processed here. The shared ref is
 * kept in step so the settings panel reflects the same value.
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
  // Persist synchronously (like useReaderSettings' persistFontSize) so a
  // getReaderLaunchMode() call right after a change reads the new value, with no
  // watcher flush in between.
  const next = parseReaderLaunchMode(mode);
  launchMode.value = next;
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(STORAGE_KEY, next);
  }
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
