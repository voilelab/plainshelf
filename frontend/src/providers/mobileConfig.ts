import { Preferences } from '@capacitor/preferences';

import { normalizeApiBase, setActiveShelfID, setApiBase } from '@/api/client';
import { LOCAL_API_BASE } from '@/providers/localShelf';
import { SecureStorage } from '@/providers/secureStorage';
import { shelfSourceNeedsSecret, type ShelfSourceKind } from '@/utils/shelfSourceCheck';

const KEY_ENTRIES = 'plainshelf.mobile.shelves';
const KEY_ACTIVE_ENTRY_ID = 'plainshelf.mobile.activeShelfId';
const SECRET_KEY_PREFIX = 'plainshelf.mobile.secret.';
const KEY_RESTORE_PROMPT_DISMISSED = 'plainshelf.mobile.restorePromptDismissed';

const LEGACY_KEY_MODE = 'plainshelf.mobile.mode';
const LEGACY_KEY_SERVER_URL = 'plainshelf.mobile.serverUrl';
const LEGACY_KEY_TOKEN = 'plainshelf.mobile.token';
const LEGACY_KEY_SHELF_ID = 'plainshelf.mobile.shelfId';
const LEGACY_KEY_PCLOUD_CLIENT_ID = 'plainshelf.mobile.pcloud.clientId';
const LEGACY_KEY_PCLOUD_ACCESS_TOKEN = 'plainshelf.mobile.pcloud.accessToken';
const LEGACY_KEY_PCLOUD_HOST = 'plainshelf.mobile.pcloud.host';
const LEGACY_KEY_PCLOUD_SHELF_ROOT = 'plainshelf.mobile.pcloud.shelfRoot';

export type ShelfSourceType = 'server' | 'pcloud' | 'local' | 'dav';

export const SOURCE_TYPE_LABEL_KEYS: Record<ShelfSourceType, string> = {
  server: 'mobileConnect.modeServer',
  pcloud: 'mobileConnect.modePCloud',
  local: 'mobileConnect.modeLocal',
  dav: 'mobileConnect.modeRemote'
};

interface ShelfEntryBase {
  id: string;
  name: string;
}

export interface ServerShelfEntry extends ShelfEntryBase {
  type: 'server';
  serverUrl: string;
  shelfId: string;
}

export interface PCloudShelfEntry extends ShelfEntryBase {
  type: 'pcloud';
  pcloudClientId: string;
  pcloudHost: string;
  pcloudShelfRoot: string;
  pcloudUserId?: string;
}

interface LocalShelfEntry extends ShelfEntryBase {
  type: 'local';
  localShelfId: string;
}

export interface DavShelfEntry extends ShelfEntryBase {
  type: 'dav';
  davSource: ShelfSourceKind;
  davUrl: string;
  davUsername?: string;
}

export type ShelfEntry = ServerShelfEntry | PCloudShelfEntry | LocalShelfEntry | DavShelfEntry;

const DAV_SHELF_ID = 'shelf';

let entries: ShelfEntry[] = [];
let activeEntryID = '';
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

// Web Crypto Level 2 restricts randomUUID to secure contexts; getRandomValues remains available on insecure LAN preview origins.
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
      ...(pcloudUserId ? { pcloudUserId } : {})
    };
  }

  if (record.type === 'local') {
    return { id, type: 'local', name, localShelfId: normalizeString(record.localShelfId) };
  }

  if (record.type === 'dav') {
    const davUsername = normalizeString(record.davUsername);
    return {
      id,
      type: 'dav',
      name,
      davSource: record.davSource === 'webdav' ? 'webdav' : 'plainshelf',
      davUrl: normalizeString(record.davUrl),
      ...(davUsername ? { davUsername } : {})
    };
  }

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

async function persistShelfEntries(next: ShelfEntry[], nextActiveID: string): Promise<void> {
  await Promise.all([
    Preferences.set({ key: KEY_ENTRIES, value: JSON.stringify(next) }),
    write(KEY_ACTIVE_ENTRY_ID, nextActiveID)
  ]);
}

async function readLegacyPCloudToken(): Promise<string> {
  const { value } = await SecureStorage.get({ key: LEGACY_KEY_PCLOUD_ACCESS_TOKEN });
  const stored = (value ?? '').trim();
  if (stored) {
    return stored;
  }
  return read(LEGACY_KEY_PCLOUD_ACCESS_TOKEN);
}

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

function pcloudShelfRootName(shelfRoot: string): string {
  const segments = shelfRoot.split('/').filter((segment) => segment.length > 0);
  return segments[segments.length - 1] ?? shelfRoot;
}

function parseURL(raw: string): URL | null {
  try {
    return new URL(raw);
  } catch {
    return null;
  }
}

function davHost(url: string): string {
  return parseURL(url)?.host || url;
}

function davScopeURL(entry: DavShelfEntry): string {
  const url = parseURL(entry.davUrl);
  if (!url) {
    return entry.davUrl;
  }
  url.username = entry.davUsername ?? '';
  url.password = '';
  url.search = '';
  url.hash = '';
  return url.toString();
}

