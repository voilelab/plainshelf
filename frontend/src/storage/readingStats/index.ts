import { getActiveShelfID, getApiBase, isMockApiMode } from '@/api/client';
import {
  addDesktopReadingSeconds,
  hasDesktopReadingStatsBinding,
  readDesktopReadingStats
} from '@/api/desktop';
import { getShell } from '@/providers/shell';
import {
  buildDeviceDocumentKey,
  DeviceDocumentStore
} from '@/storage/deviceDocument';
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
  MOBILE_READING_STATS_PATH,
  createInMemoryReadingStatsStorage,
  createLocalStorageReadingStatsStorage,
  type ReadingStatsStorage
} from './storage';

export { READING_STATS_RETENTION_DAYS } from './document';
export type { ReadingStatsDocument } from './document';
export { mockReadingSecondsForDate } from './mock';
export type { ReadingStatsStorage } from './storage';

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

type ReadingStatsBackend = Pick<ReadingStatsStore, 'range' | 'add'>;

class DesktopReadingStats implements ReadingStatsBackend {
  async range(key: string, from: string, to: string): Promise<Record<string, number>> {
    const doc = parseReadingStatsDocument(await readDesktopReadingStats());
    return getReadingStatsRange(doc, key, from, to);
  }

  add(key: string, date: string, seconds: number): Promise<void> {
    return addDesktopReadingSeconds(key, date, seconds);
  }
}

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

function createReadingStatsBackend(): ReadingStatsBackend {
  if (isMockApiMode()) {
    return new ReadingStatsStore(createMockReadingStatsStorage());
  }

  if (hasDesktopReadingStatsBinding()) {
    return new DesktopReadingStats();
  }

  const fromShell = getShell()?.createDeviceDocumentStorage?.(MOBILE_READING_STATS_PATH);
  return new ReadingStatsStore(fromShell ?? createLocalStorageReadingStatsStorage());
}

let store: ReadingStatsBackend | null = null;

function getReadingStatsStore(): ReadingStatsBackend {
  if (!store) {
    store = createReadingStatsBackend();
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

export function getReadingActivityRange(from: string, to: string): Promise<Record<string, number>> {
  return getReadingStatsStore().range(requireStatsKey(), from, to);
}

export function addReadingSeconds(bookID: string, seconds: number, date: string): Promise<void> {
  if (!bookID.trim()) {
    return Promise.resolve();
  }
  return getReadingStatsStore().add(requireStatsKey(), date, seconds);
}
