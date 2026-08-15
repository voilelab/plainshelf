import { describe, expect, it } from 'vitest';

import {
  BOOK_CACHE_SCHEMA_VERSION,
  bookPackagePath,
  findBookCacheFiles,
  indexBookCacheByPath,
  isCoveredByBookCache,
  parseBookCacheFile,
  pickNewestBookCache
} from './bookCacheFile';
import type { BookPackageRef, PCloudFileRef } from './bookpkg';
import type { PCloudItem } from './types';

function validCache(overrides: Record<string, unknown> = {}): unknown {
  return {
    schema_version: BOOK_CACHE_SCHEMA_VERSION,
    writer_id: 'writer01',
    timestamp: 1_700_000_000,
    generator: 'plainshelf/test',
    layers: ['/', 'Fiction'],
    books: {
      a: { path: 'books/Fiction/alpha.bookpkg', meta: { id: 'a', title: 'Alpha' } }
    },
    ...overrides
  };
}

function fileRef(name: string, modified: string, fileid = 1): PCloudFileRef {
  return { fileid, name, size: 10, modified };
}

describe('parseBookCacheFile', () => {
  it('accepts a well-formed cache', () => {
    const cache = parseBookCacheFile(validCache());

    expect(cache?.writer_id).toBe('writer01');
    expect(cache?.books.a?.path).toBe('books/Fiction/alpha.bookpkg');
  });

  // Each rejection costs one full scan and nothing else, so the parser is
  // deliberately strict: a half-valid payload would surface as a silently
  // truncated library instead of a cache miss.
  it.each([
    ['a non-object', 42],
    ['an array', []],
    ['null', null],
    ['a future schema version', validCache({ schema_version: BOOK_CACHE_SCHEMA_VERSION + 1 })],
    ['a missing schema version', validCache({ schema_version: undefined })],
    ['an empty writer id', validCache({ writer_id: '' })],
    ['a non-string writer id', validCache({ writer_id: 7 })],
    ['a missing timestamp', validCache({ timestamp: undefined })],
    ['a zero timestamp', validCache({ timestamp: 0 })],
    ['a non-numeric timestamp', validCache({ timestamp: '1700000000' })],
    ['layers that are not an array', validCache({ layers: 'Fiction' })],
    ['a non-string layer', validCache({ layers: ['/', 3] })],
    ['books as an array', validCache({ books: [] })],
    ['an entry without a path', validCache({ books: { a: { meta: { id: 'a' } } } })],
    ['an entry with an empty path', validCache({ books: { a: { path: '', meta: { id: 'a' } } } })],
    ['an entry without meta', validCache({ books: { a: { path: 'books/a.bookpkg' } } })],
    ['an entry whose meta has no id', validCache({ books: { a: { path: 'books/a.bookpkg', meta: {} } } })]
  ])('rejects %s', (_label, value) => {
    expect(parseBookCacheFile(value)).toBeNull();
  });

  it('accepts a cache with no books', () => {
    // An empty shelf is a legitimate answer, and treating it as a miss would
    // make every listing pay for a scan that finds nothing.
    expect(parseBookCacheFile(validCache({ books: {} }))?.books).toEqual({});
  });
});

describe('findBookCacheFiles', () => {
  const shelfRoot: PCloudItem = {
    name: 'shelf',
    isfolder: true,
    folderid: 1,
    contents: [
      { name: 'books', isfolder: true, folderid: 2, contents: [] },
      {
        name: 'app',
        isfolder: true,
        folderid: 3,
        contents: [
          { name: 'library.lock', isfolder: false, fileid: 10, size: 0, modified: 'x' },
          { name: 'book-cache-one.json', isfolder: false, fileid: 11, size: 5, modified: 'x' },
          { name: 'book-cache-two.json', isfolder: false, fileid: 12, size: 5, modified: 'x' },
          { name: 'book-cache-notjson.txt', isfolder: false, fileid: 13, size: 5, modified: 'x' },
          { name: 'book-cache-dir.json', isfolder: true, folderid: 14, contents: [] }
        ]
      }
    ]
  };

  it('finds only the cache files directly in app/', () => {
    expect(findBookCacheFiles(shelfRoot).map((file) => file.name)).toEqual([
      'book-cache-one.json',
      'book-cache-two.json'
    ]);
  });

  it('returns nothing when the shelf has no app folder', () => {
    expect(findBookCacheFiles({ name: 'shelf', isfolder: true, folderid: 1, contents: [] })).toEqual([]);
  });
});

