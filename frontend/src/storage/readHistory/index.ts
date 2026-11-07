import { getActiveShelfID, getApiBase, isMockApiMode } from '@/api/client';
import {
  clearDesktopReadHistory,
  hasDesktopReadHistoryBinding,
  readDesktopReadHistory,
  recordDesktopReadHistory,
  setDesktopReadHistoryLimit
} from '@/api/desktop';
import { mockBooks } from '@/api/mocks/books';
import { getShell } from '@/providers/shell';
import {
  buildDeviceDocumentKey,
  DeviceDocumentStore
} from '@/storage/deviceDocument';
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

type ReadHistoryBackend = Pick<
  ReadHistoryStore,
  'list' | 'add' | 'clear' | 'getLimit' | 'setLimit'
>;

class DesktopReadHistory implements ReadHistoryBackend {
  private queue: Promise<unknown> = Promise.resolve();

  private enqueue(write: () => Promise<void>): Promise<void> {
    const next = this.queue.then(write);
    this.queue = next.catch(() => undefined);
    return next;
  }

  private async read(): Promise<ReadHistoryDocument> {
    return parseReadHistoryDocument(await readDesktopReadHistory());
  }

  async list(shelfID: string): Promise<string[]> {
    return getShelfReadHistory(await this.read(), shelfID);
  }

  add(shelfID: string, bookID: string): Promise<void> {
    return this.enqueue(() => recordDesktopReadHistory(shelfID, bookID));
  }

  clear(shelfID: string): Promise<void> {
    return this.enqueue(() => clearDesktopReadHistory(shelfID));
  }

  async getLimit(): Promise<number> {
    return (await this.read()).limit;
  }

  setLimit(limit: number): Promise<void> {
    return this.enqueue(() => setDesktopReadHistoryLimit(limit));
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

function createReadHistoryBackend(): ReadHistoryBackend {
  if (isMockApiMode()) {
    return new ReadHistoryStore(createMockReadHistoryStorage());
  }

  if (hasDesktopReadHistoryBinding()) {
    return new DesktopReadHistory();
  }

  const fromShell = getShell()?.createDeviceDocumentStorage?.(MOBILE_READ_HISTORY_PATH);
  return new ReadHistoryStore(fromShell ?? createLocalStorageReadHistoryStorage());
}

let store: ReadHistoryBackend | null = null;

function getReadHistoryStore(): ReadHistoryBackend {
  if (!store) {
    store = createReadHistoryBackend();
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
