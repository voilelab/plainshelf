import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { PaginatedBooks } from '@/types/book';

const supportsShelfRefresh = vi.fn();
const refreshShelf = vi.fn();
const getShelfFetchedAt = vi.fn();
const listBooks = vi.fn();
const listLayers = vi.fn();

// Whether the backend dates its stored listing at all. Absence of the method is
// the signal, not a null return, so it has to be togglable per test.
let datesItsListing = true;

// The composable reads the provider through the same accessor the stores it
// drives use, so one mock covers all three.
vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({
    supportsShelfRefresh,
    refreshShelf,
    ...(datesItsListing ? { getShelfFetchedAt } : {}),
    listBooks,
    listLayers
  })
}));

const { useShelfRefresh } = await import('./useShelfRefresh');
const { useBookStore } = await import('./useBookStore');
const { ShelfScanInProgressError } = await import('@/api/shelves');
const { t } = await import('@/i18n');

function page(ids: string[]): PaginatedBooks {
  const items = ids.map((id) => ({ id, title: id })) as PaginatedBooks['items'];
  return { items, total: items.length, page: 1, pageSize: items.length };
}

beforeEach(() => {
  datesItsListing = true;
  supportsShelfRefresh.mockReset().mockReturnValue(true);
  refreshShelf.mockReset().mockResolvedValue(undefined);
  getShelfFetchedAt.mockReset().mockResolvedValue(null);
  listBooks.mockReset().mockResolvedValue(page(['a']));
  listLayers.mockReset().mockResolvedValue(['/']);

  const shelf = useShelfRefresh();
  shelf.error.value = '';
  shelf.refreshing.value = false;
  shelf.lastSyncedAt.value = null;
  shelf.lastResult.value = null;
});

describe('useShelfRefresh', () => {
  it('reports unsupported when the provider does not offer a manual update', async () => {
    supportsShelfRefresh.mockReturnValue(false);

    const shelf = useShelfRefresh();
    await shelf.refresh();

    expect(shelf.supported).toBe(false);
    expect(refreshShelf).not.toHaveBeenCalled();
  });

  it('updates the shelf, then reloads books, layers and the sync time', async () => {
    getShelfFetchedAt.mockResolvedValue(1_700_000_000_000);
    const shelf = useShelfRefresh();

    await shelf.refresh();

    expect(refreshShelf).toHaveBeenCalledTimes(1);
    expect(listBooks).toHaveBeenCalled();
    expect(listLayers).toHaveBeenCalled();
    expect(shelf.lastSyncedAt.value).toBe(1_700_000_000_000);
    expect(shelf.refreshing.value).toBe(false);
    expect(shelf.error.value).toBe('');
  });

  it('keeps the current book list when the update fails', async () => {
    const { books } = useBookStore();
    const shelf = useShelfRefresh();
    await shelf.refresh();
    expect(books.value.map((book) => book.id)).toEqual(['a']);

    refreshShelf.mockRejectedValue(new Error('pCloud is unreachable.'));
    listBooks.mockClear();
    await shelf.refresh();

    expect(shelf.error.value).toBe('pCloud is unreachable.');
    expect(listBooks).not.toHaveBeenCalled();
    expect(books.value.map((book) => book.id)).toEqual(['a']);
    expect(shelf.refreshing.value).toBe(false);
  });

  it('ignores a second press while one update is running', async () => {
    let release = (): void => {};
    refreshShelf.mockImplementation(() => new Promise<void>((resolve) => {
      release = resolve;
    }));
    const shelf = useShelfRefresh();

    const first = shelf.refresh();
    await shelf.refresh();
    expect(refreshShelf).toHaveBeenCalledTimes(1);

    release();
    await first;
  });

  it('reads the last sync time without updating the shelf', async () => {
    getShelfFetchedAt.mockResolvedValue(4_242);
    const shelf = useShelfRefresh();

    await shelf.loadLastSyncedAt();

    expect(shelf.lastSyncedAt.value).toBe(4_242);
    expect(refreshShelf).not.toHaveBeenCalled();
  });

  // A server has no stored listing to date; what it can say is what the walk
  // found, which is what the toolbar shows in place of a sync time.
  it('keeps what the update found when the backend reports it', async () => {
    refreshShelf.mockResolvedValue({ bookCount: 12, layerCount: 3 });
    const shelf = useShelfRefresh();

    await shelf.refresh();

    expect(shelf.lastResult.value).toEqual({ bookCount: 12, layerCount: 3 });
  });

  it('reports no counts for a backend that returns none', async () => {
    const shelf = useShelfRefresh();

    await shelf.refresh();

    expect(shelf.lastResult.value).toBeNull();
  });

  // "Never updated" is only true of a backend that dates its stored listing. A
  // server has no such date, so the toolbar must be able to leave the label out
  // rather than claim the shelf has never been read.
  it('reports whether the backend dates its listing at all', async () => {
    expect(useShelfRefresh().tracksLastSynced).toBe(true);

    datesItsListing = false;
    const shelf = useShelfRefresh();

    expect(shelf.tracksLastSynced).toBe(false);
    await shelf.loadLastSyncedAt();
    expect(getShelfFetchedAt).not.toHaveBeenCalled();
  });

  // Not a failure: another client is already walking the shelf, and the answer
  // is to wait for it. The api and provider layers hold no strings, so the
  // composable is where it gets a message a user can read.
  it('explains a refusal to start a second walk', async () => {
    refreshShelf.mockRejectedValue(new ShelfScanInProgressError('running-one'));
    const shelf = useShelfRefresh();

    await shelf.refresh();

    expect(shelf.error.value).toBe(t('library.scanInProgress'));
    expect(shelf.error.value).not.toBe('');
  });
});
