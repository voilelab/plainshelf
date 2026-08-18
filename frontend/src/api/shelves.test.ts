import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { fetchJsonMock } = vi.hoisted(() => ({
  fetchJsonMock: vi.fn()
}));

vi.mock('./client', async () => {
  const actual = await vi.importActual<typeof import('./client')>('./client');
  return {
    fetchJson: fetchJsonMock,
    buildShelfApiPath: (path: string) => path,
    getActiveShelfID: () => '',
    setActiveShelfID: vi.fn(),
    isMockApiMode: () => false,
    normalizeApiBase: actual.normalizeApiBase
  };
});

const { listServerShelves, listShelves } = await import('./shelves');
const { registerShell } = await import('@/providers/shell');

/** Stands in for a shell whose shelf list is device-local. */
function installShelfProvidingShell(shelf: { id: string; name: string } | null): void {
  registerShell({
    createProvider: () => {
      throw new Error('not used by these tests');
    },
    activeShelfInfo: () => shelf
  });
}

afterEach(() => {
  registerShell(null);
});

describe('listShelves', () => {
  // On the mobile shell the app-wide shelf list is the one entry the device is
  // pointed at: the other entries belong to other servers and other pCloud
  // folders, so no server can enumerate them.
  it('answers from the shell without a request when one supplies a shelf', async () => {
    installShelfProvidingShell({ id: '/PlainShelf/default-shelf', name: 'default-shelf' });

    await expect(listShelves()).resolves.toEqual([
      { id: '/PlainShelf/default-shelf', name: 'default-shelf' }
    ]);
    // Issuing the request would fail and, worse, ensureActiveShelf would then
    // clear the id the cache scope is keyed on.
    expect(fetchJsonMock).not.toHaveBeenCalled();
  });

  it('does the same for a server-backed shell entry', async () => {
    installShelfProvidingShell({ id: 'main', name: 'main' });

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
  it('asks the server even while a shell supplies an active shelf', async () => {
    installShelfProvidingShell({ id: 'main', name: 'main' });
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
