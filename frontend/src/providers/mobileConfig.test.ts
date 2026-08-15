import { beforeEach, describe, expect, it, vi } from 'vitest';

const {
  prefGet,
  prefSet,
  prefRemove,
  secureGet,
  secureSet,
  secureRemove,
  setApiBaseMock,
  setActiveShelfIDMock
} = vi.hoisted(() => ({
  prefGet: vi.fn(),
  prefSet: vi.fn(),
  prefRemove: vi.fn(),
  secureGet: vi.fn(),
  secureSet: vi.fn(),
  secureRemove: vi.fn(),
  setApiBaseMock: vi.fn(),
  setActiveShelfIDMock: vi.fn()
}));

vi.mock('@capacitor/preferences', () => ({
  Preferences: { get: prefGet, set: prefSet, remove: prefRemove }
}));

vi.mock('@/providers/secureStorage', () => ({
  SecureStorage: { get: secureGet, set: secureSet, remove: secureRemove }
}));

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client');
  return {
    normalizeApiBase: actual.normalizeApiBase,
    setApiBase: setApiBaseMock,
    setActiveShelfID: setActiveShelfIDMock
  };
});

// api/client.ts reads the persisted shelf id at module load, and this suite
// imports it for real to prove shelfEntryTarget normalizes a base exactly the
// way setApiBase does.
Object.defineProperty(globalThis, 'window', {
  configurable: true,
  writable: true,
  value: {
    localStorage: {
      getItem: () => null,
      setItem: () => undefined,
      removeItem: () => undefined
    }
  }
});

const {
  applyActiveShelfEntry,
  deleteShelfEntry,
  getActiveShelfEntry,
  getActiveShelfEntryToken,
  getShelfEntries,
  initMobileConfig,
  isShelfEntryConfigured,
  isShelfEntryUsable,
  loadShelfEntries,
  newShelfEntry,
  setActiveShelfEntry,
  shelfEntryDisplayName,
  shelfEntryTarget,
  upsertShelfEntry
} = await import('./mobileConfig');
const { buildDeviceDocumentKey } = await import('@/storage/deviceDocument');

type ServerShelfEntry = import('./mobileConfig').ServerShelfEntry;
type PCloudShelfEntry = import('./mobileConfig').PCloudShelfEntry;

const ENTRIES_KEY = 'plainshelf.mobile.shelves';
const ACTIVE_KEY = 'plainshelf.mobile.activeShelfId';

const store = new Map<string, string>();
const secureStore = new Map<string, string>();

function resetStores(): void {
  store.clear();
  secureStore.clear();
  prefGet.mockReset().mockImplementation(async ({ key }: { key: string }) => ({
    value: store.get(key) ?? null
  }));
  prefSet.mockReset().mockImplementation(async ({ key, value }: { key: string; value: string }) => {
    store.set(key, value);
  });
  prefRemove.mockReset().mockImplementation(async ({ key }: { key: string }) => {
    store.delete(key);
  });
  secureGet.mockReset().mockImplementation(async ({ key }: { key: string }) => ({
    value: secureStore.get(key) ?? null
  }));
  secureSet.mockReset().mockImplementation(
    async ({ key, value }: { key: string; value: string }) => {
      secureStore.set(key, value);
    }
  );
  secureRemove.mockReset().mockImplementation(async ({ key }: { key: string }) => {
    secureStore.delete(key);
  });
}

function serverEntry(overrides: Partial<ServerShelfEntry> = {}): ServerShelfEntry {
  return {
    id: 'entry-1',
    type: 'server',
    name: 'Home',
    serverUrl: 'http://host:20000',
    shelfId: 'main',
    ...overrides
  };
}

function pcloudEntry(overrides: Partial<PCloudShelfEntry> = {}): PCloudShelfEntry {
  return {
    id: 'entry-2',
    type: 'pcloud',
    name: 'Cloud',
    pcloudClientId: 'user-app-key',
    pcloudHost: 'api.pcloud.com',
    pcloudShelfRoot: '/PlainShelf/shelf',
    ...overrides
  };
}

/** The cache scope key providers/cacheScope.ts would derive after applying. */
function appliedScopeKey(): string {
  const apiBase = setApiBaseMock.mock.lastCall?.[0] ?? '';
  const shelfID = setActiveShelfIDMock.mock.lastCall?.[0] ?? '';
  return buildDeviceDocumentKey(apiBase, shelfID);
}

/** Puts the module back in its pre-init state between cases. */
async function reinit(): Promise<void> {
  await initMobileConfig();
}

