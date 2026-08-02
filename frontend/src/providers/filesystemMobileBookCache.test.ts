import { beforeEach, describe, expect, it, vi } from 'vitest';

import type {
  DeleteFileOptions,
  MkdirOptions,
  ReaddirOptions,
  ReadFileOptions,
  RenameOptions,
  RmdirOptions,
  StatOptions,
  WriteFileOptions
} from '@capacitor/filesystem';

import type { Book, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import type { CachedBookManifest } from './mobileBookCache';

// In-memory filesystem model shared between the vi.mock factory (hoisted to
// the top of the module) and the tests. Keys are normalized as
// `<DIRECTORY>/<path>` with no leading/trailing slashes, mirroring how the
// real web plugin namespaces paths under the Capacitor Directory.
const fsModel = vi.hoisted(() => {
  const store = new Map<string, { type: 'file'; data: string } | { type: 'directory' }>();
  const normalize = (directory: string | undefined, path: string): string => {
    const clean = path.replace(/^\/+|\/+$/g, '');
    return directory ? `${directory}/${clean}` : clean;
  };
  return { store, normalize };
});

vi.mock('@capacitor/filesystem', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@capacitor/filesystem')>();
  const { store, normalize } = fsModel;

  const parentOf = (key: string): string => {
    const index = key.lastIndexOf('/');
    return index === -1 ? '' : key.slice(0, index);
  };

  const createParents = (key: string): void => {
    let parent = parentOf(key);
    while (parent.length > 0) {
      const existing = store.get(parent);
      if (existing?.type === 'file') {
        throw new Error('The supplied path is a file, not a directory.');
      }
      store.set(parent, { type: 'directory' });
      parent = parentOf(parent);
    }
  };

  const immediateChildren = (key: string): string[] => {
    const prefix = `${key}/`;
    const children: string[] = [];
    for (const candidate of store.keys()) {
      if (candidate.startsWith(prefix) && !candidate.slice(prefix.length).includes('/')) {
        children.push(candidate);
      }
    }
    return children;
  };

  const Filesystem = {
    async writeFile(options: WriteFileOptions) {
      const key = normalize(options.directory, options.path);
      if (typeof options.data !== 'string') {
        throw new Error('Mock filesystem only supports string data.');
      }
      const existing = store.get(key);
      if (existing?.type === 'directory') {
        throw new Error('The supplied path is a directory.');
      }
      if (options.recursive) {
        createParents(key);
      } else if (key.includes('/') && store.get(parentOf(key))?.type !== 'directory') {
        throw new Error('Parent directory does not exist.');
      }
      store.set(key, { type: 'file', data: options.data });
      return { uri: key };
    },

    async readFile(options: ReadFileOptions) {
      const entry = store.get(normalize(options.directory, options.path));
      if (entry === undefined || entry.type !== 'file') {
        throw new Error('File does not exist.');
      }
      return { data: entry.data };
    },

    async stat(options: StatOptions) {
      const key = normalize(options.directory, options.path);
      const entry = store.get(key);
      if (entry === undefined) {
        throw new Error('Entry does not exist.');
      }
      return {
        type: entry.type,
        size: entry.type === 'file' ? entry.data.length : 0,
        mtime: 0,
        uri: key
      };
    },

    async deleteFile(options: DeleteFileOptions) {
      const key = normalize(options.directory, options.path);
      const entry = store.get(key);
      if (entry === undefined || entry.type !== 'file') {
        throw new Error('File does not exist.');
      }
      store.delete(key);
    },

    async rmdir(options: RmdirOptions) {
      const key = normalize(options.directory, options.path);
      const entry = store.get(key);
      if (entry === undefined || entry.type !== 'directory') {
        throw new Error('Folder does not exist.');
      }
      const descendants = Array.from(store.keys()).filter((candidate) =>
        candidate.startsWith(`${key}/`)
      );
      if (descendants.length > 0 && !options.recursive) {
        throw new Error('Folder is not empty.');
      }
      for (const descendant of descendants) {
        store.delete(descendant);
      }
      store.delete(key);
    },

    async readdir(options: ReaddirOptions) {
      const key = normalize(options.directory, options.path);
      const entry = store.get(key);
      if (entry === undefined || entry.type !== 'directory') {
        throw new Error('Folder does not exist.');
      }
      return {
        files: immediateChildren(key).map((childKey) => {
          const child = store.get(childKey)!;
          return {
            name: childKey.slice(key.length + 1),
            type: child.type,
            size: child.type === 'file' ? child.data.length : 0,
            mtime: 0,
            uri: childKey
          };
        })
      };
    },

    async mkdir(options: MkdirOptions) {
      const key = normalize(options.directory, options.path);
      const existing = store.get(key);
      if (existing?.type === 'file') {
        throw new Error('The supplied path is a file, not a directory.');
      }
      if (existing !== undefined && !options.recursive) {
        throw new Error('Directory exists.');
      }
      if (options.recursive) {
        createParents(key);
      } else if (key.includes('/') && store.get(parentOf(key))?.type !== 'directory') {
        throw new Error('Parent directory does not exist.');
      }
      store.set(key, { type: 'directory' });
    },

    async rename(options: RenameOptions) {
      const from = normalize(options.directory, options.from);
      const to = normalize(options.toDirectory ?? options.directory, options.to);
      const entry = store.get(from);
      if (entry === undefined) {
        throw new Error('Entry does not exist.');
      }
      if (store.get(to) !== undefined) {
        throw new Error('Destination exists.');
      }
      if (to.includes('/') && store.get(parentOf(to))?.type !== 'directory') {
        throw new Error('Parent directory does not exist.');
      }

      const moved: [string, { type: 'file'; data: string } | { type: 'directory' }][] = [
        [to, entry]
      ];
      for (const [key, value] of store) {
        if (key.startsWith(`${from}/`)) {
          moved.push([`${to}${key.slice(from.length)}`, value]);
        }
      }
      // Collect first, then delete, then insert: the destination may live inside
      // a subtree that the delete pass would otherwise sweep away.
      store.delete(from);
      for (const key of Array.from(store.keys())) {
        if (key.startsWith(`${from}/`)) {
          store.delete(key);
        }
      }
      for (const [key, value] of moved) {
        store.set(key, value);
      }
    }
  };

  return { ...actual, Filesystem };
});

