import { getActiveShelfID, getApiBase, isMockApiMode } from '@/api/client';
import { hasDesktopReadingProgressBinding } from '@/api/desktop';
import { isWailsRuntime } from '@/providers/runtime';
import type { BookmarkPayload, ReadingProgress } from '@/types/book';
import { buildDeviceDocumentKey, DeviceDocumentStore } from '@/storage/deviceDocument';
import {
  createReadingProgressDocument,
  getBookReadingEntry,
  getBookReadingOffset,
  getShelfReadingEntries,
  parseReadingProgressDocument,
  serializeReadingProgressDocument,
  withBookReadingOffset,
  type ProgressEntry,
  type ReadingProgressDocument
} from './document';
import {
  createDesktopReadingProgressStorage,
  createInMemoryReadingProgressStorage,
  createLocalStorageReadingProgressStorage,
  type ReadingProgressStorage
} from './storage';

export type { ProgressEntry, ReadingProgressDocument } from './document';
export type { ReadingProgressStorage } from './storage';

export class ReadingProgressStore extends DeviceDocumentStore<ReadingProgressDocument> {
  constructor(storage: ReadingProgressStorage) {
    super(storage, parseReadingProgressDocument, serializeReadingProgressDocument);
  }

  async get(shelfKey: string, bookID: string): Promise<ReadingProgress> {
    return { char_offset: getBookReadingOffset(await this.read(), shelfKey, bookID) };
  }

  async getEntry(shelfKey: string, bookID: string): Promise<ProgressEntry | null> {
    return getBookReadingEntry(await this.read(), shelfKey, bookID);
  }

  async getEntries(shelfKey: string): Promise<Record<string, ProgressEntry>> {
    return getShelfReadingEntries(await this.read(), shelfKey);
  }

  async save(shelfKey: string, bookID: string, progress: BookmarkPayload): Promise<void> {
    await this.mutate((doc) =>
      withBookReadingOffset(doc, shelfKey, bookID, progress.char_offset, progress.at)
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
        const shelfKey = buildDeviceDocumentKey(getApiBase(), getActiveShelfID());
        let doc = createReadingProgressDocument();
        doc = withBookReadingOffset(doc, shelfKey, 'book-1', 17);
        doc = withBookReadingOffset(doc, shelfKey, 'book-2', 50);
        doc = withBookReadingOffset(doc, shelfKey, 'book-3', 40);
        await backing.save(serializeReadingProgressDocument(doc));
      }
      return backing.load();
    },
    save: (text: string) => backing.save(text)
  };
}

function createReadingProgressStorage(): ReadingProgressStorage {
  if (isMockApiMode()) {
    return createMockReadingProgressStorage();
  }

  if (isWailsRuntime() && hasDesktopReadingProgressBinding()) {
    return createDesktopReadingProgressStorage();
  }

  return createLocalStorageReadingProgressStorage();
}

let store: ReadingProgressStore | null = null;

function getReadingProgressStore(): ReadingProgressStore {
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

export function getLocalReadingEntry(bookID: string): Promise<ProgressEntry | null> {
  return getReadingProgressStore().getEntry(requireProgressKey(), bookID);
}

export function getLocalReadingEntries(): Promise<Record<string, ProgressEntry>> {
  return getReadingProgressStore().getEntries(requireProgressKey());
}

export function saveLocalReadingProgress(
  bookID: string,
  progress: BookmarkPayload
): Promise<void> {
  return getReadingProgressStore().save(requireProgressKey(), bookID, progress);
}