beforeEach(() => {
  resetStores();
  (window as unknown as { plainshelf?: unknown }).plainshelf = undefined;
  setApiBaseMock.mockReset();
  setActiveShelfIDMock.mockReset();
});

describe('token wiring', () => {
  // The regression this pins: a `protect_read` server requires a token for
  // reads too, so without this wiring a native install cannot browse one at
  // all. The e2e suite cannot catch it — its preview loads index.html from the
  // Go server, which injects window.__PLAINSHELF_SECURITY__, a source a real
  // APK never has.
  it('exposes the active entry token to the API client via window.plainshelf', async () => {
    store.set(ENTRIES_KEY, JSON.stringify([serverEntry()]));
    store.set(ACTIVE_KEY, 'entry-1');
    secureStore.set('plainshelf.mobile.secret.entry-1', 'secret-token');

    await reinit();

    expect(await window.plainshelf?.getApiToken?.()).toBe('secret-token');
  });

  it('reads the token live, so an update applies without an app restart', async () => {
    store.set(ENTRIES_KEY, JSON.stringify([serverEntry()]));
    store.set(ACTIVE_KEY, 'entry-1');
    secureStore.set('plainshelf.mobile.secret.entry-1', 'first');
    await reinit();
    expect(await window.plainshelf?.getApiToken?.()).toBe('first');

    secureStore.set('plainshelf.mobile.secret.entry-1', 'second');
    expect(await window.plainshelf?.getApiToken?.()).toBe('second');
  });

  // Provider construction runs at bootstrap and cannot await, so the active
  // entry's secret is also latched.
  it('latches the active token for synchronous callers', async () => {
    store.set(ENTRIES_KEY, JSON.stringify([pcloudEntry()]));
    store.set(ACTIVE_KEY, 'entry-2');
    secureStore.set('plainshelf.mobile.secret.entry-2', 'pcloud-token');

    await reinit();

    expect(getActiveShelfEntryToken()).toBe('pcloud-token');
  });

  it('keeps each entry token separate, so two pCloud accounts do not share one', async () => {
    await reinit();
    await upsertShelfEntry(pcloudEntry({ id: 'a' }), 'token-a');
    await upsertShelfEntry(pcloudEntry({ id: 'b' }), 'token-b');

    expect(secureStore.get('plainshelf.mobile.secret.a')).toBe('token-a');
    expect(secureStore.get('plainshelf.mobile.secret.b')).toBe('token-b');
  });
});

describe('applying an entry', () => {
  it('applies the server URL and shelf id for a server entry', async () => {
    await reinit();
    await upsertShelfEntry(serverEntry());

    expect(setApiBaseMock).toHaveBeenLastCalledWith('http://host:20000');
    expect(setActiveShelfIDMock).toHaveBeenLastCalledWith('main');
  });

  // The API base and shelf id double as the identity that scopes device-local
  // book data (providers/cacheScope.ts). Left empty, two pCloud folders — or a
  // pCloud folder and a same-origin web install — would share one cache.
  it('derives a distinct cache identity for a pCloud entry', async () => {
    await reinit();
    await upsertShelfEntry(
      pcloudEntry({ pcloudHost: 'eapi.pcloud.com', pcloudShelfRoot: '/PlainShelf/default-shelf' })
    );

    expect(setApiBaseMock).toHaveBeenLastCalledWith('pcloud://eapi.pcloud.com');
    expect(setActiveShelfIDMock).toHaveBeenLastCalledWith('/PlainShelf/default-shelf');
  });

  // setApiBase normalizes what it is given, so a server URL saved with a
  // trailing slash produced one scope key while the shelf was active and a
  // different one when its downloads were located for deletion — the delete
  // then swept a directory that never existed and left the real one on disk.
  it('normalizes a server URL the same way the API client does', () => {
    expect(shelfEntryTarget(serverEntry({ serverUrl: 'http://host:20000/' }))).toEqual({
      apiBase: 'http://host:20000',
      shelfID: 'main'
    });
    expect(shelfEntryTarget(serverEntry({ serverUrl: ' http://host:20000// ' })).apiBase).toBe(
      'http://host:20000'
    );
  });

  it('separates two folders in one pCloud account', async () => {
    expect(shelfEntryTarget(pcloudEntry({ pcloudShelfRoot: '/a' }))).toEqual({
      apiBase: 'pcloud://api.pcloud.com',
      shelfID: '/a'
    });
    expect(shelfEntryTarget(pcloudEntry({ pcloudShelfRoot: '/b' }))).toEqual({
      apiBase: 'pcloud://api.pcloud.com',
      shelfID: '/b'
    });
  });

  // Host and folder path are not unique across accounts. Without the account
  // id these two share one cache: each shows the other's books, and removing
  // either deletes the other's downloads.
  it('separates the same folder path in two pCloud accounts', () => {
    const first = shelfEntryTarget(pcloudEntry({ pcloudUserId: '111', pcloudShelfRoot: '/shelf' }));
    const second = shelfEntryTarget(pcloudEntry({ pcloudUserId: '222', pcloudShelfRoot: '/shelf' }));

    expect(first).toEqual({ apiBase: 'pcloud://api.pcloud.com/111', shelfID: '/shelf' });
    expect(first).not.toEqual(second);
  });

  // The account id is recorded at authorization, so entries that predate it
  // have none — and must keep deriving the scope their downloads live under.
  it('leaves the scope unchanged for an entry with no recorded account', () => {
    expect(shelfEntryTarget(pcloudEntry({ pcloudShelfRoot: '/shelf' })).apiBase).toBe(
      'pcloud://api.pcloud.com'
    );
  });

  it('does not invent an account id when reloading a stored entry', async () => {
    store.set(ENTRIES_KEY, JSON.stringify([pcloudEntry()]));
    store.set(ACTIVE_KEY, 'entry-2');

    const [reloaded] = (await loadShelfEntries()).entries;
    expect(reloaded && 'pcloudUserId' in reloaded).toBe(false);
  });

  it('keeps a recorded account id across a save and reload', async () => {
    await reinit();
    await upsertShelfEntry(pcloudEntry({ pcloudUserId: '111' }));

    const [reloaded] = (await loadShelfEntries()).entries;
    expect(reloaded).toMatchObject({ pcloudUserId: '111' });
  });

  it('clears the client when the device has no shelf left', async () => {
    store.set(ENTRIES_KEY, JSON.stringify([serverEntry()]));
    store.set(ACTIVE_KEY, 'entry-1');
    await reinit();

    await deleteShelfEntry('entry-1');

    expect(setApiBaseMock).toHaveBeenLastCalledWith('');
    expect(setActiveShelfIDMock).toHaveBeenLastCalledWith('');
    expect(getActiveShelfEntry()).toBeNull();
  });
});

