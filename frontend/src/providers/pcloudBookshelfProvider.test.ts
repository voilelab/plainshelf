import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/api/client';
import { PCloudClient } from '@/api/pcloud/client';
import { PCloudError } from '@/api/pcloud/errors';
import type { PCloudItem } from '@/api/pcloud/types';
import type { BookshelfWriter } from './bookshelfProvider';
import { isWritableProvider } from './index';
import { PCloudBookshelfProvider, pcloudCoverUrl } from './pcloudBookshelfProvider';
import { InMemoryShelfSnapshotStore, SHELF_SNAPSHOT_VERSION } from './shelfSnapshotStore';
import type { ShelfSnapshotStore } from './shelfSnapshotStore';

vi.mock('@/storage/readHistory', () => ({
  addReadHistory: vi.fn().mockResolvedValue(undefined),
  clearReadHistory: vi.fn().mockResolvedValue(undefined),
  getReadHistoryIDs: vi.fn().mockResolvedValue([])
}));

const SHELF_ROOT = '/PlainShelf/default-shelf';

let nextFolderID = 100;
let nextFileID = 1000;

interface FileSpec {
  name: string;
  body: string;
  modified?: string;
}

/** Files keyed by the id the fake pCloud hands out, so downloads can resolve. */
let fileBodies: Map<number, string>;

function file(spec: FileSpec): PCloudItem {
  const fileid = nextFileID++;
  fileBodies.set(fileid, spec.body);
  return {
    name: spec.name,
    isfolder: false,
    fileid,
    size: spec.body.length,
    modified: spec.modified ?? 'Sun, 16 Mar 2014 17:26:04 +0000'
  };
}

function folder(name: string, contents: PCloudItem[]): PCloudItem {
  return { name, isfolder: true, folderid: nextFolderID++, contents };
}

interface BookSpec {
  id: string;
  title: string;
  cover?: string;
  currentSource?: string;
  content?: string;
  charCount?: number;
  /** Illustrations in the source's assets/ directory, name to body. */
  assets?: Record<string, string>;
}

function bookPackage(spec: BookSpec): PCloudItem {
  const sourceID = spec.currentSource ?? '20240101-120000';
  const assetFiles = Object.entries(spec.assets ?? {}).map(([name, body]) => file({ name, body }));
  const contents: PCloudItem[] = [
    file({
      name: 'book.json',
      body: JSON.stringify({
        schema_version: 1,
        id: spec.id,
        title: spec.title,
        authors: ['Author'],
        cover: spec.cover ?? '',
        current_source: sourceID
      })
    }),
    folder('sources', [
      folder(sourceID, [
        file({
          name: 'meta.json',
          body: JSON.stringify({
            id: sourceID,
            created_at: '2024-01-01T12:00:00Z',
            comment: '',
            md5_hash: 'abc',
            char_count: spec.charCount ?? 42,
          })
        }),
        file({ name: 'source.txt', body: spec.content ?? 'book text' }),
        ...(assetFiles.length > 0 ? [folder('assets', assetFiles)] : [])
      ])
    ])
  ];

  if (spec.cover) {
    contents.push(file({ name: spec.cover, body: 'cover-bytes' }));
  }

  return folder(`${spec.title}.bookpkg`, contents);
}

function indexFolders(item: PCloudItem, into: Map<number, PCloudItem>): void {
  if (!item.isfolder || item.folderid === undefined) {
    return;
  }
  into.set(item.folderid, item);
  for (const child of item.contents ?? []) {
    indexFolders(child, into);
  }
}

/**
 * A fake pCloud that answers listfolder by folder id and serves downloads from
 * the bodies the tree was built with. Counts requests so the caching and
 * concurrency behaviour can be asserted.
 *
 * The fixture is mounted under SHELF_ROOT so path resolution has real folders
 * to walk segment by segment, which is how the client addresses folders.
 */