// Imported after vi.mock so the implementation binds to the mocked plugin,
// and Filesystem/Directory here refer to the mock (used to seed corrupt data).
import { Directory, Filesystem } from '@capacitor/filesystem';
import { setActiveShelfID, setApiBase } from '@/api/client';
import { buildDeviceDocumentKey } from '@/storage/deviceDocument';
import { FilesystemMobileBookCache } from './filesystemMobileBookCache';

const SERVER_A = 'http://10.0.2.2:20000';
const SERVER_B = 'http://192.168.1.50:20000';
const SHELF_A = 'default_shelf';
const SHELF_B = 'comics';

/** Points the API client — and therefore the cache — at one (server, shelf). */
function connectTo(apiBase: string, shelfID: string): void {
  setApiBase(apiBase);
  setActiveShelfID(shelfID);
}

/** Mirrors the production layout, so tests can seed and assert raw paths. */
function booksDirFor(apiBase: string, shelfID: string): string {
  return `plainshelf-cache/scopes/${encodeURIComponent(buildDeviceDocumentKey(apiBase, shelfID))}/books`;
}

const LEGACY_BOOKS_DIR = 'plainshelf-cache/books';

function makeBook(id: string): Book {
  return {
    id,
    title: `Title of ${id}`,
    authors: ['Author A', 'Author B'],
    tags: ['fiction'],
    layers: ['shelf-1']
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

function makeManifest(bookId: string, sourceIds: string[]): CachedBookManifest {
  return {
    book: makeBook(bookId),
    sources: sourceIds.map(makeSource),
    downloaded_at: '2026-07-10T12:00:00Z',
    local_version: 'v-local',
    remote_version: 'v-remote'
  };
}

async function saveFullBook(
  cache: FilesystemMobileBookCache,
  bookId: string,
  sourceId: string
): Promise<void> {
  await cache.saveDownloadedBook(makeManifest(bookId, [sourceId]));
  await cache.saveCachedBookContent(bookId, { content: `content of ${bookId}` });
  await cache.saveCachedSourceContent(bookId, sourceId, `source text of ${bookId}/${sourceId}`);
  await cache.saveReadProgress(bookId, { char_offset: 42, percent: 0.5 });
}

describe('FilesystemMobileBookCache', () => {
  let cache: FilesystemMobileBookCache;
  let booksDir: string;

  beforeEach(() => {
    fsModel.store.clear();
    connectTo(SERVER_A, SHELF_A);
    booksDir = booksDirFor(SERVER_A, SHELF_A);
    cache = new FilesystemMobileBookCache();
  });

  it('round-trips manifest, content, source content and progress', async () => {
    const bookId = 'book-1';
    const sourceId = 'source-1';
    await saveFullBook(cache, bookId, sourceId);

    const listed = await cache.listDownloadedBooks();
    expect(listed).toHaveLength(1);
    expect(listed[0]).toMatchObject({
      id: bookId,
      title: `Title of ${bookId}`,
      download_state: 'downloaded',
      downloaded_at: '2026-07-10T12:00:00Z',
      local_version: 'v-local',
      remote_version: 'v-remote'
    });

    const cached = await cache.getCachedBook(bookId);
    expect(cached).toMatchObject({ id: bookId, download_state: 'downloaded' });

    expect(await cache.getDownloadState(bookId)).toBe('downloaded');

    const sources = await cache.listCachedSources(bookId);
    expect(sources).toEqual([makeSource(sourceId)]);

    expect(await cache.getCachedSource(bookId, sourceId)).toEqual(makeSource(sourceId));

    expect(await cache.getCachedBookContent(bookId)).toEqual({
      content: `content of ${bookId}`
    });

    expect(await cache.getCachedSourceContent(bookId, sourceId)).toBe(
      `source text of ${bookId}/${sourceId}`
    );

    expect(await cache.getReadProgress(bookId)).toEqual({ char_offset: 42, percent: 0.5 });
  });

  it('ignores orphan directories without manifest.json in listDownloadedBooks', async () => {
    await saveFullBook(cache, 'real-book', 'src-1');
    // Orphan: a directory under books/ containing a stray file but no manifest.
    await Filesystem.writeFile({
      path: `${booksDir}/orphan-dir/leftover.txt`,
      data: 'junk',
      directory: Directory.Data,
      recursive: true
    });

    const listed = await cache.listDownloadedBooks();
    expect(listed.map((book) => book.id)).toEqual(['real-book']);
  });

  it('ignores books with malformed manifest JSON', async () => {
    await saveFullBook(cache, 'good-book', 'src-1');
    const badId = 'bad-book';
    await Filesystem.writeFile({
      path: `${booksDir}/${encodeURIComponent(badId)}/manifest.json`,
      data: '{ this is not valid JSON',
      directory: Directory.Data,
      recursive: true
    });

    const listed = await cache.listDownloadedBooks();
    expect(listed.map((book) => book.id)).toEqual(['good-book']);

    expect(await cache.getCachedBook(badId)).toBeNull();
  });

  it('ignores manifests that are valid JSON but the wrong shape', async () => {
    await saveFullBook(cache, 'good-book', 'src-1');
    const wrongShapes: Record<string, string> = {
      'empty-object': '{}',
      'array-manifest': '[]',
      'book-not-object': '{"book": "nope", "sources": []}',
      'missing-sources': '{"book": {"id": "missing-sources"}}'
    };
    for (const [badId, data] of Object.entries(wrongShapes)) {
      await Filesystem.writeFile({
        path: `${booksDir}/${encodeURIComponent(badId)}/manifest.json`,
        data,
        directory: Directory.Data,
        recursive: true
      });
    }

    const listed = await cache.listDownloadedBooks();
    expect(listed.map((book) => book.id)).toEqual(['good-book']);

    for (const badId of Object.keys(wrongShapes)) {
      expect(await cache.getCachedBook(badId)).toBeNull();
      expect(await cache.getDownloadState(badId)).toBe('not_downloaded');
    }
  });

  it('removeDownloadedBook removes everything for one book and nothing of another', async () => {
    await saveFullBook(cache, 'book-A', 'src-A');
    await saveFullBook(cache, 'book-B', 'src-B');

    await cache.removeDownloadedBook('book-A');

    // Book A is fully gone.
    expect(await cache.getCachedBook('book-A')).toBeNull();
    expect(await cache.getDownloadState('book-A')).toBe('not_downloaded');
    expect(await cache.getCachedBookContent('book-A')).toBeNull();
    expect(await cache.getCachedSourceContent('book-A', 'src-A')).toBeNull();
    expect(await cache.listCachedSources('book-A')).toEqual([]);
    expect(await cache.getReadProgress('book-A')).toBeNull();
    expect((await cache.listDownloadedBooks()).map((book) => book.id)).toEqual(['book-B']);

    // Book B is fully intact.
    expect(await cache.getCachedBook('book-B')).toMatchObject({
      id: 'book-B',
      download_state: 'downloaded'
    });
    expect(await cache.getCachedBookContent('book-B')).toEqual({ content: 'content of book-B' });
    expect(await cache.getCachedSourceContent('book-B', 'src-B')).toBe(
      'source text of book-B/src-B'
    );
    expect(await cache.listCachedSources('book-B')).toEqual([makeSource('src-B')]);
    expect(await cache.getReadProgress('book-B')).toEqual({ char_offset: 42, percent: 0.5 });
  });

  it('removeDownloadedBook is idempotent for unknown book ids', async () => {
    await expect(cache.removeDownloadedBook('never-existed')).resolves.toBeUndefined();
  });

  it('round-trips ids with special characters without collisions', async () => {
    const bookA = '中文 book#1';
    const sourceA = 'src/01?.txt';
    // Deliberately chosen so encodeURIComponent(bookB) !== encodeURIComponent(bookA)
    // even though bookB is the encoded form of a slash-containing id.
    const bookB = 'a/b';
    const bookC = 'a%2Fb';

    await saveFullBook(cache, bookA, sourceA);
    await saveFullBook(cache, bookB, 'src-plain');
    await saveFullBook(cache, bookC, 'src-plain');

    expect(await cache.getCachedBook(bookA)).toMatchObject({ id: bookA });
    expect(await cache.getCachedSourceContent(bookA, sourceA)).toBe(
      `source text of ${bookA}/${sourceA}`
    );
    expect(await cache.getReadProgress(bookA)).toEqual({ char_offset: 42, percent: 0.5 });

    // b and c must not collide: each reads back its own data.
    expect(await cache.getCachedBookContent(bookB)).toEqual({ content: `content of ${bookB}` });
    expect(await cache.getCachedBookContent(bookC)).toEqual({ content: `content of ${bookC}` });
    expect((await cache.listCachedSources(bookB))[0].comment).toBe('comment for src-plain');

    const ids = (await cache.listDownloadedBooks()).map((book) => book.id).sort();
    expect(ids).toEqual([bookA, bookB, bookC].sort());

    // Removing one of the near-colliding ids must not affect the other.
    await cache.removeDownloadedBook(bookB);
    expect(await cache.getCachedBook(bookB)).toBeNull();
    expect(await cache.getCachedBook(bookC)).toMatchObject({ id: bookC });
    expect(await cache.getCachedBookContent(bookC)).toEqual({ content: `content of ${bookC}` });
  });

  it('returns null for missing content, source content and source meta', async () => {
    expect(await cache.getCachedBookContent('missing-book')).toBeNull();
    expect(await cache.getCachedSourceContent('missing-book', 'missing-src')).toBeNull();
    expect(await cache.getCachedSource('missing-book', 'missing-src')).toBeNull();

    // Also for a downloaded book whose content/source files were never written.
    await cache.saveDownloadedBook(makeManifest('manifest-only', ['src-1']));
    expect(await cache.getCachedBookContent('manifest-only')).toBeNull();
    expect(await cache.getCachedSourceContent('manifest-only', 'src-1')).toBeNull();
  });

  it('stores progress independently of download state', async () => {
    const bookId = 'not-downloaded-book';
    expect(await cache.getReadProgress(bookId)).toBeNull();

    await cache.saveReadProgress(bookId, { char_offset: 100, percent: 0.25 });

    expect(await cache.getReadProgress(bookId)).toEqual({ char_offset: 100, percent: 0.25 });
    expect(await cache.getDownloadState(bookId)).toBe('not_downloaded');
    expect(await cache.getCachedBook(bookId)).toBeNull();
    // The progress-only directory must not surface as a downloaded book.
    expect(await cache.listDownloadedBooks()).toEqual([]);
  });

  it('round-trips a cover blob, preserving bytes and MIME type', async () => {
    const bookId = 'cover-book';
    const bytes = new Uint8Array([0, 1, 2, 250, 251, 252, 253, 254, 255]);
    await cache.saveCachedCover(bookId, new Blob([bytes], { type: 'image/png' }));

    const readBack = await cache.getCachedCover(bookId);
    expect(readBack).not.toBeNull();
    expect(readBack!.type).toBe('image/png');
    expect(new Uint8Array(await readBack!.arrayBuffer())).toEqual(bytes);
  });

  it('returns null for a missing cover', async () => {
    expect(await cache.getCachedCover('never-had-a-cover')).toBeNull();
  });

  it('removeDownloadedBook also drops the cached cover', async () => {
    const bookId = 'book-with-cover';
    await saveFullBook(cache, bookId, 'src-1');
    await cache.saveCachedCover(bookId, new Blob([new Uint8Array([9, 8, 7])], { type: 'image/jpeg' }));
    expect(await cache.getCachedCover(bookId)).not.toBeNull();

    await cache.removeDownloadedBook(bookId);
    expect(await cache.getCachedCover(bookId)).toBeNull();
  });

  it('persists size accounting on the manifest and returns it via listDownloadedManifests', async () => {
    const manifest: CachedBookManifest = {
      ...makeManifest('sized-book', ['src-1', 'src-2']),
      size_bytes: 1234,
      size_breakdown: { content: 1000, sources: 200, cover: 34 }
    };
    await cache.saveDownloadedBook(manifest);

    const listed = await cache.listDownloadedManifests();
    expect(listed).toHaveLength(1);
    expect(listed[0]).toMatchObject({
      book: { id: 'sized-book' },
      size_bytes: 1234,
      size_breakdown: { content: 1000, sources: 200, cover: 34 }
    });
    expect(listed[0].sources.map((source) => source.id)).toEqual(['src-1', 'src-2']);
  });

  it('listDownloadedManifests omits size fields for manifests saved without them', async () => {
    await cache.saveDownloadedBook(makeManifest('no-size-book', ['src-1']));

    const [manifest] = await cache.listDownloadedManifests();
    expect(manifest.book.id).toBe('no-size-book');
    expect(manifest.size_bytes).toBeUndefined();
    expect(manifest.size_breakdown).toBeUndefined();
  });

  it('listDownloadedManifests ignores orphan and malformed directories', async () => {
    await cache.saveDownloadedBook(makeManifest('good-book', ['src-1']));
    await Filesystem.writeFile({
      path: `${booksDir}/orphan-dir/leftover.txt`,
      data: 'junk',
      directory: Directory.Data,
      recursive: true
    });

    const listed = await cache.listDownloadedManifests();
    expect(listed.map((manifest) => manifest.book.id)).toEqual(['good-book']);
  });
});

// A book id is only unique within one shelf, so the same id routinely means two
// different books across shelves or servers. Downloads must not share storage.
describe('FilesystemMobileBookCache scoping', () => {
  let cache: FilesystemMobileBookCache;

  beforeEach(() => {
    fsModel.store.clear();
    connectTo(SERVER_A, SHELF_A);
    cache = new FilesystemMobileBookCache();
  });

  it('keeps the same book id separate across two shelves on one server', async () => {
    const bookId = 'collide';

    await saveFullBook(cache, bookId, 'src-a');
    await cache.saveCachedCover(bookId, new Blob([new Uint8Array([1])], { type: 'image/png' }));

    connectTo(SERVER_A, SHELF_B);
    expect(await cache.getCachedBook(bookId)).toBeNull();
    expect(await cache.getDownloadState(bookId)).toBe('not_downloaded');
    expect(await cache.getCachedBookContent(bookId)).toBeNull();
    expect(await cache.getReadProgress(bookId)).toBeNull();
    expect(await cache.getCachedCover(bookId)).toBeNull();

    await cache.saveDownloadedBook(makeManifest(bookId, ['src-b']));
    await cache.saveCachedBookContent(bookId, { content: 'shelf B content' });
    await cache.saveReadProgress(bookId, { char_offset: 7, percent: 0.1 });

    connectTo(SERVER_A, SHELF_A);
    expect((await cache.getCachedBookContent(bookId))?.content).toBe(`content of ${bookId}`);
    expect(await cache.getReadProgress(bookId)).toEqual({ char_offset: 42, percent: 0.5 });
    expect((await cache.listCachedSources(bookId)).map((source) => source.id)).toEqual(['src-a']);
    expect(await cache.getCachedCover(bookId)).not.toBeNull();
  });

  it('keeps identically named shelves on different servers separate', async () => {
    await saveFullBook(cache, 'book-1', 'src-1');

    connectTo(SERVER_B, SHELF_A);
    expect(await cache.listDownloadedBooks()).toEqual([]);

    await saveFullBook(cache, 'book-2', 'src-2');
    expect((await cache.listDownloadedBooks()).map((book) => book.id)).toEqual(['book-2']);

    connectTo(SERVER_A, SHELF_A);
    expect((await cache.listDownloadedBooks()).map((book) => book.id)).toEqual(['book-1']);
  });

  it('listDownloadedManifests returns only the connected shelf', async () => {
    await cache.saveDownloadedBook(makeManifest('book-1', ['src-1']));

    connectTo(SERVER_A, SHELF_B);
    await cache.saveDownloadedBook(makeManifest('book-2', ['src-2']));

    expect((await cache.listDownloadedManifests()).map((manifest) => manifest.book.id)).toEqual([
      'book-2'
    ]);

    connectTo(SERVER_A, SHELF_A);
    expect((await cache.listDownloadedManifests()).map((manifest) => manifest.book.id)).toEqual([
      'book-1'
    ]);
  });

  it('removeDownloadedBook leaves the same id on another shelf intact', async () => {
    const bookId = 'collide';
    await saveFullBook(cache, bookId, 'src-a');

    connectTo(SERVER_A, SHELF_B);
    await saveFullBook(cache, bookId, 'src-b');
    await cache.removeDownloadedBook(bookId);
    expect(await cache.getCachedBook(bookId)).toBeNull();

    connectTo(SERVER_A, SHELF_A);
    expect(await cache.getCachedBook(bookId)).not.toBeNull();
    expect((await cache.getCachedBookContent(bookId))?.content).toBe(`content of ${bookId}`);
  });
});

describe('FilesystemMobileBookCache legacy migration', () => {
  // Writes a fully downloaded book straight to disk, bypassing the cache — the
  // migration runs on first use, so seeding through a cache instance would
  // already have triggered it.
  async function seedBookAt(booksDir: string, bookId: string, sourceId: string): Promise<void> {
    const dir = `${booksDir}/${encodeURIComponent(bookId)}`;
    const write = async (path: string, data: string): Promise<void> => {
      await Filesystem.writeFile({ path, data, directory: Directory.Data, recursive: true });
    };
    await write(`${dir}/manifest.json`, JSON.stringify(makeManifest(bookId, [sourceId])));
    await write(`${dir}/content.txt`, `content of ${bookId}`);
    await write(`${dir}/sources/${encodeURIComponent(sourceId)}.txt`, `source text of ${bookId}`);
    await write(`${dir}/progress.json`, JSON.stringify({ char_offset: 42, percent: 0.5 }));
  }

  /** Seeds the pre-scope layout: a book directly under plainshelf-cache/books. */
  async function seedLegacyBook(bookId: string, sourceId: string): Promise<void> {
    await seedBookAt(LEGACY_BOOKS_DIR, bookId, sourceId);
  }

  beforeEach(() => {
    fsModel.store.clear();
    connectTo(SERVER_A, SHELF_A);
  });

  it('adopts pre-scope downloads into the connected shelf', async () => {
    await seedLegacyBook('legacy-book', 'src-1');

    const cache = new FilesystemMobileBookCache();
    expect((await cache.listDownloadedBooks()).map((book) => book.id)).toEqual(['legacy-book']);
    expect((await cache.getCachedBookContent('legacy-book'))?.content).toBe(
      'content of legacy-book'
    );
    expect(await cache.getCachedSourceContent('legacy-book', 'src-1')).toBe(
      'source text of legacy-book'
    );
    expect(await cache.getReadProgress('legacy-book')).toEqual({ char_offset: 42, percent: 0.5 });

    // Moved, not copied.
    expect(fsModel.store.has(`${Directory.Data}/${LEGACY_BOOKS_DIR}`)).toBe(false);
  });

  it('does not adopt pre-scope downloads into a shelf that already has its own', async () => {
    await seedLegacyBook('legacy-book', 'src-1');
    await seedBookAt(booksDirFor(SERVER_A, SHELF_A), 'own-book', 'src-2');

    const cache = new FilesystemMobileBookCache();
    expect((await cache.listDownloadedBooks()).map((book) => book.id)).toEqual(['own-book']);
    // The legacy directory is kept rather than merged or deleted: merging could
    // overwrite this shelf's books with same-id books from another one.
    expect(fsModel.store.has(`${Directory.Data}/${LEGACY_BOOKS_DIR}`)).toBe(true);
  });

  it('adopts pre-scope downloads only into the shelf connected at the time', async () => {
    await seedLegacyBook('legacy-book', 'src-1');

    const cache = new FilesystemMobileBookCache();
    expect((await cache.listDownloadedBooks()).map((book) => book.id)).toEqual(['legacy-book']);

    connectTo(SERVER_B, SHELF_B);
    expect(await cache.listDownloadedBooks()).toEqual([]);
  });

  it('runs at most once, so a later write is not undone by a second move', async () => {
    await seedLegacyBook('legacy-book', 'src-1');

    const cache = new FilesystemMobileBookCache();
    await cache.saveDownloadedBook(makeManifest('added-later', ['src-2']));

    expect((await cache.listDownloadedBooks()).map((book) => book.id).sort()).toEqual([
      'added-later',
      'legacy-book'
    ]);
  });
});
