import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { BookshelfReader } from './bookshelfProvider';

const { getLocalReadingProgress, saveLocalReadingProgress } = vi.hoisted(() => ({
  getLocalReadingProgress: vi.fn(),
  saveLocalReadingProgress: vi.fn()
}));

vi.mock('@/storage/readingProgress', () => ({
  getLocalReadingProgress,
  saveLocalReadingProgress
}));

const { ServerBookshelfProvider } = await import('./serverBookshelfProvider');

describe('ServerBookshelfProvider reading progress', () => {
  beforeEach(() => {
    getLocalReadingProgress.mockReset();
    saveLocalReadingProgress.mockReset();
  });

  it('reads and writes device-local progress without using the book API', async () => {
    getLocalReadingProgress.mockResolvedValue({ char_offset: 42 });
    saveLocalReadingProgress.mockResolvedValue(undefined);
    const provider = new ServerBookshelfProvider();

    await expect(provider.getReadProgress('book-1')).resolves.toEqual({ char_offset: 42 });
    await provider.saveReadProgress('book-1', { char_offset: 99 });

    expect(getLocalReadingProgress).toHaveBeenCalledWith('book-1');
    expect(saveLocalReadingProgress).toHaveBeenCalledWith('book-1', { char_offset: 99 });
  });
});

describe('ServerBookshelfProvider shelf fallback', () => {
  it('does not offer a persisted shelf to fall back to', () => {
    // Typed as the interface on purpose: the property is optional there, and
    // "absent" is only a meaningful assertion against the shape callers hold.
    const provider: BookshelfReader = new ServerBookshelfProvider();

    // The optional capability must be absent, not merely return ''. Callers
    // reach it as `provider.getPersistedShelfID?.() ?? ''`, so "this backend
    // has no device-local shelf identity" is expressed by the method not
    // existing — a server's shelf list is its own source of truth, and a
    // failure to fetch it leaves nothing to fall back to.
    expect(provider.getPersistedShelfID).toBeUndefined();
  });
});