function makeFetch(tree: PCloudItem, onDownload?: (fileid: number) => Promise<Response> | null) {
  const calls = { listfolder: 0, recursiveListfolder: 0, getfilelink: 0, download: 0 };

  const segments = SHELF_ROOT.split('/').filter((segment) => segment.length > 0);
  let mounted: PCloudItem = { ...tree, name: segments[segments.length - 1] };
  for (let index = segments.length - 2; index >= 0; index -= 1) {
    mounted = { name: segments[index], isfolder: true, folderid: nextFolderID++, contents: [mounted] };
  }
  const accountRoot: PCloudItem = { name: '/', isfolder: true, folderid: 0, contents: [mounted] };

  const foldersByID = new Map<number, PCloudItem>();
  indexFolders(accountRoot, foldersByID);

  const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
    const url = new URL(String(input));

    if (url.pathname === '/listfolder') {
      calls.listfolder += 1;
      if (url.searchParams.get('recursive') === '1') {
        calls.recursiveListfolder += 1;
      }
      const folder = foldersByID.get(Number(url.searchParams.get('folderid')));
      if (!folder) {
        return Promise.resolve(jsonResponse({ result: 2005, error: 'Directory does not exist.' }));
      }
      return Promise.resolve(jsonResponse({ result: 0, metadata: folder }));
    }

    if (url.pathname === '/getfilelink') {
      calls.getfilelink += 1;
      const fileid = url.searchParams.get('fileid');
      return Promise.resolve(jsonResponse({ result: 0, hosts: ['c1.pcloud.com'], path: `/dl/${fileid}` }));
    }

    calls.download += 1;
    const fileid = Number(url.pathname.replace('/dl/', ''));
    const override = onDownload?.(fileid);
    if (override) {
      return override;
    }
    const body = fileBodies.get(fileid);
    if (body === undefined) {
      return Promise.resolve(new Response('missing', { status: 404, statusText: 'Not Found' }));
    }
    return Promise.resolve(new Response(body));
  });

  return { fetchImpl, calls };
}

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' }
  });
}

function makeProvider(
  tree: PCloudItem,
  overrides: {
    nowImpl?: () => number;
    snapshotStore?: ShelfSnapshotStore;
    onDownload?: (fileid: number) => Promise<Response> | null;
  } = {}
) {
  const { onDownload, ...providerOptions } = overrides;
  const { fetchImpl, calls } = makeFetch(tree, onDownload);
  const client = new PCloudClient({
    host: 'api.pcloud.com',
    accessToken: 'token',
    fetchImpl: fetchImpl as unknown as typeof fetch,
    sleepImpl: async () => {}
  });

  const provider = new PCloudBookshelfProvider({ client, shelfRoot: SHELF_ROOT, ...providerOptions });
  return { provider, calls, fetchImpl };
}

function shelfTree(books: PCloudItem[], extra: PCloudItem[] = []): PCloudItem {
  return folder('default-shelf', [folder('books', books), ...extra]);
}

