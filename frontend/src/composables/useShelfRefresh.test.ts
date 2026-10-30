import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { PaginatedBooks } from '@/types/book';

const capability = vi.hoisted(() => ({ rescan: true, snapshotAt: null as number | null }));
const refreshShelf = vi.fn();

const listBooks = vi.fn();
const listFolders = vi.fn();

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({
    refreshShelf,
    listBooks,
    listFolders
  })
}));

const { useShelfRefresh } = await import('./useShelfRefresh');
const { useBookStore } = await import('./useBookStore');
const { useToasts } = await import('./useToasts');
const { ShelfScanInProgressError, ShelfScanRateLimitedError } = await import('@/api/shelves');
const { t } = await import('@/i18n');

function toastMessages(): string[] {
  return useToasts().toasts.value.map((toast) => toast.message);
}

function clearToasts(): void {
  const { toasts, dismissToast } = useToasts();
  for (const toast of [...toasts.value]) {
    dismissToast(toast.id);
  }
}

function page(ids: string[]): PaginatedBooks {
  const items = ids.map((id) => ({ id, title: id })) as PaginatedBooks['items'];
  return { items, total: items.length, page: 1, pageSize: items.length };
}

beforeEach(() => {
  capability.rescan = true;
  capability.snapshotAt = null;
  refreshShelf.mockReset().mockResolvedValue(undefined);
  listBooks.mockReset().mockResolvedValue(page(['a']));
  listFolders.mockReset().mockResolvedValue(['/']);

  const shelf = useShelfRefresh();
  shelf.error.value = '';
  shelf.refreshing.value = false;
  shelf.lastSyncedAt.value = null;
  clearToasts();
});

describe('useShelfRefresh', () => {
  it('reports unsupported when the provider does not offer a manual update', async () => {
    capability.rescan = false;

    const shelf = useShelfRefresh();
    await shelf.refresh();

    expect(shelf.supported.value).toBe(false);
    expect(refreshShelf).not.toHaveBeenCalled();
  });

  it('updates the shelf, then reloads books, folders and the sync time', async () => {
    capability.snapshotAt = 1_700_000_000_000;
    const shelf = useShelfRefresh();

    await shelf.refresh();

    expect(refreshShelf).toHaveBeenCalledTimes(1);
    expect(listBooks).toHaveBeenCalled();
    expect(listFolders).toHaveBeenCalled();
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
    capability.snapshotAt = 4_242;
    const shelf = useShelfRefresh();

    await shelf.loadLastSyncedAt();

    expect(shelf.lastSyncedAt.value).toBe(4_242);
    expect(refreshShelf).not.toHaveBeenCalled();
  });

  it('announces what the update found in a toast', async () => {
    refreshShelf.mockResolvedValue({ bookCount: 12, folderCount: 3 });
    const shelf = useShelfRefresh();

    await shelf.refresh();

    expect(toastMessages()).toEqual([t('library.scanFound', { books: 12, folders: 3 })]);
    expect(shelf.lastSyncedAt.value).toBeNull();
  });

  it('raises no toast for a backend that reports no counts', async () => {
    const shelf = useShelfRefresh();

    await shelf.refresh();

    expect(toastMessages()).toEqual([]);
  });

  it('reports whether the backend dates its listing at all', async () => {
    capability.snapshotAt = 42;
    expect(useShelfRefresh().tracksLastSynced.value).toBe(true);

    capability.snapshotAt = null;
    const shelf = useShelfRefresh();

    expect(shelf.tracksLastSynced.value).toBe(false);
    await shelf.loadLastSyncedAt();
  });

  it('explains a refusal to start a second walk in a toast, not as a page error', async () => {
    refreshShelf.mockRejectedValue(new ShelfScanInProgressError('running-one'));
    const shelf = useShelfRefresh();

    await shelf.refresh();

    expect(toastMessages()).toEqual([t('library.scanInProgress')]);
    expect(shelf.error.value).toBe('');
  });

  it('tells the user how long to wait when the server refuses the pace', async () => {
    refreshShelf.mockRejectedValue(new ShelfScanRateLimitedError(7));
    const shelf = useShelfRefresh();

    await shelf.refresh();

    expect(toastMessages()).toEqual([t('library.scanRateLimited', { seconds: 7 })]);
    expect(shelf.error.value).toBe('');
  });

  it('leaves a genuine failure on the page rather than in a toast', async () => {
    refreshShelf.mockRejectedValue(new Error('pCloud is unreachable.'));
    const shelf = useShelfRefresh();

    await shelf.refresh();

    expect(shelf.error.value).toBe('pCloud is unreachable.');
    expect(toastMessages()).toEqual([]);
  });
});

vi.mock('./useShelfCapabilities', async () => {
  const { computed } = await import('vue');
  return { useShelfCapabilities: () => ({
    rescan: computed(() => capability.rescan),
    snapshotAt: computed(() => capability.snapshotAt)
  }) };
});
vi.mock('./useShelvesStore', () => ({ useShelvesStore: () => ({ fetchShelves: async () => {} }) }));
