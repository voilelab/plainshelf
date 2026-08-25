import { Directory, Encoding, Filesystem } from '@capacitor/filesystem';

import { isMissingError } from '@/providers/mobileCacheFs';
import type { DeviceDocumentStorage } from '@/storage/deviceDocument';

/**
 * Capacitor (Android) native shell. Directory.Data is app-private internal
 * storage, needs no runtime permission, and — unlike IndexedDB or localStorage
 * in an Android WebView — is not subject to silent storage eviction.
 */
function createFilesystemDocumentStorage(path: string): DeviceDocumentStorage {
  return {
    async load(): Promise<string | null> {
      let result;
      try {
        result = await Filesystem.readFile({
          path,
          directory: Directory.Data,
          encoding: Encoding.UTF8
        });
      } catch (error) {
        // Only a missing file — the normal state on a fresh install — means
        // "nothing stored yet". Any other read failure must propagate:
        // reporting it as null would let the next write replace an unread
        // document and wipe every shelf's data after a transient error.
        if (isMissingError(error)) {
          return null;
        }
        throw error;
      }

      if (typeof result.data !== 'string') {
        throw new Error(`Unexpected contents in ${path}`);
      }
      return result.data;
    },
    async save(text: string): Promise<void> {
      await Filesystem.writeFile({
        path,
        data: text,
        directory: Directory.Data,
        encoding: Encoding.UTF8,
        recursive: true
      });
    }
  };
}

/**
 * The device-local document backend this shell provides.
 *
 * Lives here rather than in `storage/` because it is the only backend that
 * needs Capacitor: keeping it on this side of the seam is what stops
 * `@capacitor/filesystem` — and the cache module it borrows `isMissingError`
 * from — being pulled into the web and desktop bundles by the reading-history
 * and reading-stats stores.
 */
export function createMobileDeviceDocumentStorage(path: string): DeviceDocumentStorage {
  return createFilesystemDocumentStorage(path);
}
