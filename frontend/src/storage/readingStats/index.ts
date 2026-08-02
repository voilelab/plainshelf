import { getActiveShelfID, getApiBase, isMockApiMode } from '@/api/client';
import { hasDesktopReadingStatsBinding } from '@/api/desktop';
// Imported from providers/runtime rather than the providers barrel: the barrel
// pulls in the providers, which import this module.
import { isMobileRuntime, isWailsRuntime } from '@/providers/runtime';
import { buildDeviceDocumentKey, DeviceDocumentStore } from '@/storage/deviceDocument';
import {
  createReadingStatsDocument,
  getReadingStatsRange,
  parseReadingStatsDocument,
  READING_STATS_RETENTION_DAYS,
  serializeReadingStatsDocument,
  toIsoDate,
  withReadingSeconds,
  type ReadingStatsDocument
} from './document';
import { mockReadingSecondsForDate } from './mock';
import {
  createDesktopReadingStatsStorage,
  createFilesystemReadingStatsStorage,
  createInMemoryReadingStatsStorage,
  createLocalStorageReadingStatsStorage,
  type ReadingStatsStorage
} from './storage';

export { READING_STATS_RETENTION_DAYS } from './document';
export type { ReadingStatsDocument } from './document';
export { mockReadingSecondsForDate } from './mock';
export type { ReadingStatsStorage } from './storage';

/**
 * Daily reading time, kept on the device that did the reading. The server has
 * no part in it: no API call, no shelf file, nothing shared between clients.
 * That is what lets the heatmap keep filling in while offline, and against a
 * read-only server.
 */
export class ReadingStatsStore extends DeviceDocumentStore<ReadingStatsDocument> {
  constructor(storage: ReadingStatsStorage) {
    super(storage, parseReadingStatsDocument, serializeReadingStatsDocument);
  }

  async range(key: string, from: string, to: string): Promise<Record<string, number>> {
    return getReadingStatsRange(await this.read(), key, from, to);
  }

  async add(key: string, date: string, seconds: number): Promise<void> {
    await this.mutate((doc) => withReadingSeconds(doc, key, date, seconds));
  }
}

// Mock mode keeps the dev dashboard populated the way the old mock API did: a
// year of deterministic activity, seeded lazily so the active shelf is already
// known.
function createMockReadingStatsStorage(): ReadingStatsStorage {
  const backing = createInMemoryReadingStatsStorage();
  let seeded = false;
  return {
    async load(): Promise<string | null> {
      if (!seeded) {
        seeded = true;
        const doc = createReadingStatsDocument();
        const days: Record<string, number> = {};
        const cursor = new Date();
        for (let i = 0; i < READING_STATS_RETENTION_DAYS; i += 1) {
          const date = toIsoDate(cursor);
          const seconds = mockReadingSecondsForDate(date);
          if (seconds > 0) {
            days[date] = seconds;
          }
          cursor.setDate(cursor.getDate() - 1);
        }
        doc.shelves[buildDeviceDocumentKey(getApiBase(), getActiveShelfID())] = days;
        await backing.save(serializeReadingStatsDocument(doc));
      }
      return backing.load();
    },
    save: (text: string) => backing.save(text)
  };
}

export function createReadingStatsStorage(): ReadingStatsStorage {
  if (isMockApiMode()) {
    return createMockReadingStatsStorage();
  }

  // `?desktop-shell-preview=1` reports the Wails runtime in an ordinary
  // browser, where the Go bindings do not exist; fall back to localStorage
  // there rather than failing every write.
  if (isWailsRuntime() && hasDesktopReadingStatsBinding()) {
    return createDesktopReadingStatsStorage();
  }

  if (isMobileRuntime()) {
    return createFilesystemReadingStatsStorage();
  }

  return createLocalStorageReadingStatsStorage();
}

let store: ReadingStatsStore | null = null;

export function getReadingStatsStore(): ReadingStatsStore {
  if (!store) {
    store = new ReadingStatsStore(createReadingStatsStorage());
  }
  return store;
}

function requireStatsKey(): string {
  const shelfID = getActiveShelfID().trim();
  if (!shelfID) {
    throw new Error('No shelf selected.');
  }
  return buildDeviceDocumentKey(getApiBase(), shelfID);
}

/**
 * Map of "YYYY-MM-DD" -> total seconds read that day, for [from, to]
 * inclusive. Days with no reading are omitted.
 */
export function getReadingActivityRange(from: string, to: string): Promise<Record<string, number>> {
  return getReadingStatsStore().range(requireStatsKey(), from, to);
}

/**
 * Records a stretch of reading. `bookID` is required — a stretch that cannot
 * be attributed to a book is a bug in the caller — but it is not persisted:
 * nothing displays a per-book breakdown.
 */
export function addReadingSeconds(bookID: string, seconds: number, date: string): Promise<void> {
  if (!bookID.trim()) {
    return Promise.resolve();
  }
  return getReadingStatsStore().add(requireStatsKey(), date, seconds);
}
