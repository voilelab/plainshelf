import { callWails, hasWailsMethod, wailsHost } from '@/api/wailsBindings';
import { isDesktopShellPreview, isWailsRuntime } from '@/providers/runtime';

type PreferenceListener = (value: string | null) => void;

interface PreferenceBackend {
  get(key: string): string | null;
  set(key: string, value: string | null): void;
}

interface PreferenceBinding {
  GetPreferences: () => Promise<Record<string, string> | null>;
  SetPreference: (key: string, value: string) => Promise<void>;
}

const listeners = new Map<string, Set<PreferenceListener>>();

function notify(key: string, value: string | null): void {
  for (const listener of listeners.get(key) ?? []) {
    listener(value);
  }
}

function hasWindow(): boolean {
  return typeof window !== 'undefined';
}

function createBrowserBackend(): PreferenceBackend {
  const windowless = new Map<string, string>();
  return {
    get(key) {
      if (!hasWindow()) return windowless.get(key) ?? null;
      try {
        return window.localStorage.getItem(key);
      } catch {
        return null;
      }
    },
    set(key, value) {
      if (!hasWindow()) {
        if (value === null) windowless.delete(key);
        else windowless.set(key, value);
        return;
      }
      try {
        if (value === null) {
          window.localStorage.removeItem(key);
        } else {
          window.localStorage.setItem(key, value);
        }
      } catch {
        return;
      }
    }
  };
}

function createSnapshotBackend(
  initial: Record<string, string>,
  persist?: (key: string, value: string) => Promise<void>
): PreferenceBackend {
  const values = new Map(Object.entries(initial));
  let writes: Promise<void> = Promise.resolve();
  return {
    get: (key) => values.get(key) ?? null,
    set(key, value) {
      if (value === null) {
        values.delete(key);
      } else {
        values.set(key, value);
      }
      if (!persist) return;
      writes = writes
        .then(() => persist(key, value ?? ''))
        .catch((err: unknown) => console.error(`Failed to save preference ${key}:`, err));
    }
  };
}

let backend: PreferenceBackend = createBrowserBackend();

function bindStorageEvents(): void {
  if (!hasWindow() || typeof window.addEventListener !== 'function') return;
  window.addEventListener('storage', (event) => {
    if (event.key !== null && event.storageArea === window.localStorage) {
      notify(event.key, event.newValue);
    }
  });
}

bindStorageEvents();

function hasPreferenceBinding(): boolean {
  const app = wailsHost();
  return app !== null && hasWailsMethod(app, 'GetPreferences') && hasWailsMethod(app, 'SetPreference');
}

async function readHostPreferences(): Promise<Record<string, string>> {
  const app = wailsHost() ?? 'DesktopApp';
  try {
    return (await callWails<PreferenceBinding, 'GetPreferences'>(app, 'GetPreferences')) ?? {};
  } catch (err) {
    console.error('Failed to load preferences:', err);
    return {};
  }
}

export async function loadPreferences(): Promise<void> {
  if (hasPreferenceBinding()) {
    const app = wailsHost() ?? 'DesktopApp';
    backend = createSnapshotBackend(await readHostPreferences(), (key, value) =>
      callWails<PreferenceBinding, 'SetPreference'>(app, 'SetPreference', key, value)
    );
    return;
  }
  if (isWailsRuntime() && !isDesktopShellPreview()) {
    console.error('Preferences binding not available; preferences will not be saved.');
    backend = createSnapshotBackend({});
  }
}

export function getPreference(key: string): string | null {
  return backend.get(key);
}

export function setPreference(key: string, value: string | null): void {
  if (backend.get(key) === value) return;
  backend.set(key, value);
  notify(key, value);
}

export function subscribePreference(key: string, listener: PreferenceListener): () => void {
  let set = listeners.get(key);
  if (!set) {
    set = new Set();
    listeners.set(key, set);
  }
  set.add(listener);
  return () => {
    set.delete(listener);
  };
}