describe('the entry list', () => {
  it('adds, replaces by id, and makes the saved entry active', async () => {
    await reinit();
    await upsertShelfEntry(serverEntry());
    await upsertShelfEntry(pcloudEntry());
    expect(getShelfEntries()).toHaveLength(2);
    expect(getActiveShelfEntry()?.id).toBe('entry-2');

    await upsertShelfEntry(serverEntry({ name: 'Renamed' }));

    expect(getShelfEntries()).toHaveLength(2);
    expect(getShelfEntries()[0]?.name).toBe('Renamed');
    expect(getActiveShelfEntry()?.id).toBe('entry-1');
  });

  it('drops an entry and its token', async () => {
    await reinit();
    await upsertShelfEntry(serverEntry(), 'tok');
    await upsertShelfEntry(pcloudEntry(), 'pcloud-tok');

    await deleteShelfEntry('entry-1');

    expect(getShelfEntries().map((entry) => entry.id)).toEqual(['entry-2']);
    expect(secureStore.has('plainshelf.mobile.secret.entry-1')).toBe(false);
  });

  it('promotes the first remaining entry when the active one is deleted', async () => {
    await reinit();
    await upsertShelfEntry(serverEntry());
    await upsertShelfEntry(pcloudEntry());

    await deleteShelfEntry('entry-2');

    expect(getActiveShelfEntry()?.id).toBe('entry-1');
  });

  it('refuses to activate an id that is not in the list', async () => {
    await reinit();
    await upsertShelfEntry(serverEntry());

    await setActiveShelfEntry('nope');

    expect(getActiveShelfEntry()?.id).toBe('entry-1');
  });

  it('survives an emptied list without looking like a fresh install', async () => {
    await reinit();
    await upsertShelfEntry(serverEntry());
    await deleteShelfEntry('entry-1');

    // An absent key is what marks an install as not yet migrated, so an
    // emptied list must still leave a stored list behind.
    expect(store.get(ENTRIES_KEY)).toBe('[]');
  });

  it('falls back to the first entry when the stored active id is stale', async () => {
    store.set(ENTRIES_KEY, JSON.stringify([serverEntry(), pcloudEntry()]));
    store.set(ACTIVE_KEY, 'deleted-long-ago');

    expect((await loadShelfEntries()).activeEntryID).toBe('entry-1');
  });

  it('mints a distinct id per new entry', () => {
    expect(newShelfEntry('server').id).not.toBe(newShelfEntry('server').id);
  });
});

