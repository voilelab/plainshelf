import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '../api/client';
import type { Book, PaginatedBooks, ReadingProgress } from '../types/book';
import type { SourceMeta } from '../types/source';
import type { BookshelfProvider } from './bookshelfProvider';
import { InMemoryMobileBookCache } from './mobileBookCache';
import { MobileBookshelfProvider } from './mobileBookshelfProvider';

// These tests cover the "device online but server unreachable" fallback
// path (e.g. phone on LTE away from the home LAN): navigator.onLine is
// true, so the provider tries the remote first, but the remote call fails
// with a transport-level error (no HTTP response at all) rather than the
// server replying with an error status. In that case the provider must
// fall back to the offline cache instead of surfacing the error, mirroring
// (but not gated behind) the existing isOnline()===false offline path.

function makeBook(id: string, overrides: Partial<Book> = {}): Book {
  return {
    id,
    title: `Title of ${id}`,
    authors: ['Author A'],
    tags: [],
    layers: ['shelf-1'],
    ...overrides
  };
}

function makeSource(id: string): SourceMeta {
  return {
    id,
    created_at: '2026-07-01T00:00:00Z',
    comment: `comment for ${id}`,
    md5_hash: `md5-${id}`
  };
}

function unreachableError(): TypeError {
  return new TypeError('Failed to fetch');
}

function timeoutError(): ApiError {
  return new ApiError('Request timed out — the shelf may be slow or unavailable.', { isTimeout: true });
}

function statusError(status: number): ApiError {
  return new ApiError(`HTTP ${status}`, { status });
}

async function seedDownloadedBook(cache: InMemoryMobileBookCache, id: string, sourceId = 'src-1'): Promise<void> {
  await cache.saveDownloadedBook({
    book: makeBook(id),
    sources: [makeSource(sourceId)],
    downloaded_at: '2026-07-10T12:00:00Z',
    local_version: 'v-local',
    remote_version: 'v-remote'
  });
}

describe('MobileBookshelfProvider — server-unreachable-while-online fallback', () => {
  let cache: InMemoryMobileBookCache;

  beforeEach(() => {
    cache = new InMemoryMobileBookCache();
  });

  function makeProvider(remote: Partial<BookshelfProvider>, isOnline = () => true): MobileBookshelfProvider {
    return new MobileBookshelfProvider(remote as BookshelfProvider, cache, isOnline);
  }

  describe('listBooks', () => {
    it('falls back to the cache when the remote is unreachable (transport error)', async () => {
      await seedDownloadedBook(cache, 'book-1');
      const listBooks = vi.fn().mockRejectedValue(unreachableError());
      const provider = makeProvider({ listBooks });

      const result = await provider.listBooks(1, 20);

      expect(result.items.map((book) => book.id)).toEqual(['book-1']);
      expect(listBooks).toHaveBeenCalledTimes(1);
    });

    it('falls back to the cache when the remote request times out', async () => {
      await seedDownloadedBook(cache, 'book-1');
      const listBooks = vi.fn().mockRejectedValue(timeoutError());
      const provider = makeProvider({ listBooks });

      const result = await provider.listBooks(1, 20);

      expect(result.items.map((book) => book.id)).toEqual(['book-1']);
    });

    it('rethrows a real HTTP error status (e.g. 503) instead of falling back', async () => {
      await seedDownloadedBook(cache, 'book-1');
      const listBooks = vi.fn().mockRejectedValue(statusError(503));
      const provider = makeProvider({ listBooks });

      await expect(provider.listBooks(1, 20)).rejects.toMatchObject({ status: 503 });
    });

    it('does not call remote at all when isOnline() is false', async () => {
      await seedDownloadedBook(cache, 'book-1');
      const listBooks = vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 } as PaginatedBooks);
      const provider = makeProvider({ listBooks }, () => false);

      const result = await provider.listBooks(1, 20);

      expect(result.items.map((book) => book.id)).toEqual(['book-1']);
      expect(listBooks).not.toHaveBeenCalled();
    });
  });

  describe('getBook', () => {
    it('falls back to the cached book when unreachable and cache has it', async () => {
      await seedDownloadedBook(cache, 'book-1');
      const getBook = vi.fn().mockRejectedValue(unreachableError());
      const provider = makeProvider({ getBook });

      const result = await provider.getBook('book-1');

      expect(result.id).toBe('book-1');
    });

    it('rethrows the original transport error when unreachable and cache misses', async () => {
      const originalError = unreachableError();
      const getBook = vi.fn().mockRejectedValue(originalError);
      const provider = makeProvider({ getBook });

      await expect(provider.getBook('missing-book')).rejects.toBe(originalError);
    });
  });

  describe('listSources / getSource', () => {
    it('listSources falls back to cached sources when unreachable', async () => {
      await seedDownloadedBook(cache, 'book-1', 'src-1');
      const listSources = vi.fn().mockRejectedValue(unreachableError());
      const provider = makeProvider({ listSources });

      const result = await provider.listSources('book-1');

      expect(result.map((source) => source.id)).toEqual(['src-1']);
    });

    it('getSource falls back to the cached source when unreachable', async () => {
      await seedDownloadedBook(cache, 'book-1', 'src-1');
      const getSource = vi.fn().mockRejectedValue(unreachableError());
      const provider = makeProvider({ getSource });

      const result = await provider.getSource('book-1', 'src-1');

      expect(result.id).toBe('src-1');
    });
  });

  describe('getReadProgress', () => {
    it('returns a zero offset when unreachable and no cached progress exists', async () => {
      const getReadProgress = vi.fn().mockRejectedValue(unreachableError());
      const provider = makeProvider({ getReadProgress });

      const result: ReadingProgress = await provider.getReadProgress('book-1');

      expect(result).toEqual({ char_offset: 0 });
    });
  });

  describe('addReadHistory', () => {
    it('resolves silently when unreachable', async () => {
      const addReadHistory = vi.fn().mockRejectedValue(unreachableError());
      const provider = makeProvider({ addReadHistory });

      await expect(provider.addReadHistory('book-1')).resolves.toBeUndefined();
    });

    it('rejects with a real HTTP error status instead of swallowing it', async () => {
      const addReadHistory = vi.fn().mockRejectedValue(statusError(500));
      const provider = makeProvider({ addReadHistory });

      await expect(provider.addReadHistory('book-1')).rejects.toMatchObject({ status: 500 });
    });
  });
});

