import { getActiveShelfID, getApiBase, isMockApiMode } from '@/api/client';
import { hasDesktopReadHistoryBinding } from '@/api/desktop';
import { mockBooks } from '@/api/mocks/books';
// Imported from providers/runtime rather than the providers barrel: the barrel
// pulls in the providers, which import this module.
import { isWailsRuntime } from '@/providers/runtime';
import { getShell } from '@/providers/shell';
import { buildDeviceDocumentKey, DeviceDocumentStore } from '@/storage/deviceDocument';
import {
  createReadHistoryDocument,
  getShelfReadHistory,
  parseReadHistoryDocument,
  serializeReadHistoryDocument,
  withClearedShelf,
  withLimit,
  withReadEntry,
  type ReadHistoryDocument
} from './document';
import {
  MOBILE_READ_HISTORY_PATH,
  createDesktopReadHistoryStorage,
  createInMemoryReadHistoryStorage,
  createLocalStorageReadHistoryStorage,
  type ReadHistoryStorage
} from './storage';

export { DEFAULT_READ_HISTORY_LIMIT } from './document';
export type { ReadHistoryDocument } from './document';
export type { ReadHistoryStorage } from './storage';

/**
 * Reading history, kept on the device that did the reading. The server has no
 * part in it: no API call, no shelf file, nothing shared between clients.
 *
 * Mutation serialization comes from DeviceDocumentStore: without it, opening
 * the reader (addReadHistory) while the settings page saves a new limit would
 * let one overwrite the other's document.
 */
export class ReadHistoryStore extends DeviceDocumentStore<ReadHistoryDocument> {
  constructor(storage: ReadHistoryStorage) {
    super(storage, parseReadHistoryDocument, serializeReadHistoryDocument);
  }

  async list(shelfID: string): Promise<string[]> {
    return getShelfReadHistory(await this.read(), shelfID);
  }

  async add(shelfID: string, bookID: string): Promise<void> {
    await this.mutate((doc) => withReadEntry(doc, shelfID, bookID));
  }

  async clear(shelfID: string): Promise<void> {
    await this.mutate((doc) => withClearedShelf(doc, shelfID));
  }

  async getLimit(): Promise<number> {
    return (await this.read()).limit;
  }

  async setLimit(limit: number): Promise<void> {
    await this.mutate((doc) => withLimit(doc, limit));
  }
}

// Mock mode keeps the dev UI populated the way the old mock API did: the three
// first mock books, seeded lazily so the active shelf is already known.
function createMockReadHistoryStorage(): ReadHistoryStorage {
  const backing = createInMemoryReadHistoryStorage();
  let seeded = false;
  return {
    async load(): Promise<string | null> {
      if (!seeded) {
        seeded = true;
        const doc = createReadHistoryDocument();
        doc.shelves[buildReadHistoryKey(getApiBase(), getActiveShelfID())] = mockBooks
          .slice(0, 3)
          .map((book) => book.id);
        await backing.save(serializeReadHistoryDocument(doc));
      }
      return backing.load();
    },
    save: (text: string) => backing.save(text)
  };
}

export function createReadHistoryStorage(): ReadHistoryStorage {
  if (isMockApiMode()) {
    return createMockReadHistoryStorage();
  }

  // `?desktop-shell-preview=1` reports the Wails runtime in an ordinary
  // browser, where the Go bindings do not exist; fall back to localStorage
  // there rather than failing every write.
  if (isWailsRuntime() && hasDesktopReadHistoryBinding()) {
    return createDesktopReadHistoryStorage();
  }

  // A shell that keeps documents somewhere other than the browser says so;
  // the mobile one writes to app-private storage, which an Android WebView
  // does not silently evict the way it may evict localStorage.
  const fromShell = getShell()?.createDeviceDocumentStorage?.(MOBILE_READ_HISTORY_PATH);
  if (fromShell) {
    return fromShell;
  }

  return createLocalStorageReadHistoryStorage();
}

let store: ReadHistoryStore | null = null;

export function getReadHistoryStore(): ReadHistoryStore {
  if (!store) {
    store = new ReadHistoryStore(createReadHistoryStorage());
  }
  return store;
}

/** The key a shelf's history is stored under. See buildDeviceDocumentKey. */
export const buildReadHistoryKey = buildDeviceDocumentKey;

function requireHistoryKey(): string {
  const shelfID = getActiveShelfID().trim();
  if (!shelfID) {
    throw new Error('No shelf selected.');
  }
  return buildReadHistoryKey(getApiBase(), shelfID);
}

export function getReadHistoryIDs(): Promise<string[]> {
  return getReadHistoryStore().list(requireHistoryKey());
}

export function addReadHistory(bookID: string): Promise<void> {
  return getReadHistoryStore().add(requireHistoryKey(), bookID);
}

export function clearReadHistory(): Promise<void> {
  return getReadHistoryStore().clear(requireHistoryKey());
}

export function getReadHistoryLimit(): Promise<number> {
  return getReadHistoryStore().getLimit();
}

export function setReadHistoryLimit(limit: number): Promise<void> {
  return getReadHistoryStore().setLimit(limit);
}
