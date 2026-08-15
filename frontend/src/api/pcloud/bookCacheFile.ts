import type { BookJson, PCloudFileRef } from './bookpkg';
import { BOOKS_FOLDER } from './bookpkg';
import type { BookPackageRef } from './bookpkg';
import type { PCloudItem } from './types';

/**
 * Reads the book cache a PlainShelf server or desktop app exports into a
 * shelf's `app/` folder (see shelf/shelf_cache_export.go).
 *
 * Without it, listing a pCloud shelf costs two requests per book — one to get a
 * download link for its book.json, one to fetch it — which is slow on a phone
 * and can hit pCloud's rate limit. The exported file carries every book.json in
 * one download, so the cost stops growing with the size of the shelf.
 *
 * It does not remove the recursive listing: pCloud addresses files by id, and
 * ids are what the covers, sources and assets are fetched with later. The
 * listing is a single request and supplies them; the cache replaces the
 * per-book downloads, which is the expensive half.
 */

/** The exported format this reader understands (BookCacheSchemaVersion in Go). */
export const BOOK_CACHE_SCHEMA_VERSION = 1;

export const APP_FOLDER = 'app';
export const BOOK_CACHE_FILE_PREFIX = 'book-cache-';
export const BOOK_CACHE_FILE_SUFFIX = '.json';

export interface BookCacheEntry {
  /** Package directory relative to the shelf root, e.g. `books/Fiction/dune.bookpkg`. */
  path: string;
  meta: BookJson;
}

export interface BookCacheFile {
  schema_version: number;
  writer_id: string;
  /**
   * When the writer last walked the shelf, in Unix seconds. A book.json modified
   * after this was changed after the walk and is not covered by `books`.
   */
  timestamp: number;
  generator?: string;
  layers: string[];
  books: Record<string, BookCacheEntry>;
}

/**
 * Every exported cache in the shelf's `app/` folder, taken from the recursive
 * listing that was already fetched — so finding them costs no request.
 *
 * There can be several: each machine that opens the shelf writes its own,
 * named after its installation, and none of them can see the others' clocks.
 */
export function findBookCacheFiles(shelfRoot: PCloudItem): PCloudFileRef[] {
  const appFolder = shelfRoot.contents?.find((item) => item.isfolder && item.name === APP_FOLDER);
  if (!appFolder) {
    return [];
  }

  const files: PCloudFileRef[] = [];
  for (const item of appFolder.contents ?? []) {
    if (item.isfolder || item.fileid === undefined) {
      continue;
    }
    if (!item.name.startsWith(BOOK_CACHE_FILE_PREFIX) || !item.name.endsWith(BOOK_CACHE_FILE_SUFFIX)) {
      continue;
    }
    files.push({
      fileid: item.fileid,
      name: item.name,
      size: item.size ?? 0,
      modified: item.modified ?? ''
    });
  }
  return files;
}

/**
 * Picks the cache to trust when several machines write into the same shelf.
 *
 * Ordered by the listing's modification time rather than by the `timestamp`
 * inside each file, because choosing on content would mean downloading all of
 * them first. The name breaks ties so the choice is stable between runs rather
 * than dependent on listing order.
 */
export function pickNewestBookCache(files: PCloudFileRef[]): PCloudFileRef | null {
  let newest: PCloudFileRef | null = null;
  let newestAt = Number.NEGATIVE_INFINITY;

  for (const file of files) {
    const at = Date.parse(file.modified);
    const comparable = Number.isNaN(at) ? Number.NEGATIVE_INFINITY : at;
    if (!newest || comparable > newestAt || (comparable === newestAt && file.name > newest.name)) {
      newest = file;
      newestAt = comparable;
    }
  }

  return newest;
}

/**
 * Rejects anything that is not a cache this reader wrote.
 *
 * Strict on purpose, and for the same reason the device snapshot is
 * (shelfSnapshotStore.ts): the book list is built from this file, so a
 * half-valid payload would surface as a silently truncated library instead of
 * the cache miss it really is. Every rejection just costs one full scan.
 */
export function parseBookCacheFile(value: unknown): BookCacheFile | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return null;
  }

  const cache = value as Partial<BookCacheFile>;
  if (cache.schema_version !== BOOK_CACHE_SCHEMA_VERSION) {
    return null;
  }
  if (typeof cache.writer_id !== 'string' || !cache.writer_id) {
    return null;
  }
  if (typeof cache.timestamp !== 'number' || !Number.isFinite(cache.timestamp) || cache.timestamp <= 0) {
    return null;
  }
  if (!Array.isArray(cache.layers) || !cache.layers.every((layer) => typeof layer === 'string')) {
    return null;
  }
  if (typeof cache.books !== 'object' || cache.books === null || Array.isArray(cache.books)) {
    return null;
  }
  if (!Object.values(cache.books).every((entry) => isBookCacheEntry(entry))) {
    return null;
  }

  return cache as BookCacheFile;
}

function isBookCacheEntry(value: unknown): value is BookCacheEntry {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const entry = value as Partial<BookCacheEntry>;
  if (typeof entry.path !== 'string' || !entry.path) {
    return false;
  }
  return typeof entry.meta === 'object' && entry.meta !== null && typeof entry.meta.id === 'string';
}

/**
 * The package's path relative to the shelf root, in the form the exporter
 * writes — this is what matches a listed package to a cache entry.
 *
 * A book's folder name comes from its title when it was created and its ID does
 * not, so the path is the only key both sides can agree on without reading
 * book.json, which is precisely what the cache is here to avoid.
 */
export function bookPackagePath(pkg: BookPackageRef): string {
  return [BOOKS_FOLDER, ...pkg.layers, pkg.folderName].join('/');
}

/** Indexes a cache by package path for lookup during a listing walk. */
export function indexBookCacheByPath(cache: BookCacheFile): Map<string, BookJson> {
  const byPath = new Map<string, BookJson>();
  for (const entry of Object.values(cache.books)) {
    byPath.set(entry.path, entry.meta);
  }
  return byPath;
}

/**
 * Whether a listed book.json is old enough for the cache to speak for it.
 *
 * A file modified after the walk was changed after the cache was built, so it
 * has to be read. Anything unparseable is treated as newer: re-reading a book
 * needlessly costs two requests, whereas trusting a stale entry shows the user
 * the wrong metadata.
 */
export function isCoveredByBookCache(meta: PCloudFileRef | undefined, cacheTimestamp: number): boolean {
  if (!meta) {
    return false;
  }
  const modifiedAt = Date.parse(meta.modified);
  if (Number.isNaN(modifiedAt)) {
    return false;
  }
  return modifiedAt <= cacheTimestamp * 1000;
}