beforeEach(() => {
  fileBodies = new Map();
  nextFolderID = 100;
  nextFileID = 1000;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('shelf loading', () => {
  it('lists books with their layers', async () => {
    const tree = folder('default-shelf', [
      folder('books', [
        bookPackage({ id: 'root1', title: 'Root Book' }),
        folder('Fiction', [bookPackage({ id: 'fic1', title: 'Fiction Book' })])
      ])
    ]);
    const { provider } = makeProvider(tree);

    const page = await provider.listBooks(1, 10);

    expect(page.total).toBe(2);
    expect(page.items.map((book) => book.id).sort()).toEqual(['fic1', 'root1']);
    expect(page.items.find((book) => book.id === 'fic1')?.layers).toEqual(['Fiction']);
  });

  it('walks the tree once and reuses the result for every later read', async () => {
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await provider.listBooks(1, 10);
    await provider.listBooks(1, 10);
    await provider.getBook('a');

    expect(calls.recursiveListfolder).toBe(1);
  });

  it('collapses concurrent listings onto a single walk', async () => {
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await Promise.all([provider.listBooks(1, 10), provider.listBooks(1, 10), provider.listBooks(1, 10)]);

    expect(calls.recursiveListfolder).toBe(1);
  });

  it('never re-walks on its own, however much time passes', async () => {
    let clock = 0;
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]), {
      nowImpl: () => clock
    });

    await provider.listBooks(1, 10);

    clock = 30 * 24 * 60 * 60 * 1_000;
    await provider.listBooks(1, 10);
    await provider.listLayers();
    await provider.getBook('a');

    expect(calls.recursiveListfolder).toBe(1);
  });

  it('re-walks on refreshShelf but does not re-read unchanged book.json', async () => {
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await provider.listBooks(1, 10);
    const downloadsAfterFirst = calls.download;

    await provider.refreshShelf();

    expect(calls.recursiveListfolder).toBe(2);
    // Size and modified time are unchanged, so the metadata is served from cache.
    expect(calls.download).toBe(downloadsAfterFirst);
  });

  it('joins the first scan instead of walking twice when refreshed during it', async () => {
    // No stored listing, so the initial load becomes a walk of its own; a
    // refresh pressed while it runs must not start a second one.
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]), {
      snapshotStore: new InMemoryShelfSnapshotStore()
    });

    await Promise.all([provider.listBooks(1, 10), provider.refreshShelf()]);

    expect(calls.recursiveListfolder).toBe(1);
  });

  it('still reaches pCloud when refreshed during a restore from the device', async () => {
    const tree = shelfTree([bookPackage({ id: 'a', title: 'A' })]);
    const store = new InMemoryShelfSnapshotStore();
    await makeProvider(tree, { snapshotStore: store }).provider.listBooks(1, 10);

    const { provider, calls } = makeProvider(tree, { snapshotStore: store });
    await Promise.all([provider.listBooks(1, 10), provider.refreshShelf()]);

    // The restore answered from the device, so the refresh owed a real walk.
    expect(calls.recursiveListfolder).toBe(1);
    expect(await provider.getShelfFetchedAt()).not.toBeNull();
  });

  it('collapses concurrent refreshes onto a single walk', async () => {
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await Promise.all([provider.refreshShelf(), provider.refreshShelf(), provider.refreshShelf()]);

    expect(calls.recursiveListfolder).toBe(1);
  });

  it('keeps the previous listing when a refresh fails', async () => {
    const { provider, calls, fetchImpl } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await provider.listBooks(1, 10);
    const walksAfterFirst = calls.recursiveListfolder;

    // Persistent, not once: the client retries a network failure internally.
    const reachable = fetchImpl.getMockImplementation();
    fetchImpl.mockRejectedValue(new TypeError('Failed to fetch'));
    await expect(provider.refreshShelf()).rejects.toBeInstanceOf(ApiError);
    fetchImpl.mockImplementation(reachable!);

    const page = await provider.listBooks(1, 10);
    expect(page.items.map((book) => book.id)).toEqual(['a']);
    // Served from the snapshot the failed refresh left alone.
    expect(calls.recursiveListfolder).toBe(walksAfterFirst);
  });

  it('scans once when the store is empty, and saves what it found', async () => {
    const store = new InMemoryShelfSnapshotStore();
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]), {
      snapshotStore: store,
      nowImpl: () => 1_700_000_000_000
    });

    await provider.listBooks(1, 10);

    expect(calls.recursiveListfolder).toBe(1);
    expect(await store.load()).toMatchObject({
      version: SHELF_SNAPSHOT_VERSION,
      shelf_root: SHELF_ROOT,
      fetched_at: 1_700_000_000_000,
      books: [{ meta: { id: 'a', title: 'A' } }]
    });
  });

  it('serves a fresh process from the store without contacting pCloud', async () => {
    const tree = shelfTree([bookPackage({ id: 'a', title: 'A' }), bookPackage({ id: 'b', title: 'B' })]);
    const store = new InMemoryShelfSnapshotStore();
    const first = makeProvider(tree, { snapshotStore: store });
    await first.provider.listBooks(1, 10);

    // A second provider over the same store stands in for the next app launch.
    const { provider, calls } = makeProvider(tree, { snapshotStore: store });
    const page = await provider.listBooks(1, 10);
    await provider.listLayers();
    await provider.getBook('b');

    expect(page.items.map((book) => book.id)).toEqual(['a', 'b']);
    expect(calls.recursiveListfolder).toBe(0);
    expect(calls.download).toBe(0);
  });

  it('warms the metadata cache, so a later refresh re-reads no unchanged book.json', async () => {
    const tree = shelfTree([bookPackage({ id: 'a', title: 'A' })]);
    const store = new InMemoryShelfSnapshotStore();
    await makeProvider(tree, { snapshotStore: store }).provider.listBooks(1, 10);

    const { provider, calls } = makeProvider(tree, { snapshotStore: store });
    await provider.listBooks(1, 10);
    await provider.refreshShelf();

    expect(calls.recursiveListfolder).toBe(1);
    expect(calls.download).toBe(0);
  });

  it('ignores a snapshot taken from a different shelf root', async () => {
    const tree = shelfTree([bookPackage({ id: 'a', title: 'A' })]);
    const store = new InMemoryShelfSnapshotStore();
    await store.save({
      version: SHELF_SNAPSHOT_VERSION,
      shelf_root: '/PlainShelf/some-other-shelf',
      fetched_at: 1,
      layers: ['/'],
      books: []
    });

    const { provider, calls } = makeProvider(tree, { snapshotStore: store });
    const page = await provider.listBooks(1, 10);

    expect(page.items.map((book) => book.id)).toEqual(['a']);
    expect(calls.recursiveListfolder).toBe(1);
  });

  it('reports when pCloud was last walked without triggering a walk', async () => {
    const store = new InMemoryShelfSnapshotStore();
    const tree = shelfTree([bookPackage({ id: 'a', title: 'A' })]);
    await makeProvider(tree, { snapshotStore: store, nowImpl: () => 4_242 }).provider.listBooks(1, 10);

    const { provider, calls } = makeProvider(tree, { snapshotStore: store });

    expect(await provider.getShelfFetchedAt()).toBe(4_242);
    expect(calls.recursiveListfolder).toBe(0);
  });

  it('reports no sync time before the first scan', async () => {
    const { provider } = makeProvider(shelfTree([]), { snapshotStore: new InMemoryShelfSnapshotStore() });

    expect(await provider.getShelfFetchedAt()).toBeNull();
  });

  it('advertises manual shelf refresh', () => {
    const { provider } = makeProvider(shelfTree([]));

    expect(provider.supportsShelfRefresh()).toBe(true);
  });

  it('paginates client-side', async () => {
    const books = Array.from({ length: 5 }, (_, index) =>
      bookPackage({ id: `b${index}`, title: `Book ${index}` })
    );
    const { provider } = makeProvider(shelfTree(books));

    const page = await provider.listBooks(2, 2);

    expect(page).toMatchObject({ total: 5, page: 2, pageSize: 2 });
    expect(page.items).toHaveLength(2);
  });

  it('rejects a folder that is not a shelf', async () => {
    const { provider } = makeProvider(folder('holiday-photos', [file({ name: 'a.jpg', body: 'x' })]));

    await expect(provider.listBooks()).rejects.toBeInstanceOf(ApiError);
  });

  it('orders books by id so pagination is stable across scans', async () => {
    // pCloud promises no listing order, and the pages are slices of this array,
    // so an unstable order would show a book twice or skip it entirely.
    const { provider } = makeProvider(
      shelfTree([
        bookPackage({ id: 'ccc', title: 'C' }),
        bookPackage({ id: 'aaa', title: 'A' }),
        bookPackage({ id: 'bbb', title: 'B' })
      ])
    );

    const page = await provider.listBooks(1, 10);

    expect(page.items.map((book) => book.id)).toEqual(['aaa', 'bbb', 'ccc']);
  });

  it('propagates a connectivity failure instead of caching an empty shelf', async () => {
    // Swallowing this would hand the wrapper a successful empty listing, and it
    // would show an empty library rather than falling back to downloaded books.
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]), {
      onDownload: () => Promise.reject(new TypeError('Failed to fetch'))
    });

    await expect(provider.listBooks(1, 10)).rejects.toMatchObject({ name: 'ApiError', isTimeout: true });
  });

  it('does not cache a failed scan', async () => {
    let failing = true;
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]), {
      onDownload: () => (failing ? Promise.reject(new TypeError('Failed to fetch')) : null)
    });

    await expect(provider.listBooks(1, 10)).rejects.toBeInstanceOf(ApiError);
    failing = false;

    await expect(provider.listBooks(1, 10)).resolves.toMatchObject({ total: 1 });
  });

  it('skips an unreadable book rather than failing the whole shelf', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {});
    const broken = folder('broken.bookpkg', [file({ name: 'book.json', body: '{ not json' })]);
    const noMeta = folder('empty.bookpkg', [file({ name: 'cover.jpg', body: 'x' })]);
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'ok', title: 'OK' }), broken, noMeta]));

    const page = await provider.listBooks(1, 10);

    expect(page.items.map((book) => book.id)).toEqual(['ok']);
    expect(page.total).toBe(1);
  });
});

