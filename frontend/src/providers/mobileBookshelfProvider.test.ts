import { beforeEach, describe, expect, it, vi } from 'vitest';
import { strToU8, zipSync } from 'fflate';

const { addReadHistoryMock, clearReadHistoryMock, getReadHistoryIDsMock } = vi.hoisted(() => ({
  addReadHistoryMock: vi.fn(),
  clearReadHistoryMock: vi.fn(),
  getReadHistoryIDsMock: vi.fn()
}));

// isNativePlatform gates whether the provider exposes saveBookContentToFile; it
// defaults to false so every existing test constructs the browser-shaped
// provider (method absent), and the export suite below flips it to true.
const { isNativePlatformMock, writeFileMock } = vi.hoisted(() => ({
  isNativePlatformMock: vi.fn(() => false),
  writeFileMock: vi.fn()
}));

vi.mock('@capacitor/core', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@capacitor/core')>();
  return {
    ...actual,
    Capacitor: { ...actual.Capacitor, isNativePlatform: isNativePlatformMock }
  };
});

vi.mock('@capacitor/filesystem', () => ({
  Directory: { Documents: 'DOCUMENTS', Data: 'DATA' },
  Encoding: { UTF8: 'utf8' },
  Filesystem: { writeFile: writeFileMock }
}));

// Reading history is device-local; the provider must never route it through
// the remote. Stubbing the store keeps these tests off real device storage.
vi.mock('@/storage/readHistory', () => ({
  addReadHistory: addReadHistoryMock,
  clearReadHistory: clearReadHistoryMock,
  getReadHistoryIDs: getReadHistoryIDsMock
}));

