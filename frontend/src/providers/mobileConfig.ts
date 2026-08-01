import { Preferences } from '@capacitor/preferences';

import { setActiveShelfID, setApiBase } from '@/api/client';

// Native (Capacitor) builds load a static bundle with no backend to inject the
// server address or selected shelf. We persist those in Capacitor Preferences
// (native key-value storage) and apply them to the API client at startup. Small
// scalar settings only — book content lives on the filesystem.
const KEY_SERVER_URL = 'plainshelf.mobile.serverUrl';
const KEY_SHELF_ID = 'plainshelf.mobile.shelfId';

// Retired: the mobile client is read-only and never sends an access token.
// Kept only so initMobileConfig can clear it from upgrading installs.
const LEGACY_KEY_TOKEN = 'plainshelf.mobile.token';

export interface MobileConnectionConfig {
  serverUrl: string;
  shelfId: string;
}

async function read(key: string): Promise<string> {
  const { value } = await Preferences.get({ key });
  return (value ?? '').trim();
}

async function write(key: string, value: string): Promise<void> {
  const trimmed = (value ?? '').trim();
  if (trimmed) {
    await Preferences.set({ key, value: trimmed });
  } else {
    await Preferences.remove({ key });
  }
}

export async function loadMobileConnectionConfig(): Promise<MobileConnectionConfig> {
  const [serverUrl, shelfId] = await Promise.all([read(KEY_SERVER_URL), read(KEY_SHELF_ID)]);
  return { serverUrl, shelfId };
}

export async function saveMobileConnectionConfig(config: Partial<MobileConnectionConfig>): Promise<void> {
  const writes: Promise<void>[] = [];
  if (config.serverUrl !== undefined) {
    writes.push(write(KEY_SERVER_URL, config.serverUrl));
  }
  if (config.shelfId !== undefined) {
    writes.push(write(KEY_SHELF_ID, config.shelfId));
  }
  await Promise.all(writes);
  await applyMobileConnectionConfig(config);
}

/**
 * Push saved connection settings into the API client. Called once at startup
 * (before the app mounts) and again whenever the settings UI saves new values,
 * so changes take effect without an app restart.
 */
export async function applyMobileConnectionConfig(config: Partial<MobileConnectionConfig>): Promise<void> {
  if (config.serverUrl !== undefined) {
    setApiBase(config.serverUrl);
  }
  if (config.shelfId !== undefined) {
    setActiveShelfID(config.shelfId);
  }
}

export async function initMobileConfig(): Promise<void> {
  // Drop the access token an older, write-capable build may have stored. The
  // app no longer sends one, so leaving it on disk is a credential with no use.
  await Preferences.remove({ key: LEGACY_KEY_TOKEN });

  const config = await loadMobileConnectionConfig();
  await applyMobileConnectionConfig(config);
}
