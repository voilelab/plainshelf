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

vi.mock('@/api/client', () => ({
  setApiBase: setApiBaseMock,
  setActiveShelfID: setActiveShelfIDMock
}));

Object.defineProperty(globalThis, 'window', {
  configurable: true,
  writable: true,
  value: {}
});

const {
  applyMobileConnectionConfig,
  getMobileConnectionConfig,
  isConnectionConfigured,
  loadMobileConnectionConfig,
  saveMobileConnectionConfig
} = await import('./mobileConfig');

type Config = Awaited<ReturnType<typeof loadMobileConnectionConfig>>;

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

function config(overrides: Partial<Config> = {}): Config {
  return {
    mode: 'server',
    serverUrl: '',
    token: '',
    shelfId: '',
    pcloudClientId: '',
    pcloudAccessToken: '',
    pcloudHost: '',
    pcloudShelfRoot: '',
    ...overrides
  };
}

describe('mobileConfig token wiring', () => {
  beforeEach(() => {
    resetStores();
    (window as unknown as { plainshelf?: unknown }).plainshelf = undefined;
    setApiBaseMock.mockReset();
    setActiveShelfIDMock.mockReset();
  });

  // The regression this pins: a `protect_read` server requires a token for
  // reads too, so without this wiring a native install cannot browse one at
  // all. The e2e suite cannot catch it — its preview loads index.html from the
  // Go server, which injects window.__PLAINSHELF_SECURITY__, a source a real
  // APK never has.
  it('exposes the stored token to the API client via window.plainshelf', async () => {
    store.set('plainshelf.mobile.token', 'secret-token');

    await applyMobileConnectionConfig(config({ serverUrl: 'http://host:20000' }));

    expect(await window.plainshelf?.getApiToken?.()).toBe('secret-token');
  });

  it('reads the token live, so an update applies without an app restart', async () => {
    store.set('plainshelf.mobile.token', 'first');
    await applyMobileConnectionConfig(config({ serverUrl: 'http://host:20000' }));
    expect(await window.plainshelf?.getApiToken?.()).toBe('first');

    store.set('plainshelf.mobile.token', 'second');
    expect(await window.plainshelf?.getApiToken?.()).toBe('second');
  });

  it('persists and reloads the token alongside the server URL and shelf', async () => {
    await saveMobileConnectionConfig({
      serverUrl: 'http://host:20000',
      token: 'secret-token',
      shelfId: 'main'
    });

    expect(await loadMobileConnectionConfig()).toEqual(
      config({ serverUrl: 'http://host:20000', token: 'secret-token', shelfId: 'main' })
    );
  });

  it('clears a stored token when saved empty', async () => {
    store.set('plainshelf.mobile.token', 'secret-token');

    await saveMobileConnectionConfig({ token: '' });

    expect(store.has('plainshelf.mobile.token')).toBe(false);
    expect(await window.plainshelf?.getApiToken?.()).toBe('');
  });
});

