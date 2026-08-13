import { beforeEach, describe, expect, it, vi } from 'vitest';

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
