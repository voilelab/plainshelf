import { Preferences } from '@capacitor/preferences';

import { normalizeApiBase, setActiveShelfID, setApiBase } from '@/api/client';
import { SecureStorage } from '@/providers/secureStorage';

// Native (Capacitor) builds load a static bundle with no backend to inject the
// server address, token, or selected shelf, so the device keeps its own list of
// shelves: a phone can hold a couple of PlainShelf servers and a couple of
// pCloud folders side by side and switch between them.
//
// Non-secret fields live in Capacitor Preferences as one JSON array; every
// token is kept per entry in Android Keystore-backed storage, so no secret is
// ever written into that array.
//
// A token is not what makes the app read-only — api/client.ts rejects every
// mutation before it is sent. A server token is only needed to read from a
// `protect_read` server.
const KEY_ENTRIES = 'plainshelf.mobile.shelves';
const KEY_ACTIVE_ENTRY_ID = 'plainshelf.mobile.activeShelfId';
const SECRET_KEY_PREFIX = 'plainshelf.mobile.secret.';

// Keys written by the single-connection layout this replaces. Read once during
// migration and then removed; nothing else may reference them.
const LEGACY_KEY_MODE = 'plainshelf.mobile.mode';
const LEGACY_KEY_SERVER_URL = 'plainshelf.mobile.serverUrl';
const LEGACY_KEY_TOKEN = 'plainshelf.mobile.token';
const LEGACY_KEY_SHELF_ID = 'plainshelf.mobile.shelfId';
const LEGACY_KEY_PCLOUD_CLIENT_ID = 'plainshelf.mobile.pcloud.clientId';
const LEGACY_KEY_PCLOUD_ACCESS_TOKEN = 'plainshelf.mobile.pcloud.accessToken';
const LEGACY_KEY_PCLOUD_HOST = 'plainshelf.mobile.pcloud.host';
const LEGACY_KEY_PCLOUD_SHELF_ROOT = 'plainshelf.mobile.pcloud.shelfRoot';

/**
 * Where one shelf on the device is read from.
 *
 * `server` is a PlainShelf server reachable over the network. `pcloud` reads a
 * shelf straight out of pCloud with no server involved — the one case a server
 * cannot cover, because a phone cannot mount cloud storage the way a host can.
 */
export type ShelfSourceType = 'server' | 'pcloud';

interface ShelfEntryBase {
  /**
   * Opaque device-local handle, never sent anywhere. Deliberately *not* the
   * cache scope key: device data is scoped by where the shelf lives (see
   * cacheScope.ts), so renaming an entry must not orphan its downloads.
   */
  id: string;
  /** What the shelf picker shows. */
  name: string;
}

export interface ServerShelfEntry extends ShelfEntryBase {
  type: 'server';
  serverUrl: string;
  /** Which shelf on that server, as the server names it. */
  shelfId: string;
}

export interface PCloudShelfEntry extends ShelfEntryBase {
  type: 'pcloud';
  /**
   * The user's own pCloud application key. PlainShelf ships no default: the
   * project registers no pCloud app, so each user supplies one they created.
   */
  pcloudClientId: string;
  /** Regional API host, learned during authorization rather than guessed. */
  pcloudHost: string;
  /** Path of the shelf directory on pCloud, e.g. `/PlainShelf/default-shelf`. */
  pcloudShelfRoot: string;
  /**
   * pCloud's numeric user id, recorded at authorization because host and folder
   * path alone are not unique: two accounts in one region can both hold
   * `/PlainShelf/shelf` and would otherwise share a cache scope. Optional, so
   * entries saved before it existed keep the scope their downloads live under.
   */
  pcloudUserId?: string;
}

export type ShelfEntry = ServerShelfEntry | PCloudShelfEntry;

/**
 * Kept here so callers that cannot await — provider construction runs
 * synchronously at bootstrap — can still see which shelf is active. main.ts
 * awaits initMobileConfig() first, so it is populated by the time anything asks.
 */
let entries: ShelfEntry[] = [];
let activeEntryID = '';
/**
 * The active entry's token, latched for the same reason the list is: the pCloud
 * provider is constructed synchronously at bootstrap and PCloudClient refuses
 * to exist without one. Only the active entry's secret is resolved eagerly —
 * the rest are read on demand, so a device with several pCloud shelves still
 * touches the Keystore once per launch.
 */
let activeEntryToken = '';

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

function secretKey(entryID: string): string {
  return `${SECRET_KEY_PREFIX}${entryID}`;
}

export async function getShelfEntryToken(entryID: string): Promise<string> {
  if (!entryID) {
    return '';
  }
  const { value } = await SecureStorage.get({ key: secretKey(entryID) });
  return (value ?? '').trim();
}

