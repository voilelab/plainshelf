import { describe, expect, it, vi } from 'vitest';
import { DEFAULT_READ_HISTORY_LIMIT } from './document';
import { ReadHistoryStore } from './index';
import {
  READ_HISTORY_STORAGE_KEY,
  createInMemoryReadHistoryStorage,
  createLocalStorageReadHistoryStorage,
  type ReadHistoryStorage
} from './storage';

function makeStore(storage: ReadHistoryStorage = createInMemoryReadHistoryStorage()): ReadHistoryStore {
  return new ReadHistoryStore(storage);
}

describe('ReadHistoryStore', () => {
  it('starts empty with the default limit', async () => {
    const store = makeStore();

    expect(await store.list('shelf-1')).toEqual([]);
    expect(await store.getLimit()).toBe(DEFAULT_READ_HISTORY_LIMIT);
  });

  it('persists across store instances backed by the same storage', async () => {
    const storage = createInMemoryReadHistoryStorage();
    await makeStore(storage).add('shelf-1', 'book-1');

    expect(await makeStore(storage).list('shelf-1')).toEqual(['book-1']);
  });

  it('keeps the most recently read book first and clears per shelf', async () => {
    const store = makeStore();

    await store.add('shelf-1', 'book-1');
    await store.add('shelf-2', 'book-9');
    await store.add('shelf-1', 'book-2');
    await store.add('shelf-1', 'book-1');

    expect(await store.list('shelf-1')).toEqual(['book-1', 'book-2']);

    await store.clear('shelf-1');

    expect(await store.list('shelf-1')).toEqual([]);
    expect(await store.list('shelf-2')).toEqual(['book-9']);
  });

  it('applies a new limit to already stored history', async () => {
    const store = makeStore();
    await store.add('shelf-1', 'book-1');
    await store.add('shelf-1', 'book-2');

    await store.setLimit(1);

    expect(await store.getLimit()).toBe(1);
    expect(await store.list('shelf-1')).toEqual(['book-2']);
  });

  it('records nothing once the limit is 0', async () => {
    const store = makeStore();
    await store.setLimit(0);

    await store.add('shelf-1', 'book-1');

    expect(await store.list('shelf-1')).toEqual([]);
  });

  // Read-modify-write over one document: concurrent mutations must queue, or
  // the reader's addReadHistory and the settings page's setLimit overwrite each
  // other's copy of the document.
  it('does not lose concurrent writes', async () => {
    const store = makeStore();

    await Promise.all([
      store.add('shelf-1', 'book-1'),
      store.add('shelf-1', 'book-2'),
      store.add('shelf-1', 'book-3')
    ]);

    expect((await store.list('shelf-1')).sort()).toEqual(['book-1', 'book-2', 'book-3']);
  });

  it('keeps accepting writes after a failed one', async () => {
    const backing = createInMemoryReadHistoryStorage();
    const save = vi
      .fn<ReadHistoryStorage['save']>()
      .mockRejectedValueOnce(new Error('quota exceeded'))
      .mockImplementation((text) => backing.save(text));
    const store = makeStore({ load: () => backing.load(), save });

    await expect(store.add('shelf-1', 'book-1')).rejects.toThrow('quota exceeded');
    await store.add('shelf-1', 'book-2');

    expect(await store.list('shelf-1')).toEqual(['book-2']);
  });

  it('recovers from a corrupt stored document instead of throwing', async () => {
    const store = makeStore(createInMemoryReadHistoryStorage('not json'));

    expect(await store.list('shelf-1')).toEqual([]);

    await store.add('shelf-1', 'book-1');

    expect(await store.list('shelf-1')).toEqual(['book-1']);
  });
});

describe('createLocalStorageReadHistoryStorage', () => {
  it('reads and writes the browser storage key', async () => {
    const backing = new Map<string, string>();
    vi.stubGlobal('window', {
      localStorage: {
        getItem: (key: string) => backing.get(key) ?? null,
        setItem: (key: string, value: string) => void backing.set(key, value)
      }
    });

    try {
      const store = makeStore(createLocalStorageReadHistoryStorage());
      await store.add('shelf-1', 'book-1');

      expect(backing.has(READ_HISTORY_STORAGE_KEY)).toBe(true);
      expect(await store.list('shelf-1')).toEqual(['book-1']);
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
