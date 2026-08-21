import { describe, expect, it, vi } from 'vitest';

import { isWritableProvider } from '@/providers';
import type { BookshelfProvider, BookshelfReader } from '@/providers/bookshelfProvider';
import { ServerBookshelfProvider } from '@/providers/serverBookshelfProvider';
import { ReaderBookshelfProvider } from './readerBookshelfProvider';

function readerOver(source: Partial<BookshelfReader> = {}): ReaderBookshelfProvider {
  return new ReaderBookshelfProvider(source as BookshelfReader);
}

describe('ReaderBookshelfProvider', () => {
  // This is what turns the editing UI off. useWriteAccess asks
  // isWritableProvider(), and everything that depends on it — the edit actions,
  // the trash and maintenance nav, the server settings tabs, the logs — follows
  // from the answer. A writable provider pointed at a reading server would
  // render all of it and fail on every click.
  it('is not a writable provider, unlike the one it wraps', () => {
    expect(isWritableProvider(readerOver() as BookshelfProvider)).toBe(false);
    expect(isWritableProvider(new ServerBookshelfProvider())).toBe(true);
  });

  it('forwards the reads to the backend', async () => {
    const listBooks = vi.fn().mockResolvedValue({ items: [], total: 0 });
    const getSourceContent = vi.fn().mockResolvedValue('# Chapter');

    const reader = readerOver({ listBooks, getSourceContent });

    await reader.listBooks(2, 20, { includeCharCount: true });
    await expect(reader.getSourceContent('book-1', 'source-1')).resolves.toBe('# Chapter');

    expect(listBooks).toHaveBeenCalledWith(2, 20, { includeCharCount: true });
    expect(getSourceContent).toHaveBeenCalledWith('book-1', 'source-1');
  });

  // A reading server mounts neither the trash listing nor the duplicate scan.
  // The pages that show them are blocked by the shell's route policy; this is
  // the second line, so a stray caller gets an empty result rather than a 404.
  it('answers the shelf-wide reads a reading server does not mount as empty', async () => {
    const listTrashedBooks = vi.fn();
    const getDuplicateBookGroups = vi.fn();

    const reader = readerOver({ listTrashedBooks, getDuplicateBookGroups });

    await expect(reader.listTrashedBooks()).resolves.toEqual([]);
    await expect(reader.getDuplicateBookGroups()).resolves.toEqual([]);
    expect(listTrashedBooks).not.toHaveBeenCalled();
    expect(getDuplicateBookGroups).not.toHaveBeenCalled();
  });

  // supportsShelfRefresh is what the manual-update button asks, and the rescan
  // endpoint is not mounted either. Absent rather than false so the answer comes
  // from the surface itself.
  it('offers no manual shelf refresh', () => {
    // Typed as the interface, where both members are optional, so the assertion
    // is about the object rather than about what the class happens to declare.
    const reader: BookshelfReader = readerOver({
      supportsShelfRefresh: () => true,
      refreshShelf: vi.fn()
    });

    expect(reader.supportsShelfRefresh).toBeUndefined();
    expect(reader.refreshShelf).toBeUndefined();
  });

  it('reads a source asset through the backend', async () => {
    const blob = new Blob(['png']);
    const getSourceAsset = vi.fn().mockResolvedValue(blob);

    await expect(
      readerOver({ getSourceAsset }).getSourceAsset('book-1', 'source-1', 'fig.png')
    ).resolves.toBe(blob);
    expect(getSourceAsset).toHaveBeenCalledWith('book-1', 'source-1', 'fig.png');
  });

  // The reader falls back to an image's alt text when this rejects, so a backend
  // without the method costs a chapter its illustration rather than the chapter.
  it('rejects an asset read the backend cannot answer', async () => {
    await expect(readerOver().getSourceAsset('book-1', 'source-1', 'fig.png')).rejects.toThrow();
  });
});
