import { Directory, Encoding, Filesystem } from '@capacitor/filesystem';

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

// Matches the "file is not there" errors the Capacitor plugin and its web
// fallback raise, mirroring FilesystemMobileBookCache.isMissingError.
function isMissingFileError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error);
  return /not exist|does not exist|enoent|no such file|not found/i.test(message);
}

/**
 * Capacitor (Android) native shell. Directory.Data is app-private internal
 * storage, needs no runtime permission, and — unlike IndexedDB or localStorage
 * in an Android WebView — is not subject to silent storage eviction.
 */
export function createFilesystemDocumentStorage(path: string): DeviceDocumentStorage {
  return {
    async load(): Promise<string | null> {
      let result;
      try {
        result = await Filesystem.readFile({
          path,
          directory: Directory.Data,
          encoding: Encoding.UTF8
        });
      } catch (error) {
        // Only a missing file — the normal state on a fresh install — means
        // "nothing stored yet". Any other read failure must propagate:
        // reporting it as null would let the next write replace an unread
        // document and wipe every shelf's data after a transient error.
        if (isMissingFileError(error)) {
          return null;
        }
        throw error;
      }

      if (typeof result.data !== 'string') {
        throw new Error(`Unexpected contents in ${path}`);
      }
      return result.data;
    },
    async save(text: string): Promise<void> {
      await Filesystem.writeFile({
        path,
        data: text,
        directory: Directory.Data,
        encoding: Encoding.UTF8,
        recursive: true
      });
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
