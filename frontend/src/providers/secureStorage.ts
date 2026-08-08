import { registerPlugin } from '@capacitor/core';
import { Preferences } from '@capacitor/preferences';

interface SecureStorageGetOptions {
  key: string;
}

interface SecureStorageSetOptions extends SecureStorageGetOptions {
  value: string;
}

interface SecureStorageGetResult {
  value: string | null;
}

interface SecureStoragePlugin {
  get(options: SecureStorageGetOptions): Promise<SecureStorageGetResult>;
  set(options: SecureStorageSetOptions): Promise<void>;
  remove(options: SecureStorageGetOptions): Promise<void>;
}

const WEB_STORAGE_PREFIX = 'plainshelf.secure.';

function webStorageKey(key: string): string {
  return `${WEB_STORAGE_PREFIX}${key}`;
}

const webSecureStorage: SecureStoragePlugin = {
  async get({ key }) {
    return Preferences.get({ key: webStorageKey(key) });
  },
  async set({ key, value }) {
    await Preferences.set({ key: webStorageKey(key), value });
  },
  async remove({ key }) {
    await Preferences.remove({ key: webStorageKey(key) });
  }
};

/**
 * Android resolves the native plugin, whose values are encrypted with a
 * non-exportable Keystore key. The Preferences-backed web implementation is
 * only for the browser mobile-preview harness; its separate key namespace
 * keeps secure-storage emulation distinct from legacy-token migration data.
 */
export const SecureStorage = registerPlugin<SecureStoragePlugin>('PlainShelfSecureStorage', {
  web: webSecureStorage
});
