import { Directory, Encoding, Filesystem } from '@capacitor/filesystem';

import { isMissingError } from '@/providers/mobileCacheFs';
import type { DeviceDocumentStorage } from '@/storage/deviceDocument';

// Capacitor Filesystem 8.1.3 maps Directory.Data to Android context.filesDir, using app-private files without external-storage permission.
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

export function createMobileDeviceDocumentStorage(path: string): DeviceDocumentStorage {
  return createFilesystemDocumentStorage(path);
}
