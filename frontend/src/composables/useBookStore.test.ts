import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const listBooks = vi.fn();

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ listBooks })
}));

async function freshStore() {
  vi.resetModules();
  const { ApiError } = await import('@/api/client');
  const { useBookStore } = await import('./useBookStore');
  return {
    ...useBookStore(),
    initializing: () => new ApiError('shelf is initializing', { status: 503 })
  };
}

beforeEach(() => {
  listBooks.mockReset();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useBookStore', () => {
  it('keeps retrying while the shelf reports it is initializing', async () => {
    const { books, loading, error, shelfInitializing, shelfUnreachable, fetchBooks, initializing } =
      await freshStore();
    listBooks
      .mockRejectedValueOnce(initializing())
      .mockRejectedValueOnce(initializing())
      .mockResolvedValueOnce({ items: [{ id: 'a' }] });

    await fetchBooks();

    expect(shelfInitializing.value).toBe(true);
    expect(loading.value).toBe(true);
    expect(error.value).toBe('');

    await vi.advanceTimersByTimeAsync(3000);
    expect(shelfInitializing.value).toBe(true);

    await vi.advanceTimersByTimeAsync(3000);
    expect(listBooks).toHaveBeenCalledTimes(3);
    expect(books.value).toEqual([{ id: 'a' }]);
    expect(loading.value).toBe(false);
    expect(shelfInitializing.value).toBe(false);
    expect(shelfUnreachable.value).toBe(false);
  });

  it('gives up as unreachable after the auto-retry budget is spent', async () => {
    const { loading, error, shelfInitializing, shelfUnreachable, fetchBooks, initializing } =
      await freshStore();
    listBooks.mockRejectedValue(initializing());

    await fetchBooks();
    await vi.advanceTimersByTimeAsync(3000 * 10);

    expect(listBooks).toHaveBeenCalledTimes(10);
    expect(shelfUnreachable.value).toBe(true);
    expect(shelfInitializing.value).toBe(false);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('');
  });

  it('does not strand the page in loading when a concurrent load succeeds', async () => {
    const { books, loading, error, shelfInitializing, fetchBooks, initializing } =
      await freshStore();
    listBooks.mockRejectedValueOnce(initializing()).mockResolvedValueOnce({ items: [{ id: 'a' }] });

    await Promise.all([fetchBooks(), fetchBooks()]);

    expect(books.value).toEqual([{ id: 'a' }]);
    expect(shelfInitializing.value).toBe(false);
    expect(loading.value).toBe(false);
    expect(error.value).toBe('');

    await vi.advanceTimersByTimeAsync(3000);
    expect(listBooks).toHaveBeenCalledTimes(2);
  });

  it('reports any other failure without retrying', async () => {
    const { loading, error, shelfUnreachable, fetchBooks } = await freshStore();
    listBooks.mockRejectedValue(new Error('boom'));

    await fetchBooks();

    expect(listBooks).toHaveBeenCalledTimes(1);
    expect(loading.value).toBe(false);
    expect(shelfUnreachable.value).toBe(false);
    expect(error.value).toBe('boom');
  });

  it('restarts the retry budget when the user retries by hand', async () => {
    const { shelfUnreachable, fetchBooks, initializing } = await freshStore();
    listBooks.mockRejectedValue(initializing());

    await fetchBooks();
    await vi.advanceTimersByTimeAsync(3000 * 10);
    expect(listBooks).toHaveBeenCalledTimes(10);
    expect(shelfUnreachable.value).toBe(true);

    listBooks.mockReset();
    listBooks.mockResolvedValue({ items: [] });
    await fetchBooks();

    expect(listBooks).toHaveBeenCalledTimes(1);
    expect(shelfUnreachable.value).toBe(false);
  });
});
