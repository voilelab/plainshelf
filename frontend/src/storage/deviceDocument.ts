/**
 * Where a client keeps a device-local document (reading history, reading
 * stats). Deliberately format-blind: parsing, merging and trimming live in each
 * feature's document module so every platform shares one implementation, and
 * each backend only has to persist a string.
 */
export interface DeviceDocumentStorage {
  /** Returns the stored text, or null when nothing has been stored yet. */
  load(): Promise<string | null>;
  save(text: string): Promise<void>;
}

/** Browser default. */
export function createLocalStorageDocumentStorage(key: string): DeviceDocumentStorage {
  return {
    async load(): Promise<string | null> {
      if (typeof window === 'undefined') {
        return null;
      }
      return window.localStorage.getItem(key);
    },
    async save(text: string): Promise<void> {
      if (typeof window === 'undefined') {
        return;
      }
      window.localStorage.setItem(key, text);
    }
  };
}

/**
 * Wails desktop. Goes through the Go bindings rather than the WebView's own
 * storage: the document lands in the app's config directory next to
 * shelves.json, so clearing WebView data (or the WebView picking a different
 * profile) does not take the document with it. The bindings return an empty
 * string when the device has not stored one yet.
 */
export function createDesktopDocumentStorage(bindings: {
  read: () => Promise<string>;
  write: (text: string) => Promise<void>;
}): DeviceDocumentStorage {
  return {
    async load(): Promise<string | null> {
      const text = await bindings.read();
      return text === '' ? null : text;
    },
    async save(text: string): Promise<void> {
      await bindings.write(text);
    }
  };
}

/** Mock API mode and unit tests. */
export function createInMemoryDocumentStorage(
  initial: string | null = null
): DeviceDocumentStorage {
  let stored = initial;
  return {
    async load(): Promise<string | null> {
      return stored;
    },
    async save(text: string): Promise<void> {
      stored = text;
    }
  };
}

/**
 * A single stored document, mutated read-modify-write. Every mutation is queued
 * behind the previous one; without that, two concurrent callers (opening the
 * reader while the settings page saves, or two reading-time ticks) would each
 * read the same document and one would overwrite the other's change.
 */
export class DeviceDocumentStore<T> {
  private queue: Promise<unknown> = Promise.resolve();

  constructor(
    private readonly storage: DeviceDocumentStorage,
    private readonly parse: (text: string | null) => T,
    private readonly serialize: (doc: T) => string
  ) {}

  protected async read(): Promise<T> {
    return this.parse(await this.storage.load());
  }

  /**
   * Applies `apply` to the stored document and persists the result. An `apply`
   * that returns its input unchanged writes nothing.
   */
  protected mutate(apply: (doc: T) => T): Promise<void> {
    const next = this.queue.then(async () => {
      const current = await this.read();
      const updated = apply(current);
      if (updated !== current) {
        await this.storage.save(this.serialize(updated));
      }
    });
    // Keep the chain alive after a rejected mutation so a single failure (a
    // full localStorage, a filesystem hiccup) does not wedge later writes.
    this.queue = next.catch(() => undefined);
    return next;
  }
}

/**
 * The key a shelf's device-local data is stored under. Shelf id alone is not
 * enough on a client that can be repointed at another server (the Android
 * shell): two servers commonly both call a shelf `default_shelf` while their
 * book ids mean different things. Same-origin clients (web, desktop) have an
 * empty API base and keep the bare shelf id.
 */
export function buildDeviceDocumentKey(apiBase: string, shelfID: string): string {
  const base = apiBase.trim();
  return base ? `${base}|${shelfID}` : shelfID;
}
