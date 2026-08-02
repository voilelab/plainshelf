import { Directory, Encoding, Filesystem } from '@capacitor/filesystem';

import { readDesktopReadHistory, writeDesktopReadHistory } from '@/api/desktop';

/**
 * Where a client keeps its reading-history document. Deliberately format-blind:
 * merging, deduplication and trimming live in document.ts so every platform
 * shares one implementation, and each backend only has to persist a string.
 */
export interface ReadHistoryStorage {
  /** Returns the stored text, or null when nothing has been stored yet. */
  load(): Promise<string | null>;
  save(text: string): Promise<void>;
}

export const READ_HISTORY_STORAGE_KEY = 'plainshelf.readHistory';

/** Browser default. */
export function createLocalStorageReadHistoryStorage(
  key: string = READ_HISTORY_STORAGE_KEY
): ReadHistoryStorage {
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
 * profile) does not take the reading history with it.
 */
export function createDesktopReadHistoryStorage(): ReadHistoryStorage {
  return {
    async load(): Promise<string | null> {
      const text = await readDesktopReadHistory();
      return text === '' ? null : text;
    },
    async save(text: string): Promise<void> {
      await writeDesktopReadHistory(text);
    }
  };
}

// Sibling of FilesystemMobileBookCache's `plainshelf-cache/books`. Directory.Data
// is app-private internal storage, needs no runtime permission, and — unlike
// IndexedDB or localStorage in an Android WebView — is not subject to silent
// storage eviction.
export const MOBILE_READ_HISTORY_PATH = 'plainshelf-cache/read-history.json';

/** Capacitor (Android) native shell. */
export function createFilesystemReadHistoryStorage(
  path: string = MOBILE_READ_HISTORY_PATH
): ReadHistoryStorage {
  return {
    async load(): Promise<string | null> {
      try {
        const result = await Filesystem.readFile({
          path,
          directory: Directory.Data,
          encoding: Encoding.UTF8
        });
        return typeof result.data === 'string' ? result.data : null;
      } catch {
        // Missing file (the common case on a fresh install) and plugin errors
        // alike mean "no history yet"; document.ts turns null into a default.
        return null;
      }
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
export function createInMemoryReadHistoryStorage(initial: string | null = null): ReadHistoryStorage {
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