export function shelfEntryDisplayName(entry: ShelfEntry): string {
  if (entry.name) {
    return entry.name;
  }
  if (entry.type === 'pcloud') {
    return pcloudShelfRootName(entry.pcloudShelfRoot);
  }
  if (entry.type === 'local') {
    return entry.localShelfId;
  }
  if (entry.type === 'dav') {
    return davHost(entry.davUrl);
  }
  return entry.shelfId || entry.serverUrl;
}

export function shelfEntryTarget(entry: ShelfEntry | null): {
  apiBase: string;
  shelfID: string;
} {
  if (!entry) {
    return { apiBase: '', shelfID: '' };
  }
  if (entry.type === 'local') {
    return { apiBase: LOCAL_API_BASE, shelfID: entry.localShelfId };
  }
  if (entry.type === 'dav') {
    if (!entry.davUrl) {
      return { apiBase: '', shelfID: DAV_SHELF_ID };
    }
    return { apiBase: normalizeApiBase(`${entry.davSource}+${davScopeURL(entry)}`), shelfID: DAV_SHELF_ID };
  }
  if (entry.type === 'pcloud') {
    if (!entry.pcloudHost) {
      return { apiBase: '', shelfID: entry.pcloudShelfRoot };
    }
    const account = entry.pcloudUserId ? `/${entry.pcloudUserId}` : '';
    return {
      apiBase: normalizeApiBase(`pcloud://${entry.pcloudHost}${account}`),
      shelfID: entry.pcloudShelfRoot
    };
  }
  return { apiBase: normalizeApiBase(entry.serverUrl), shelfID: entry.shelfId };
}

export function isShelfEntryConfigured(entry: ShelfEntry | null | undefined): boolean {
  if (!entry) {
    return false;
  }
  if (entry.type === 'pcloud') {
    return Boolean(entry.pcloudHost && entry.pcloudShelfRoot);
  }
  if (entry.type === 'local') {
    return Boolean(entry.localShelfId);
  }
  if (entry.type === 'dav') {
    return Boolean(entry.davUrl);
  }
  return Boolean(entry.serverUrl && entry.shelfId);
}

export async function isShelfEntryUsable(entry: ShelfEntry | null | undefined): Promise<boolean> {
  if (!isShelfEntryConfigured(entry) || !entry) {
    return false;
  }
  const needsToken =
    entry.type === 'pcloud' || (entry.type === 'dav' && shelfSourceNeedsSecret(entry.davSource, entry.davUsername));
  if (!needsToken) {
    return true;
  }
  return Boolean(await getShelfEntryToken(entry.id));
}

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

export function getShelfEntries(): ShelfEntry[] {
  return entries;
}

export function getActiveShelfEntry(): ShelfEntry | null {
  return entries.find((entry) => entry.id === activeEntryID) ?? null;
}

export function getActiveShelfEntryID(): string {
  return activeEntryID;
}

export async function applyActiveShelfEntry(): Promise<void> {
  const entry = getActiveShelfEntry();
  const { apiBase, shelfID } = shelfEntryTarget(entry);
  setApiBase(apiBase);
  setActiveShelfID(shelfID);

  const entryID = entry?.id ?? '';
  activeEntryToken = await getShelfEntryToken(entryID);

  window.plainshelf = {
    getApiToken: async () => getShelfEntryToken(entryID)
  };
}

export function getActiveShelfEntryToken(): string {
  return activeEntryToken;
}

async function commit(next: ShelfEntry[], nextActiveID: string): Promise<void> {
  await persistShelfEntries(next, nextActiveID);
  entries = next;
  activeEntryID = nextActiveID;
  await applyActiveShelfEntry();
}

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

export async function addShelfEntry(entry: ShelfEntry): Promise<void> {
  if (entries.some((existing) => existing.id === entry.id)) {
    return;
  }
  await commit([...entries, entry], activeEntryID);
}

export async function deleteShelfEntry(entryID: string): Promise<void> {
  const next = entries.filter((entry) => entry.id !== entryID);
  if (next.length === entries.length) {
    return;
  }

  await setShelfEntryToken(entryID, '');
  const nextActiveID = entryID === activeEntryID ? (next[0]?.id ?? '') : activeEntryID;
  await commit(next, nextActiveID);
}

export async function setActiveShelfEntry(entryID: string): Promise<void> {
  if (!entries.some((entry) => entry.id === entryID)) {
    return;
  }
  await commit(entries, entryID);
}

export async function isRestorePromptDismissed(): Promise<boolean> {
  return (await read(KEY_RESTORE_PROMPT_DISMISSED)) === 'true';
}

export async function dismissRestorePrompt(): Promise<void> {
  await write(KEY_RESTORE_PROMPT_DISMISSED, 'true');
}

export function newShelfEntry(type: ShelfSourceType): ShelfEntry {
  const id = newShelfEntryID();
  if (type === 'pcloud') {
    return { id, type, name: '', pcloudClientId: '', pcloudHost: '', pcloudShelfRoot: '' };
  }
  if (type === 'local') {
    return { id, type, name: '', localShelfId: '' };
  }
  if (type === 'dav') {
    return { id, type, name: '', davSource: 'plainshelf', davUrl: '' };
  }
  return { id, type, name: '', serverUrl: '', shelfId: '' };
}

export async function initMobileConfig(): Promise<void> {
  const loaded = await loadShelfEntries();
  entries = loaded.entries;
  activeEntryID = loaded.activeEntryID;
  await applyActiveShelfEntry();
}
