// @vitest-environment jsdom

import { createApp, defineComponent, h } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/api/client';
import type { Book, PaginatedBooks } from '@/types/book';

const listBooks = vi.fn();
const getReadingActivityRange = vi.fn();
const getReadHistoryIDs = vi.fn();
const getLocalReadingEntry = vi.fn();
const getLocalReadingEntries = vi.fn();

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => ({ listBooks })
}));

vi.mock('@/storage/readingStats', () => ({
  getReadingActivityRange: (from: string, to: string) => getReadingActivityRange(from, to)
}));

vi.mock('@/storage/readHistory', () => ({
  getReadHistoryIDs: () => getReadHistoryIDs()
}));

vi.mock('@/storage/readingProgress', () => ({
  getLocalReadingEntry: (bookID: string) => getLocalReadingEntry(bookID),
  getLocalReadingEntries: () => getLocalReadingEntries()
}));

const { useDashboardData } = await import('./useDashboardData');

function makeBook(overrides: Partial<Book> = {}): Book {
  return {
    id: 'b1',
    title: 'Book',
    authors: [],
    tags: [],
    folders: [],
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
  getReadHistoryIDs.mockReset();
  getReadHistoryIDs.mockResolvedValue([]);
  getLocalReadingEntry.mockReset();
  getLocalReadingEntry.mockResolvedValue(null);
  getLocalReadingEntries.mockReset();
  getLocalReadingEntries.mockResolvedValue({});
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

  it('loads a genuinely empty shelf without flagging it as initializing', async () => {
    listBooks.mockResolvedValue(page([]));

    const store = useDashboardData();
    await store.fetchDashboardData();

    // An empty shelf is a clean, finished load: the page shows its guided empty
    // state. It must be told apart from a shelf still initializing (503, tested
    // next), where books is likewise empty but loading and shelfInitializing
    // stay true — so a cold start never renders the empty-shelf guidance.
    expect(store.books.value).toHaveLength(0);
    expect(store.loading.value).toBe(false);
    expect(store.shelfInitializing.value).toBe(false);
    expect(store.error.value).toBe('');
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

  it('stops retrying once the page it belongs to is unmounted', async () => {
    let rejectFirst: ((reason: unknown) => void) | undefined;
    listBooks.mockImplementationOnce(() => new Promise<PaginatedBooks>((_resolve, reject) => {
      rejectFirst = reject;
    }));

    let store: ReturnType<typeof useDashboardData> | undefined;
    const Host = defineComponent({
      setup() {
        store = useDashboardData();
        return () => h('div');
      }
    });
    const app = createApp(Host);
    app.mount(document.createElement('div'));

    const pending = store?.fetchDashboardData();
    app.unmount();

    // The request was still in flight at unmount, so cancelling the (absent)
    // timer is not enough: the rejection must not schedule a new one.
    rejectFirst?.(shelfInitializing());
    await pending;
    await vi.runAllTimersAsync();

    expect(listBooks).toHaveBeenCalledTimes(1);
    expect(store?.shelfInitializing.value).toBe(false);
  });

  describe('recent reading', () => {
    it('builds the list in history order with a progress bar when char_count is known', async () => {
      listBooks.mockResolvedValue(
        page([
          makeBook({ id: 'b1', char_count: 100 }),
          makeBook({ id: 'b2', char_count: 200 })
        ])
      );
      getReadHistoryIDs.mockResolvedValue(['b2', 'b1']);
      getLocalReadingEntry.mockImplementation((id: string) =>
        Promise.resolve(id === 'b2' ? { offset: 50, at: 1000 } : { offset: 0, at: 500 })
      );

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.recentReading.value.map((item) => item.book.id)).toEqual(['b2', 'b1']);
      // 50/200 -> 25%. b1 has an entry at offset 0, so it is a real 0%, not "no bar".
      expect(store.recentReading.value[0]).toMatchObject({ percent: 25, lastReadAt: 1000 });
      expect(store.recentReading.value[1]).toMatchObject({ percent: 0, lastReadAt: 500 });
    });

    it('omits the progress bar but keeps the card when char_count is missing', async () => {
      listBooks.mockResolvedValue(page([makeBook({ id: 'b1' })]));
      getReadHistoryIDs.mockResolvedValue(['b1']);
      getLocalReadingEntry.mockResolvedValue({ offset: 30, at: 2000 });

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.recentReading.value).toHaveLength(1);
      expect(store.recentReading.value[0]).toMatchObject({ percent: null, lastReadAt: 2000 });
    });

    it('shows the card without a progress bar or time when there is no local entry', async () => {
      // The mobile shell keeps progress elsewhere, so getLocalReadingEntry is null
      // even though char_count is known: the card must not fall back to a false 0%.
      listBooks.mockResolvedValue(page([makeBook({ id: 'b1', char_count: 100 })]));
      getReadHistoryIDs.mockResolvedValue(['b1']);
      getLocalReadingEntry.mockResolvedValue(null);

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.recentReading.value[0]).toMatchObject({ percent: null, lastReadAt: null });
    });

    it('is empty and reads no progress when there is no reading history', async () => {
      listBooks.mockResolvedValue(page([makeBook({ id: 'b1', char_count: 100 })]));
      getReadHistoryIDs.mockResolvedValue([]);

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.recentReading.value).toEqual([]);
      expect(getLocalReadingEntry).not.toHaveBeenCalled();
    });

    it('drops history ids with no matching book and caps the list at six', async () => {
      const books = Array.from({ length: 8 }, (_, i) => makeBook({ id: `b${i}`, char_count: 100 }));
      listBooks.mockResolvedValue(page(books));
      // "ghost" was read but no longer exists; the rest exceed the cap of six.
      getReadHistoryIDs.mockResolvedValue([
        'ghost',
        'b0',
        'b1',
        'b2',
        'b3',
        'b4',
        'b5',
        'b6',
        'b7'
      ]);

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.recentReading.value.map((item) => item.book.id)).toEqual([
        'b0',
        'b1',
        'b2',
        'b3',
        'b4',
        'b5'
      ]);
    });

    it('keeps the loaded dashboard even if reading progress cannot be read', async () => {
      listBooks.mockResolvedValue(page([makeBook({ id: 'b1', char_count: 100 })]));
      getReadHistoryIDs.mockResolvedValue(['b1']);
      getLocalReadingEntry.mockRejectedValue(new Error('storage exploded'));

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.error.value).toBe('');
      expect(store.books.value).toHaveLength(1);
      expect(store.recentReading.value).toEqual([]);
    });
  });

  describe('in progress', () => {
    it('counts books with a positive offset that are not finished', async () => {
      listBooks.mockResolvedValue(
        page([
          makeBook({ id: 'started', char_count: 100 }),
          makeBook({ id: 'finished', char_count: 100 }),
          makeBook({ id: 'untouched', char_count: 100 })
        ])
      );
      getLocalReadingEntries.mockResolvedValue({
        started: { offset: 40, at: 10 },
        finished: { offset: 100, at: 20 }
        // "untouched" has no entry at all.
      });

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.inProgress.value).toBe(1);
    });

    it('is zero when no book has any progress', async () => {
      listBooks.mockResolvedValue(
        page([makeBook({ id: 'b1', char_count: 100 }), makeBook({ id: 'b2', char_count: 200 })])
      );
      getLocalReadingEntries.mockResolvedValue({});

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.inProgress.value).toBe(0);
    });

    it('is zero when every book with progress has been read to the end', async () => {
      listBooks.mockResolvedValue(
        page([makeBook({ id: 'b1', char_count: 100 }), makeBook({ id: 'b2', char_count: 200 })])
      );
      getLocalReadingEntries.mockResolvedValue({
        b1: { offset: 100, at: 10 },
        b2: { offset: 250, at: 20 } // past the end still clamps to 100%.
      });

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.inProgress.value).toBe(0);
    });

    it('ignores reset tombstones but counts progress with an unknown length', async () => {
      listBooks.mockResolvedValue(
        page([
          makeBook({ id: 'reset', char_count: 100 }),
          makeBook({ id: 'nolen' }) // char_count missing -> percent is unknown.
        ])
      );
      getLocalReadingEntries.mockResolvedValue({
        reset: { offset: 0, at: 30 }, // tombstone: opened, then reset to the start.
        nolen: { offset: 5, at: 40 }
      });

      const store = useDashboardData();
      await store.fetchDashboardData();

      // The reset book is not in progress; the unknown-length book has real
      // progress and no way to prove it is finished, so it stays counted.
      expect(store.inProgress.value).toBe(1);
    });

    it('still counts a book that is a few characters short of the end', async () => {
      // 199/200 rounds to 100% for display, but the book is not finished; the
      // count must compare the offset to the length unrounded.
      listBooks.mockResolvedValue(page([makeBook({ id: 'almost', char_count: 200 })]));
      getLocalReadingEntries.mockResolvedValue({ almost: { offset: 199, at: 10 } });

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.inProgress.value).toBe(1);
    });

    it('degrades to zero when the progress store cannot be read', async () => {
      listBooks.mockResolvedValue(page([makeBook({ id: 'b1', char_count: 100 })]));
      getLocalReadingEntries.mockRejectedValue(new Error('storage exploded'));

      const store = useDashboardData();
      await store.fetchDashboardData();

      expect(store.error.value).toBe('');
      expect(store.books.value).toHaveLength(1);
      expect(store.inProgress.value).toBe(0);
    });
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