describe('pickNewestBookCache', () => {
  it('picks the most recently modified file', () => {
    const files = [
      fileRef('book-cache-old.json', 'Sun, 16 Mar 2014 17:26:04 +0000', 1),
      fileRef('book-cache-new.json', 'Tue, 18 Mar 2014 17:26:04 +0000', 2)
    ];

    expect(pickNewestBookCache(files)?.name).toBe('book-cache-new.json');
  });

  it('breaks ties by name so the choice is stable between runs', () => {
    const same = 'Sun, 16 Mar 2014 17:26:04 +0000';
    const files = [fileRef('book-cache-aaa.json', same, 1), fileRef('book-cache-zzz.json', same, 2)];

    expect(pickNewestBookCache(files)?.name).toBe('book-cache-zzz.json');
    expect(pickNewestBookCache([...files].reverse())?.name).toBe('book-cache-zzz.json');
  });

  it('still returns a file when no modification time can be parsed', () => {
    expect(pickNewestBookCache([fileRef('book-cache-one.json', '')])?.name).toBe('book-cache-one.json');
  });

  it('returns null when there is nothing to pick', () => {
    expect(pickNewestBookCache([])).toBeNull();
  });
});

describe('bookPackagePath', () => {
  const pkg = (folderName: string, layers: string[]): BookPackageRef => ({
    folderName,
    folderid: 1,
    layers,
    files: {},
    sources: []
  });

  it('matches the path the exporter writes', () => {
    expect(bookPackagePath(pkg('alpha.bookpkg', []))).toBe('books/alpha.bookpkg');
    expect(bookPackagePath(pkg('alpha.bookpkg', ['Fiction', 'Classics']))).toBe(
      'books/Fiction/Classics/alpha.bookpkg'
    );
  });
});

describe('indexBookCacheByPath', () => {
  it('keys entries by their package path', () => {
    const cache = parseBookCacheFile(validCache());
    expect(cache).not.toBeNull();

    const byPath = indexBookCacheByPath(cache!);
    expect(byPath.get('books/Fiction/alpha.bookpkg')?.title).toBe('Alpha');
    expect(byPath.get('books/alpha.bookpkg')).toBeUndefined();
  });
});

describe('isCoveredByBookCache', () => {
  const walkedAt = Math.floor(Date.parse('Sun, 16 Mar 2014 17:26:04 +0000') / 1000);

  it('covers a book.json last modified before the walk', () => {
    expect(isCoveredByBookCache(fileRef('book.json', 'Sat, 15 Mar 2014 17:26:04 +0000'), walkedAt)).toBe(true);
  });

  it('covers a book.json modified exactly at the walk', () => {
    expect(isCoveredByBookCache(fileRef('book.json', 'Sun, 16 Mar 2014 17:26:04 +0000'), walkedAt)).toBe(true);
  });

  it('does not cover a book.json modified after the walk', () => {
    expect(isCoveredByBookCache(fileRef('book.json', 'Mon, 17 Mar 2014 17:26:04 +0000'), walkedAt)).toBe(false);
  });

  // Erring towards a wasted download rather than towards showing stale
  // metadata: two extra requests are recoverable, wrong titles are not.
  it('does not cover a file with an unreadable modification time', () => {
    expect(isCoveredByBookCache(fileRef('book.json', 'whenever'), walkedAt)).toBe(false);
  });

  it('does not cover a package with no book.json at all', () => {
    expect(isCoveredByBookCache(undefined, walkedAt)).toBe(false);
  });
});
