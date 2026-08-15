import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/api/client';
import type { Book, PaginatedBooks } from '@/types/book';

const listBooks = vi.fn();

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ listBooks })
}));

const { useCharCountIndex } = await import('./useCharCountIndex');

function makeBook(overrides: Partial<Book> = {}): Book {
  return {
    id: 'b1',
    title: 'Book',
    authors: [],
    tags: [],
    layers: [],
    ...overrides
  };
}

function page(items: Book[]): PaginatedBooks {
  return { items, total: items.length, page: 1, pageSize: items.length };
}

const shelfInitializing = () => new ApiError('shelf initializing', { status: 503 });

beforeEach(() => {
  listBooks.mockReset();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useCharCountIndex', () => {
  it('requests char counts and indexes them by book ID', async () => {
    listBooks.mockResolvedValue(page([
      makeBook({ id: 'b1', char_count: 12 }),
      makeBook({ id: 'b2', char_count: 0 }),
      makeBook({ id: 'b3' })
    ]));

    const { load, counts, ready, loading } = useCharCountIndex();
    await load();

    expect(listBooks).toHaveBeenCalledWith(1, Number.MAX_SAFE_INTEGER, { includeCharCount: true });
    expect(counts.value.get('b1')).toBe(12);
    expect(counts.value.get('b2')).toBe(0);
    // Present in the listing but without a count: distinguishable from a book
    // that is not in the index at all.
    expect(counts.value.has('b3')).toBe(true);
    expect(counts.value.get('b3')).toBeUndefined();
    expect(ready.value).toBe(true);
    expect(loading.value).toBe(false);
  });

  it('does not refetch once loaded', async () => {
    listBooks.mockResolvedValue(page([makeBook({ char_count: 1 })]));

    const index = useCharCountIndex();
    await index.load();
    await index.load();

    expect(listBooks).toHaveBeenCalledTimes(1);
  });

  it('refetches after invalidate', async () => {
    listBooks.mockResolvedValue(page([makeBook({ char_count: 1 })]));

    const index = useCharCountIndex();
    await index.load();
    index.invalidate();
    expect(index.ready.value).toBe(false);
    expect(index.counts.value.size).toBe(0);

    await index.load();
    expect(listBooks).toHaveBeenCalledTimes(2);
  });

  it('refetches on refresh while keeping the cached counts readable', async () => {
    let resolveSecond: ((value: PaginatedBooks) => void) | undefined;
    listBooks
      .mockResolvedValueOnce(page([makeBook({ id: 'b1', char_count: 1 })]))
      .mockImplementationOnce(() => new Promise<PaginatedBooks>((resolve) => {
        resolveSecond = resolve;
      }));

    const index = useCharCountIndex();
    await index.load();

    const refreshing = index.refresh();
    // The previous counts stay usable for the duration of the request, so a
    // caller gated on `ready` does not collapse into a loading state.
    expect(index.ready.value).toBe(true);
    expect(index.counts.value.get('b1')).toBe(1);

    resolveSecond?.(page([makeBook({ id: 'b1', char_count: 99 })]));
    await refreshing;

    expect(listBooks).toHaveBeenCalledTimes(2);
    expect(index.counts.value.get('b1')).toBe(99);
  });

  it('keeps loading and retries while the shelf is still initializing', async () => {
    listBooks
      .mockRejectedValueOnce(shelfInitializing())
      .mockResolvedValueOnce(page([makeBook({ id: 'b1', char_count: 12 })]));

    const { load, counts, loading, error } = useCharCountIndex();
    await load();

    expect(error.value).toBe('');
    expect(loading.value).toBe(true);

    await vi.runOnlyPendingTimersAsync();

    expect(counts.value.get('b1')).toBe(12);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('');
    expect(listBooks).toHaveBeenCalledTimes(2);
  });

  it('gives up with an error after the retry budget is exhausted', async () => {
    listBooks.mockRejectedValue(shelfInitializing());

    const { load, loading, error, ready } = useCharCountIndex();
    await load();
    await vi.runAllTimersAsync();

    expect(listBooks).toHaveBeenCalledTimes(10);
    expect(loading.value).toBe(false);
    expect(ready.value).toBe(false);
    expect(error.value).toBeTruthy();
  });

  it('surfaces a non-503 failure immediately without retrying', async () => {
    listBooks.mockRejectedValue(new ApiError('boom', { status: 500 }));

    const { load, loading, error } = useCharCountIndex();
    await load();

    expect(listBooks).toHaveBeenCalledTimes(1);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('boom');
    expect(vi.getTimerCount()).toBe(0);
  });

  it('discards a response that invalidate() has already superseded', async () => {
    let resolveFirst: ((value: PaginatedBooks) => void) | undefined;
    listBooks
      .mockImplementationOnce(() => new Promise<PaginatedBooks>((resolve) => {
        resolveFirst = resolve;
      }))
      .mockResolvedValueOnce(page([makeBook({ id: 'b1', char_count: 99 })]));

    const index = useCharCountIndex();
    const first = index.load();
    index.invalidate();
    resolveFirst?.(page([makeBook({ id: 'b1', char_count: 1 })]));
    await first;

    expect(index.ready.value).toBe(false);
    expect(index.counts.value.size).toBe(0);

    await index.load();
    expect(index.counts.value.get('b1')).toBe(99);
  });
});
