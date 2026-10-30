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
  it('answers from the shell without a request when one supplies a shelf', async () => {
    installShelfProvidingShell({
      id: '/PlainShelf/default-shelf',
      name: 'default-shelf',
      readOnly: true
    });

    await expect(listShelves()).resolves.toEqual([
      { id: '/PlainShelf/default-shelf', name: 'default-shelf', readOnly: true }
    ]);
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

  it('marks the request as writing nothing, and takes the 409 body as a result', async () => {
    fetchJsonMock.mockResolvedValue({ scan_id: 'abc', book_count: 0, folder_count: 0 });

    await rescanShelf();

    expect(fetchJsonMock).toHaveBeenCalledWith(
      '/scans',
      { method: 'POST' },
      expect.objectContaining({ readOnlySafe: true, acceptStatuses: [409, 429] })
    );
  });

  it('rejects with the running scan when the shelf is already being walked', async () => {
    fetchJsonMock.mockResolvedValue({ scan_id: 'running-one' });

    await expect(rescanShelf()).rejects.toBeInstanceOf(ShelfScanInProgressError);
  });

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

it('preserves engine capabilities and snapshot without accepting malformed members', async () => {
  fetchJsonMock.mockResolvedValue([{
    id: 'cloud', name: 'Cloud', kind: 'virtual', read_only: true,
    capabilities: { rescan: false, download: true, char_count: false, fingerprint: false, unknown: true },
    offline: { snapshot_at: '2026-10-16T09:12:44Z' }
  }, { id: 'old', name: 'Old' }, {
    id: 'malformed', name: 'Malformed', capabilities: { rescan: 'false', download: 1 }
  }]);
  const shelves = await listServerShelves();
  expect(shelves[0]).toEqual({
    id: 'cloud', name: 'Cloud', kind: 'virtual', readOnly: true,
    capabilities: { rescan: false, download: true, char_count: false, fingerprint: false },
    offline: { snapshot_at: '2026-10-16T09:12:44Z' }
  });
  expect(shelves[1].capabilities).toBeUndefined();
  expect(shelves[2].capabilities).toEqual({});
});
