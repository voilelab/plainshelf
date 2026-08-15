import { beforeEach, describe, expect, it, vi } from 'vitest';

const { fetchJsonMock, getActiveShelfEntryMock } = vi.hoisted(() => ({
  fetchJsonMock: vi.fn(),
  getActiveShelfEntryMock: vi.fn()
}));

vi.mock('./client', async () => {
  const actual = await vi.importActual<typeof import('./client')>('./client');
  return {
    fetchJson: fetchJsonMock,
    buildShelfApiPath: (path: string) => path,
    getActiveShelfID: () => '',
    setActiveShelfID: vi.fn(),
    isMockApiMode: () => false,
    // Reached through mobileConfig's shelfEntryTarget, which normalizes the
    // base the same way the real client does.
    normalizeApiBase: actual.normalizeApiBase
  };
});

vi.mock('@/providers/mobileConfig', async () => {
  const actual = await vi.importActual<typeof import('@/providers/mobileConfig')>(
    '@/providers/mobileConfig'
  );
  return {
    ...actual,
    getActiveShelfEntry: getActiveShelfEntryMock
  };
});

const { activeMobileShelfInfo, listServerShelves, listShelves } = await import('./shelves');
type ShelfEntry = import('@/providers/mobileConfig').ShelfEntry;

function serverEntry(overrides: Record<string, unknown> = {}): ShelfEntry {
  return {
    id: 'entry-1',
    type: 'server',
    name: '',
    serverUrl: 'http://host',
    shelfId: 'main',
    ...overrides
  } as ShelfEntry;
}

function pcloudEntry(overrides: Record<string, unknown> = {}): ShelfEntry {
  return {
    id: 'entry-2',
    type: 'pcloud',
    name: '',
    pcloudClientId: '',
    pcloudHost: 'api.pcloud.com',
    pcloudShelfRoot: '/PlainShelf/default-shelf',
    ...overrides
  } as ShelfEntry;
}

beforeEach(() => {
  fetchJsonMock.mockReset();
  getActiveShelfEntryMock.mockReset().mockReturnValue(null);
});

describe('activeMobileShelfInfo', () => {
  it('names a pCloud shelf after the last segment of the folder path', () => {
    getActiveShelfEntryMock.mockReturnValue(pcloudEntry());

    // The id is the full path, matching the active shelf id mobileConfig sets,
    // so the device-local cache scope agrees with the picker.
    expect(activeMobileShelfInfo()).toEqual({
      id: '/PlainShelf/default-shelf',
      name: 'default-shelf'
    });
  });

  it('tolerates a trailing slash', () => {
    getActiveShelfEntryMock.mockReturnValue(pcloudEntry({ pcloudShelfRoot: '/shelf/' }));

    expect(activeMobileShelfInfo()?.name).toBe('shelf');
  });

  it('uses the entry name when the user gave it one', () => {
    getActiveShelfEntryMock.mockReturnValue(serverEntry({ name: 'Living room' }));

    expect(activeMobileShelfInfo()).toEqual({ id: 'main', name: 'Living room' });
  });

  it('returns null off the mobile shell and for an entry with no shelf yet', () => {
    expect(activeMobileShelfInfo()).toBeNull();

    getActiveShelfEntryMock.mockReturnValue(serverEntry({ shelfId: '' }));
    expect(activeMobileShelfInfo()).toBeNull();
  });
});

describe('listShelves', () => {
  // On the mobile shell the app-wide shelf list is the one entry the device is
  // pointed at: the other entries belong to other servers and other pCloud
  // folders, so no server can enumerate them.
  it('answers from the active entry without a request on the mobile shell', async () => {
    getActiveShelfEntryMock.mockReturnValue(pcloudEntry());

    await expect(listShelves()).resolves.toEqual([
      { id: '/PlainShelf/default-shelf', name: 'default-shelf' }
    ]);
    // Issuing the request would fail and, worse, ensureActiveShelf would then
    // clear the id the cache scope is keyed on.
    expect(fetchJsonMock).not.toHaveBeenCalled();
  });

  it('does the same for a server entry', async () => {
    getActiveShelfEntryMock.mockReturnValue(serverEntry());

    await expect(listShelves()).resolves.toEqual([{ id: 'main', name: 'main' }]);
    expect(fetchJsonMock).not.toHaveBeenCalled();
  });

  it('asks the server everywhere else', async () => {
    fetchJsonMock.mockResolvedValue([{ id: 'main', name: 'Main' }]);

    await expect(listShelves()).resolves.toEqual([{ id: 'main', name: 'Main' }]);
    expect(fetchJsonMock).toHaveBeenCalledWith('/api/shelves');
  });
});

describe('listServerShelves', () => {
  // The shelf-entry form has to ask a server the user is still typing in what
  // shelves it offers, so this one must never collapse to the active entry.
  it('asks the server even while a shelf entry is active', async () => {
    getActiveShelfEntryMock.mockReturnValue(serverEntry());
    fetchJsonMock.mockResolvedValue([
      { id: 'main', name: 'Main' },
      { id: 'other', name: 'Other' }
    ]);

    await expect(listServerShelves()).resolves.toEqual([
      { id: 'main', name: 'Main' },
      { id: 'other', name: 'Other' }
    ]);
  });

  it('drops malformed entries from a server response', async () => {
    fetchJsonMock.mockResolvedValue([{ id: 'main', name: 'Main' }, { id: '' }, null, 'nope']);

    await expect(listServerShelves()).resolves.toEqual([{ id: 'main', name: 'Main' }]);
  });
});