async function setShelfEntryToken(entryID: string, token: string): Promise<void> {
  const trimmed = (token ?? '').trim();
  if (trimmed) {
    await SecureStorage.set({ key: secretKey(entryID), value: trimmed });
  } else {
    await SecureStorage.remove({ key: secretKey(entryID) });
  }
}

/**
 * `getRandomValues` rather than `randomUUID`: the latter is secure-context
 * gated, and the `?mobile-shell-preview=1` harness is served over plain HTTP.
 * Not a secret either way.
 */
function newShelfEntryID(): string {
  const cryptoObj = globalThis.crypto;
  if (cryptoObj?.getRandomValues) {
    const bytes = new Uint8Array(16);
    cryptoObj.getRandomValues(bytes);
    return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
  }
  return `${Date.now().toString(16)}-${Math.random().toString(16).slice(2)}`;
}

function normalizeString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

/**
 * Defensive on purpose: a truncated or hand-edited blob must cost the user the
 * entries it can no longer describe, not the whole app. An entry missing its id
 * is dropped rather than regenerated — a new id would point its device data at
 * a scope that has nothing in it.
 */
function parseShelfEntry(raw: unknown): ShelfEntry | null {
  if (!raw || typeof raw !== 'object') {
    return null;
  }

  const record = raw as Record<string, unknown>;
  const id = normalizeString(record.id);
  if (!id) {
    return null;
  }
  const name = normalizeString(record.name);

  if (record.type === 'pcloud') {
    const pcloudUserId = normalizeString(record.pcloudUserId);
    return {
      id,
      type: 'pcloud',
      name,
      pcloudClientId: normalizeString(record.pcloudClientId),
      pcloudHost: normalizeString(record.pcloudHost),
      pcloudShelfRoot: normalizeString(record.pcloudShelfRoot),
      // Left off entirely when absent, so an entry that predates it keeps
      // deriving the scope key it already has data under.
      ...(pcloudUserId ? { pcloudUserId } : {})
    };
  }

  // Anything unrecognised is a server entry, matching how the single-connection
  // layout treated an absent or unknown mode.
  return {
    id,
    type: 'server',
    name,
    serverUrl: normalizeString(record.serverUrl),
    shelfId: normalizeString(record.shelfId)
  };
}

function parseShelfEntries(raw: string): ShelfEntry[] {
  if (!raw) {
    return [];
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return [];
  }

  if (!Array.isArray(parsed)) {
    return [];
  }
  return parsed.flatMap((item) => {
    const entry = parseShelfEntry(item);
    return entry ? [entry] : [];
  });
}

/**
 * The list is written even when empty — an absent key is what marks an install
 * as not yet migrated, so an emptied list must still leave a stored `[]` behind
 * rather than look like a fresh single-connection install on the next launch.
 */
async function persistShelfEntries(next: ShelfEntry[], nextActiveID: string): Promise<void> {
  await Promise.all([
    Preferences.set({ key: KEY_ENTRIES, value: JSON.stringify(next) }),
    write(KEY_ACTIVE_ENTRY_ID, nextActiveID)
  ]);
}

/**
 * Resolves the pCloud token of the single-connection layout, from either of the
 * two places it could be: Keystore-backed storage, or — on an install that
 * predates that — plaintext Preferences.
 */
async function readLegacyPCloudToken(): Promise<string> {
  const { value } = await SecureStorage.get({ key: LEGACY_KEY_PCLOUD_ACCESS_TOKEN });
  const stored = (value ?? '').trim();
  if (stored) {
    return stored;
  }
  return read(LEGACY_KEY_PCLOUD_ACCESS_TOKEN);
}

/**
 * The migrated entry must resolve to exactly the (API base, shelf id) pair the
 * old layout applied, because that pair is the cache scope key: anything else
 * leaves every downloaded book on disk but invisible.
 *
 * Ordering matters as much as the mapping: the new entry, its token and the
 * active id are all committed before a single legacy key is removed, so an
 * interrupted migration re-runs from an untouched source.
 */