import { ApiError, setActiveShelfID, setApiBase } from '@/api/client';
import type { Book, PaginatedBooks, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import type { BookshelfReader } from './bookshelfProvider';
import { InMemoryMobileBookCache } from './mobileBookCache';
import { InMemoryMobileCoverCache } from './mobileCoverCache';
import { ServerBookshelfProvider } from './serverBookshelfProvider';
import { DOWNLOAD_SHELF_CHANGED_ERROR, MobileBookshelfProvider } from './mobileBookshelfProvider';

const SERVER_A = 'http://10.0.2.2:20000';
const SERVER_B = 'http://192.168.1.50:20000';
const SHELF_A = 'default_shelf';
const SHELF_B = 'comics';

/** Points the API client — and therefore the provider's scoping — at one shelf. */
function connectTo(apiBase: string, shelfID: string): void {
  setApiBase(apiBase);
  setActiveShelfID(shelfID);
}

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
    folders: ['shelf-1'],
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

async function seedDownloadedBook(
  cache: InMemoryMobileBookCache,
  id: string,
  sourceId = 'src-1'
): Promise<void> {
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

  function makeProvider(remote: Partial<BookshelfReader>, isOnline = () => true): MobileBookshelfProvider {
    return new MobileBookshelfProvider(remote as BookshelfReader, cache, isOnline);
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

});

describe('MobileBookshelfProvider — device-local reading history', () => {
  let cache: InMemoryMobileBookCache;

  beforeEach(() => {
    cache = new InMemoryMobileBookCache();
    addReadHistoryMock.mockReset().mockResolvedValue(undefined);
    clearReadHistoryMock.mockReset().mockResolvedValue(undefined);
    getReadHistoryIDsMock.mockReset().mockResolvedValue([]);
  });

  function makeProvider(remote: Partial<BookshelfReader>, isOnline = () => true): MobileBookshelfProvider {
    return new MobileBookshelfProvider(remote as BookshelfReader, cache, isOnline);
  }

  it('records into device storage without calling the remote, even offline', async () => {
    const addReadHistory = vi.fn();
    const provider = makeProvider({ addReadHistory }, () => false);

    await provider.addReadHistory('book-1');

    expect(addReadHistoryMock).toHaveBeenCalledWith('book-1');
    expect(addReadHistory).not.toHaveBeenCalled();
  });

  it('clears device storage without calling the remote', async () => {
    const clearReadHistory = vi.fn();
    const provider = makeProvider({ clearReadHistory });

    await provider.clearReadHistory();

    expect(clearReadHistoryMock).toHaveBeenCalledTimes(1);
    expect(clearReadHistory).not.toHaveBeenCalled();
  });

  it('resolves history books from the offline cache when the device is offline', async () => {
    await seedDownloadedBook(cache, 'book-1');
    getReadHistoryIDsMock.mockResolvedValue(['book-1']);
    const listBooks = vi.fn();
    const provider = makeProvider({ listBooks }, () => false);

    const books = await provider.listReadHistoryBooks();

    expect(books.map((book) => book.id)).toEqual(['book-1']);
    expect(listBooks).not.toHaveBeenCalled();
  });

  it('drops history entries whose book no longer exists', async () => {
    getReadHistoryIDsMock.mockResolvedValue(['book-1', 'gone']);
    const listBooks = vi.fn().mockResolvedValue({
      items: [makeBook('book-1')],
      total: 1,
      page: 1,
      pageSize: 20
    } satisfies PaginatedBooks);
    const provider = makeProvider({ listBooks });

    const books = await provider.listReadHistoryBooks();

    expect(books.map((book) => book.id)).toEqual(['book-1']);
  });
});

// The cover-write cache sync from PR #220 review lived here: on a downloaded
// book, uploadBookCover/deleteBookCover invalidated and refreshed the local
// cover blob so getBookCover would not keep serving a stale one. Those methods
// are gone with the rest of the mobile write surface, so the tests went with
// them. If a mobile write mode is ever added, that is the bug to re-fix; see
// git history for MobileBookshelfProvider.refreshCachedCover.

// PR #266 review (P2): the filesystem cache is scoped by (server, shelf), but
// two pieces of state alongside it were not — the cover object-URL memo, and
// the window between a download's remote reads and its cache writes. Both let
// data cross between shelves that share a book id.
describe('MobileBookshelfProvider — (server, shelf) scoping', () => {
  let cache: InMemoryMobileBookCache;

  beforeEach(() => {
    cache = new InMemoryMobileBookCache();
    connectTo(SERVER_A, SHELF_A);
  });

  function makeProvider(remote: Partial<BookshelfReader>): MobileBookshelfProvider {
    return new MobileBookshelfProvider(remote as BookshelfReader, cache, () => true);
  }

  it('does not reuse a memoized cover object URL for the same book id on another shelf', async () => {
    await seedDownloadedBook(cache, 'book-1');
    await cache.saveCachedCover('book-1', new Blob(['cover'], { type: 'image/jpeg' }));

    let created = 0;
    const createObjectURLSpy = vi
      .spyOn(URL, 'createObjectURL')
      .mockImplementation(() => `blob:mock-${(created += 1)}`);
    const provider = makeProvider({});

    const [onShelfA] = await provider.listDownloadedBookEntries();
    expect(onShelfA.book.cover_url).toBe('blob:mock-1');

    connectTo(SERVER_A, SHELF_B);
    const [onShelfB] = await provider.listDownloadedBookEntries();
    // A fresh lookup, not shelf A's memoized URL. With a scoped cache behind it
    // this is what stops one shelf's cover appearing on another's book.
    expect(onShelfB.book.cover_url).toBe('blob:mock-2');

    // Returning to shelf A still uses shelf A's own memo rather than shelf B's.
    connectTo(SERVER_A, SHELF_A);
    const [backOnShelfA] = await provider.listDownloadedBookEntries();
    expect(backOnShelfA.book.cover_url).toBe('blob:mock-1');

    createObjectURLSpy.mockRestore();
  });

  it('memoizes covers per server, not just per shelf name', async () => {
    await seedDownloadedBook(cache, 'book-1');
    await cache.saveCachedCover('book-1', new Blob(['cover'], { type: 'image/jpeg' }));

    let created = 0;
    const createObjectURLSpy = vi
      .spyOn(URL, 'createObjectURL')
      .mockImplementation(() => `blob:mock-${(created += 1)}`);
    const provider = makeProvider({});

    await provider.listDownloadedBookEntries();
    connectTo(SERVER_B, SHELF_A);
    const [onServerB] = await provider.listDownloadedBookEntries();
    expect(onServerB.book.cover_url).toBe('blob:mock-2');

    createObjectURLSpy.mockRestore();
  });

  it('discards a download when the shelf changes while it is in flight', async () => {
    const provider = makeProvider({
      getBook: vi.fn().mockImplementation(async () => {
        // The user switches shelf while the remote reads are outstanding.
        connectTo(SERVER_A, SHELF_B);
        return makeBook('book-1');
      }),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi.fn().mockResolvedValue('source text')
    });

    await expect(provider.downloadBook('book-1')).rejects.toThrow(DOWNLOAD_SHELF_CHANGED_ERROR);

    // Nothing was written, on either shelf: a download split across two scopes
    // is worse than no download at all.
    expect(await cache.listDownloadedManifests()).toEqual([]);
    expect(await cache.getCachedBookContent('book-1')).toBeNull();
  });

  it('downloads the illustrations the text references and reads them back offline', async () => {
    const getSourceAsset = vi
      .fn()
      .mockImplementation(async (_book: string, _source: string, name: string) =>
        new Blob([`bytes of ${name}`], { type: 'image/png' })
      );
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1', { format: 'md' })),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi
        .fn()
        .mockResolvedValue(
          'before\n\n![a](assets/img-0001.png)\n\n![b](assets/img-0002.png)\n\n![again](assets/img-0001.png)\n\n![no](https://example.com/x.png)'
        ),
      getSourceAsset
    });

    await provider.downloadBook('book-1');

    // Exactly the files the reader will render: each referenced one once, and
    // nothing for a link the reader leaves as text.
    expect(getSourceAsset.mock.calls.map((call) => call[2])).toEqual(['img-0001.png', 'img-0002.png']);

    // Offline, the cache answers without touching the remote.
    const offline = new MobileBookshelfProvider({ getSourceAsset } as unknown as BookshelfReader, cache, () => false);
    const blob = await offline.getSourceAsset('book-1', 'src-1', 'img-0001.png');
    expect(await blob.text()).toBe('bytes of img-0001.png');

    const [manifest] = await cache.listDownloadedManifests();
    expect(manifest.size_breakdown?.assets).toBeGreaterThan(0);
  });

  it('downloads an indented list image that the Markdown reader renders', async () => {
    const getSourceAsset = vi.fn().mockResolvedValue(new Blob(['map'], { type: 'image/png' }));
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1', { format: 'md' })),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi.fn().mockResolvedValue('- Floor plan\n  ![map](assets/map.png)'),
      getSourceAsset
    });

    await provider.downloadBook('book-1');

    expect(getSourceAsset).toHaveBeenCalledWith('book-1', 'src-1', 'map.png');
    expect(await cache.getCachedAsset('book-1', 'src-1', 'map.png')).not.toBeNull();
  });

  it('uses the current source format when deciding to fetch illustrations', async () => {
    const getSourceAsset = vi.fn().mockResolvedValue(new Blob(['image'], { type: 'image/png' }));
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1', {
        format: 'txt',
        current_source: 'src-1'
      })),
      listSources: vi.fn().mockResolvedValue([{
        ...makeSource('src-1'),
        schema_version: 1,
        format: 'md'
      }]),
      getBookContent: vi.fn().mockResolvedValue({ content: '![map](assets/map.png)' }),
      getSourceContent: vi.fn().mockResolvedValue('![map](assets/map.png)'),
      getSourceAsset
    });

    await provider.downloadBook('book-1');

    expect(getSourceAsset).toHaveBeenCalledWith('book-1', 'src-1', 'map.png');
  });

  // A plain-text book cannot render an image, so scanning it would only fetch
  // files the reader will never ask for.
  it('does not fetch illustrations for a plain-text book', async () => {
    const getSourceAsset = vi.fn();
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1', { format: 'txt' })),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi.fn().mockResolvedValue('![a](assets/img-0001.png)'),
      getSourceAsset
    });

    await provider.downloadBook('book-1');

    expect(getSourceAsset).not.toHaveBeenCalled();
  });

  // One unreachable picture must not cost the whole download.
  it('completes the download when an illustration fails', async () => {
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1', { format: 'md' })),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi.fn().mockResolvedValue('![a](assets/img-0001.png)'),
      getSourceAsset: vi.fn().mockRejectedValue(new Error('boom'))
    });

    await provider.downloadBook('book-1');

    expect((await cache.listDownloadedManifests()).map((m) => m.book.id)).toEqual(['book-1']);
    expect(await cache.getCachedAsset('book-1', 'src-1', 'img-0001.png')).toBeNull();
  });

  // An asset has no size bound, so a book with many pictures must not put them
  // all in flight — and in memory — at once.
  it('bounds how many illustrations are fetched at a time', async () => {
    let inFlight = 0;
    let peak = 0;
    const getSourceAsset = vi.fn().mockImplementation(async () => {
      inFlight += 1;
      peak = Math.max(peak, inFlight);
      await new Promise((resolve) => setTimeout(resolve, 0));
      inFlight -= 1;
      return new Blob(['bytes'], { type: 'image/png' });
    });

    const links = Array.from({ length: 12 }, (_, i) => `![](assets/img-${i}.png)`).join('\n\n');
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1', { format: 'md' })),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi.fn().mockResolvedValue(links),
      getSourceAsset
    });

    await provider.downloadBook('book-1');

    expect(getSourceAsset).toHaveBeenCalledTimes(12);
    expect(peak).toBeLessThanOrEqual(3);
  });

  // A picture that cannot be stored is a figure the reader falls back on, not
  // a reason to lose the book: the manifest is written either way.
  it('completes the download when storing an illustration fails', async () => {
    const failingCache = new InMemoryMobileBookCache();
    failingCache.saveCachedAsset = vi.fn().mockRejectedValue(new Error('name too long'));

    const provider = new MobileBookshelfProvider(
      {
        getBook: vi.fn().mockResolvedValue(makeBook('book-1', { format: 'md' })),
        listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
        getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
        getSourceContent: vi.fn().mockResolvedValue('![a](assets/img-0001.png)'),
        getSourceAsset: vi.fn().mockResolvedValue(new Blob(['bytes'], { type: 'image/png' }))
      } as unknown as BookshelfReader,
      failingCache,
      () => true
    );

    await provider.downloadBook('book-1');

    expect((await failingCache.listDownloadedManifests()).map((m) => m.book.id)).toEqual(['book-1']);
    expect((await failingCache.listDownloadedManifests())[0].size_breakdown?.assets).toBe(0);
  });

  // The manifest is the download's commit point: getDownloadState reads it alone
  // and the mobile shell gates reading on it, so it must never appear before the
  // content it accounts for. If the content write fails, the book must be left
  // not-downloaded rather than readable-but-empty.
  it('does not mark a book downloaded when its content fails to persist', async () => {
    const failingCache = new InMemoryMobileBookCache();
    failingCache.saveCachedBookContent = vi.fn().mockRejectedValue(new Error('disk full'));

    const provider = new MobileBookshelfProvider(
      {
        getBook: vi.fn().mockResolvedValue(makeBook('book-1')),
        listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
        getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
        getSourceContent: vi.fn().mockResolvedValue('source text')
      } as unknown as BookshelfReader,
      failingCache,
      () => true
    );

    await expect(provider.downloadBook('book-1')).rejects.toThrow('disk full');
    expect(await failingCache.listDownloadedManifests()).toEqual([]);
    expect(await failingCache.getDownloadState('book-1')).toBe('not_downloaded');
  });

  it('completes a download when the shelf is unchanged', async () => {
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1')),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi.fn().mockResolvedValue('source text')
    });

    await provider.downloadBook('book-1');

    expect((await cache.listDownloadedManifests()).map((m) => m.book.id)).toEqual(['book-1']);
    expect(await cache.getCachedBookContent('book-1')).toEqual({ content: 'text' });
  });
});

