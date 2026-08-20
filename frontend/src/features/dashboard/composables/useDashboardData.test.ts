import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/api/client';
import type { Book, PaginatedBooks } from '@/types/book';

const listBooks = vi.fn();
const getReadingActivityRange = vi.fn();

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ listBooks })
}));

vi.mock('@/storage/readingStats', () => ({
  getReadingActivityRange: (from: string, to: string) => getReadingActivityRange(from, to)
}));

const { useDashboardData } = await import('./useDashboardData');

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
  getReadingActivityRange.mockReset();
  getReadingActivityRange.mockResolvedValue({});
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useDashboardData', () => {
  it('loads books and reading activity', async () => {
    listBooks.mockResolvedValue(page([makeBook({ id: 'b1', char_count: 12 })]));

    const { fetchDashboardData, books, totalChars, loading, error } = useDashboardData();
    await fetchDashboardData();

    expect(listBooks).toHaveBeenCalledWith(1, Number.MAX_SAFE_INTEGER, { includeCharCount: true });
    expect(books.value).toHaveLength(1);
    expect(totalChars.value).toBe(12);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('');
  });

  it('keeps loading and retries while the shelf is still initializing', async () => {
    listBooks
      .mockRejectedValueOnce(shelfInitializing())
      .mockResolvedValueOnce(page([makeBook({ id: 'b1' })]));

    const store = useDashboardData();
    await store.fetchDashboardData();

    // The sidebar retries through the initial scan, so the dashboard must not
    // strand on an error the shelf resolves on its own.
    expect(store.error.value).toBe('');
    expect(store.loading.value).toBe(true);
    expect(store.shelfInitializing.value).toBe(true);

    await vi.runOnlyPendingTimersAsync();

    expect(listBooks).toHaveBeenCalledTimes(2);
    expect(store.books.value).toHaveLength(1);
    expect(store.loading.value).toBe(false);
    expect(store.shelfInitializing.value).toBe(false);
    expect(store.error.value).toBe('');
  });

  it('gives up with an error after the retry budget is exhausted', async () => {
    listBooks.mockRejectedValue(shelfInitializing());

    const store = useDashboardData();
    await store.fetchDashboardData();
    await vi.runAllTimersAsync();

    expect(listBooks).toHaveBeenCalledTimes(10);
    expect(store.loading.value).toBe(false);
    expect(store.shelfInitializing.value).toBe(false);
    expect(store.error.value).toBeTruthy();
  });

  it('reports non-503 failures immediately', async () => {
    listBooks.mockRejectedValue(new ApiError('boom', { status: 500 }));

    const store = useDashboardData();
    await store.fetchDashboardData();

    expect(listBooks).toHaveBeenCalledTimes(1);
    expect(store.loading.value).toBe(false);
    expect(store.shelfInitializing.value).toBe(false);
    expect(store.error.value).toBe('boom');
  });

  it('cancels a pending retry when a manual refresh starts', async () => {
    listBooks
      .mockRejectedValueOnce(shelfInitializing())
      .mockResolvedValue(page([makeBook({ id: 'b1' })]));

    const store = useDashboardData();
    await store.fetchDashboardData();
    expect(store.shelfInitializing.value).toBe(true);

    await store.fetchDashboardData();

    expect(store.shelfInitializing.value).toBe(false);
    expect(store.books.value).toHaveLength(1);

    // The cancelled retry must not fire a third request afterwards.
    await vi.runAllTimersAsync();
    expect(listBooks).toHaveBeenCalledTimes(2);
  });
});