describe('layers', () => {
  it('lists every layer directory, including one holding no books', async () => {
    const tree = folder('default-shelf', [
      folder('books', [
        bookPackage({ id: 'a', title: 'A' }),
        folder('Fiction', [bookPackage({ id: 'b', title: 'B' })]),
        folder('Empty', [])
      ])
    ]);
    const { provider } = makeProvider(tree);

    await expect(provider.listLayers()).resolves.toEqual(['/', 'Empty', 'Fiction']);
  });

  it('serves layers from the same walk as the book list', async () => {
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await provider.listBooks(1, 10);
    await provider.listLayers();

    expect(calls.recursiveListfolder).toBe(1);
  });

  it('reports a failed walk rather than an empty layer tree', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]), {
      onDownload: () => Promise.reject(new TypeError('Failed to fetch'))
    });

    await expect(provider.listLayers()).rejects.toBeInstanceOf(ApiError);
  });
});

describe('reading a book', () => {
  it('serves the current source content', async () => {
    const { provider } = makeProvider(
      shelfTree([bookPackage({ id: 'a', title: 'A', content: 'chapter one' })])
    );

    await expect(provider.getBookContent('a')).resolves.toEqual({ content: 'chapter one' });
  });

  it('lists sources and reads one by id', async () => {
    const { provider } = makeProvider(
      shelfTree([bookPackage({ id: 'a', title: 'A', currentSource: '20240301-090000' })])
    );

    const sources = await provider.listSources('a');
    expect(sources.map((source) => source.id)).toEqual(['20240301-090000']);

    await expect(provider.getSource('a', '20240301-090000')).resolves.toMatchObject({ md5_hash: 'abc' });
    await expect(provider.getSourceContent('a', '20240301-090000')).resolves.toBe('book text');
  });

  it('serves an illustration from a source assets/ directory', async () => {
    const { provider } = makeProvider(
      shelfTree([
        bookPackage({
          id: 'a',
          title: 'A',
          content: '![map](assets/img-0001.png)',
          assets: { 'img-0001.png': 'png-bytes' }
        })
      ])
    );

    const blob = await provider.getSourceAsset('a', '20240101-120000', 'img-0001.png');
    expect(await blob.text()).toBe('png-bytes');
  });

  // A missing picture must read as missing, not as the shelf being unreachable:
  // the mobile wrapper treats a reachability failure as grounds to fall back to
  // its cache, and the reader shows alt text for a real miss.
  it('reports an illustration the shelf does not carry', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await expect(provider.getSourceAsset('a', '20240101-120000', 'nope.png')).rejects.toMatchObject({
      name: 'ApiError',
      isTimeout: false
    });
  });

  it('reports an unknown book and an unknown source', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    // Not a reachability problem, so it must not be reported as one: the mobile
    // wrapper would otherwise serve a deleted book from its offline cache.
    await expect(provider.getBook('missing')).rejects.toMatchObject({ name: 'ApiError', isTimeout: false });
    await expect(provider.getSource('a', 'missing')).rejects.toMatchObject({ name: 'ApiError', isTimeout: false });
  });

  it('fills char_count only when asked, and only for the requested page', async () => {
    const books = Array.from({ length: 4 }, (_, index) =>
      bookPackage({ id: `b${index}`, title: `Book ${index}`, charCount: 100 + index })
    );
    const { provider } = makeProvider(shelfTree(books));

    const plain = await provider.listBooks(1, 2);
    expect(plain.items.every((book) => book.char_count === undefined)).toBe(true);

    const withCounts = await provider.listBooks(1, 2, { includeCharCount: true });
    expect(withCounts.items.map((book) => book.char_count)).toEqual([100, 101]);
  });
});

