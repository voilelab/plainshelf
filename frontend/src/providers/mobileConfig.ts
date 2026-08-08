import { Preferences } from '@capacitor/preferences';

import { setActiveShelfID, setApiBase } from '@/api/client';
import { SecureStorage } from '@/providers/secureStorage';

// Native (Capacitor) builds load a static bundle with no backend to inject the
// server address, token, or selected shelf. We persist non-secret settings in
// Capacitor Preferences and apply them to the API client at startup. The
// long-lived pCloud bearer token is kept separately in Android Keystore-backed
// storage. Small scalar settings only — book content lives on the filesystem.
//
// The token is not what makes the app read-only — api/client.ts rejects every
// mutation before it is sent, token or not. The app issues no writes at all, so
// the token is only needed to read from a `protect_read` server, which requires
// one for reads as well.
const KEY_MODE = 'plainshelf.mobile.mode';
const KEY_SERVER_URL = 'plainshelf.mobile.serverUrl';
const KEY_TOKEN = 'plainshelf.mobile.token';
const KEY_SHELF_ID = 'plainshelf.mobile.shelfId';
const KEY_PCLOUD_CLIENT_ID = 'plainshelf.mobile.pcloud.clientId';
const KEY_PCLOUD_ACCESS_TOKEN = 'plainshelf.mobile.pcloud.accessToken';
const KEY_PCLOUD_HOST = 'plainshelf.mobile.pcloud.host';
const KEY_PCLOUD_SHELF_ROOT = 'plainshelf.mobile.pcloud.shelfRoot';

/**
 * Where the library comes from.
 *
 * `server` is a PlainShelf server reachable over the network. `pcloud` reads a
 * shelf straight out of pCloud with no server involved — the one case a server
 * cannot cover, because a phone cannot mount cloud storage the way a host can.
 */
export type ConnectionMode = 'server' | 'pcloud';

export interface MobileConnectionConfig {
  mode: ConnectionMode;
  serverUrl: string;
  token: string;
  shelfId: string;
  /**
   * The user's own pCloud application key. PlainShelf ships no default: the
   * project registers no pCloud app, so each user supplies one they created.
   */
  pcloudClientId: string;
  pcloudAccessToken: string;
  /** Regional API host, learned during authorization rather than guessed. */
  pcloudHost: string;
  /** Path of the shelf directory on pCloud, e.g. `/PlainShelf/default-shelf`. */
  pcloudShelfRoot: string;
}

/**
 * The configuration currently applied to this process.
 *
 * Kept here so callers that cannot await — provider construction runs
 * synchronously at bootstrap, before the router exists — can still see which
 * mode is active. main.ts awaits initMobileConfig() first, so it is populated
 * by the time anything asks.
 */
let currentConfig: MobileConnectionConfig | null = null;

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

async function readPCloudAccessToken(): Promise<string> {
  const { value } = await SecureStorage.get({ key: KEY_PCLOUD_ACCESS_TOKEN });
  const stored = (value ?? '').trim();
  if (stored) {
    // Finish cleanup if a previous migration encrypted the value but the app
    // stopped before removing the plaintext copy.
    await Preferences.remove({ key: KEY_PCLOUD_ACCESS_TOKEN });
    return stored;
  }

  // One-time migration for installs that predate Keystore-backed storage. Do
  // not remove the legacy value until the encrypted write has succeeded.
  const legacy = await read(KEY_PCLOUD_ACCESS_TOKEN);
  if (!legacy) {
    return '';
  }

  await SecureStorage.set({ key: KEY_PCLOUD_ACCESS_TOKEN, value: legacy });
  await Preferences.remove({ key: KEY_PCLOUD_ACCESS_TOKEN });
  return legacy;
}

async function writePCloudAccessToken(value: string): Promise<void> {
  const trimmed = (value ?? '').trim();
  if (trimmed) {
    await SecureStorage.set({ key: KEY_PCLOUD_ACCESS_TOKEN, value: trimmed });
  } else {
    await SecureStorage.remove({ key: KEY_PCLOUD_ACCESS_TOKEN });
  }

  // Also erase any plaintext copy left by an older app version.
  await Preferences.remove({ key: KEY_PCLOUD_ACCESS_TOKEN });
}

