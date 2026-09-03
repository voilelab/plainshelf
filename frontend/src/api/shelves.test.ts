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

const { ShelfScanInProgressError, ShelfScanRateLimitedError, listServerShelves, listShelves, rescanShelf } =
  await import('./shelves');
const { registerShell } = await import('@/providers/shell');

/** Stands in for a shell whose shelf list is device-local. */
function installShelfProvidingShell(
  shelf: { id: string; name: string; readOnly?: boolean } | null
): void {
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
    installShelfProvidingShell({
      id: '/PlainShelf/default-shelf',
      name: 'default-shelf',
      readOnly: true
    });

    await expect(listShelves()).resolves.toEqual([
      { id: '/PlainShelf/default-shelf', name: 'default-shelf', readOnly: true }
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
    fetchJsonMock.mockResolvedValue([{ id: 'main', name: 'Main', read_only: false }]);

    await expect(listShelves()).resolves.toEqual([{ id: 'main', name: 'Main', readOnly: false }]);
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
      { id: 'main', name: 'Main', readOnly: false },
      { id: 'other', name: 'Other', readOnly: false }
    ]);
  });

  it('drops malformed entries from a server response', async () => {
    fetchJsonMock.mockResolvedValue([{ id: 'main', name: 'Main' }, { id: '' }, null, 'nope']);

    await expect(listServerShelves()).resolves.toEqual([
      { id: 'main', name: 'Main', readOnly: false }
    ]);
  });

  // The whole point of the field: a writable and a read-only shelf side by side
  // must not look the same to the client, or the UI has nothing to gate on.
  it('carries each shelf read-only state independently', async () => {
    fetchJsonMock.mockResolvedValue([
      { id: 'archive', name: 'Archive', read_only: true },
      { id: 'main', name: 'Main', read_only: false }
    ]);

    await expect(listServerShelves()).resolves.toEqual([
      { id: 'archive', name: 'Archive', readOnly: true },
      { id: 'main', name: 'Main', readOnly: false }
    ]);
  });

  // A server predating the field never opened a shelf read-only, so defaulting
  // to writable is what keeps its write buttons on screen. Anything that is not
  // literally `true` — a string, a number, a missing key — reads as writable.
  it('treats a missing or non-boolean read_only as writable', async () => {
    fetchJsonMock.mockResolvedValue([
      { id: 'old', name: 'Old' },
      { id: 'odd', name: 'Odd', read_only: 'true' }
    ]);

    await expect(listServerShelves()).resolves.toEqual([
      { id: 'old', name: 'Old', readOnly: false },
      { id: 'odd', name: 'Odd', readOnly: false }
    ]);
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
      expect.objectContaining({ readOnlySafe: true, acceptStatuses: [409, 429] })
    );
  });

  // A 409 body carries no counts, which is how it is told apart from a walk
  // that ran and genuinely found an empty shelf.
  it('rejects with the running scan when the shelf is already being walked', async () => {
    fetchJsonMock.mockResolvedValue({ scan_id: 'running-one' });

    await expect(rescanShelf()).rejects.toBeInstanceOf(ShelfScanInProgressError);
  });

  // The 429 body carries no counts either, so a client keying only on their
  // absence would report a running walk that does not exist.
  it('rejects with the wait when the server refuses the pace, not a running walk', async () => {
    fetchJsonMock.mockResolvedValue({ retry_after_seconds: 7, message: 'too many rescans' });

    await expect(rescanShelf()).rejects.toBeInstanceOf(ShelfScanRateLimitedError);
    await expect(rescanShelf()).rejects.not.toBeInstanceOf(ShelfScanInProgressError);
    await expect(rescanShelf()).rejects.toMatchObject({ retryAfterSeconds: 7 });
  });

  it('does not mistake an empty shelf for a refusal', async () => {
    fetchJsonMock.mockResolvedValue({ scan_id: 'abc', scanned_at: 1, book_count: 0, folder_count: 1 });

    await expect(rescanShelf()).resolves.toEqual({ bookCount: 0, folderCount: 1 });
  });
});
