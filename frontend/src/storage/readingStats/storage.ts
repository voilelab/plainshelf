import { readDesktopReadingStats, writeDesktopReadingStats } from '@/api/desktop';
import {
  createDesktopDocumentStorage,
  createInMemoryDocumentStorage,
  createLocalStorageDocumentStorage,
  type DeviceDocumentStorage
} from '@/storage/deviceDocument';

// Reading stats are one of the device-local documents; the backends themselves
// live in storage/deviceDocument.ts. This module only picks the locations.
export type ReadingStatsStorage = DeviceDocumentStorage;

const READING_STATS_STORAGE_KEY = 'plainshelf.readingStats';

/** Browser default. */
export function createLocalStorageReadingStatsStorage(
  key: string = READING_STATS_STORAGE_KEY
): ReadingStatsStorage {
  return createLocalStorageDocumentStorage(key);
}

/** Wails desktop: a JSON file next to shelves.json, not WebView storage. */
export function createDesktopReadingStatsStorage(): ReadingStatsStorage {
  return createDesktopDocumentStorage({
    read: readDesktopReadingStats,
    write: writeDesktopReadingStats
  });
}

// Sibling of the read-history document and FilesystemMobileBookCache's
// `plainshelf-cache/scopes`. One file for every shelf, keyed internally by the
// same buildDeviceDocumentKey the cache uses for its scope directory.
export const MOBILE_READING_STATS_PATH = 'plainshelf-cache/reading-stats.json';

/** Mock API mode and unit tests. */
export function createInMemoryReadingStatsStorage(
  initial: string | null = null
): ReadingStatsStorage {
  return createInMemoryDocumentStorage(initial);
}