function normalizeMode(value: string): ConnectionMode {
  // Anything unrecognised — including the absent key on an install that predates
  // pCloud support — is a server connection.
  return value === 'pcloud' ? 'pcloud' : 'server';
}

export async function loadMobileConnectionConfig(): Promise<MobileConnectionConfig> {
  const [
    mode,
    serverUrl,
    token,
    shelfId,
    pcloudClientId,
    pcloudAccessToken,
    pcloudHost,
    pcloudShelfRoot
  ] = await Promise.all([
    read(KEY_MODE),
    read(KEY_SERVER_URL),
    read(KEY_TOKEN),
    read(KEY_SHELF_ID),
    read(KEY_PCLOUD_CLIENT_ID),
    readPCloudAccessToken(),
    read(KEY_PCLOUD_HOST),
    read(KEY_PCLOUD_SHELF_ROOT)
  ]);

  return {
    mode: normalizeMode(mode),
    serverUrl,
    token,
    shelfId,
    pcloudClientId,
    pcloudAccessToken,
    pcloudHost,
    pcloudShelfRoot
  };
}

/** The applied configuration, or null before initMobileConfig() has run. */
export function getMobileConnectionConfig(): MobileConnectionConfig | null {
  return currentConfig;
}

/**
 * Whether this configuration is complete enough to open the library.
 *
 * Each mode has its own answer, which is why the router guard asks here rather
 * than testing fields itself.
 */
export function isConnectionConfigured(config: MobileConnectionConfig): boolean {
  if (config.mode === 'pcloud') {
    return Boolean(config.pcloudAccessToken && config.pcloudHost && config.pcloudShelfRoot);
  }
  return Boolean(config.serverUrl && config.shelfId);
}

export async function saveMobileConnectionConfig(
  config: Partial<MobileConnectionConfig>
): Promise<void> {
  const entries: Array<[string, string | undefined]> = [
    [KEY_MODE, config.mode],
    [KEY_SERVER_URL, config.serverUrl],
    [KEY_TOKEN, config.token],
    [KEY_SHELF_ID, config.shelfId],
    [KEY_PCLOUD_CLIENT_ID, config.pcloudClientId],
    [KEY_PCLOUD_HOST, config.pcloudHost],
    [KEY_PCLOUD_SHELF_ROOT, config.pcloudShelfRoot]
  ];

  const writes: Array<Promise<void>> = entries
    .filter(([, value]) => value !== undefined)
    .map(([key, value]) => write(key, value as string));
  if (config.pcloudAccessToken !== undefined) {
    writes.push(writePCloudAccessToken(config.pcloudAccessToken));
  }

  await Promise.all(writes);

  await applyMobileConnectionConfig(await loadMobileConnectionConfig());
}

/**
 * Pushes connection settings into the API client. Called once at startup
 * (before the app mounts) and again whenever the settings UI saves new values.
 *
 * The API base and shelf id are also the identity that scopes device-local book
 * data (see providers/cacheScope.ts), so a pCloud connection sets them too even
 * though it never builds an HTTP URL from them. Using a `pcloud://` base has a
 * second, deliberate effect: any API call that slips through in pCloud mode
 * fails visibly instead of quietly resolving against the WebView's own origin.
 */
export async function applyMobileConnectionConfig(config: MobileConnectionConfig): Promise<void> {
  if (config.mode === 'pcloud') {
    setApiBase(config.pcloudHost ? `pcloud://${config.pcloudHost}` : '');
    setActiveShelfID(config.pcloudShelfRoot);
  } else {
    setApiBase(config.serverUrl);
    setActiveShelfID(config.shelfId);
  }

  currentConfig = config;

  // The API client reads the token via window.plainshelf.getApiToken (reusing
  // the Electron branch). Read live from Preferences so a token update applies
  // immediately without touching this closure.
  window.plainshelf = {
    getApiToken: async () => read(KEY_TOKEN)
  };
}

export async function initMobileConfig(): Promise<void> {
  await applyMobileConnectionConfig(await loadMobileConnectionConfig());
}