describe('MobileBookshelfProvider — asset bundle download', () => {
  let cache: InMemoryMobileBookCache;

  beforeEach(() => {
    cache = new InMemoryMobileBookCache();
    connectTo(SERVER_A, SHELF_A);
  });

  function makeProvider(remote: Partial<BookshelfReader>): MobileBookshelfProvider {
    return new MobileBookshelfProvider(remote as BookshelfReader, cache, () => true);
  }

  // The whole point of the ticket: one request for the source's figures instead
  // of one per image. The per-image path must not be touched when the bundle is
  // available.
  it('takes the whole-bundle path when the backend offers it, storing each figure', async () => {
    const zip = zipSync({
      'img-0001.png': strToU8('bytes of img-0001.png'),
      'img-0002.png': strToU8('bytes of img-0002.png')
      // img-0003.png is referenced by the text but absent from the archive — a
      // file the text names that was never uploaded.
    });
    const getSourceAssetsBundle = vi
      .fn()
      .mockResolvedValue(new Blob([zip], { type: 'application/zip' }));
    const getSourceAsset = vi.fn();
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1', { format: 'md' })),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi
        .fn()
        .mockResolvedValue(
          '![a](assets/img-0001.png)\n\n![b](assets/img-0002.png)\n\n![c](assets/img-0003.png)'
        ),
      getSourceAssetsBundle,
      getSourceAsset
    });

    await provider.downloadBook('book-1');

    // One request, carrying exactly the referenced names, and never the
    // per-image path.
    expect(getSourceAssetsBundle).toHaveBeenCalledWith('book-1', 'src-1', [
      'img-0001.png',
      'img-0002.png',
      'img-0003.png'
    ]);
    expect(getSourceAsset).not.toHaveBeenCalled();

    // Each figure the archive carried is stored, with the MIME its name implies;
    // the referenced-but-absent one is simply not.
    const first = await cache.getCachedAsset('book-1', 'src-1', 'img-0001.png');
    expect(await first!.text()).toBe('bytes of img-0001.png');
    expect(first!.type).toBe('image/png');
    expect(await (await cache.getCachedAsset('book-1', 'src-1', 'img-0002.png'))!.text()).toBe(
      'bytes of img-0002.png'
    );
    expect(await cache.getCachedAsset('book-1', 'src-1', 'img-0003.png')).toBeNull();

    const [manifest] = await cache.listDownloadedManifests();
    expect(manifest.size_breakdown?.assets).toBeGreaterThan(0);
  });

  // The bundle is held whole in memory, so a book with many figures is fetched
  // in bounded chunks rather than one giant archive — the request-count win is
  // kept while the Android memory bound survives.
  it('splits a large source into bounded bundle requests', async () => {
    const getSourceAssetsBundle = vi
      .fn()
      .mockImplementation(async (_book: string, _source: string, names: string[]) => {
        const files: Record<string, Uint8Array> = {};
        for (const name of names) {
          files[name] = strToU8(`bytes of ${name}`);
        }
        return new Blob([zipSync(files)], { type: 'application/zip' });
      });
    const getSourceAsset = vi.fn();
    const links = Array.from({ length: 20 }, (_, i) => `![](assets/img-${i}.png)`).join('\n\n');
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1', { format: 'md' })),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi.fn().mockResolvedValue(links),
      getSourceAssetsBundle,
      getSourceAsset
    });

    await provider.downloadBook('book-1');

    // 20 figures at a chunk size of 16 is two requests, neither over the bound,
    // and never the per-image path.
    const requested = getSourceAssetsBundle.mock.calls.map((call) => call[2] as string[]);
    expect(requested).toHaveLength(2);
    expect(Math.max(...requested.map((names) => names.length))).toBeLessThanOrEqual(16);
    expect(getSourceAsset).not.toHaveBeenCalled();

    // Every figure across both chunks was stored, once.
    const allRequested = requested.flat();
    expect(new Set(allRequested).size).toBe(20);
    for (let i = 0; i < 20; i += 1) {
      expect(await cache.getCachedAsset('book-1', 'src-1', `img-${i}.png`)).not.toBeNull();
    }
  });

  // A bundle request that fails (here a transport error) must not cost the book
  // its illustrations: the download drops to the per-file path unchanged.
  it('falls back to per-file fetches when the bundle request fails', async () => {
    const getSourceAssetsBundle = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));
    const getSourceAsset = vi
      .fn()
      .mockImplementation(async (_book: string, _source: string, name: string) =>
        new Blob([`bytes of ${name}`], { type: 'image/png' })
      );
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1', { format: 'md' })),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi
        .fn()
        .mockResolvedValue('![a](assets/img-0001.png)\n\n![b](assets/img-0002.png)'),
      getSourceAssetsBundle,
      getSourceAsset
    });

    await provider.downloadBook('book-1');

    expect(getSourceAssetsBundle).toHaveBeenCalledTimes(1);
    // Every referenced figure was fetched one at a time and stored.
    expect(getSourceAsset.mock.calls.map((call) => call[2]).sort()).toEqual([
      'img-0001.png',
      'img-0002.png'
    ]);
    expect(await (await cache.getCachedAsset('book-1', 'src-1', 'img-0001.png'))!.text()).toBe(
      'bytes of img-0001.png'
    );
    expect((await cache.listDownloadedManifests()).map((m) => m.book.id)).toEqual(['book-1']);
  });

  // A well-formed response that is not a valid archive is a bundle failure like
  // any other, and drops to the per-file path rather than losing the figures.
  it('falls back to per-file fetches when the bundle is a malformed archive', async () => {
    const getSourceAssetsBundle = vi
      .fn()
      .mockResolvedValue(new Blob([new Uint8Array([1, 2, 3, 4])], { type: 'application/zip' }));
    const getSourceAsset = vi.fn().mockResolvedValue(new Blob(['fallback'], { type: 'image/png' }));
    const provider = makeProvider({
      getBook: vi.fn().mockResolvedValue(makeBook('book-1', { format: 'md' })),
      listSources: vi.fn().mockResolvedValue([makeSource('src-1')]),
      getBookContent: vi.fn().mockResolvedValue({ content: 'text' }),
      getSourceContent: vi.fn().mockResolvedValue('![a](assets/img-0001.png)'),
      getSourceAssetsBundle,
      getSourceAsset
    });

    await provider.downloadBook('book-1');

    expect(getSourceAsset).toHaveBeenCalledWith('book-1', 'src-1', 'img-0001.png');
    expect(await cache.getCachedAsset('book-1', 'src-1', 'img-0001.png')).not.toBeNull();
  });
});