describe('covers', () => {
  it('marks a book that has a cover so the mobile cover path runs', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A', cover: 'cover.jpg' })]));

    const book = await provider.getBook('a');

    expect(book.cover).toBe('cover.jpg');
    // useCoverSrc treats cover_url as a presence flag on mobile: falsy means
    // "no cover", so a book with one must carry something truthy.
    expect(book.cover_url).toBe(pcloudCoverUrl('a'));
  });

  it('leaves cover_url unset when book.json names no cover', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    expect((await provider.getBook('a')).cover_url).toBeUndefined();
  });

  it('leaves cover_url unset when the named cover file is missing', async () => {
    const pkg = folder('a.bookpkg', [
      file({ name: 'book.json', body: JSON.stringify({ id: 'a', title: 'A', cover: 'gone.png' }) })
    ]);
    const { provider } = makeProvider(shelfTree([pkg]));

    expect((await provider.getBook('a')).cover_url).toBeUndefined();
  });

  it('downloads cover bytes as a blob', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A', cover: 'cover.jpg' })]));

    const blob = await provider.getBookCover('a');

    expect(await blob.text()).toBe('cover-bytes');
  });

  it('reports a book with no cover', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await expect(provider.getBookCover('a')).rejects.toBeInstanceOf(ApiError);
  });
});