describe('connection mode', () => {
  beforeEach(() => {
    resetStores();
    setApiBaseMock.mockReset();
    setActiveShelfIDMock.mockReset();
  });

  // An install that predates pCloud support has no mode key stored.
  it('treats a missing or unknown mode as a server connection', async () => {
    expect((await loadMobileConnectionConfig()).mode).toBe('server');

    store.set('plainshelf.mobile.mode', 'something-else');
    expect((await loadMobileConnectionConfig()).mode).toBe('server');
  });

  it('applies the server URL and shelf id in server mode', async () => {
    await applyMobileConnectionConfig(
      config({ serverUrl: 'http://host:20000', shelfId: 'main' })
    );

    expect(setApiBaseMock).toHaveBeenCalledWith('http://host:20000');
    expect(setActiveShelfIDMock).toHaveBeenCalledWith('main');
  });

  // The API base and shelf id double as the identity that scopes device-local
  // book data (providers/cacheScope.ts). Left empty, two pCloud folders — or a
  // pCloud folder and a same-origin web install — would share one cache.
  it('derives a distinct cache identity in pCloud mode', async () => {
    await applyMobileConnectionConfig(
      config({
        mode: 'pcloud',
        pcloudHost: 'eapi.pcloud.com',
        pcloudShelfRoot: '/PlainShelf/default-shelf'
      })
    );

    expect(setApiBaseMock).toHaveBeenCalledWith('pcloud://eapi.pcloud.com');
    expect(setActiveShelfIDMock).toHaveBeenCalledWith('/PlainShelf/default-shelf');
  });

  it('ignores the server fields while in pCloud mode', async () => {
    await applyMobileConnectionConfig(
      config({
        mode: 'pcloud',
        serverUrl: 'http://host:20000',
        shelfId: 'main',
        pcloudHost: 'api.pcloud.com',
        pcloudShelfRoot: '/shelf'
      })
    );

    expect(setApiBaseMock).toHaveBeenCalledWith('pcloud://api.pcloud.com');
    expect(setActiveShelfIDMock).toHaveBeenCalledWith('/shelf');
  });

  it('round-trips pCloud settings', async () => {
    await saveMobileConnectionConfig({
      mode: 'pcloud',
      pcloudClientId: 'user-app-key',
      pcloudAccessToken: 'tok',
      pcloudHost: 'api.pcloud.com',
      pcloudShelfRoot: '/PlainShelf/shelf'
    });

    expect(await loadMobileConnectionConfig()).toEqual(
      config({
        mode: 'pcloud',
        pcloudClientId: 'user-app-key',
        pcloudAccessToken: 'tok',
        pcloudHost: 'api.pcloud.com',
        pcloudShelfRoot: '/PlainShelf/shelf'
      })
    );
    expect(secureStore.get('plainshelf.mobile.pcloud.accessToken')).toBe('tok');
    expect(store.has('plainshelf.mobile.pcloud.accessToken')).toBe(false);
  });

  it('migrates a legacy plaintext pCloud token only after encrypting it', async () => {
    store.set('plainshelf.mobile.pcloud.accessToken', 'legacy-token');

    expect((await loadMobileConnectionConfig()).pcloudAccessToken).toBe('legacy-token');
    expect(secureStore.get('plainshelf.mobile.pcloud.accessToken')).toBe('legacy-token');
    expect(store.has('plainshelf.mobile.pcloud.accessToken')).toBe(false);
    expect(secureSet).toHaveBeenCalledBefore(prefRemove);
  });

  it('removes a stale legacy copy when an encrypted pCloud token already exists', async () => {
    secureStore.set('plainshelf.mobile.pcloud.accessToken', 'encrypted-token');
    store.set('plainshelf.mobile.pcloud.accessToken', 'legacy-token');

    expect((await loadMobileConnectionConfig()).pcloudAccessToken).toBe('encrypted-token');
    expect(store.has('plainshelf.mobile.pcloud.accessToken')).toBe(false);
  });

  it('does not delete a legacy pCloud token when encrypted migration fails', async () => {
    store.set('plainshelf.mobile.pcloud.accessToken', 'legacy-token');
    secureSet.mockRejectedValueOnce(new Error('Keystore unavailable'));

    await expect(loadMobileConnectionConfig()).rejects.toThrow('Keystore unavailable');
    expect(store.get('plainshelf.mobile.pcloud.accessToken')).toBe('legacy-token');
  });

  it('clears both encrypted and legacy copies of the pCloud token', async () => {
    secureStore.set('plainshelf.mobile.pcloud.accessToken', 'encrypted-token');
    store.set('plainshelf.mobile.pcloud.accessToken', 'legacy-token');

    await saveMobileConnectionConfig({ pcloudAccessToken: '' });

    expect(secureStore.has('plainshelf.mobile.pcloud.accessToken')).toBe(false);
    expect(store.has('plainshelf.mobile.pcloud.accessToken')).toBe(false);
  });

  it('publishes the applied config for synchronous callers', async () => {
    // Provider construction runs at bootstrap, before anything can await.
    await applyMobileConnectionConfig(config({ mode: 'pcloud', pcloudHost: 'api.pcloud.com' }));

    expect(getMobileConnectionConfig()?.mode).toBe('pcloud');
  });

  describe('isConnectionConfigured', () => {
    it('requires a server URL and a shelf in server mode', () => {
      expect(isConnectionConfigured(config({ serverUrl: 'http://h', shelfId: 'main' }))).toBe(true);
      expect(isConnectionConfigured(config({ serverUrl: 'http://h' }))).toBe(false);
      expect(isConnectionConfigured(config({ shelfId: 'main' }))).toBe(false);
    });

    it('requires a token, a host and a folder in pCloud mode', () => {
      const complete = config({
        mode: 'pcloud',
        pcloudAccessToken: 'tok',
        pcloudHost: 'api.pcloud.com',
        pcloudShelfRoot: '/shelf'
      });

      expect(isConnectionConfigured(complete)).toBe(true);
      expect(isConnectionConfigured({ ...complete, pcloudAccessToken: '' })).toBe(false);
      expect(isConnectionConfigured({ ...complete, pcloudHost: '' })).toBe(false);
      expect(isConnectionConfigured({ ...complete, pcloudShelfRoot: '' })).toBe(false);
    });

    // A server URL left over from a previous connection must not satisfy the
    // pCloud check, or the router would let an unconfigured shelf through.
    it('does not let one mode satisfy the other', () => {
      expect(
        isConnectionConfigured(config({ mode: 'pcloud', serverUrl: 'http://h', shelfId: 'main' }))
      ).toBe(false);
    });
  });
});
