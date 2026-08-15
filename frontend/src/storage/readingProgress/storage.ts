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

/** Browser default. */
export function createLocalStorageReadingProgressStorage(
  key: string = READING_PROGRESS_STORAGE_KEY
): ReadingProgressStorage {
  return createLocalStorageDocumentStorage(key);
}

/** Wails desktop: a JSON file next to shelves.json, not WebView storage. */
export function createDesktopReadingProgressStorage(): ReadingProgressStorage {
  return createDesktopDocumentStorage({
    read: readDesktopReadingProgress,
    write: writeDesktopReadingProgress
  });
}

/** Mock API mode and unit tests. */
export function createInMemoryReadingProgressStorage(
  initial: string | null = null
): ReadingProgressStorage {
  return createInMemoryDocumentStorage(initial);
}