// The whole list is read before the app mounts and again on every navigation,
// so a blob it cannot understand has to degrade to "no shelves" — which the
// router answers with the setup page — rather than take the app down.
describe('a damaged list', () => {
  it.each([
    ['not JSON', 'not json at all'],
    ['not an array', '{"entries":[]}'],
    ['an entry with no id', '[{"type":"server","serverUrl":"http://h"}]']
  ])('parses %s as an empty list', async (_label, raw) => {
    store.set(ENTRIES_KEY, raw);

    await expect(loadShelfEntries()).resolves.toEqual({ entries: [], activeEntryID: '' });
  });

  it('keeps the entries it can still read', async () => {
    store.set(ENTRIES_KEY, JSON.stringify([serverEntry(), { type: 'server' }]));

    expect((await loadShelfEntries()).entries).toHaveLength(1);
  });

  // An entry stored before pCloud support, or with a type nobody recognises,
  // is a server entry — the same rule the single-connection layout applied to
  // an absent mode key.
  it('reads an unknown type as a server entry', async () => {
    store.set(ENTRIES_KEY, JSON.stringify([{ id: 'x', type: 'something-else', shelfId: 'main' }]));

    expect((await loadShelfEntries()).entries[0]?.type).toBe('server');
  });
});

// The load-bearing property: an install upgraded from the single-connection
// layout must resolve to exactly the (API base, shelf id) pair it had before,
// because that pair is the cache scope key. Anything else leaves every
// downloaded book on disk but invisible.
describe('migration from the single connection', () => {
  it('carries a server connection over with its cache scope intact', async () => {
    store.set('plainshelf.mobile.mode', 'server');
    store.set('plainshelf.mobile.serverUrl', 'http://192.168.1.10:20000');
    store.set('plainshelf.mobile.shelfId', 'default_shelf');
    store.set('plainshelf.mobile.token', 'legacy-token');

    await reinit();

    const entry = getActiveShelfEntry();
    expect(entry).toMatchObject({
      type: 'server',
      serverUrl: 'http://192.168.1.10:20000',
      shelfId: 'default_shelf'
    });
    expect(appliedScopeKey()).toBe('http://192.168.1.10:20000|default_shelf');
    expect(secureStore.get(`plainshelf.mobile.secret.${entry?.id}`)).toBe('legacy-token');
    expect(store.has('plainshelf.mobile.token')).toBe(false);
    expect(store.has('plainshelf.mobile.serverUrl')).toBe(false);
  });

  it('carries a pCloud connection over with its cache scope intact', async () => {
    store.set('plainshelf.mobile.mode', 'pcloud');
    store.set('plainshelf.mobile.pcloud.clientId', 'user-app-key');
    store.set('plainshelf.mobile.pcloud.host', 'eapi.pcloud.com');
    store.set('plainshelf.mobile.pcloud.shelfRoot', '/PlainShelf/default-shelf');
    secureStore.set('plainshelf.mobile.pcloud.accessToken', 'encrypted-token');

    await reinit();

    const entry = getActiveShelfEntry();
    expect(entry).toMatchObject({
      type: 'pcloud',
      pcloudClientId: 'user-app-key',
      pcloudHost: 'eapi.pcloud.com',
      pcloudShelfRoot: '/PlainShelf/default-shelf'
    });
    expect(appliedScopeKey()).toBe('pcloud://eapi.pcloud.com|/PlainShelf/default-shelf');
    expect(secureStore.get(`plainshelf.mobile.secret.${entry?.id}`)).toBe('encrypted-token');
    expect(secureStore.has('plainshelf.mobile.pcloud.accessToken')).toBe(false);
  });

  // Installs that predate Keystore-backed storage kept the pCloud token in
  // plaintext Preferences.
  it('picks up a plaintext pCloud token and encrypts it', async () => {
    store.set('plainshelf.mobile.mode', 'pcloud');
    store.set('plainshelf.mobile.pcloud.host', 'api.pcloud.com');
    store.set('plainshelf.mobile.pcloud.shelfRoot', '/shelf');
    store.set('plainshelf.mobile.pcloud.accessToken', 'plaintext-token');

    await reinit();

    const entry = getActiveShelfEntry();
    expect(secureStore.get(`plainshelf.mobile.secret.${entry?.id}`)).toBe('plaintext-token');
    expect(store.has('plainshelf.mobile.pcloud.accessToken')).toBe(false);
  });

  // The whole point of the ordering: a migration that dies partway must be
  // resumable from an untouched source rather than lose the connection.
  it('leaves the old keys alone when writing the new token fails', async () => {
    store.set('plainshelf.mobile.mode', 'server');
    store.set('plainshelf.mobile.serverUrl', 'http://host:20000');
    store.set('plainshelf.mobile.shelfId', 'main');
    store.set('plainshelf.mobile.token', 'legacy-token');
    secureSet.mockRejectedValueOnce(new Error('Keystore unavailable'));

    await expect(loadShelfEntries()).rejects.toThrow('Keystore unavailable');

    expect(store.get('plainshelf.mobile.token')).toBe('legacy-token');
    expect(store.get('plainshelf.mobile.serverUrl')).toBe('http://host:20000');
    expect(store.has(ENTRIES_KEY)).toBe(false);
  });

  it('does not run twice', async () => {
    store.set('plainshelf.mobile.mode', 'server');
    store.set('plainshelf.mobile.serverUrl', 'http://host:20000');
    store.set('plainshelf.mobile.shelfId', 'main');

    const first = await loadShelfEntries();
    const second = await loadShelfEntries();

    expect(second.entries).toHaveLength(1);
    expect(second.entries[0]?.id).toBe(first.entries[0]?.id);
  });

  it('invents nothing on a fresh install', async () => {
    await expect(loadShelfEntries()).resolves.toEqual({ entries: [], activeEntryID: '' });
    expect(store.get(ENTRIES_KEY)).toBe('[]');
  });

  it('invents nothing from a half-filled connection', async () => {
    store.set('plainshelf.mobile.mode', 'server');
    store.set('plainshelf.mobile.token', 'orphan-token');

    expect((await loadShelfEntries()).entries).toEqual([]);
  });
});

