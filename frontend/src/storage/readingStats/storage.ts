import {
  createInMemoryDocumentStorage,
  createLocalStorageDocumentStorage,
  type DeviceDocumentStorage
} from '@/storage/deviceDocument';

export type ReadingStatsStorage = DeviceDocumentStorage;

const READING_STATS_STORAGE_KEY = 'plainshelf.readingStats';

export function createLocalStorageReadingStatsStorage(
  key: string = READING_STATS_STORAGE_KEY
): ReadingStatsStorage {
  return createLocalStorageDocumentStorage(key);
}

export const MOBILE_READING_STATS_PATH = 'plainshelf-cache/reading-stats.json';

export function createInMemoryReadingStatsStorage(
  initial: string | null = null
): ReadingStatsStorage {
  return createInMemoryDocumentStorage(initial);
}
