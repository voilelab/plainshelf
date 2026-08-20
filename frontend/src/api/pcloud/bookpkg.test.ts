import { describe, expect, it } from 'vitest';

import {
  BOOK_META_SCHEMA_VERSION,
  collectBookPackages,
  collectLayers,
  findBooksFolder,
  findCoverFile,
  findCurrentSource,
  isSchemaNewerThanSupported,
  parseBookJson,
  toBook,
  toSourceMeta
} from './bookpkg';
import { PCloudError } from './errors';
import type { PCloudItem } from './types';

let nextID = 1;

function folder(name: string, contents: PCloudItem[] = []): PCloudItem {
  return { name, isfolder: true, folderid: nextID++, contents };
}

function file(name: string, overrides: Partial<PCloudItem> = {}): PCloudItem {
  return {
    name,
    isfolder: false,
    fileid: nextID++,
    size: 128,
    modified: 'Sun, 16 Mar 2014 17:26:04 +0000',
    ...overrides
  };
}

function bookPackage(name: string, sourceIDs: string[] = ['20240101-120000']): PCloudItem {
  return folder(name, [
    file('book.json'),
    file('cover.jpg'),
    file('CURRENT_SOURCE.txt'),
    folder(
      'sources',
      sourceIDs.map((id) => folder(id, [file('meta.json'), file('source.txt')]))
    )
  ]);
}

describe('findBooksFolder', () => {
  it('locates books/ in a shelf root', () => {
    const root = folder('default-shelf', [folder('books'), folder('app')]);
    expect(findBooksFolder(root)?.name).toBe('books');
  });

  it('returns undefined when the folder is not a shelf', () => {
    expect(findBooksFolder(folder('holiday-photos', [file('a.jpg')]))).toBeUndefined();
  });
});

describe('collectBookPackages', () => {
  it('records layers from the directory path and stops at .bookpkg', () => {
    const books = folder('books', [
      bookPackage('root-book.bookpkg'),
      folder('Fiction', [
        bookPackage('a.bookpkg'),
        folder('Classics', [bookPackage('b.bookpkg')])
      ])
    ]);

    const packages = collectBookPackages(books);
    const byName = new Map(packages.map((pkg) => [pkg.folderName, pkg]));

    expect(packages).toHaveLength(3);
    expect(byName.get('root-book.bookpkg')?.layers).toEqual([]);
    expect(byName.get('a.bookpkg')?.layers).toEqual(['Fiction']);
    expect(byName.get('b.bookpkg')?.layers).toEqual(['Fiction', 'Classics']);
  });

  it('does not read a package out of a system directory', () => {
    const books = folder('books', [
      folder('@eaDir', [bookPackage('thumbnail.bookpkg')]),
      folder('#recycle', [bookPackage('deleted.bookpkg')]),
      bookPackage('kept.bookpkg')
    ]);

    expect(collectBookPackages(books).map((pkg) => pkg.folderName)).toEqual(['kept.bookpkg']);
  });

  it('captures book.json, sibling files and sources', () => {
    const [pkg] = collectBookPackages(folder('books', [bookPackage('a.bookpkg')]));

    expect(pkg.meta?.name).toBe('book.json');
    expect(pkg.meta?.size).toBe(128);
    expect(pkg.meta?.modified).toBe('Sun, 16 Mar 2014 17:26:04 +0000');
    expect(Object.keys(pkg.files).sort()).toEqual([
      'CURRENT_SOURCE.txt',
      'book.json',
      'cover.jpg'
    ]);
    expect(pkg.sources).toHaveLength(1);
    expect(pkg.sources[0].meta?.name).toBe('meta.json');
    expect(pkg.sources[0].content?.name).toBe('source.txt');
  });

  it('indexes the illustrations in a source assets/ directory', () => {
    const root = folder('shelf', [
      folder('books', [
        folder('art.bookpkg', [
          file('book.json'),
          folder('sources', [
            folder('20240101-120000', [
              file('meta.json'),
              file('source.txt'),
              folder('assets', [file('img-0001.png'), file('A Map.JPEG')])
            ])
          ])
        ])
      ])
    ]);

    const [pkg] = collectBookPackages(findBooksFolder(root)!);

    expect(Object.keys(pkg.sources[0].assets).sort()).toEqual(['A Map.JPEG', 'img-0001.png']);
    expect(pkg.sources[0].assets['img-0001.png'].fileid).toBeGreaterThan(0);
    // assets/ is a directory inside the source, never a source of its own.
    expect(pkg.sources.map((source) => source.id)).toEqual(['20240101-120000']);
  });

  it('gives a source with no assets directory an empty record', () => {
    const root = folder('shelf', [folder('books', [bookPackage('plain.bookpkg')])]);

    const [pkg] = collectBookPackages(findBooksFolder(root)!);

    expect(pkg.sources[0].assets).toEqual({});
  });

  it('sorts sources chronologically regardless of listing order', () => {
    const [pkg] = collectBookPackages(
      folder('books', [bookPackage('a.bookpkg', ['20240301-090000', '20230101-120000'])])
    );

    expect(pkg.sources.map((source) => source.id)).toEqual([
      '20230101-120000',
      '20240301-090000'
    ]);
  });

  it('leaves meta undefined for a package with no book.json', () => {
    const broken = folder('broken.bookpkg', [file('cover.jpg')]);
    const [pkg] = collectBookPackages(folder('books', [broken]));

    expect(pkg.meta).toBeUndefined();
    expect(pkg.sources).toEqual([]);
  });
});

