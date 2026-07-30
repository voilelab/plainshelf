import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '../api/client';
import type { PaginatedBooks } from '../types/book';

const listBooks = vi.fn();

vi.mock('../providers', () => ({
  getBookshelfProvider: () => ({ listBooks })
}));

const { useCharCountBooks } = await import('./useCharCountBooks');

function page(items: PaginatedBooks['items']): PaginatedBooks {
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

describe('useCharCountBooks', () => {
  it('requests char counts', async () => {
    listBooks.mockResolvedValue(page([]));

    const { fetchBooks, loading } = useCharCountBooks();
    await fetchBooks();

    expect(listBooks).toHaveBeenCalledWith(1, Number.MAX_SAFE_INTEGER, { includeCharCount: true });
    expect(loading.value).toBe(false);
  });

  it('keeps loading and retries while the shelf is still initializing', async () => {
    const book = { id: 'b1', title: 'Book', authors: [], tags: [], layers: [], char_count: 12 };
    listBooks.mockRejectedValueOnce(shelfInitializing()).mockResolvedValueOnce(page([book]));

    const { fetchBooks, books, loading, error } = useCharCountBooks();
    await fetchBooks();

    // Still waiting on the shelf: no error surfaced, page stays in its loading state.
    expect(error.value).toBe('');
    expect(loading.value).toBe(true);

    await vi.runOnlyPendingTimersAsync();

    expect(books.value).toEqual([book]);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('');
    expect(listBooks).toHaveBeenCalledTimes(2);
  });

  it('gives up with an error after the retry budget is exhausted', async () => {
    listBooks.mockRejectedValue(shelfInitializing());

    const { fetchBooks, loading, error } = useCharCountBooks();
    await fetchBooks();
    await vi.runAllTimersAsync();

    expect(listBooks).toHaveBeenCalledTimes(10);
    expect(loading.value).toBe(false);
    expect(error.value).toBeTruthy();
  });

  it('surfaces a non-503 failure immediately without retrying', async () => {
    listBooks.mockRejectedValue(new ApiError('boom', { status: 500 }));

    const { fetchBooks, loading, error } = useCharCountBooks();
    await fetchBooks();

    expect(listBooks).toHaveBeenCalledTimes(1);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('boom');
    expect(vi.getTimerCount()).toBe(0);
  });
});
