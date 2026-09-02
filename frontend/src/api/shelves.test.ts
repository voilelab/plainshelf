import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { ShelfInfo } from './shelves';

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

const { ShelfScanInProgressError, listServerShelves, listShelves, rescanShelf } = await import('./shelves');
const { registerShell } = await import('@/providers/shell');

/** Stands in for a shell whose shelf list is device-local. */
function installShelfProvidingShell(shelf: ShelfInfo | null): void {
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
    installShelfProvidingShell({ id: '/PlainShelf/default-shelf', name: 'default-shelf', readOnly: false });

    await expect(listShelves()).resolves.toEqual([
      { id: '/PlainShelf/default-shelf', name: 'default-shelf', readOnly: false }
    ]);
    // Issuing the request would fail and, worse, ensureActiveShelf would then
    // clear the id the cache scope is keyed on.
    expect(fetchJsonMock).not.toHaveBeenCalled();
  });

  it('does the same for a server-backed shell entry', async () => {
    installShelfProvidingShell({ id: 'main', name: 'main', readOnly: false });

    await expect(listShelves()).resolves.toEqual([{ id: 'main', name: 'main', readOnly: false }]);
    expect(fetchJsonMock).not.toHaveBeenCalled();
  });

  it('asks the server everywhere else', async () => {
    fetchJsonMock.mockResolvedValue([{ id: 'main', name: 'Main' }]);

    await expect(listShelves()).resolves.toEqual([{ id: 'main', name: 'Main', readOnly: false }]);
    expect(fetchJsonMock).toHaveBeenCalledWith('/api/shelves');
  });
});

describe('listServerShelves', () => {
  // The shelf-entry form has to ask a server the user is still typing in what
  // shelves it offers, so this one must never collapse to the active entry.
  it('asks the server even while a shell supplies an active shelf', async () => {
    installShelfProvidingShell({ id: 'main', name: 'main', readOnly: false });
    fetchJsonMock.mockResolvedValue([
      { id: 'main', name: 'Main' },
      { id: 'other', name: 'Other' }
    ]);

    await expect(listServerShelves()).resolves.toEqual([
      { id: 'main', name: 'Main', readOnly: false },
      { id: 'other', name: 'Other', readOnly: false }
    ]);
  });

  // Per-shelf read_only is what lets the UI drop a read-only shelf's write
  // entries instead of waiting for the 409 the server would answer with.
  it('reports the read_only each shelf carries', async () => {
    fetchJsonMock.mockResolvedValue([
      { id: 'main', name: 'Main', read_only: false },
      { id: 'archive', name: 'Archive', read_only: true }
    ]);

    await expect(listServerShelves()).resolves.toEqual([
      { id: 'main', name: 'Main', readOnly: false },
      { id: 'archive', name: 'Archive', readOnly: true }
    ]);
  });

  // An older server has no such field, and a shelf it does not call read-only
  // is not one this client may guess at.
  it('treats a missing read_only as writable', async () => {
    fetchJsonMock.mockResolvedValue([{ id: 'main', name: 'Main' }]);

    await expect(listServerShelves()).resolves.toEqual([{ id: 'main', name: 'Main', readOnly: false }]);
  });

  it('drops malformed entries from a server response', async () => {
    fetchJsonMock.mockResolvedValue([{ id: 'main', name: 'Main' }, { id: '' }, null, 'nope']);

    await expect(listServerShelves()).resolves.toEqual([{ id: 'main', name: 'Main', readOnly: false }]);
  });
});

describe('rescanShelf', () => {
  it('reports what the walk found', async () => {
    fetchJsonMock.mockResolvedValue({ scan_id: 'abc', scanned_at: 1_700_000_000, book_count: 12, folder_count: 3 });

    await expect(rescanShelf()).resolves.toEqual({ bookCount: 12, folderCount: 3 });
  });

  // The gate that would otherwise reject this POST is the one that exists to
  // stop writes, and a rescan is not one; the server draws the same exception.
  it('marks the request as writing nothing, and takes the 409 body as a result', async () => {
    fetchJsonMock.mockResolvedValue({ scan_id: 'abc', book_count: 0, folder_count: 0 });

    await rescanShelf();

    expect(fetchJsonMock).toHaveBeenCalledWith(
      '/scans',
      { method: 'POST' },
      expect.objectContaining({ readOnlySafe: true, acceptStatuses: [409] })
    );
  });

  // A 409 body carries no counts, which is how it is told apart from a walk
  // that ran and genuinely found an empty shelf.
  it('rejects with the running scan when the shelf is already being walked', async () => {
    fetchJsonMock.mockResolvedValue({ scan_id: 'running-one' });

    await expect(rescanShelf()).rejects.toBeInstanceOf(ShelfScanInProgressError);
  });

  it('does not mistake an empty shelf for a refusal', async () => {
    fetchJsonMock.mockResolvedValue({ scan_id: 'abc', scanned_at: 1, book_count: 0, folder_count: 1 });

    await expect(rescanShelf()).resolves.toEqual({ bookCount: 0, folderCount: 1 });
  });
});