async function migrateSingleConnection(): Promise<ShelfEntry[]> {
  const [mode, serverUrl, token, shelfId, pcloudClientId, pcloudHost, pcloudShelfRoot] =
    await Promise.all([
      read(LEGACY_KEY_MODE),
      read(LEGACY_KEY_SERVER_URL),
      read(LEGACY_KEY_TOKEN),
      read(LEGACY_KEY_SHELF_ID),
      read(LEGACY_KEY_PCLOUD_CLIENT_ID),
      read(LEGACY_KEY_PCLOUD_HOST),
      read(LEGACY_KEY_PCLOUD_SHELF_ROOT)
    ]);
  const pcloudAccessToken = await readLegacyPCloudToken();

  const id = newShelfEntryID();
  let entry: ShelfEntry | null = null;
  let secret = '';

  if (mode === 'pcloud') {
    if (pcloudHost || pcloudShelfRoot || pcloudClientId || pcloudAccessToken) {
      entry = {
        id,
        type: 'pcloud',
        name: pcloudShelfRootName(pcloudShelfRoot),
        pcloudClientId,
        pcloudHost,
        pcloudShelfRoot
      };
      secret = pcloudAccessToken;
    }
  } else if (serverUrl || shelfId) {
    entry = { id, type: 'server', name: shelfId || serverUrl, serverUrl, shelfId };
    secret = token;
  }

  if (!entry) {
    // Nothing worth carrying over — a fresh install, or a connection so
    // half-filled it could not be opened anyway. The empty list is still
    // written so this does not re-run on every launch.
    await persistShelfEntries([], '');
    await clearLegacyKeys();
    return [];
  }

  await setShelfEntryToken(id, secret);
  await persistShelfEntries([entry], id);
  await clearLegacyKeys();
  return [entry];
}

async function clearLegacyKeys(): Promise<void> {
  await Promise.all([
    Preferences.remove({ key: LEGACY_KEY_MODE }),
    Preferences.remove({ key: LEGACY_KEY_SERVER_URL }),
    Preferences.remove({ key: LEGACY_KEY_TOKEN }),
    Preferences.remove({ key: LEGACY_KEY_SHELF_ID }),
    Preferences.remove({ key: LEGACY_KEY_PCLOUD_CLIENT_ID }),
    Preferences.remove({ key: LEGACY_KEY_PCLOUD_HOST }),
    Preferences.remove({ key: LEGACY_KEY_PCLOUD_SHELF_ROOT }),
    Preferences.remove({ key: LEGACY_KEY_PCLOUD_ACCESS_TOKEN }),
    SecureStorage.remove({ key: LEGACY_KEY_PCLOUD_ACCESS_TOKEN })
  ]);
}

/** Last path segment of a pCloud folder, the same name its shelf shows under. */
function pcloudShelfRootName(shelfRoot: string): string {
  const segments = shelfRoot.split('/').filter((segment) => segment.length > 0);
  return segments[segments.length - 1] ?? shelfRoot;
}

/** A display name for an entry that was saved without one. */
export function shelfEntryDisplayName(entry: ShelfEntry): string {
  if (entry.name) {
    return entry.name;
  }
  return entry.type === 'pcloud'
    ? pcloudShelfRootName(entry.pcloudShelfRoot)
    : entry.shelfId || entry.serverUrl;
}

/**
 * Where an entry's library lives, in api/client.ts's terms — and the cache scope
 * identity, which is why both applying an entry and locating a deleted entry's
 * downloads go through this one function.
 *
 * A pCloud entry never builds an HTTP URL from these but still gets a
 * `pcloud://` base, so any API call that slips through fails visibly instead of
 * resolving against the WebView's own origin.
 */
export function shelfEntryTarget(entry: ShelfEntry | null): {
  apiBase: string;
  shelfID: string;
} {
  if (!entry) {
    return { apiBase: '', shelfID: '' };
  }
  if (entry.type === 'pcloud') {
    if (!entry.pcloudHost) {
      return { apiBase: '', shelfID: entry.pcloudShelfRoot };
    }
    // Part of the identity when known: two accounts both holding
    // `/PlainShelf/shelf` would otherwise share one cache, showing each other's
    // books and taking each other's downloads down on delete.
    const account = entry.pcloudUserId ? `/${entry.pcloudUserId}` : '';
    return {
      apiBase: normalizeApiBase(`pcloud://${entry.pcloudHost}${account}`),
      shelfID: entry.pcloudShelfRoot
    };
  }
  // Normalized here, not just on the way into the API client: a URL saved with
  // a trailing slash would otherwise scope one way while active and another way
  // when its downloads are located for deletion, so the delete would sweep a
  // directory that never existed.
  return { apiBase: normalizeApiBase(entry.serverUrl), shelfID: entry.shelfId };
}

/**
 * Field-level only, so it stays synchronous for the bootstrap path that must
 * choose a provider before anything can await. Whether the shelf can actually be
 * opened is isShelfEntryUsable(), which also needs the Keystore token.
 */
export function isShelfEntryConfigured(entry: ShelfEntry | null | undefined): boolean {
  if (!entry) {
    return false;
  }
  if (entry.type === 'pcloud') {
    return Boolean(entry.pcloudHost && entry.pcloudShelfRoot);
  }
  return Boolean(entry.serverUrl && entry.shelfId);
}

/**
 * What the router guard needs to know.
 *
 * A pCloud shelf is unusable without its bearer token — no anonymous read, no
 * server to refuse — so letting the guard through would land the user in a
 * library whose every read fails rather than on the form that re-authorizes it.
 * The token can be missing while the metadata survives: preferences restored
 * onto a device whose Keystore data did not come with them.
 *
 * A server shelf deliberately does not require one: a token is only needed for
 * `protect_read`, so demanding it would lock out a good connection.
 */