describe('isShelfEntryConfigured', () => {
  it('requires a server URL and a shelf for a server entry', () => {
    expect(isShelfEntryConfigured(serverEntry())).toBe(true);
    expect(isShelfEntryConfigured(serverEntry({ shelfId: '' }))).toBe(false);
    expect(isShelfEntryConfigured(serverEntry({ serverUrl: '' }))).toBe(false);
  });

  it('requires a host and a folder for a pCloud entry', () => {
    expect(isShelfEntryConfigured(pcloudEntry())).toBe(true);
    expect(isShelfEntryConfigured(pcloudEntry({ pcloudHost: '' }))).toBe(false);
    expect(isShelfEntryConfigured(pcloudEntry({ pcloudShelfRoot: '' }))).toBe(false);
  });

  it('treats a device with no active entry as unconfigured', () => {
    expect(isShelfEntryConfigured(null)).toBe(false);
  });
});

describe('isShelfEntryUsable', () => {
  // Without this the router lets the user into a library whose every read
  // fails: createMobileSource falls back to the server provider while the API
  // base is still `pcloud://…`, so nothing can answer. The form that could
  // re-authorize is the only useful destination.
  it('refuses a pCloud entry whose token is gone', async () => {
    store.set(ENTRIES_KEY, JSON.stringify([pcloudEntry()]));
    store.set(ACTIVE_KEY, 'entry-2');
    await reinit();

    await expect(isShelfEntryUsable(pcloudEntry())).resolves.toBe(false);

    secureStore.set('plainshelf.mobile.secret.entry-2', 'tok');
    await expect(isShelfEntryUsable(pcloudEntry())).resolves.toBe(true);
  });

  // The opposite rule for a server: a token is only needed when the server sets
  // `protect_read`, so requiring one would lock a working connection out.
  it('accepts a server entry with no token', async () => {
    await expect(isShelfEntryUsable(serverEntry())).resolves.toBe(true);
  });

  it('refuses an incomplete entry and a device with none', async () => {
    await expect(isShelfEntryUsable(serverEntry({ shelfId: '' }))).resolves.toBe(false);
    await expect(isShelfEntryUsable(null)).resolves.toBe(false);
  });
});

describe('shelfEntryDisplayName', () => {
  it('prefers the name the user gave it', () => {
    expect(shelfEntryDisplayName(serverEntry({ name: 'Living room' }))).toBe('Living room');
  });

  it('falls back to the shelf id, then the folder name', () => {
    expect(shelfEntryDisplayName(serverEntry({ name: '' }))).toBe('main');
    expect(shelfEntryDisplayName(pcloudEntry({ name: '' }))).toBe('shelf');
  });
});

describe('applyActiveShelfEntry', () => {
  it('is idempotent, so a form can restore it after probing a server', async () => {
    store.set(ENTRIES_KEY, JSON.stringify([serverEntry()]));
    store.set(ACTIVE_KEY, 'entry-1');
    await reinit();

    setApiBaseMock.mockClear();
    await applyActiveShelfEntry();

    expect(setApiBaseMock).toHaveBeenCalledWith('http://host:20000');
  });
});
