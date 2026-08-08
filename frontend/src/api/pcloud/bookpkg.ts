import type { Book, BookFormat, SplitConfig } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import { normalizeSplitConfig } from '@/utils/splitConfig';
import { PCloudDataError } from './errors';
import type { PCloudItem } from './types';

// Layout constants mirroring the Go shelf (shelf/shelf.go, shelf/book.go,
// shelf/source.go) and documented in docs/concepts/data-model.md. They are
// duplicated here rather than derived because this client reads the on-disk
// format directly, without the server in between.
export const BOOKS_FOLDER = 'books';
export const BOOK_EXTENSION = '.bookpkg';
export const BOOK_META_FILE = 'book.json';
export const SOURCES_FOLDER = 'sources';
export const SOURCE_META_FILE = 'meta.json';
export const SOURCE_FILE = 'source.txt';

/**
 * The book.json schema version this reader understands, matching
 * `BookMetaSchemaVersion` in shelf/book.go.
 *
 * A newer file is still read: the Go side reads best-effort too and only
 * refuses to *write* it (see `Book.EnsureWritable`). Since this client never
 * writes, the version is surfaced for the UI to warn about rather than used to
 * hide the book.
 */
export const BOOK_META_SCHEMA_VERSION = 1;

/**
 * A file located in the recursive listing.
 *
 * `size` and `modified` come straight from the listing, so a caller can decide
 * whether a cached copy is still current without spending a request — the same
 * freshness rule the Go shelf applies via `FileStat` (shelf/filestate.go).
 */
export interface PCloudFileRef {
  fileid: number;
  name: string;
  size: number;
  modified: string;
}

export interface BookSourceRef {
  id: string;
  folderid: number;
  meta?: PCloudFileRef;
  content?: PCloudFileRef;
}

export interface BookPackageRef {
  /** Directory name including the `.bookpkg` suffix. Not the book's identity. */
  folderName: string;
  folderid: number;
  /** Directory names between `books/` and this package, outermost first. */
  layers: string[];
  /** Absent when the package has no book.json and cannot be read. */
  meta?: PCloudFileRef;
  /** Files directly inside the package, keyed by name (cover, pointer file, …). */
  files: Record<string, PCloudFileRef>;
  sources: BookSourceRef[];
}

/** The on-disk shape of book.json — see `BookMeta` in shelf/book.go. */
export interface BookJson {
  schema_version?: number;
  id: string;
  title: string;
  format?: string;
  tags?: string[];
  identifiers?: Record<string, string>;
  cover?: string;
  authors?: string[];
  language?: string;
  comments?: string;
  star?: number;
  created_at?: string;
  updated_at?: string;
  published_at?: string;
  current_source?: string;
}

function toFileRef(item: PCloudItem): PCloudFileRef | undefined {
  if (item.isfolder || item.fileid === undefined) {
    return undefined;
  }
  return {
    fileid: item.fileid,
    name: item.name,
    size: item.size ?? 0,
    modified: item.modified ?? ''
  };
}

function findFolder(parent: PCloudItem | undefined, name: string): PCloudItem | undefined {
  return parent?.contents?.find((item) => item.isfolder && item.name === name);
}

/**
 * Locates the `books/` directory inside a listed shelf root.
 *
 * Returns undefined rather than throwing so a caller can tell "this folder is
 * not a shelf" from "the request failed", which are different problems for the
 * user to fix.
 */
export function findBooksFolder(shelfRoot: PCloudItem): PCloudItem | undefined {
  return findFolder(shelfRoot, BOOKS_FOLDER);
}

/**
 * Walks a listed `books/` tree and returns every book package it contains.
 *
 * Directories are layers until one ends in `.bookpkg`; that one is a book and is
 * not descended into further, matching how the Go shelf scans the tree
 * (shelf/shelf_book.go).
 */
export function collectBookPackages(booksFolder: PCloudItem): BookPackageRef[] {
  const packages: BookPackageRef[] = [];

  const walk = (folder: PCloudItem, layers: string[]): void => {
    for (const item of folder.contents ?? []) {
      if (!item.isfolder || item.folderid === undefined) {
        continue;
      }

      if (item.name.endsWith(BOOK_EXTENSION)) {
        packages.push(readBookPackage(item, layers));
        continue;
      }

      walk(item, [...layers, item.name]);
    }
  };

  walk(booksFolder, []);
  return packages;
}

/**
 * Lists every layer under a listed `books/` tree, in the shape `getLayers()`
 * returns (`'/'` for the top level, `Fiction/Classics` for a nested one).
 *
 * Derived from the directories themselves, not from the books found in them, so
 * a layer holding no books is still listed — the Go side walks real directories
 * too (`iterateLayers` in shelf/shelf_layer.go) and `books/` itself counts as
 * the "no layer" group.
 */
export function collectLayers(booksFolder: PCloudItem): string[] {
  const paths = new Set<string>(['/']);

  const walk = (folder: PCloudItem, segments: string[]): void => {
    for (const item of folder.contents ?? []) {
      if (!item.isfolder || item.folderid === undefined || item.name.endsWith(BOOK_EXTENSION)) {
        continue;
      }
      const next = [...segments, item.name];
      paths.add(next.join('/'));
      walk(item, next);
    }
  };

  walk(booksFolder, []);
  return Array.from(paths).sort((a, b) => a.localeCompare(b));
}

function readBookPackage(pkg: PCloudItem, layers: string[]): BookPackageRef {
  const files: Record<string, PCloudFileRef> = {};
  let sources: BookSourceRef[] = [];

  for (const item of pkg.contents ?? []) {
    if (item.isfolder) {
      if (item.name === SOURCES_FOLDER) {
        sources = readSources(item);
      }
      continue;
    }

    const ref = toFileRef(item);
    if (ref) {
      files[ref.name] = ref;
    }
  }

  return {
    folderName: pkg.name,
    folderid: pkg.folderid as number,
    layers,
    meta: files[BOOK_META_FILE],
    files,
    sources
  };
}

