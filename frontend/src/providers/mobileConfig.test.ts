import { beforeEach, describe, expect, it, vi } from 'vitest';

const { prefGet, prefSet, prefRemove, setApiBaseMock, setActiveShelfIDMock } = vi.hoisted(() => ({
  prefGet: vi.fn(),
  prefSet: vi.fn(),
  prefRemove: vi.fn(),
  setApiBaseMock: vi.fn(),
  setActiveShelfIDMock: vi.fn()
}));

vi.mock('@capacitor/preferences', () => ({
  Preferences: { get: prefGet, set: prefSet, remove: prefRemove }
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

const { applyMobileConnectionConfig, loadMobileConnectionConfig, saveMobileConnectionConfig } =
  await import('./mobileConfig');

const store = new Map<string, string>();

describe('mobileConfig token wiring', () => {
  beforeEach(() => {
    store.clear();
    (window as unknown as { plainshelf?: unknown }).plainshelf = undefined;
    prefGet.mockReset().mockImplementation(async ({ key }: { key: string }) => ({
      value: store.get(key) ?? null
    }));
    prefSet.mockReset().mockImplementation(async ({ key, value }: { key: string; value: string }) => {
      store.set(key, value);
    });
    prefRemove.mockReset().mockImplementation(async ({ key }: { key: string }) => {
      store.delete(key);
    });
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

    await applyMobileConnectionConfig({ serverUrl: 'http://host:20000' });

    expect(await window.plainshelf?.getApiToken?.()).toBe('secret-token');
  });

  it('reads the token live, so an update applies without an app restart', async () => {
    store.set('plainshelf.mobile.token', 'first');
    await applyMobileConnectionConfig({ serverUrl: 'http://host:20000' });
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

    expect(await loadMobileConnectionConfig()).toEqual({
      serverUrl: 'http://host:20000',
      token: 'secret-token',
      shelfId: 'main'
    });
  });

  it('clears a stored token when saved empty', async () => {
    store.set('plainshelf.mobile.token', 'secret-token');

    await saveMobileConnectionConfig({ token: '' });

    expect(store.has('plainshelf.mobile.token')).toBe(false);
    expect(await window.plainshelf?.getApiToken?.()).toBe('');
  });
});
