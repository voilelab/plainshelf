import { readDesktopReadHistory, writeDesktopReadHistory } from '@/api/desktop';
import {
  createDesktopDocumentStorage,
  createInMemoryDocumentStorage,
  createLocalStorageDocumentStorage,
  type DeviceDocumentStorage
} from '@/storage/deviceDocument';

export type ReadHistoryStorage = DeviceDocumentStorage;

export const READ_HISTORY_STORAGE_KEY = 'plainshelf.readHistory';

export function createLocalStorageReadHistoryStorage(
  key: string = READ_HISTORY_STORAGE_KEY
): ReadHistoryStorage {
  return createLocalStorageDocumentStorage(key);
}

export function createDesktopReadHistoryStorage(): ReadHistoryStorage {
  return createDesktopDocumentStorage({
    read: readDesktopReadHistory,
    write: writeDesktopReadHistory
  });
}

export const MOBILE_READ_HISTORY_PATH = 'plainshelf-cache/read-history.json';

export function createInMemoryReadHistoryStorage(initial: string | null = null): ReadHistoryStorage {
  return createInMemoryDocumentStorage(initial);
}