// PR #220 review (P2, useCoverSrc.ts:42): a downloaded book's cover write
// (upload/delete) must invalidate and refresh the local cover cache, or
// getBookCover keeps serving the stale cached blob after the remote cover
// changed.
describe('MobileBookshelfProvider — cover write cache sync', () => {
  let cache: InMemoryMobileBookCache;

  beforeEach(() => {
    cache = new InMemoryMobileBookCache();
  });

  function makeProvider(remote: Partial<BookshelfProvider>): MobileBookshelfProvider {
    return new MobileBookshelfProvider(remote as BookshelfProvider, cache, () => true);
  }

  it('uploadBookCover re-fetches the remote cover and getBookCover returns the new blob for a downloaded book', async () => {
    await seedDownloadedBook(cache, 'book-1');
    await cache.saveCachedCover('book-1', new Blob(['old-cover'], { type: 'image/jpeg' }));

    const newCoverBlob = new Blob(['new-cover'], { type: 'image/jpeg' });
    const uploadBookCover = vi.fn().mockResolvedValue(undefined);
    const getBookCover = vi.fn().mockResolvedValue(newCoverBlob);
    const provider = makeProvider({ uploadBookCover, getBookCover });

    await provider.uploadBookCover('book-1', new File(['x'], 'cover.png'));

    expect(getBookCover).toHaveBeenCalledWith('book-1');
    const result = await provider.getBookCover('book-1');
    expect(result).toBe(newCoverBlob);
  });

  it('deleteBookCover clears the cached cover so getBookCover falls through to remote', async () => {
    await seedDownloadedBook(cache, 'book-1');
    await cache.saveCachedCover('book-1', new Blob(['old-cover'], { type: 'image/jpeg' }));

    const remoteCoverAfterDelete = new Blob(['remote-fallback'], { type: 'image/jpeg' });
    const deleteBookCover = vi.fn().mockResolvedValue(undefined);
    const getBookCover = vi.fn().mockResolvedValue(remoteCoverAfterDelete);
    const provider = makeProvider({ deleteBookCover, getBookCover });

    await provider.deleteBookCover('book-1');

    expect(await cache.getCachedCover('book-1')).toBeNull();

    const result = await provider.getBookCover('book-1');
    expect(getBookCover).toHaveBeenCalledWith('book-1');
    expect(result).toBe(remoteCoverAfterDelete);
  });

  it('uploadBookCover does not create a cache entry for a book that is not downloaded', async () => {
    const newCoverBlob = new Blob(['new-cover'], { type: 'image/jpeg' });
    const uploadBookCover = vi.fn().mockResolvedValue(undefined);
    const getBookCover = vi.fn().mockResolvedValue(newCoverBlob);
    const provider = makeProvider({ uploadBookCover, getBookCover });

    await provider.uploadBookCover('book-not-downloaded', new File(['x'], 'cover.png'));

    expect(uploadBookCover).toHaveBeenCalledWith('book-not-downloaded', expect.any(File));
    expect(getBookCover).not.toHaveBeenCalled();
    expect(await cache.getCachedCover('book-not-downloaded')).toBeNull();
  });

  it('revokes the previously memoized cover object URL after a cover upload', async () => {
    await seedDownloadedBook(cache, 'book-1');
    await cache.saveCachedCover('book-1', new Blob(['old-cover'], { type: 'image/jpeg' }));

    const createObjectURLSpy = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue('blob:mock-url');
    const revokeObjectURLSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);

    const uploadBookCover = vi.fn().mockResolvedValue(undefined);
    const getBookCover = vi.fn().mockResolvedValue(new Blob(['new-cover'], { type: 'image/jpeg' }));
    const provider = makeProvider({ uploadBookCover, getBookCover });

    // Populate coverUrlCache via a cached-cover read path before the upload.
    await provider.listDownloadedBookEntries();
    expect(createObjectURLSpy).toHaveBeenCalledTimes(1);

    await provider.uploadBookCover('book-1', new File(['x'], 'cover.png'));

    expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:mock-url');

    createObjectURLSpy.mockRestore();
    revokeObjectURLSpy.mockRestore();
  });

  it('uploadBookCoverBlob follows the same refresh path as uploadBookCover', async () => {
    await seedDownloadedBook(cache, 'book-1');
    await cache.saveCachedCover('book-1', new Blob(['old-cover'], { type: 'image/jpeg' }));

    const newCoverBlob = new Blob(['new-cover-from-blob'], { type: 'image/jpeg' });
    const uploadBookCoverBlob = vi.fn().mockResolvedValue(undefined);
    const getBookCover = vi.fn().mockResolvedValue(newCoverBlob);
    const provider = makeProvider({ uploadBookCoverBlob, getBookCover });

    await provider.uploadBookCoverBlob('book-1', new Blob(['raw']));

    expect(uploadBookCoverBlob).toHaveBeenCalledWith('book-1', expect.any(Blob));
    expect(getBookCover).toHaveBeenCalledWith('book-1');
    const result = await provider.getBookCover('book-1');
    expect(result).toBe(newCoverBlob);
  });
});