describe('MobileBookshelfProvider — export to Documents', () => {
  beforeEach(() => {
    isNativePlatformMock.mockReturnValue(false);
    writeFileMock.mockReset();
    writeFileMock.mockResolvedValue({ uri: 'file:///documents/PlainShelf/book.txt' });
  });

  // The browser preview must keep falling through to useBookActions' blob
  // download; only the native shell can write to Documents.
  it('does not expose saveBookContentToFile off a native platform', () => {
    isNativePlatformMock.mockReturnValue(false);
    const provider = new MobileBookshelfProvider(
      {} as unknown as BookshelfReader,
      new InMemoryMobileBookCache(),
      () => true
    );

    expect(provider.saveBookContentToFile).toBeUndefined();
  });

  it('writes book content to the shared Documents folder and returns its location', async () => {
    isNativePlatformMock.mockReturnValue(true);
    const remote = {
      downloadBookContent: vi
        .fn()
        .mockResolvedValue(new Blob(['book body'], { type: 'text/plain;charset=utf-8' }))
    };
    const provider = new MobileBookshelfProvider(
      remote as unknown as BookshelfReader,
      new InMemoryMobileBookCache(),
      () => true
    );

    const location = await provider.saveBookContentToFile!('book-1', 'My Book.txt');

    expect(location).toBe('Documents/PlainShelf/My Book.txt');
    expect(writeFileMock).toHaveBeenCalledWith({
      path: 'PlainShelf/My Book.txt',
      data: 'book body',
      directory: 'DOCUMENTS',
      encoding: 'utf8',
      recursive: true
    });
  });
});

