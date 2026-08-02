import { describe, expect, it } from 'vitest';

import { toIsoDate } from './document';
import { ReadingStatsStore } from './index';
import { createInMemoryReadingStatsStorage, type ReadingStatsStorage } from './storage';

const TODAY = toIsoDate(new Date());
const KEY = 'shelf-1';

function makeStore(
  storage: ReadingStatsStorage = createInMemoryReadingStatsStorage()
): ReadingStatsStore {
  return new ReadingStatsStore(storage);
}

describe('ReadingStatsStore', () => {
  it('starts empty', async () => {
    expect(await makeStore().range(KEY, '2026-01-01', '2026-12-31')).toEqual({});
  });

  it('records reading time and reads it back', async () => {
    const store = makeStore();

    await store.add(KEY, TODAY, 45);

    expect(await store.range(KEY, TODAY, TODAY)).toEqual({ [TODAY]: 45 });
  });

  it('persists across store instances backed by the same storage', async () => {
    const storage = createInMemoryReadingStatsStorage();
    await makeStore(storage).add(KEY, TODAY, 45);

    expect(await makeStore(storage).range(KEY, TODAY, TODAY)).toEqual({ [TODAY]: 45 });
  });

  // Two heartbeat ticks can land while a read is still in flight; without the
  // store's mutation queue one would overwrite the other's document.
  it('does not lose concurrent ticks', async () => {
    const store = makeStore();

    await Promise.all([store.add(KEY, TODAY, 45), store.add(KEY, TODAY, 45), store.add(KEY, TODAY, 45)]);

    expect(await store.range(KEY, TODAY, TODAY)).toEqual({ [TODAY]: 135 });
  });

  it('keeps shelves independent', async () => {
    const store = makeStore();

    await store.add(KEY, TODAY, 45);
    await store.add('shelf-2', TODAY, 10);

    expect(await store.range(KEY, TODAY, TODAY)).toEqual({ [TODAY]: 45 });
    expect(await store.range('shelf-2', TODAY, TODAY)).toEqual({ [TODAY]: 10 });
  });

  it('tolerates a corrupt stored document instead of failing the dashboard', async () => {
    const store = makeStore(createInMemoryReadingStatsStorage('{not json'));

    expect(await store.range(KEY, TODAY, TODAY)).toEqual({});
  });
});
