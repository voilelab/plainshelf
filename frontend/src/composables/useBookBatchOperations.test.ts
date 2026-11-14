import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const startBookBatch = vi.hoisted(() => vi.fn());

vi.mock('@/providers', () => ({
  bookshelfWriter: () => ({ startBookBatch, getTaskChain: () => new Promise(() => {}) })
}));
vi.mock('@/composables/useBookStore', () => ({ useBookStore: () => ({ fetchBooks: vi.fn() }) }));
vi.mock('@/composables/useFolderStore', () => ({ useFolderStore: () => ({ fetchFolders: vi.fn() }) }));

import { useBookBatchOperations } from './useBookBatchOperations';

describe('useBookBatchOperations.startSetSeries', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    startBookBatch.mockReset().mockResolvedValue('chain-1');
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('sends the ids in the given order with the name and starting number', async () => {
    const batch = useBookBatchOperations();
    await batch.startSetSeries(['b1', 'b2', 'b10'], { name: 'Saga', startIndex: 1 }, {});

    expect(startBookBatch).toHaveBeenCalledWith({
      operation: 'set_series',
      book_ids: ['b1', 'b2', 'b10'],
      series_name: 'Saga',
      start_index: 1
    });
    expect(batch.operation.value).toBe('set_series');
    vi.clearAllTimers();
    batch.chain.value = null;
  });

  it('sends a clear request without a name or starting number', async () => {
    const batch = useBookBatchOperations();
    await batch.startSetSeries(['b1', 'b2'], null, {});

    expect(startBookBatch).toHaveBeenCalledWith({ operation: 'set_series', book_ids: ['b1', 'b2'], clear: true });
    vi.clearAllTimers();
    batch.chain.value = null;
  });
});
