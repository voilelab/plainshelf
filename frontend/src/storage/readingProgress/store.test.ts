import { describe, expect, it, vi } from 'vitest';
import { ReadingProgressStore, buildReadingProgressKey } from './index';
import {
  READING_PROGRESS_STORAGE_KEY,
  createInMemoryReadingProgressStorage,
  createLocalStorageReadingProgressStorage,
  type ReadingProgressStorage
} from './storage';

function makeStore(
  storage: ReadingProgressStorage = createInMemoryReadingProgressStorage()
): ReadingProgressStore {
  return new ReadingProgressStore(storage);
}

describe('ReadingProgressStore', () => {
  it('starts at zero and persists across store instances', async () => {
    const storage = createInMemoryReadingProgressStorage();
    expect(await makeStore(storage).get('main', 'book-1')).toEqual({ char_offset: 0 });

    await makeStore(storage).save('main', 'book-1', { char_offset: 42 });
    expect(await makeStore(storage).get('main', 'book-1')).toEqual({ char_offset: 42 });
  });

  it('keeps concurrent writes and shelves independent', async () => {
    const store = makeStore();
    await Promise.all([
      store.save('shelf-1', 'book-1', { char_offset: 10 }),
      store.save('shelf-1', 'book-2', { char_offset: 20 }),
      store.save('shelf-2', 'book-1', { char_offset: 30 })
    ]);

    expect(await store.get('shelf-1', 'book-1')).toEqual({ char_offset: 10 });
    expect(await store.get('shelf-1', 'book-2')).toEqual({ char_offset: 20 });
    expect(await store.get('shelf-2', 'book-1')).toEqual({ char_offset: 30 });
  });

  it('recovers from corrupt storage and keeps accepting writes after a failed save', async () => {
    const backing = createInMemoryReadingProgressStorage('not json');
    const save = vi
      .fn<ReadingProgressStorage['save']>()
      .mockRejectedValueOnce(new Error('quota exceeded'))
      .mockImplementation((text) => backing.save(text));
    const store = makeStore({ load: () => backing.load(), save });

    expect(await store.get('main', 'book-1')).toEqual({ char_offset: 0 });
    await expect(store.save('main', 'book-1', { char_offset: 10 })).rejects.toThrow('quota exceeded');
    await store.save('main', 'book-2', { char_offset: 20 });
    expect(await store.get('main', 'book-2')).toEqual({ char_offset: 20 });
  });
});

describe('buildReadingProgressKey', () => {
  it('uses the bare shelf id for same-origin clients and separates remote servers', () => {
    expect(buildReadingProgressKey('', 'default_shelf')).toBe('default_shelf');
    expect(buildReadingProgressKey('http://server-a', 'default_shelf')).not.toBe(
      buildReadingProgressKey('http://server-b', 'default_shelf')
    );
  });
});

describe('createLocalStorageReadingProgressStorage', () => {
  it('reads and writes the browser storage key', async () => {
    const backing = new Map<string, string>();
    vi.stubGlobal('window', {
      localStorage: {
        getItem: (key: string) => backing.get(key) ?? null,
        setItem: (key: string, value: string) => void backing.set(key, value)
      }
    });

    try {
      const store = makeStore(createLocalStorageReadingProgressStorage());
      await store.save('main', 'book-1', { char_offset: 42 });
      expect(backing.has(READING_PROGRESS_STORAGE_KEY)).toBe(true);
      expect(await store.get('main', 'book-1')).toEqual({ char_offset: 42 });
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