describe('read-only behaviour', () => {
  // The shelf-mutating methods are absent, not refused: BookshelfWriter is
  // optional and PCloudBookshelfProvider does not implement it. Enumerating
  // them here would not compile, which is the improvement — so this pins the
  // structural claim instead, and writeAccess.test.ts covers what a caller
  // asking for a writer gets.
  it('does not implement the shelf write surface', () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    expect(isWritableProvider(provider)).toBe(false);

    // @ts-expect-error a pCloud shelf must never satisfy BookshelfWriter
    const asWriter: BookshelfWriter = provider;
    expect(asWriter).toBeDefined();
  });

  // Still refused rather than absent: saveReadProgress is on the read surface,
  // because on mobile it is a device-local write that this backend cannot
  // stand in for. Silently discarding would let a caller believe it stored.
  it('refuses to store reading progress', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await expect(provider.saveReadProgress('a', { char_offset: 1 })).rejects.toMatchObject({
      name: 'ApiError',
      status: 403
    });
  });

  it('reports a zero reading position instead of failing', async () => {
    // MobileBookshelfProvider only asks the wrapped provider after its own
    // device-local store came up empty; throwing here would look like a failure
    // to open the book.
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await expect(provider.getReadProgress('a')).resolves.toEqual({ char_offset: 0 });
  });

  it('returns empty results for the server-only views', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await expect(provider.getDuplicateBookGroups()).resolves.toEqual([]);
    await expect(provider.listTrashedBooks()).resolves.toEqual([]);
  });

  it('requires a shelf path', () => {
    const client = new PCloudClient({
      host: 'api.pcloud.com',
      accessToken: 'token',
      fetchImpl: vi.fn() as unknown as typeof fetch
    });

    expect(() => new PCloudBookshelfProvider({ client, shelfRoot: '  ' })).toThrow(PCloudError);
  });
});

