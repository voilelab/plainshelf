import { readDesktopReadHistory, writeDesktopReadHistory } from '@/api/desktop';
import {
  createDesktopDocumentStorage,
  createInMemoryDocumentStorage,
  createLocalStorageDocumentStorage,
  type DeviceDocumentStorage
} from '@/storage/deviceDocument';

// Reading history is one of the device-local documents; the backends themselves
// live in storage/deviceDocument.ts. This module only picks the locations.
export type ReadHistoryStorage = DeviceDocumentStorage;

export const READ_HISTORY_STORAGE_KEY = 'plainshelf.readHistory';

/** Browser default. */
export function createLocalStorageReadHistoryStorage(
  key: string = READ_HISTORY_STORAGE_KEY
): ReadHistoryStorage {
  return createLocalStorageDocumentStorage(key);
}

/** Wails desktop: a JSON file next to shelves.json, not WebView storage. */
export function createDesktopReadHistoryStorage(): ReadHistoryStorage {
  return createDesktopDocumentStorage({
    read: readDesktopReadHistory,
    write: writeDesktopReadHistory
  });
}

// Sibling of FilesystemMobileBookCache's `plainshelf-cache/scopes`. One file for
// every shelf: the document keys its shelves internally, using the same
// buildDeviceDocumentKey the cache uses for its scope directory.
export const MOBILE_READ_HISTORY_PATH = 'plainshelf-cache/read-history.json';

/** Mock API mode and unit tests. */
export function createInMemoryReadHistoryStorage(initial: string | null = null): ReadHistoryStorage {
  return createInMemoryDocumentStorage(initial);
}
