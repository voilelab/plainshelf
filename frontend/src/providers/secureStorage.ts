import { registerPlugin } from '@capacitor/core';

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

/** Android-only storage whose values are encrypted with a non-exportable Keystore key. */
export const SecureStorage = registerPlugin<SecureStoragePlugin>('PlainShelfSecureStorage');
