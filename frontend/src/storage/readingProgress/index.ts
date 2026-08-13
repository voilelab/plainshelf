import { getActiveShelfID, getApiBase, isMockApiMode } from '@/api/client';
import { hasDesktopReadingProgressBinding } from '@/api/desktop';
import { isWailsRuntime } from '@/providers/runtime';
import type { BookmarkPayload, ReadingProgress } from '@/types/book';
import { buildDeviceDocumentKey, DeviceDocumentStore } from '@/storage/deviceDocument';
import {
  createReadingProgressDocument,
  getBookReadingOffset,
  parseReadingProgressDocument,
  serializeReadingProgressDocument,
  withBookReadingOffset,
  type ReadingProgressDocument
} from './document';
import {
  createDesktopReadingProgressStorage,
  createInMemoryReadingProgressStorage,
  createLocalStorageReadingProgressStorage,
  type ReadingProgressStorage
} from './storage';

export type { ReadingProgressDocument } from './document';
export type { ReadingProgressStorage } from './storage';

export class ReadingProgressStore extends DeviceDocumentStore<ReadingProgressDocument> {
  constructor(storage: ReadingProgressStorage) {
    super(storage, parseReadingProgressDocument, serializeReadingProgressDocument);
  }

  async get(shelfKey: string, bookID: string): Promise<ReadingProgress> {
    return { char_offset: getBookReadingOffset(await this.read(), shelfKey, bookID) };
  }

  async save(shelfKey: string, bookID: string, progress: BookmarkPayload): Promise<void> {
    await this.mutate((doc) =>
      withBookReadingOffset(doc, shelfKey, bookID, progress.char_offset)
    );
  }
}

function createMockReadingProgressStorage(): ReadingProgressStorage {
  const backing = createInMemoryReadingProgressStorage();
  let seeded = false;
  return {
    async load(): Promise<string | null> {
      if (!seeded) {
        seeded = true;
        const doc = createReadingProgressDocument();
        // Seed lazily so bootstrap has selected the active shelf. Small,
        // deterministic offsets keep the mock detail view populated while the
        // real percentage is derived from the mock content's UTF-16 length.
        doc.shelves[buildDeviceDocumentKey(getApiBase(), getActiveShelfID())] = {
          'book-1': 17,
          'book-2': 50,
          'book-3': 40
        };
        await backing.save(serializeReadingProgressDocument(doc));
      }
      return backing.load();
    },
    save: (text: string) => backing.save(text)
  };
}

export function createReadingProgressStorage(): ReadingProgressStorage {
  if (isMockApiMode()) {
    return createMockReadingProgressStorage();
  }

  // A desktop-shell preview in an ordinary browser has no Wails bindings.
  if (isWailsRuntime() && hasDesktopReadingProgressBinding()) {
    return createDesktopReadingProgressStorage();
  }

  return createLocalStorageReadingProgressStorage();
}

let store: ReadingProgressStore | null = null;

export function getReadingProgressStore(): ReadingProgressStore {
  if (!store) {
    store = new ReadingProgressStore(createReadingProgressStorage());
  }
  return store;
}

export const buildReadingProgressKey = buildDeviceDocumentKey;

function requireProgressKey(): string {
  const shelfID = getActiveShelfID().trim();
  if (!shelfID) {
    throw new Error('No shelf selected.');
  }
  return buildReadingProgressKey(getApiBase(), shelfID);
}

export function getLocalReadingProgress(bookID: string): Promise<ReadingProgress> {
  return getReadingProgressStore().get(requireProgressKey(), bookID);
}

export function saveLocalReadingProgress(
  bookID: string,
  progress: BookmarkPayload
): Promise<void> {
  return getReadingProgressStore().save(requireProgressKey(), bookID, progress);
}