describe('exported book cache', () => {
  // Every fixture file defaults to this modification time, so a cache stamped
  // later than it covers the whole shelf.
  const FIXTURE_MODIFIED_SECONDS = Math.floor(Date.parse('Sun, 16 Mar 2014 17:26:04 +0000') / 1000);
  const CACHE_TIMESTAMP = FIXTURE_MODIFIED_SECONDS + 3600;

  interface CacheSpec {
    name?: string;
    modified?: string;
    timestamp?: number;
    writerID?: string;
    schemaVersion?: number;
    layers?: string[];
    books?: Record<string, { path: string; meta: Record<string, unknown> }>;
    /** Replaces the whole body, for malformed-file cases. */
    rawBody?: string;
  }

  function cachedBook(id: string, title: string, folderName: string, layers: string[] = []) {
    return {
      path: ['books', ...layers, folderName].join('/'),
      meta: {
        schema_version: 1,
        id,
        title,
        authors: ['Author'],
        cover: '',
        current_source: '20240101-120000'
      }
    };
  }

  function bookCacheItem(spec: CacheSpec = {}): PCloudItem {
    const body =
      spec.rawBody ??
      JSON.stringify({
        schema_version: spec.schemaVersion ?? 1,
        writer_id: spec.writerID ?? 'writer01',
        timestamp: spec.timestamp ?? CACHE_TIMESTAMP,
        generator: 'plainshelf/test',
        layers: spec.layers ?? ['/'],
        books: spec.books ?? {}
      });

    return file({
      name: spec.name ?? 'book-cache-writer01.json',
      body,
      modified: spec.modified
    });
  }

  function shelfWithCache(books: PCloudItem[], caches: PCloudItem[]): PCloudItem {
    return shelfTree(books, [folder('app', [file({ name: 'library.lock', body: '' }), ...caches])]);
  }

  it('lists books from the cache without downloading any book.json', async () => {
    const tree = shelfWithCache(
      [bookPackage({ id: 'a', title: 'Alpha' }), bookPackage({ id: 'b', title: 'Beta' })],
      [
        bookCacheItem({
          books: {
            // Titles differ from the on-disk fixtures purely so the assertion
            // can prove the listing came from the cache and not from a download.
            a: cachedBook('a', 'Alpha From Cache', 'Alpha.bookpkg'),
            b: cachedBook('b', 'Beta From Cache', 'Beta.bookpkg')
          }
        })
      ]
    );

    const { provider, calls } = makeProvider(tree);
    const page = await provider.listBooks(1, 10);

    expect(page.items.map((book) => book.title)).toEqual(['Alpha From Cache', 'Beta From Cache']);
    // One download: the cache itself. Without it this shelf costs two per book.
    expect(calls.download).toBe(1);
    expect(calls.recursiveListfolder).toBe(1);
  });

  it('re-reads only the books changed after the cache was written', async () => {
    const stale = bookPackage({ id: 'a', title: 'Alpha' });
    const changed = folder('Beta.bookpkg', [
      file({
        name: 'book.json',
        body: JSON.stringify({
          schema_version: 1,
          id: 'b',
          title: 'Beta Updated',
          authors: ['Author'],
          cover: '',
          current_source: '20240101-120000'
        }),
        // Modified after the walk, so the cache cannot speak for it.
        modified: 'Mon, 17 Mar 2014 17:26:04 +0000'
      })
    ]);

    const tree = shelfWithCache(
      [stale, changed],
      [
        bookCacheItem({
          books: {
            a: cachedBook('a', 'Alpha From Cache', 'Alpha.bookpkg'),
            b: cachedBook('b', 'Beta Before The Change', 'Beta.bookpkg')
          }
        })
      ]
    );

    const { provider, calls } = makeProvider(tree);
    const page = await provider.listBooks(1, 10);

    expect(page.items.map((book) => book.title)).toEqual(['Alpha From Cache', 'Beta Updated']);
    // The cache plus the one book.json that moved on.
    expect(calls.download).toBe(2);
  });

  it('reads a book missing from the cache rather than dropping it', async () => {
    const tree = shelfWithCache(
      [bookPackage({ id: 'a', title: 'Alpha' }), bookPackage({ id: 'b', title: 'Beta' })],
      [bookCacheItem({ books: { a: cachedBook('a', 'Alpha From Cache', 'Alpha.bookpkg') } })]
    );

    const { provider, calls } = makeProvider(tree);
    const page = await provider.listBooks(1, 10);

    expect(page.items.map((book) => book.title)).toEqual(['Alpha From Cache', 'Beta']);
    expect(calls.download).toBe(2);
  });

  it.each([
    ['a corrupt file', { rawBody: '{not json' }],
    ['an unknown schema version', { schemaVersion: 99 }],
    ['a payload missing required fields', { rawBody: JSON.stringify({ schema_version: 1, books: {} }) }]
  ])('falls back to a full scan on %s', async (_label, spec) => {
    vi.spyOn(console, 'warn').mockImplementation(() => {});

    const tree = shelfWithCache([bookPackage({ id: 'a', title: 'Alpha' })], [bookCacheItem(spec as CacheSpec)]);

    const { provider, calls } = makeProvider(tree);
    const page = await provider.listBooks(1, 10);

    // The listing is still correct; only the saving is lost.
    expect(page.items.map((book) => book.title)).toEqual(['Alpha']);
    expect(calls.download).toBe(2);
  });

  it('uses the most recently written cache when several machines wrote one', async () => {
    const tree = shelfWithCache(
      [bookPackage({ id: 'a', title: 'Alpha' })],
      [
        bookCacheItem({
          name: 'book-cache-olddesktop.json',
          modified: 'Sun, 16 Mar 2014 18:00:00 +0000',
          books: { a: cachedBook('a', 'From The Old Desktop', 'Alpha.bookpkg') }
        }),
        bookCacheItem({
          name: 'book-cache-newlaptop.json',
          modified: 'Tue, 18 Mar 2014 18:00:00 +0000',
          books: { a: cachedBook('a', 'From The New Laptop', 'Alpha.bookpkg') }
        })
      ]
    );

    const { provider, calls } = makeProvider(tree);
    const page = await provider.listBooks(1, 10);

    expect(page.items.map((book) => book.title)).toEqual(['From The New Laptop']);
    // Only the chosen cache is fetched; the others are never downloaded.
    expect(calls.download).toBe(1);
  });

  it('matches cache entries by layer path, not by folder name alone', async () => {
    const tree = shelfWithCache(
      [folder('Fiction', [bookPackage({ id: 'a', title: 'Alpha' })])],
      [
        bookCacheItem({
          layers: ['/', 'Fiction'],
          books: {
            // Recorded at the top level; the book actually lives under Fiction,
            // so this entry must not be applied to it.
            a: cachedBook('a', 'Wrong Layer', 'Alpha.bookpkg')
          }
        })
      ]
    );

    const { provider, calls } = makeProvider(tree);
    const page = await provider.listBooks(1, 10);

    expect(page.items.map((book) => book.title)).toEqual(['Alpha']);
    expect(page.items[0]?.layers).toEqual(['Fiction']);
    expect(calls.download).toBe(2);
  });

  it('does not fetch the cache on a refresh that has nothing to read', async () => {
    const tree = shelfWithCache(
      [bookPackage({ id: 'a', title: 'Alpha' })],
      [bookCacheItem({ books: { a: cachedBook('a', 'Alpha From Cache', 'Alpha.bookpkg') } })]
    );

    const { provider, calls } = makeProvider(tree);
    await provider.listBooks(1, 10);
    expect(calls.download).toBe(1);

    // Every book.json is already known and unchanged, so there is nothing the
    // cache could answer — fetching it would make refreshing cost more than it
    // did before the cache existed.
    await provider.refreshShelf();
    expect(calls.download).toBe(1);
  });

  it('scans normally when the shelf has no exported cache', async () => {
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'Alpha' })]));

    const page = await provider.listBooks(1, 10);

    expect(page.items.map((book) => book.title)).toEqual(['Alpha']);
    expect(calls.download).toBe(1);
  });
});

describe('PCloudBookshelfProvider listing cost', () => {
  // The filter is hidden here because answering costs one meta.json download
  // per book on a metered transport — a property of this backend, not of the
  // device asking. A phone pointed at a self-hosted server keeps the filter.
  it('declares character-count listing unaffordable', () => {
    const provider = new PCloudBookshelfProvider({
      client: {} as ConstructorParameters<typeof PCloudBookshelfProvider>[0]['client'],
      shelfRoot: '/plainshelf'
    });

    expect(provider.supportsCharCountListing()).toBe(false);
  });
});
