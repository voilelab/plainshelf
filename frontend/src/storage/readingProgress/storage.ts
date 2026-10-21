import {
  readDesktopReadingProgress,
  writeDesktopReadingProgress
} from '@/api/desktop';
import {
  createDesktopDocumentStorage,
  createInMemoryDocumentStorage,
  createLocalStorageDocumentStorage,
  type DeviceDocumentStorage
} from '@/storage/deviceDocument';

export type ReadingProgressStorage = DeviceDocumentStorage;

export const READING_PROGRESS_STORAGE_KEY = 'plainshelf.readingProgress';

export function createLocalStorageReadingProgressStorage(
  key: string = READING_PROGRESS_STORAGE_KEY
): ReadingProgressStorage {
  return createLocalStorageDocumentStorage(key);
}

export function createDesktopReadingProgressStorage(): ReadingProgressStorage {
  return createDesktopDocumentStorage({
    read: readDesktopReadingProgress,
    write: writeDesktopReadingProgress
  });
}

export function createInMemoryReadingProgressStorage(
  initial: string | null = null
): ReadingProgressStorage {
  return createInMemoryDocumentStorage(initial);
}
