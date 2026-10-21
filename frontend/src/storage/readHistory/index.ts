import { getActiveShelfID, getApiBase, isMockApiMode } from '@/api/client';
import { hasDesktopReadHistoryBinding } from '@/api/desktop';
import { mockBooks } from '@/api/mocks/books';
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

function createReadHistoryStorage(): ReadHistoryStorage {
  if (isMockApiMode()) {
    return createMockReadHistoryStorage();
  }

  if (isWailsRuntime() && hasDesktopReadHistoryBinding()) {
    return createDesktopReadHistoryStorage();
  }

  const fromShell = getShell()?.createDeviceDocumentStorage?.(MOBILE_READ_HISTORY_PATH);
  if (fromShell) {
    return fromShell;
  }

  return createLocalStorageReadHistoryStorage();
}

let store: ReadHistoryStore | null = null;

function getReadHistoryStore(): ReadHistoryStore {
  if (!store) {
    store = new ReadHistoryStore(createReadHistoryStorage());
  }
  return store;
}

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