function readSources(sourcesFolder: PCloudItem): BookSourceRef[] {
  const sources: BookSourceRef[] = [];

  for (const item of sourcesFolder.contents ?? []) {
    if (!item.isfolder || item.folderid === undefined) {
      continue;
    }

    const entry: BookSourceRef = { id: item.name, folderid: item.folderid };
    for (const child of item.contents ?? []) {
      const ref = toFileRef(child);
      if (!ref) {
        continue;
      }
      if (ref.name === SOURCE_META_FILE) {
        entry.meta = ref;
      } else if (ref.name === SOURCE_FILE) {
        entry.content = ref;
      }
    }

    sources.push(entry);
  }

  // Source ids are `YYYYMMDD-HHMMSS` timestamps, so lexical order is
  // chronological. pCloud does not promise a listing order.
  sources.sort((a, b) => a.id.localeCompare(b.id));
  return sources;
}

/** Resolves the cover file a book.json points at, if it is present. */
export function findCoverFile(pkg: BookPackageRef, meta: BookJson): PCloudFileRef | undefined {
  const name = meta.cover?.trim();
  return name ? pkg.files[name] : undefined;
}

/** Finds the source a book.json marks current, falling back to the newest one. */
export function findCurrentSource(pkg: BookPackageRef, meta: BookJson): BookSourceRef | undefined {
  const current = meta.current_source?.trim();
  if (current) {
    const match = pkg.sources.find((source) => source.id === current);
    if (match) {
      return match;
    }
  }
  return pkg.sources[pkg.sources.length - 1];
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
}

/**
 * Validates a decoded book.json.
 *
 * `id` is required and used verbatim: it is assigned once when the book is
 * created and never recomputed from the folder name or title
 * (docs/concepts/data-model.md). Deriving an id here — from the directory name
 * or from pCloud's fileid — would break reading progress the moment a book is
 * renamed or moved.
 */
export function parseBookJson(raw: unknown): BookJson {
  if (!raw || typeof raw !== 'object') {
    throw new PCloudDataError('book.json is not a JSON object.');
  }

  const data = raw as Record<string, unknown>;
  const id = typeof data.id === 'string' ? data.id.trim() : '';
  if (!id) {
    throw new PCloudDataError('book.json has no id.');
  }

  return {
    schema_version: typeof data.schema_version === 'number' ? data.schema_version : undefined,
    id,
    title: typeof data.title === 'string' ? data.title : '',
    format: typeof data.format === 'string' ? data.format : undefined,
    tags: asStringArray(data.tags),
    identifiers:
      data.identifiers && typeof data.identifiers === 'object'
        ? (data.identifiers as Record<string, string>)
        : undefined,
    cover: typeof data.cover === 'string' ? data.cover : undefined,
    authors: asStringArray(data.authors),
    language: typeof data.language === 'string' ? data.language : undefined,
    comments: typeof data.comments === 'string' ? data.comments : undefined,
    star: typeof data.star === 'number' ? data.star : undefined,
    created_at: typeof data.created_at === 'string' ? data.created_at : undefined,
    updated_at: typeof data.updated_at === 'string' ? data.updated_at : undefined,
    published_at: typeof data.published_at === 'string' ? data.published_at : undefined,
    current_source: typeof data.current_source === 'string' ? data.current_source : undefined
  };
}

/** Reports a book.json written by a newer build than this reader knows about. */
export function isSchemaNewerThanSupported(meta: BookJson): boolean {
  return (meta.schema_version ?? 0) > BOOK_META_SCHEMA_VERSION;
}

/**
 * Maps a book.json onto the UI's Book type.
 *
 * `cover_url` is left unset here: there is no stable URL to point an `<img>` at,
 * because pCloud download links expire, and this function does not know whether
 * the named cover file actually exists. The provider fills it in with a
 * presence marker once it has resolved the file, and the bytes are fetched as a
 * blob — which is what the mobile runtime requires anyway
 * (frontend/src/composables/useCoverSrc.ts).
 */
export function toBook(meta: BookJson, layers: string[]): Book {
  return {
    id: meta.id,
    title: meta.title,
    authors: meta.authors ?? [],
    language: meta.language,
    format: (meta.format as BookFormat) || 'txt',
    tags: meta.tags ?? [],
    // book.json spells this "comments"; the UI type uses the singular.
    comment: meta.comments,
    cover: meta.cover?.trim() ?? '',
    layers,
    created_at: meta.created_at,
    updated_at: meta.updated_at,
    published_at: meta.published_at,
    current_source: meta.current_source,
    star: meta.star ?? 0,
    identifiers: meta.identifiers
  };
}

/** Maps a source's meta.json onto the UI's SourceMeta type. */
export function toSourceMeta(raw: unknown, fallbackID: string): SourceMeta {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const splitConfig = normalizeSplitConfig(data.split_config);

  return {
    id: typeof data.id === 'string' && data.id.trim() ? data.id : fallbackID,
    created_at: typeof data.created_at === 'string' ? data.created_at : '',
    comment: typeof data.comment === 'string' ? data.comment : '',
    md5_hash: typeof data.md5_hash === 'string' ? data.md5_hash : '',
    line_count: typeof data.line_count === 'number' ? data.line_count : undefined,
    char_count: typeof data.char_count === 'number' ? data.char_count : undefined,
    split_config: { type: splitConfig.type }
  };
}

/** Extracts the reader's split configuration from a source's meta.json. */
export function toSplitConfig(raw: unknown): SplitConfig {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return normalizeSplitConfig(data.split_config);
}
