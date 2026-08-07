import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/api/client';
import { PCloudClient } from '@/api/pcloud/client';
import { PCloudError } from '@/api/pcloud/errors';
import type { PCloudItem } from '@/api/pcloud/types';
import { PCloudBookshelfProvider, pcloudCoverUrl } from './pcloudBookshelfProvider';

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
}

function bookPackage(spec: BookSpec): PCloudItem {
  const sourceID = spec.currentSource ?? '20240101-120000';
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
            split_config: { type: 'line_count', line_count: 40 }
          })
        }),
        file({ name: 'source.txt', body: spec.content ?? 'book text' })
      ])
    ])
  ];

  if (spec.cover) {
    contents.push(file({ name: spec.cover, body: 'cover-bytes' }));
  }

  return folder(`${spec.title}.bookpkg`, contents);
}

/**
 * A fake pCloud that answers listfolder from a fixed tree and serves downloads
 * from the bodies the tree was built with. Counts requests so the caching and
 * concurrency behaviour can be asserted.
 */
function makeFetch(tree: PCloudItem, onDownload?: (fileid: number) => Promise<Response> | null) {
  const calls = { listfolder: 0, getfilelink: 0, download: 0 };

  const fetchImpl = vi.fn().mockImplementation((input: RequestInfo | URL) => {
    const url = new URL(String(input));

    if (url.pathname === '/listfolder') {
      calls.listfolder += 1;
      return Promise.resolve(jsonResponse({ result: 0, metadata: tree }));
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
    scanIntervalMs?: number;
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

  it('walks the tree once and reuses the result within the scan interval', async () => {
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await provider.listBooks(1, 10);
    await provider.listBooks(1, 10);
    await provider.getBook('a');

    expect(calls.listfolder).toBe(1);
  });

  it('collapses concurrent listings onto a single walk', async () => {
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await Promise.all([provider.listBooks(1, 10), provider.listBooks(1, 10), provider.listBooks(1, 10)]);

    expect(calls.listfolder).toBe(1);
  });

  it('re-walks after the scan interval but does not re-read unchanged book.json', async () => {
    let clock = 0;
    const { provider, calls } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]), {
      nowImpl: () => clock,
      scanIntervalMs: 1_000
    });

    await provider.listBooks(1, 10);
    const downloadsAfterFirst = calls.download;

    clock = 5_000;
    await provider.listBooks(1, 10);

    expect(calls.listfolder).toBe(2);
    // Size and modified time are unchanged, so the metadata is served from cache.
    expect(calls.download).toBe(downloadsAfterFirst);
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

    expect(calls.listfolder).toBe(1);
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

  it('reads the split config from the current source meta', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    await expect(provider.getBookSplitConfig('a')).resolves.toEqual({ type: 'line_count', line_count: 40 });
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
  it('refuses every mutation', async () => {
    const { provider } = makeProvider(shelfTree([bookPackage({ id: 'a', title: 'A' })]));

    const mutations: Array<[string, () => Promise<unknown>]> = [
      ['updateBook', () => provider.updateBook('a', { title: 'x' })],
      ['updateBookLayer', () => provider.updateBookLayer('a', 'L')],
      ['deleteBook', () => provider.deleteBook('a')],
      ['updateBookSplitConfig', () => provider.updateBookSplitConfig('a', { type: 'none' })],
      ['uploadBookCoverBlob', () => provider.uploadBookCoverBlob('a', new Blob(['x']))],
      ['deleteBookCover', () => provider.deleteBookCover('a')],
      ['restoreTrashedBook', () => provider.restoreTrashedBook('a')],
      ['deleteTrashedBook', () => provider.deleteTrashedBook('a')],
      ['emptyTrash', () => provider.emptyTrash()],
      ['startBookBatch', () => provider.startBookBatch({} as never)],
      ['getTaskChain', () => provider.getTaskChain('t')],
      ['createSource', () => provider.createSource('a')],
      ['deleteSource', () => provider.deleteSource('a', 's')],
      ['setCurrentSource', () => provider.setCurrentSource('a', 's')],
      ['updateSourceContent', () => provider.updateSourceContent('a', 's', 'x')],
      ['refreshSourceMeta', () => provider.refreshSourceMeta('a', 's')],
      ['saveReadProgress', () => provider.saveReadProgress('a', { char_offset: 1 })]
    ];

    for (const [name, run] of mutations) {
      await expect(run(), `${name} should be refused`).rejects.toMatchObject({ name: 'ApiError', status: 403 });
    }
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
