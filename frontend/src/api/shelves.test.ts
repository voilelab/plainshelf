import { beforeEach, describe, expect, it, vi } from 'vitest';

const { fetchJsonMock, getMobileConnectionConfigMock } = vi.hoisted(() => ({
  fetchJsonMock: vi.fn(),
  getMobileConnectionConfigMock: vi.fn()
}));

vi.mock('./client', () => ({
  fetchJson: fetchJsonMock,
  buildShelfApiPath: (path: string) => path,
  getActiveShelfID: () => '',
  setActiveShelfID: vi.fn(),
  isMockApiMode: () => false
}));

vi.mock('@/providers/mobileConfig', () => ({
  getMobileConnectionConfig: getMobileConnectionConfigMock
}));

const { listShelves, pcloudShelfInfo } = await import('./shelves');

type Config = {
  mode: 'server' | 'pcloud';
  serverUrl: string;
  token: string;
  shelfId: string;
  pcloudClientId: string;
  pcloudAccessToken: string;
  pcloudHost: string;
  pcloudShelfRoot: string;
};

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

beforeEach(() => {
  fetchJsonMock.mockReset();
  getMobileConnectionConfigMock.mockReset().mockReturnValue(null);
});

describe('pcloudShelfInfo', () => {
  it('names the shelf after the last segment of the folder path', () => {
    getMobileConnectionConfigMock.mockReturnValue(
      config({ mode: 'pcloud', pcloudShelfRoot: '/PlainShelf/default-shelf' })
    );

    // The id is the full path, matching the active shelf id mobileConfig sets,
    // so the device-local cache scope agrees with the picker.
    expect(pcloudShelfInfo()).toEqual({ id: '/PlainShelf/default-shelf', name: 'default-shelf' });
  });

  it('tolerates a trailing slash', () => {
    getMobileConnectionConfigMock.mockReturnValue(
      config({ mode: 'pcloud', pcloudShelfRoot: '/shelf/' })
    );

    expect(pcloudShelfInfo()?.name).toBe('shelf');
  });

  it('returns null in server mode and when no folder is set', () => {
    getMobileConnectionConfigMock.mockReturnValue(config({ serverUrl: 'http://host' }));
    expect(pcloudShelfInfo()).toBeNull();

    getMobileConnectionConfigMock.mockReturnValue(config({ mode: 'pcloud' }));
    expect(pcloudShelfInfo()).toBeNull();

    getMobileConnectionConfigMock.mockReturnValue(null);
    expect(pcloudShelfInfo()).toBeNull();
  });
});

describe('listShelves', () => {
  it('answers from the configured folder without a request in pCloud mode', async () => {
    getMobileConnectionConfigMock.mockReturnValue(
      config({ mode: 'pcloud', pcloudShelfRoot: '/PlainShelf/default-shelf' })
    );

    await expect(listShelves()).resolves.toEqual([
      { id: '/PlainShelf/default-shelf', name: 'default-shelf' }
    ]);
    // There is no server to ask; issuing the request would fail and, worse,
    // ensureActiveShelf would then clear the id the cache scope is keyed on.
    expect(fetchJsonMock).not.toHaveBeenCalled();
  });

  it('still asks the server in server mode', async () => {
    getMobileConnectionConfigMock.mockReturnValue(config({ serverUrl: 'http://host' }));
    fetchJsonMock.mockResolvedValue([{ id: 'main', name: 'Main' }]);

    await expect(listShelves()).resolves.toEqual([{ id: 'main', name: 'Main' }]);
    expect(fetchJsonMock).toHaveBeenCalledWith('/api/shelves');
  });

  it('drops malformed entries from a server response', async () => {
    getMobileConnectionConfigMock.mockReturnValue(config({ serverUrl: 'http://host' }));
    fetchJsonMock.mockResolvedValue([{ id: 'main', name: 'Main' }, { id: '' }, null, 'nope']);

    await expect(listShelves()).resolves.toEqual([{ id: 'main', name: 'Main' }]);
  });
});