describe('MobileBookshelfProvider — manual shelf refresh', () => {
  function makeProvider(remote: Partial<BookshelfReader>): MobileBookshelfProvider {
    return new MobileBookshelfProvider(remote as BookshelfReader, new InMemoryMobileBookCache(), () => true);
  }

  it('reports no support when the backend refreshes its own listing', async () => {
    const provider = makeProvider({});

    expect(provider.supportsShelfRefresh()).toBe(false);
    expect(await provider.getShelfFetchedAt()).toBeNull();
    // Still safe to call: the UI hides the control, but nothing may throw.
    await expect(provider.refreshShelf()).resolves.toBeUndefined();
  });

  it('delegates to a backend whose listing is updated by hand', async () => {
    const refreshShelf = vi.fn().mockResolvedValue(undefined);
    const provider = makeProvider({
      supportsShelfRefresh: () => true,
      refreshShelf,
      getShelfFetchedAt: async () => 4_242
    });

    await provider.refreshShelf();

    expect(provider.supportsShelfRefresh()).toBe(true);
    expect(refreshShelf).toHaveBeenCalledTimes(1);
    expect(await provider.getShelfFetchedAt()).toBe(4_242);
  });

  // Only the backend can say what the walk found, so the wrapper must not eat
  // the answer on its way to the button.
  it('passes through what the backend found', async () => {
    const provider = makeProvider({
      supportsShelfRefresh: () => true,
      refreshShelf: async () => ({ bookCount: 12, folderCount: 3 })
    });

    await expect(provider.refreshShelf()).resolves.toEqual({ bookCount: 12, folderCount: 3 });
  });

  it('surfaces a failed update instead of reporting success', async () => {
    const provider = makeProvider({
      supportsShelfRefresh: () => true,
      refreshShelf: async () => {
        throw unreachableError();
      }
    });

    await expect(provider.refreshShelf()).rejects.toBeInstanceOf(TypeError);
  });
});

