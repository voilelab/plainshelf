import { Preferences } from '@capacitor/preferences';

import { setActiveShelfID, setApiBase } from '@/api/client';

// Native (Capacitor) builds load a static bundle with no backend to inject the
// server address, token, or selected shelf. We persist those in Capacitor
// Preferences (native key-value storage) and apply them to the API client at
// startup. Small scalar settings only — book content lives on the filesystem.
//
// The token is not what makes the app read-only — api/client.ts rejects every
// non-allowlisted mutation before it is sent, token or not. It exists because
// server/security.go requires a token for *any* mutating /api request, so
// without one the two allowlisted reading-telemetry POSTs (read_history,
// reading_activity) get a 401, and a `protect_read` server rejects reads too.
const KEY_SERVER_URL = 'plainshelf.mobile.serverUrl';
const KEY_TOKEN = 'plainshelf.mobile.token';
const KEY_SHELF_ID = 'plainshelf.mobile.shelfId';

export interface MobileConnectionConfig {
  serverUrl: string;
  token: string;
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
  const [serverUrl, token, shelfId] = await Promise.all([
    read(KEY_SERVER_URL),
    read(KEY_TOKEN),
    read(KEY_SHELF_ID)
  ]);
  return { serverUrl, token, shelfId };
}

export async function saveMobileConnectionConfig(config: Partial<MobileConnectionConfig>): Promise<void> {
  const writes: Promise<void>[] = [];
  if (config.serverUrl !== undefined) {
    writes.push(write(KEY_SERVER_URL, config.serverUrl));
  }
  if (config.token !== undefined) {
    writes.push(write(KEY_TOKEN, config.token));
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

  // The API client reads the token via window.plainshelf.getApiToken (reusing
  // the Electron branch). Read live from Preferences so a token update applies
  // immediately without touching this closure.
  window.plainshelf = {
    getApiToken: async () => read(KEY_TOKEN)
  };
}

export async function initMobileConfig(): Promise<void> {
  const config = await loadMobileConnectionConfig();
  await applyMobileConnectionConfig(config);
}