export async function isShelfEntryUsable(entry: ShelfEntry | null | undefined): Promise<boolean> {
  if (!isShelfEntryConfigured(entry) || !entry) {
    return false;
  }
  if (entry.type !== 'pcloud') {
    return true;
  }
  return Boolean(await getShelfEntryToken(entry.id));
}

/** Reads the stored list, migrating a single-connection install on first run. */
export async function loadShelfEntries(): Promise<{
  entries: ShelfEntry[];
  activeEntryID: string;
}> {
  const stored = await read(KEY_ENTRIES);
  if (!stored) {
    const migrated = await migrateSingleConnection();
    return { entries: migrated, activeEntryID: migrated[0]?.id ?? '' };
  }

  const loaded = parseShelfEntries(stored);
  const storedActiveID = await read(KEY_ACTIVE_ENTRY_ID);
  const active = loaded.some((entry) => entry.id === storedActiveID)
    ? storedActiveID
    : (loaded[0]?.id ?? '');
  return { entries: loaded, activeEntryID: active };
}

/** The applied list. Empty before initMobileConfig() has run. */
export function getShelfEntries(): ShelfEntry[] {
  return entries;
}

/** The applied active entry, or null when the device has none. */
export function getActiveShelfEntry(): ShelfEntry | null {
  return entries.find((entry) => entry.id === activeEntryID) ?? null;
}

export function getActiveShelfEntryID(): string {
  return activeEntryID;
}

/**
 * Called once at startup, before the app mounts, and again whenever the list
 * changes. The API base and shelf id are also the identity that scopes
 * device-local book data, so a pCloud entry sets them too.
 */
export async function applyActiveShelfEntry(): Promise<void> {
  const entry = getActiveShelfEntry();
  const { apiBase, shelfID } = shelfEntryTarget(entry);
  setApiBase(apiBase);
  setActiveShelfID(shelfID);

  const entryID = entry?.id ?? '';
  activeEntryToken = await getShelfEntryToken(entryID);

  // Resolved on each call rather than captured, so a token update applies
  // immediately without touching this closure.
  window.plainshelf = {
    getApiToken: async () => getShelfEntryToken(entryID)
  };
}

/**
 * The active entry's token for callers that cannot await — provider
 * construction at bootstrap. Everything else should read it live through
 * getShelfEntryToken().
 */
export function getActiveShelfEntryToken(): string {
  return activeEntryToken;
}

async function commit(next: ShelfEntry[], nextActiveID: string): Promise<void> {
  await persistShelfEntries(next, nextActiveID);
  entries = next;
  activeEntryID = nextActiveID;
  await applyActiveShelfEntry();
}

/**
 * Adds an entry or replaces one with the same id, and makes it active.
 *
 * `token` is written only when supplied, so editing an entry without retyping
 * its token keeps the stored one.
 */
export async function upsertShelfEntry(entry: ShelfEntry, token?: string): Promise<void> {
  if (token !== undefined) {
    await setShelfEntryToken(entry.id, token);
  }

  const index = entries.findIndex((existing) => existing.id === entry.id);
  const next = [...entries];
  if (index >= 0) {
    next[index] = entry;
  } else {
    next.push(entry);
  }
  await commit(next, entry.id);
}

/**
 * The caller is responsible for the entry's downloaded books — see
 * providers/cacheScope.ts for how to locate them. When the active entry goes,
 * the first remaining one takes over so the app has somewhere to open.
 */
export async function deleteShelfEntry(entryID: string): Promise<void> {
  const next = entries.filter((entry) => entry.id !== entryID);
  if (next.length === entries.length) {
    return;
  }

  await setShelfEntryToken(entryID, '');
  const nextActiveID = entryID === activeEntryID ? (next[0]?.id ?? '') : activeEntryID;
  await commit(next, nextActiveID);
}

/** Selects which entry the app reads from. Callers reload the app afterwards. */
export async function setActiveShelfEntry(entryID: string): Promise<void> {
  if (!entries.some((entry) => entry.id === entryID)) {
    return;
  }
  await commit(entries, entryID);
}

export function newShelfEntry(type: ShelfSourceType): ShelfEntry {
  const id = newShelfEntryID();
  return type === 'pcloud'
    ? { id, type, name: '', pcloudClientId: '', pcloudHost: '', pcloudShelfRoot: '' }
    : { id, type, name: '', serverUrl: '', shelfId: '' };
}

export async function initMobileConfig(): Promise<void> {
  const loaded = await loadShelfEntries();
  entries = loaded.entries;
  activeEntryID = loaded.activeEntryID;
  await applyActiveShelfEntry();
}