describe('MobileBookshelfProvider — persistent cover cache', () => {
  let cache: InMemoryMobileBookCache;
  let coverCache: InMemoryMobileCoverCache;

  beforeEach(() => {
    cache = new InMemoryMobileBookCache();
    coverCache = new InMemoryMobileCoverCache();
    connectTo(SERVER_A, SHELF_A);
  });

  function makeProvider(remote: Partial<BookshelfReader>, isOnline = () => true): MobileBookshelfProvider {
    return new MobileBookshelfProvider(remote as BookshelfReader, cache, isOnline, coverCache);
  }

  it('returns the cover from the persistent cache when the download cache misses', async () => {
    await coverCache.setCover('book-1', new Blob(['cached-cover'], { type: 'image/png' }));
    const getBookCover = vi.fn();
    const provider = makeProvider({ getBookCover });

    const result = await provider.getBookCover('book-1');
    expect(await result.text()).toBe('cached-cover');
    expect(getBookCover).not.toHaveBeenCalled();
  });

  it('fetches from remote and writes through to the persistent cache on a full miss', async () => {
    const getBookCover = vi.fn().mockResolvedValue(new Blob(['remote-cover'], { type: 'image/jpeg' }));
    const provider = makeProvider({ getBookCover });

    const result = await provider.getBookCover('book-1');
    expect(await result.text()).toBe('remote-cover');
    expect(getBookCover).toHaveBeenCalledTimes(1);

    // Wait for the fire-and-forget write to complete.
    await vi.waitFor(async () => {
      expect(await coverCache.getCover('book-1')).not.toBeNull();
    });
    const persisted = await coverCache.getCover('book-1');
    expect(await persisted!.text()).toBe('remote-cover');
  });

  it('prefers the download cache over the persistent cover cache', async () => {
    await seedDownloadedBook(cache, 'book-1');
    await cache.saveCachedCover('book-1', new Blob(['download-cover'], { type: 'image/jpeg' }));
    await coverCache.setCover('book-1', new Blob(['persistent-cover'], { type: 'image/png' }));

    const provider = makeProvider({});
    const result = await provider.getBookCover('book-1');
    expect(await result.text()).toBe('download-cover');
  });

  it('does not block on a cache write failure', async () => {
    coverCache.setCover = vi.fn().mockRejectedValue(new Error('disk full'));
    const getBookCover = vi.fn().mockResolvedValue(new Blob(['cover'], { type: 'image/jpeg' }));
    const provider = makeProvider({ getBookCover });

    const result = await provider.getBookCover('book-1');
    expect(await result.text()).toBe('cover');
  });
});