describe('collectLayers', () => {
  it('lists nested layers plus the top level', () => {
    const books = folder('books', [
      bookPackage('root.bookpkg'),
      folder('Fiction', [bookPackage('a.bookpkg'), folder('Classics', [bookPackage('b.bookpkg')])]),
      folder('Non-Fiction', [bookPackage('c.bookpkg')])
    ]);

    expect(collectLayers(books)).toEqual(['/', 'Fiction', 'Fiction/Classics', 'Non-Fiction']);
  });

  // Derived from directories, not from the books inside them: the Go side walks
  // real directories, so a layer created but not yet filled still exists.
  it('includes a layer that holds no books', () => {
    const books = folder('books', [folder('Empty', []), folder('Outer', [folder('Inner', [])])]);

    expect(collectLayers(books)).toEqual(['/', 'Empty', 'Outer', 'Outer/Inner']);
  });

  it('does not descend into a book package', () => {
    const books = folder('books', [bookPackage('a.bookpkg')]);

    // `sources` lives inside the package and is not a layer.
    expect(collectLayers(books)).toEqual(['/']);
  });

  it('returns just the top level for an empty shelf', () => {
    expect(collectLayers(folder('books', []))).toEqual(['/']);
  });

  // The dataset in shelf/testdata/conformance covers the directory names a real
  // shelf carries; what is pinned here is the case folding, which a fixture on a
  // case-insensitive filesystem could not express.
  it('skips system directories however they are spelled', () => {
    const books = folder('books', [
      folder('$RECYCLE.BIN', [folder('Deleted', [])]),
      folder('$Recycle.Bin', []),
      folder('@eaDir', []),
      folder('.stfolder', []),
      folder('Poetry', [folder('@EAdir', [])])
    ]);

    expect(collectLayers(books)).toEqual(['/', 'Poetry']);
  });
});

describe('parseBookJson', () => {
  it('keeps the persisted id verbatim', () => {
    const meta = parseBookJson({ id: 'a82m', title: 'Title', schema_version: 1 });
    expect(meta.id).toBe('a82m');
  });

  it('rejects a book.json with no id rather than inventing one', () => {
    expect(() => parseBookJson({ title: 'Title' })).toThrow(PCloudError);
    expect(() => parseBookJson({ id: '  ', title: 'Title' })).toThrow(PCloudError);
  });

  it('rejects a non-object payload', () => {
    expect(() => parseBookJson('nope')).toThrow(PCloudError);
    expect(() => parseBookJson(null)).toThrow(PCloudError);
  });

  it('drops non-string entries from string arrays', () => {
    const meta = parseBookJson({ id: 'x', title: 'T', authors: ['A', 3, null], tags: 'no' });
    expect(meta.authors).toEqual(['A']);
    expect(meta.tags).toEqual([]);
  });
});

