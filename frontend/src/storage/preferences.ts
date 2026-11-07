import { callWails, wailsHost } from '@/api/wailsBindings';

type PreferenceListener = (value: string | null) => void;

interface PreferenceBackend {
  get(key: string): string | null;
  set(key: string, value: string | null): void;
  refresh?(): Promise<void>;
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

interface SnapshotHost {
  read(): Promise<Record<string, string>>;
  persist(key: string, value: string): Promise<void>;
}

function createSnapshotBackend(initial: Record<string, string>, host?: SnapshotHost): PreferenceBackend {
  const values = new Map(Object.entries(initial));
  const localEdits = new Map<string, number>();
  let edits = 0;
  let writes: Promise<void> = Promise.resolve();

  function apply(key: string, value: string | null): void {
    if (value === null) {
      values.delete(key);
    } else {
      values.set(key, value);
    }
  }

  async function refresh(read: () => Promise<Record<string, string>>): Promise<void> {
    const startedAt = edits;
    await writes;
    let stored: Record<string, string>;
    try {
      stored = await read();
    } catch (err) {
      console.error('Failed to reload preferences:', err);
      return;
    }
    for (const key of new Set([...values.keys(), ...Object.keys(stored)])) {
      const next = stored[key] ?? null;
      if ((localEdits.get(key) ?? 0) > startedAt || (values.get(key) ?? null) === next) continue;
      apply(key, next);
      notify(key, next);
    }
  }

  return {
    get: (key) => values.get(key) ?? null,
    set(key, value) {
      apply(key, value);
      localEdits.set(key, ++edits);
      if (!host) return;
      writes = writes
        .then(() => host.persist(key, value ?? ''))
        .catch((err: unknown) => console.error(`Failed to save preference ${key}:`, err));
    },
    refresh: host ? () => refresh(host.read) : undefined
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

async function readHostPreferences(): Promise<Record<string, string>> {
  const app = wailsHost() ?? 'DesktopApp';
  return (await callWails<PreferenceBinding, 'GetPreferences'>(app, 'GetPreferences')) ?? {};
}

async function readInitialHostPreferences(): Promise<Record<string, string>> {
  try {
    return await readHostPreferences();
  } catch (err) {
    console.error('Failed to load preferences:', err);
    return {};
  }
}

export async function loadPreferences(): Promise<void> {
  const app = wailsHost();
  if (app !== null) {
    backend = createSnapshotBackend(await readInitialHostPreferences(), {
      read: readHostPreferences,
      persist: (key, value) => callWails<PreferenceBinding, 'SetPreference'>(app, 'SetPreference', key, value)
    });
    window.addEventListener('focus', () => void backend.refresh?.());
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