describe('MobileBookshelfProvider shelf fallback', () => {
  it('reports the shelf chosen during connection setup', () => {
    const provider = new MobileBookshelfProvider(
      {} as BookshelfReader,
      new InMemoryMobileBookCache()
    );

    connectTo(SERVER_A, SHELF_A);
    expect(provider.getPersistedShelfID()).toBe(SHELF_A);

    // Read live, not captured: the settings UI repoints this process-wide
    // state in place, and a stale answer here would strand the user on the
    // previous shelf's downloads after switching.
    connectTo(SERVER_B, SHELF_B);
    expect(provider.getPersistedShelfID()).toBe(SHELF_B);
  });
});

describe('MobileBookshelfProvider backend cost forwarding', () => {
  // The behaviour change this PR makes: a phone pointed at a self-hosted
  // server can afford character counts, because the server aggregates them.
  // Only a pCloud-backed shelf cannot.
  it('reports what the wrapped backend can afford', () => {
    const overServer = new MobileBookshelfProvider(
      new ServerBookshelfProvider(),
      new InMemoryMobileBookCache()
    );
    expect(overServer.supportsCharCountListing()).toBe(true);

    const overPCloud = new MobileBookshelfProvider(
      { supportsCharCountListing: () => false } as unknown as BookshelfReader,
      new InMemoryMobileBookCache()
    );
    expect(overPCloud.supportsCharCountListing()).toBe(false);
  });
});