describe('isSchemaNewerThanSupported', () => {
  it('flags a newer schema without hiding the book', () => {
    const newer = parseBookJson({ id: 'x', title: 'T', schema_version: BOOK_META_SCHEMA_VERSION + 1 });
    expect(isSchemaNewerThanSupported(newer)).toBe(true);
    expect(toBook(newer, []).id).toBe('x');
  });

  it('treats a missing schema_version as older, not newer', () => {
    expect(isSchemaNewerThanSupported(parseBookJson({ id: 'x', title: 'T' }))).toBe(false);
  });
});

describe('toBook', () => {
  it('maps on-disk fields onto the UI type', () => {
    const book = toBook(
      parseBookJson({
        id: 'a82m',
        title: 'Book A',
        authors: ['Author'],
        tags: ['t'],
        language: 'zh',
        comments: 'a note',
        cover: 'cover.jpg',
        star: 3,
        current_source: '20240101-120000'
      }),
      ['Fiction']
    );

    expect(book).toMatchObject({
      id: 'a82m',
      title: 'Book A',
      authors: ['Author'],
      tags: ['t'],
      language: 'zh',
      comment: 'a note',
      cover: 'cover.jpg',
      layers: ['Fiction'],
      star: 3,
      format: 'txt',
      current_source: '20240101-120000'
    });
  });

  it('never sets cover_url, because pCloud download links expire', () => {
    const book = toBook(parseBookJson({ id: 'x', title: 'T', cover: 'cover.png' }), []);
    expect(book.cover_url).toBeUndefined();
  });
});

describe('findCoverFile / findCurrentSource', () => {
  it('resolves the cover named by book.json', () => {
    const [pkg] = collectBookPackages(folder('books', [bookPackage('a.bookpkg')]));
    const meta = parseBookJson({ id: 'x', title: 'T', cover: 'cover.jpg' });

    expect(findCoverFile(pkg, meta)?.name).toBe('cover.jpg');
  });

  it('returns undefined when book.json names no cover or a missing one', () => {
    const [pkg] = collectBookPackages(folder('books', [bookPackage('a.bookpkg')]));

    expect(findCoverFile(pkg, parseBookJson({ id: 'x', title: 'T' }))).toBeUndefined();
    expect(findCoverFile(pkg, parseBookJson({ id: 'x', title: 'T', cover: 'gone.png' }))).toBeUndefined();
  });

  it('prefers current_source and falls back to the newest source', () => {
    const [pkg] = collectBookPackages(
      folder('books', [bookPackage('a.bookpkg', ['20230101-120000', '20240301-090000'])])
    );

    expect(findCurrentSource(pkg, parseBookJson({ id: 'x', title: 'T', current_source: '20230101-120000' }))?.id).toBe(
      '20230101-120000'
    );
    expect(findCurrentSource(pkg, parseBookJson({ id: 'x', title: 'T', current_source: 'missing' }))?.id).toBe(
      '20240301-090000'
    );
    expect(findCurrentSource(pkg, parseBookJson({ id: 'x', title: 'T' }))?.id).toBe('20240301-090000');
  });
});

describe('toSourceMeta', () => {
  it('maps meta.json and normalizes the split type', () => {
    const meta = toSourceMeta(
      {
        id: '20240101-120000',
        created_at: '2024-01-01T12:00:00Z',
        comment: 'c',
        md5_hash: 'abc',
        schema_version: 1,
        format: 'md',
        line_count: 10,
        char_count: 200
      },
      'fallback'
    );

    expect(meta).toEqual({
      schema_version: 1,
      id: '20240101-120000',
      created_at: '2024-01-01T12:00:00Z',
      comment: 'c',
      md5_hash: 'abc',
      format: 'md',
      line_count: 10,
      char_count: 200
    });
  });

  it('falls back to the folder id when meta.json omits one', () => {
    expect(toSourceMeta({}, '20240101-120000').id).toBe('20240101-120000');
  });

});
