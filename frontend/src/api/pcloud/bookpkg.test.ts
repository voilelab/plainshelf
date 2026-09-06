import { describe, expect, it } from 'vitest';

import {
  BOOK_META_SCHEMA_VERSION,
  collectBookPackages,
  collectFolders,
  createIgnoreRules,
  createNSFWFolderLookup,
  createNSFWRules,
  DEFAULT_IGNORE_RULES,
  findBooksFolder,
  findShelfConfigFile,
  findCoverFile,
  findCurrentSource,
  isBookNSFW,
  isSchemaNewerThanSupported,
  parseBookJson,
  parseShelfConfig,
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
  it('records folders from the directory path and stops at .bookpkg', () => {
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
    expect(byName.get('root-book.bookpkg')?.folders).toEqual([]);
    expect(byName.get('a.bookpkg')?.folders).toEqual(['Fiction']);
    expect(byName.get('b.bookpkg')?.folders).toEqual(['Fiction', 'Classics']);
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

describe('collectFolders', () => {
  it('lists nested folders plus the top level', () => {
    const books = folder('books', [
      bookPackage('root.bookpkg'),
      folder('Fiction', [bookPackage('a.bookpkg'), folder('Classics', [bookPackage('b.bookpkg')])]),
      folder('Non-Fiction', [bookPackage('c.bookpkg')])
    ]);

    expect(collectFolders(books)).toEqual(['/', 'Fiction', 'Fiction/Classics', 'Non-Fiction']);
  });

  // Derived from directories, not from the books inside them: the Go side walks
  // real directories, so a folder created but not yet filled still exists.
  it('includes a folder that holds no books', () => {
    const books = folder('books', [folder('Empty', []), folder('Outer', [folder('Inner', [])])]);

    expect(collectFolders(books)).toEqual(['/', 'Empty', 'Outer', 'Outer/Inner']);
  });

  it('does not descend into a book package', () => {
    const books = folder('books', [bookPackage('a.bookpkg')]);

    // `sources` lives inside the package and is not a folder.
    expect(collectFolders(books)).toEqual(['/']);
  });

  it('returns just the top level for an empty shelf', () => {
    expect(collectFolders(folder('books', []))).toEqual(['/']);
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

    expect(collectFolders(books)).toEqual(['/', 'Poetry']);
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

  // A book written before the member existed says nothing, which is not a mark.
  it.each([
    ['true', true, true],
    ['false', false, false],
    ['absent', undefined, false],
    ['a string', 'yes', false]
  ])('reads nsfw written as %s', (_label, written, want) => {
    expect(parseBookJson({ id: 'x', title: 'T', nsfw: written }).nsfw).toBe(want);
  });
});

describe('isSchemaNewerThanSupported', () => {
  it('flags a newer schema without hiding the book', () => {
    const newer = parseBookJson({ id: 'x', title: 'T', schema_version: BOOK_META_SCHEMA_VERSION + 1 });
    expect(isSchemaNewerThanSupported(newer)).toBe(true);
    expect(toBook(newer, []).id).toBe('x');
  });

  it('carries the answer onto the book, so the UI can say the read was partial', () => {
    const newer = parseBookJson({ id: 'x', title: 'T', schema_version: BOOK_META_SCHEMA_VERSION + 1 });
    expect(toBook(newer, []).schema_newer_than_supported).toBe(true);
  });

  it('leaves the flag off a book this reader understands', () => {
    for (const version of [undefined, BOOK_META_SCHEMA_VERSION - 1, BOOK_META_SCHEMA_VERSION]) {
      const meta = parseBookJson({ id: 'x', title: 'T', schema_version: version });
      expect(toBook(meta, []).schema_newer_than_supported).toBeUndefined();
    }
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
      folders: ['Fiction'],
      star: 3,
      format: 'txt',
      current_source: '20240101-120000'
    });
  });

  it('never sets cover_url, because pCloud download links expire', () => {
    const book = toBook(parseBookJson({ id: 'x', title: 'T', cover: 'cover.png' }), []);
    expect(book.cover_url).toBeUndefined();
  });

  // The two halves are carried apart, as the server carries them: only the
  // book's own is editable, and isBookNsfw is the one place that adds them.
  it('carries the book\'s own mark and the folder rule reaching it', () => {
    const meta = parseBookJson({ id: 'x', title: 'T', nsfw: true });
    const book = toBook(meta, ['Adult'], { path: 'Adult', reason: 'the top shelf' });

    expect(book).toMatchObject({ nsfw: true, nsfw_folder: { path: 'Adult', reason: 'the top shelf' } });
  });

  it('reports an unmarked book as unmarked rather than as unknown', () => {
    const book = toBook(parseBookJson({ id: 'x', title: 'T' }), []);

    expect(book.nsfw).toBe(false);
    expect(book.nsfw_folder).toBeUndefined();
  });

  // `omitempty` on the server's side of the same field: a client with nothing
  // to quote names the path instead, so an empty note must not read as one.
  it('omits an empty reason rather than carrying it', () => {
    const book = toBook(parseBookJson({ id: 'x', title: 'T' }), ['Adult'], { path: 'Adult', reason: '' });

    expect(book.nsfw_folder).toEqual({ path: 'Adult' });
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

describe('shelf.json', () => {
  it('finds the settings file in a shelf root', () => {
    const root = folder('default-shelf', [file('shelf.json'), folder('books'), folder('app')]);

    expect(findShelfConfigFile(root)?.name).toBe('shelf.json');
  });

  it('reports no settings file when the shelf has none', () => {
    expect(findShelfConfigFile(folder('default-shelf', [folder('books')]))).toBeUndefined();
  });

  // A directory named shelf.json is not the settings file, and reading it as one
  // would fail on a shelf that is otherwise fine.
  it('ignores a directory that carries the name', () => {
    const root = folder('default-shelf', [folder('shelf.json'), folder('books')]);

    expect(findShelfConfigFile(root)).toBeUndefined();
  });

  it('reads the directories the shelf skips, with and without a reason', () => {
    const raw = {
      schema_version: 1,
      scan: { ignored_dirs: [{ name: '@Snapshot' }, { name: '@Backup', reason: 'the NAS backup job' }] }
    };

    expect(parseShelfConfig(raw)).toEqual({
      ignoredDirs: [
        { name: '@Snapshot', reason: '' },
        { name: '@Backup', reason: 'the NAS backup job' }
      ]
    });
  });

  // An entry is always an object. A bare name would be a second shape for the
  // same thing, and the Go side would have to sniff between two types to read
  // one list.
  it('drops a bare name', () => {
    expect(parseShelfConfig({ scan: { ignored_dirs: ['@Snapshot'] } })).toEqual({ ignoredDirs: [] });
  });

  // An empty list is a shelf saying "skip nothing", which the caller must be
  // able to tell from a shelf that said nothing at all and gets the defaults.
  it('reads an empty list as a list, not as silence', () => {
    expect(parseShelfConfig({ scan: { ignored_dirs: [] } })).toEqual({ ignoredDirs: [] });
  });

  // A file written by a newer build, or one whose shape is wrong, reads as a
  // shelf that said nothing rather than making the shelf unreadable — what the
  // Go side does with the same file.
  it.each([
    ['an empty object', {}],
    ['a scan section of the wrong type', { scan: 'everything' }],
    ['a list of the wrong type', { scan: { ignored_dirs: 'everything' } }],
    ['null', null],
    ['a string', 'not a settings file'],
    ['an array', []],
    ['a schema_version of the wrong type', { schema_version: '1' }],
    ['a fractional schema_version', { schema_version: 1.5 }],
    ['a content section of the wrong type', { content: 'adult' }],
    ['an nsfw_folders list of the wrong type', { content: { nsfw_folders: 'Adult' } }]
  ])('reads no rules from %s', (_label, raw) => {
    expect(parseShelfConfig(raw)).toEqual({});
  });

  // Go decodes the file into one struct, so a member of the wrong container type
  // fails the whole file and nothing in it applies. Reading the good half here
  // would mark a folder on a phone that the server leaves unmarked.
  it.each([
    ['a broken scan section', { scan: 'everything', content: { nsfw_folders: [{ path: 'Adult' }] } }],
    ['a broken content section', { scan: { ignored_dirs: [{ name: '@Snapshot' }] }, content: 7 }]
  ])('drops the whole file over %s, not just that section', (_label, raw) => {
    expect(parseShelfConfig(raw)).toEqual({});
  });

  // null is not a wrong type: the Go struct reads it as the zero value, which is
  // the same as the member being absent.
  it('reads a null section or list as silence', () => {
    expect(parseShelfConfig({ schema_version: null, scan: null, content: { nsfw_folders: null } })).toEqual({});
  });

  // Unknown members are accepted on both sides, so a file from a newer build
  // still applies the parts this build understands.
  it('keeps reading a file that carries a member this build does not know', () => {
    const raw = { scan: { ignored_dirs: [{ name: '@Snapshot' }], future: true }, tomorrow: {} };

    expect(parseShelfConfig(raw)).toEqual({ ignoredDirs: [{ name: '@Snapshot', reason: '' }] });
  });

  // One unusable entry must not cost the rest of the file.
  it('drops entries that could not name a directory', () => {
    const raw = {
      scan: {
        ignored_dirs: [
          { name: '' },
          { name: '.' },
          { name: '..' },
          { name: 'with/separator' },
          { name: 'with\\separator' },
          17,
          null,
          '@Snapshot',
          { reason: 'no name' },
          { name: '@Snapshot' }
        ]
      }
    };

    expect(parseShelfConfig(raw)).toEqual({ ignoredDirs: [{ name: '@Snapshot', reason: '' }] });
  });

  it('reads the folders the shelf marks as adult content, with and without a reason', () => {
    const raw = {
      schema_version: 1,
      content: {
        nsfw_folders: [{ path: 'Fiction/Adult' }, { path: '/Doujin/', reason: 'the doujin shelf' }]
      }
    };

    expect(parseShelfConfig(raw)).toEqual({
      nsfwFolders: [
        { path: 'Fiction/Adult', reason: '' },
        { path: '/Doujin/', reason: 'the doujin shelf' }
      ]
    });
  });

  // The two sections are read independently: a shelf may carry either one.
  it('reads both sections of a file that carries both', () => {
    const raw = {
      scan: { ignored_dirs: [{ name: '@Snapshot' }] },
      content: { nsfw_folders: [{ path: 'Adult' }] }
    };

    expect(parseShelfConfig(raw)).toEqual({
      ignoredDirs: [{ name: '@Snapshot', reason: '' }],
      nsfwFolders: [{ path: 'Adult', reason: '' }]
    });
  });

  // As for ignored_dirs: one entry, one shape, and one unusable entry must not
  // cost the rest of the file.
  it('drops nsfw_folders entries that could not name a folder', () => {
    const raw = {
      content: {
        nsfw_folders: [
          { path: '' },
          { path: '/' },
          { path: 'Fiction/./Adult' },
          { path: 'Fiction/../Adult' },
          { path: 'with\\separator' },
          17,
          null,
          'Fiction/Adult',
          { reason: 'no path' },
          { path: 'Fiction/Adult' }
        ]
      }
    };

    expect(parseShelfConfig(raw)).toEqual({ nsfwFolders: [{ path: 'Fiction/Adult', reason: '' }] });
  });

  // The Go side unmarshals an entry into a {name|path, reason} struct, so a
  // reason of the wrong type fails the entry rather than defaulting to empty.
  // Defaulting here would skip a directory, or mark a folder, that the server
  // does not.
  it('drops an entry whose reason is not a string', () => {
    const raw = {
      scan: { ignored_dirs: [{ name: '@Snapshot', reason: 17 }, { name: 'thumbs' }] },
      content: { nsfw_folders: [{ path: 'Adult', reason: ['no'] }, { path: 'Doujin' }] }
    };

    expect(parseShelfConfig(raw)).toEqual({
      ignoredDirs: [{ name: 'thumbs', reason: '' }],
      nsfwFolders: [{ path: 'Doujin', reason: '' }]
    });
  });

  // A null reason is the member being absent, which both sides read as empty.
  it('keeps an entry whose reason is null', () => {
    const raw = { content: { nsfw_folders: [{ path: 'Adult', reason: null }] } };

    expect(parseShelfConfig(raw)).toEqual({ nsfwFolders: [{ path: 'Adult', reason: '' }] });
  });
});

describe('createNSFWRules / isBookNSFW', () => {
  const rules = (...paths: string[]) => createNSFWRules(paths.map((path) => ({ path, reason: '' })));
  const book = (nsfw: boolean) => parseBookJson({ id: 'x', title: 'T', nsfw });

  // A rule marks its own folder and everything below it, and nothing beside it.
  // "Fiction/成人漫畫" is the case worth pinning: a plain string prefix test
  // would match it against "Fiction/成人" and mark a folder nobody named.
  it.each([
    ['the marked folder itself', ['Fiction', '成人'], true],
    ['a folder below it', ['Fiction', '成人', 'Deep'], true],
    ['a name that starts with it', ['Fiction', '成人漫畫'], false],
    ['a folder beside it', ['Fiction', 'Classics'], false],
    ['the folder above it', ['Fiction'], false],
    ['the root', [], false]
  ])('marks %s', (_label, folders, want) => {
    expect(rules('Fiction/成人')(folders)).toBe(want);
  });

  // Leading, trailing and doubled separators name the same folder, while a path
  // with no segment at all would mark the whole shelf and is refused.
  it.each([
    ['/Fiction/成人', true],
    ['Fiction/成人/', true],
    ['Fiction//成人', true],
    ['', false],
    ['/', false]
  ])('reads the rule written as %s', (path, want) => {
    expect(rules(path)(['Fiction', '成人'])).toBe(want);
  });

  it('marks nothing for a shelf that listed no folder', () => {
    expect(createNSFWRules([])(['Fiction', '成人'])).toBe(false);
  });

  it('keeps the usable rules of a list that also holds an unusable one', () => {
    const rule = rules('Fiction/../Adult', 'Fiction/成人');
    expect(rule(['Fiction', '成人'])).toBe(true);
  });

  it.each([
    ['its folder', ['Fiction', '成人'], false, true],
    ['its own mark, outside any rule', [], true, true],
    ['its own mark, inside one', ['Fiction', '成人'], true, true],
    ['neither', ['Fiction', 'Classics'], false, false]
  ])('marks a book by %s', (_label, folders, own, want) => {
    expect(isBookNSFW(rules('Fiction/成人'), folders, book(own))).toBe(want);
  });

  // The asymmetry Shelf.IsBookNSFW depends on: a book may add itself, but it may
  // not take itself out of a marked folder. The failure that rules out is a book
  // that should have been marked and quietly was not; taking one book out of a
  // marked folder is done by moving it.
  it('does not let a book cancel the mark its folder carries', () => {
    const refused = parseBookJson({ id: 'x', title: 'T', nsfw: false });

    expect(refused.nsfw).toBe(false);
    expect(isBookNSFW(rules('Fiction/成人'), ['Fiction', '成人'], refused)).toBe(true);
  });
});

// The rule itself, not only whether one exists: a reader that has to say where
// a mark came from needs the entry, and the shelf's own note is a better answer
// than the path alone.
describe('createNSFWFolderLookup', () => {
  it('returns the rule that marks the path, with the note written for it', () => {
    const lookup = createNSFWFolderLookup([{ path: 'Fiction/Adult', reason: 'the top shelf' }]);

    expect(lookup(['Fiction', 'Adult', 'Deep'])).toEqual({ path: 'Fiction/Adult', reason: 'the top shelf' });
    expect(lookup(['Fiction'])).toBeUndefined();
  });

  // Mirrors NSFWRules.Match: the shallowest rule wins, because it is the one
  // that would still mark the folder if every deeper entry were removed.
  it('names the shallowest of two rules reaching one folder', () => {
    const lookup = createNSFWFolderLookup([
      { path: 'Fiction/Adult/2024', reason: 'this year' },
      { path: 'Fiction/Adult', reason: 'the top shelf' }
    ]);

    expect(lookup(['Fiction', 'Adult', '2024'])?.reason).toBe('the top shelf');
  });

  // Two entries for one path collapse the way assigning into Go's map does.
  it('keeps the last of two entries written for the same folder', () => {
    const lookup = createNSFWFolderLookup([
      { path: 'Adult', reason: 'first' },
      { path: '/Adult/', reason: 'second' }
    ]);

    expect(lookup(['Adult'])?.reason).toBe('second');
  });

  it('marks nothing for a shelf that listed no folder', () => {
    expect(createNSFWFolderLookup([])(['Adult'])).toBeUndefined();
  });
});

// Case-insensitive matching has to mean what Unicode means by it, not what
// lowercasing happens to do, and this reader has to agree with
// shelfutil.foldSegment on every case. Both halves are a folder rule silently
// missed — a book that should have been marked quietly is not.
describe('NSFW folder case folding', () => {
  const marks = (rule: string, folder: string) =>
    createNSFWRules([{ path: rule, reason: '' }])([folder]);

  it.each([
    // "Σ" lowercases to "σ" and "ς" lowercases to itself, so lowercasing alone
    // gives one letter two spellings that never meet.
    ['capital sigma rule, final sigma folder', 'ΣΕΙΡΑΣ', 'σειρας', true],
    ['final sigma rule, capital sigma folder', 'σειρας', 'ΣΕΙΡΑΣ', true],
    // Unicode's simple case folding leaves "İ" alone, so folding alone loses a
    // match that lowercasing gets right.
    ['turkish dotted capital', 'İstanbul', 'istanbul', true],
    ['turkish lowercase', 'istanbul', 'İstanbul', true],
    // The dotless i is excluded from folding with "i", so the two stay apart —
    // as they do in Go, which is what makes this a match rather than a bug.
    ['dotless i is its own letter', 'ıstanbul', 'istanbul', false],
    // "ß" uppercases to "SS", which is a spelling rather than a case pair.
    ['sharp s keeps its own spelling', 'Straße', 'Strasse', false],
    ['sharp s matches its capital', 'STRAẞE', 'straße', true],
    ['ascii', 'Fiction', 'FICTION', true],
    ['a different word still does not match', 'Fiction', 'Fictional', false]
  ])('%s', (_label, rule, folder, want) => {
    expect(marks(rule, folder)).toBe(want);
  });
});

describe('createIgnoreRules', () => {
  const dirs = (...names: string[]) => names.map((name) => ({ name, reason: '' }));

  it('skips the listed directories however they are spelled', () => {
    const ignore = createIgnoreRules(dirs('@Snapshot', 'thumbs'));
    const books = folder('books', [
      folder('@Snapshot', [folder('Nested', [])]),
      folder('@snapshot', []),
      folder('Thumbs', []),
      folder('Poetry', [folder('@SNAPSHOT', [])])
    ]);

    expect(collectFolders(books, ignore)).toEqual(['/', 'Poetry']);
  });

  it('keeps a book inside a skipped directory out of the listing', () => {
    const ignore = createIgnoreRules(dirs('@Snapshot'));
    const books = folder('books', [
      folder('@Snapshot', [bookPackage('hidden.bookpkg')]),
      bookPackage('kept.bookpkg')
    ]);

    expect(collectBookPackages(books, ignore).map((pkg) => pkg.folderName)).toEqual(['kept.bookpkg']);
  });

  // The list replaces the defaults rather than adding to them, so a shelf that
  // names its own directories no longer skips the ones it left out. The Go side
  // reads the same file the same way; the shared dataset pins it.
  it('does not skip a default name the list leaves out', () => {
    const ignore = createIgnoreRules(dirs('@Snapshot'));
    const books = folder('books', [folder('@eaDir', []), folder('@Snapshot', []), folder('Poetry', [])]);

    // localeCompare, which this reader sorts folders with, puts "@eaDir" before
    // the root.
    expect(collectFolders(books, ignore)).toEqual(['@eaDir', '/', 'Poetry']);
  });

  // Hidden directories are a rule, not a name on the list: an empty list still
  // skips them.
  it('skips hidden directories whatever the list says', () => {
    const ignore = createIgnoreRules([]);
    const books = folder('books', [folder('.git', []), folder('@eaDir', []), folder('Poetry', [])]);

    expect(collectFolders(books, ignore)).toEqual(['@eaDir', '/', 'Poetry']);
  });

  it('skips the defaults for a shelf that said nothing', () => {
    const books = folder('books', [folder('@eaDir', []), folder('lost+found', []), folder('Poetry', [])]);

    expect(collectFolders(books, DEFAULT_IGNORE_RULES)).toEqual(['/', 'Poetry']);
  });
});
