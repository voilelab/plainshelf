import {
  collectBookPackages,
  collectFolders,
  createIgnoreRules,
  createNSFWFolderLookup,
  DEFAULT_IGNORED_DIRS,
  MAX_SHELF_CONFIG_BYTES,
  findBooksFolder,
  findCoverFile,
  findCurrentSource,
  findShelfConfigFile,
  parseBookJson,
  parseShelfConfig,
  toBook,
  toSourceMeta,
} from '@/api/pcloud/bookpkg';
import type {
  BookJson,
  BookPackageRef,
  BookSourceRef,
  NSFWFolder,
  NSFWFolderLookup,
  PCloudFileRef,
  ShelfConfig
} from '@/api/pcloud/bookpkg';
import {
  bookPackagePath,
  findBookCacheFiles,
  indexBookCacheByPath,
  isCoveredByBookCache,
  parseBookCacheFile,
  pickNewestBookCache
} from '@/api/pcloud/bookCacheFile';
import type { BookCacheFile } from '@/api/pcloud/bookCacheFile';
import { zipSync } from 'fflate';
import { PCloudClient } from '@/api/pcloud/client';
import type { PCloudItem } from '@/api/pcloud/types';
import { PCloudDataError, PCloudError, isRetryablePCloudError } from '@/api/pcloud/errors';
import { reportIncident } from '@/composables/useErrorIncident';
import { getShowNsfwOnDevice } from '@/composables/useDeviceNsfwPreference';
import { ShelfVisibility } from './shelfVisibility';
import { ApiError } from '@/api/client';
import {
  addReadHistory as addLocalReadHistory,
  clearReadHistory as clearLocalReadHistory
} from '@/storage/readHistory';
import { collectReadHistoryBooks } from './readHistoryBooks';
import { SHELF_SNAPSHOT_VERSION } from './shelfSnapshotStore';
import type { PersistedShelfSnapshot, ShelfSnapshotStore } from './shelfSnapshotStore';
import type {
  BookmarkPayload,
  Book,
  BookContent,
  PaginatedBooks,
  ReadingProgress,
  TrashedBook,
  TrashedBookListing
} from '@/types/book';
import type { SourceMeta } from '@/types/source';
import type { FingerprintStatus, SimilarBookPair } from '@/api/books';
import type { BookshelfReader, ListBooksOptions } from './bookshelfProvider';

const READ_ONLY_MESSAGE = 'A pCloud shelf is read-only.';

/**
 * A valid, empty zip. Returned when no requested asset is in the listing, so the
 * caller's unzip finds nothing to store instead of the request failing — and
 * `getziplink` is never asked to zip nothing. Built once; the bytes never change.
 */
const EMPTY_ZIP = zipSync({});

/**
 * Parallel metadata reads. Each book.json costs two requests (getfilelink plus
 * the download), so some parallelism is essential on a large shelf, but pCloud
 * rate-limits and a phone's connection is finite.
 */
const METADATA_CONCURRENCY = 8;

const PAGE_SIZE_DEFAULT = 8;

/**
 * Marks a book as having a cover without naming a fetchable address:
 * `useCoverSrc` treats `cover_url` as a presence flag on mobile and resolves the
 * bytes through `getBookCover()`. A real URL cannot be used, because pCloud
 * download links come from a per-file request and expire.
 */
export function pcloudCoverUrl(bookId: string): string {
  return `pcloud:cover/${bookId}`;
}

interface LoadedBook {
  pkg: BookPackageRef;
  meta: BookJson;
  book: Book;
}

interface ShelfSnapshot {
  fetchedAt: number;
  books: LoadedBook[];
  byID: Map<string, LoadedBook>;
  /** Every folder directory, including ones holding no books. */
  folders: string[];
  /**
   * The shelf's adult-content folder rules, as read from shelf.json and as
   * persisted with the snapshot. Kept on the snapshot rather than on the
   * provider because they belong to the listing they were read with: a restore
   * carries the rules the walk that produced it saw.
   */
  nsfwFolders: NSFWFolder[];
  /** {@link nsfwFolders} compiled once, since a listing asks per book. */
  isNsfwFolder: NSFWFolderLookup;
}

/**
 * A snapshot load in progress. `restore` may still be reading the device;
 * `walk` is committed to a recursive pCloud listing.
 */
interface PendingLoad {
  kind: 'restore' | 'walk';
  promise: Promise<ShelfSnapshot>;
}

interface CachedJson {
  size: number;
  modified: string;
  value: unknown;
}

interface PCloudBookshelfProviderOptions {
  client: PCloudClient;
  /** Path of the shelf directory on pCloud, e.g. `/PlainShelf/default-shelf`. */
  shelfRoot: string;
  /**
   * Where the shelf listing survives between app runs. Omitted, the provider
   * still works but re-walks the shelf once per process, which is what this
   * whole mechanism exists to avoid — so production wiring always supplies one
   * (see providers/index.ts).
   */
  snapshotStore?: ShelfSnapshotStore;
  /** Overridable so tests do not depend on wall-clock time. */
  nowImpl?: () => number;
}

/**
 * Restates a pCloud failure in the error type the provider stack speaks.
 *
 * MobileBookshelfProvider reads reachability off `ApiError` and assumes anything
 * else is unreachable, so a raw PCloudError would send every failure — expired
 * token, deleted book — down the offline-cache path and quietly show stale
 * content. Carrying the pCloud folder's transient/permanent distinction across
 * the boundary here keeps that wrapper backend-agnostic.
 */
function toProviderError(err: unknown): unknown {
  if (!(err instanceof PCloudError)) {
    return err;
  }

  if (isRetryablePCloudError(err)) {
    // "Unreachable" is where the wrapper falls back to downloaded books, so the
    // user may never be told anything failed; raising a reference for it would
    // put a number on screen next to content that loaded fine.
    return new ApiError(err.message, { isTimeout: true, cause: err });
  }

  reportIncident(err.incident);
  return new ApiError(err.message, { status: err.status, cause: err, incident: err.incident });
}

/**
 * Runs `fn` over `items` with a bounded number in flight, preserving order.
 */
async function mapWithConcurrency<T, R>(
  items: T[],
  limit: number,
  fn: (item: T) => Promise<R>
): Promise<R[]> {
  const results = new Array<R>(items.length);
  let cursor = 0;

  const workers = Array.from({ length: Math.min(limit, items.length) }, async () => {
    for (;;) {
      const index = cursor;
      cursor += 1;
      if (index >= items.length) {
        return;
      }
      results[index] = await fn(items[index]);
    }
  });

  await Promise.all(workers);
  return results;
}

/**
 * Reads a PlainShelf shelf directly from pCloud, without a PlainShelf server.
 *
 * This exists for the Android client. A server can already keep a shelf on
 * pCloud by pointing `lib_root` at an rclone mount, but a phone cannot mount
 * anything, so it is the one place the cloud case needs its own code path.
 *
 * The shelf is read-only, which costs almost nothing here: the mobile shell is
 * already a read-only client (see api/client.ts) and reading progress, history
 * and downloads all live on the device, so nothing a reader does needs to be
 * written back.
 *
 * **The listing is updated only when asked.** Walking the shelf costs one
 * recursive listing plus a download per book.json, which on a phone is far too
 * expensive to spend on every app launch for a library that rarely changes. So
 * the listing is persisted to the device and reused indefinitely; only
 * `refreshShelf()` — behind a button the user presses — goes back to pCloud.
 * The one exception is the very first run after a connection is configured,
 * where there is no snapshot yet and an empty library would be worse than a
 * single scan.
 *
 * Intended to be wrapped by MobileBookshelfProvider, which folders the offline
 * cache and device-local progress on top.
 */
export class PCloudBookshelfProvider implements BookshelfReader {
  private readonly client: PCloudClient;
  private readonly shelfRoot: string;
  private readonly snapshotStore: ShelfSnapshotStore | null;
  private readonly now: () => number;

  /**
   * Decoded book.json and meta.json payloads keyed by pCloud file id, valid
   * only while the file's size and modified time are unchanged.
   *
   * This is the same freshness rule the Go shelf applies through `FileStat`
   * (shelf/filestate.go), and it is what keeps a re-scan cheap: the recursive
   * listing already carries size and modified time for every file, so an
   * unchanged book costs no request at all on the second walk.
   *
   * Warmed from the persisted snapshot on load, so that rule survives an app
   * restart too: a manual refresh then costs one listing plus a download only
   * for the books whose book.json actually changed.
   */
  private readonly jsonCache = new Map<number, CachedJson>();

  private snapshot: ShelfSnapshot | null = null;

  /**
   * The load in progress, if any. A caller that just wants *a* listing joins it
   * whatever it is; `refreshShelf` needs to know whether it actually reaches
   * pCloud, which a restore only does once it finds nothing stored — hence the
   * mutable `kind`.
   */
  private pending: PendingLoad | null = null;

  constructor(options: PCloudBookshelfProviderOptions) {
    this.client = options.client;
    this.shelfRoot = options.shelfRoot.trim();
    this.snapshotStore = options.snapshotStore ?? null;
    this.now = options.nowImpl ?? (() => Date.now());

    if (!this.shelfRoot) {
      throw new PCloudError('A pCloud shelf path is required.');
    }
  }

  // --- shelf loading -------------------------------------------------------

  /**
   * The listing every read works from. Never goes to pCloud when a snapshot is
   * available — in memory or on the device — regardless of its age; only an
   * install with no snapshot at all pays for a walk.
   */
  private async ensureSnapshot(): Promise<ShelfSnapshot> {
    const current = this.snapshot;
    if (current) {
      return current;
    }

    // Collapse concurrent callers onto one load: the library grid, the folder
    // tree and the dashboard all list on mount, and three simultaneous
    // recursive listings would triple the cost of opening the app.
    if (this.pending) {
      return await this.pending.promise;
    }

    return await this.start('restore', (entry) => this.restoreOrLoadSnapshot(entry));
  }

  private start(
    kind: PendingLoad['kind'],
    load: (entry: PendingLoad) => Promise<ShelfSnapshot>
  ): Promise<ShelfSnapshot> {
    const entry = { kind } as PendingLoad;
    entry.promise = load(entry).finally(() => {
      // Identity-checked: a load started while this one was running has already
      // taken the slot, and clearing it would let a third caller start a
      // duplicate walk.
      if (this.pending === entry) {
        this.pending = null;
      }
    });
    this.pending = entry;
    return entry.promise;
  }

  private async restoreOrLoadSnapshot(entry: PendingLoad): Promise<ShelfSnapshot> {
    const restored = await this.restoreSnapshot();
    if (restored) {
      // A walk started while this was reading the device has the newer listing;
      // the restored one is what it just replaced.
      this.snapshot ??= restored;
      return this.snapshot;
    }

    // Nothing stored, so this restore becomes the first scan. Re-labelled
    // before the walk starts so a refresh pressed meanwhile joins it rather
    // than paying for a second recursive listing.
    entry.kind = 'walk';
    return await this.loadSnapshot();
  }

  /** Rebuilds the in-memory snapshot from the device, without any request. */
  private async restoreSnapshot(): Promise<ShelfSnapshot | null> {
    const persisted = await this.snapshotStore?.load();
    if (!persisted || persisted.shelf_root !== this.shelfRoot) {
      return null;
    }

    const nsfwFolders = persisted.nsfw_folders ?? [];
    const isNsfwFolder = createNSFWFolderLookup(nsfwFolders);

    const books = persisted.books.map(({ pkg, meta }) => {
      if (pkg.meta) {
        this.jsonCache.set(pkg.meta.fileid, {
          size: pkg.meta.size,
          modified: pkg.meta.modified,
          value: meta
        });
      }
      return { pkg, meta, book: this.buildBook(meta, pkg, isNsfwFolder) } satisfies LoadedBook;
    });

    return {
      fetchedAt: persisted.fetched_at,
      books,
      byID: new Map(books.map((entry) => [entry.meta.id, entry])),
      folders: persisted.folders,
      nsfwFolders,
      isNsfwFolder
    };
  }

  private async persistSnapshot(snapshot: ShelfSnapshot): Promise<void> {
    await this.snapshotStore?.save({
      version: SHELF_SNAPSHOT_VERSION,
      shelf_root: this.shelfRoot,
      fetched_at: snapshot.fetchedAt,
      folders: snapshot.folders,
      books: snapshot.books.map(({ pkg, meta }) => ({ pkg, meta })),
      nsfw_folders: snapshot.nsfwFolders
    } satisfies PersistedShelfSnapshot);
  }

  /**
   * Loads the cache the shelf's server or desktop app exported, if there is one.
   *
   * Never throws: a shelf with no exporter, an older file or a corrupt one is a
   * cache miss, and the walk below still produces a correct listing — only more
   * slowly. Costs two requests, against two per book without it.
   */
  private async loadBookCache(root: PCloudItem): Promise<BookCacheFile | null> {
    const newest = pickNewestBookCache(findBookCacheFiles(root));
    if (!newest) {
      return null;
    }

    try {
      const cache = parseBookCacheFile(JSON.parse(await this.client.downloadText(newest.fileid)));
      if (!cache) {
        console.warn(`Ignoring ${newest.name}: not a book cache this version understands.`);
      }
      return cache;
    } catch (err) {
      console.warn(`Ignoring ${newest.name}: it could not be read.`, err);
      return null;
    }
  }

  /**
   * Reads the shelf's `shelf.json`, if it has one.
   *
   * Never throws: an unreadable or malformed settings file reads as a shelf that
   * said nothing, so the caller applies the defaults — matching the Go shelf,
   * because refusing to open a library over a typo in an optional file is the
   * worse failure.
   */
  private async loadShelfConfig(ref: PCloudFileRef | undefined): Promise<ShelfConfig> {
    if (!ref) {
      return {};
    }

    // The listing already carries the size, so a file too large to be settings
    // is skipped before it is downloaded — the Go side applies the same limit to
    // the same file, and a phone must not spend the data to reach that answer.
    if (ref.size > MAX_SHELF_CONFIG_BYTES) {
      console.warn(`Ignoring ${ref.name}: ${ref.size} bytes is larger than a shelf configuration is read at.`);
      return {};
    }

    try {
      return parseShelfConfig(await this.readJson(ref));
    } catch (err) {
      console.warn(`Ignoring ${ref.name}: it could not be read.`, err);
      return {};
    }
  }

  private async loadSnapshot(): Promise<ShelfSnapshot> {
    const root = await this.client.listFolderRecursive({ path: this.shelfRoot });
    const booksFolder = findBooksFolder(root);
    if (!booksFolder) {
      throw new PCloudError(`No books/ folder under ${this.shelfRoot}; this does not look like a PlainShelf shelf.`);
    }

    // The shelf's own settings decide which directories are skipped, so they are
    // read before the walk. Only a shelf that carries the file pays for it, and
    // readJson answers from the cache while its size and mtime are unchanged, so
    // a refresh does not download it again.
    const configRef = findShelfConfigFile(root);
    const config = await this.loadShelfConfig(configRef);
    const ignore = createIgnoreRules(config.ignoredDirs ?? DEFAULT_IGNORED_DIRS);
    // Undefined and empty mean the same thing for these — a shelf that marks no
    // folder — so there is no default list to fall back to.
    const nsfwFolders = config.nsfwFolders ?? [];
    const isNsfwFolder = createNSFWFolderLookup(nsfwFolders);

    const packages = collectBookPackages(booksFolder, ignore);
    // Folders stay derived from the listing rather than read from the cache. The
    // directories are in the response already, so they cost nothing here and
    // cannot be out of date, which the cache's copy can be.
    const folders = collectFolders(booksFolder, ignore);
    this.pruneJsonCache(packages, configRef);

    // Only worth two requests when something actually has to be read. A refresh
    // where no book.json changed is already free from jsonCache, and fetching
    // the exported cache to answer nothing would make refreshing more expensive
    // than it was before the cache existed.
    const needsMetadata = packages.some((pkg) => pkg.meta && !this.isJsonCached(pkg.meta));
    const bookCache = needsMetadata ? await this.loadBookCache(root) : null;
    const cachedMetaByPath = bookCache ? indexBookCacheByPath(bookCache) : null;

    const loaded = await mapWithConcurrency(packages, METADATA_CONCURRENCY, async (pkg) => {
      if (!pkg.meta) {
        console.warn(`Skipping ${pkg.folderName}: no ${'book.json'} in the package.`);
        return null;
      }

      // A book the exporter recorded and nobody has touched since needs no
      // request at all. One changed afterwards falls through to the download,
      // so a stale cache costs accuracy for nothing.
      if (bookCache && cachedMetaByPath && isCoveredByBookCache(pkg.meta, bookCache.timestamp)) {
        const cachedMeta = cachedMetaByPath.get(bookPackagePath(pkg));
        if (cachedMeta) {
          try {
            const meta = parseBookJson(cachedMeta);
            // Warm the freshness cache too, so a later refresh re-reads only
            // the books whose book.json actually changed.
            this.jsonCache.set(pkg.meta.fileid, {
              size: pkg.meta.size,
              modified: pkg.meta.modified,
              value: cachedMeta
            });
            return { pkg, meta, book: this.buildBook(meta, pkg, isNsfwFolder) } satisfies LoadedBook;
          } catch (err) {
            console.warn(`Ignoring the cached entry for ${pkg.folderName}; reading its book.json instead.`, err);
          }
        }
      }

      try {
        const meta = parseBookJson(await this.readJson(pkg.meta));
        return { pkg, meta, book: this.buildBook(meta, pkg, isNsfwFolder) } satisfies LoadedBook;
      } catch (err) {
        // Only a book that is genuinely broken is skipped. A transport failure
        // says nothing about the book, and swallowing it here would cache a
        // successful empty shelf the moment connectivity drops mid-scan —
        // leaving the wrapper no rejection to fall back to downloaded books on.
        if (!(err instanceof PCloudDataError)) {
          throw err;
        }
        console.warn(`Skipping ${pkg.folderName}: its book.json could not be read.`, err);
        return null;
      }
    });

    const books = loaded.filter((entry): entry is LoadedBook => entry !== null);
    // pCloud promises no listing order, and listBooks slices this array into
    // pages, so an unstable order would let a book shift across a page boundary
    // between scans and be seen twice or missed. Sorted by id to match what the
    // server returns (shelf/shelf_cache.go).
    books.sort((a, b) => (a.meta.id < b.meta.id ? -1 : a.meta.id > b.meta.id ? 1 : 0));

    const snapshot: ShelfSnapshot = {
      fetchedAt: this.now(),
      books,
      byID: new Map(books.map((entry) => [entry.meta.id, entry])),
      folders,
      nsfwFolders,
      isNsfwFolder
    };

    this.snapshot = snapshot;
    await this.persistSnapshot(snapshot);
    return snapshot;
  }

  /**
   * Walks pCloud again and replaces the stored listing. The only path that
   * refreshes the book list; everything else reads whatever this last wrote.
   *
   * A failure leaves the previous snapshot in place — a refresh that could not
   * reach pCloud must not cost the user the library they already had.
   */
  refreshShelf(): Promise<void> {
    return this.guarded(async () => {
      // Wait out whatever is already running before deciding. A load that
      // reached pCloud is the work this method asks for and needs no repeat; a
      // restore served from the device is not, so the walk still has to happen.
      for (let pending = this.pending; pending; pending = this.pending) {
        await pending.promise;
        if (pending.kind === 'walk') {
          return;
        }
      }

      await this.start('walk', () => this.loadSnapshot());
    });
  }

  /** When pCloud was last walked, or null before the first successful scan. */
  async getShelfFetchedAt(): Promise<number | null> {
    if (this.snapshot) {
      return this.snapshot.fetchedAt;
    }
    // Answering from the device rather than from ensureSnapshot() keeps the
    // "last updated" label from triggering the very first-run scan itself.
    const persisted = await this.snapshotStore?.load();
    return persisted && persisted.shelf_root === this.shelfRoot ? persisted.fetched_at : null;
  }

  supportsShelfRefresh(): boolean {
    return true;
  }

  /** Drops cache entries for files that no longer exist in the shelf. */
  private pruneJsonCache(packages: BookPackageRef[], configRef?: PCloudFileRef): void {
    const live = new Set<number>();
    if (configRef) {
      live.add(configRef.fileid);
    }
    for (const pkg of packages) {
      if (pkg.meta) {
        live.add(pkg.meta.fileid);
      }
      for (const source of pkg.sources) {
        if (source.meta) {
          live.add(source.meta.fileid);
        }
      }
    }

    for (const fileid of this.jsonCache.keys()) {
      if (!live.has(fileid)) {
        this.jsonCache.delete(fileid);
      }
    }
  }

  /** Whether readJson would answer this reference without a request. */
  private isJsonCached(ref: PCloudFileRef): boolean {
    const cached = this.jsonCache.get(ref.fileid);
    return cached !== undefined && cached.size === ref.size && cached.modified === ref.modified;
  }

  private async readJson(ref: PCloudFileRef): Promise<unknown> {
    const cached = this.jsonCache.get(ref.fileid);
    if (cached && cached.size === ref.size && cached.modified === ref.modified) {
      return cached.value;
    }

    const text = await this.client.downloadText(ref.fileid);
    let value: unknown;
    try {
      value = JSON.parse(text);
    } catch (cause) {
      // The bytes arrived and are unusable — a problem with this file, not with
      // the connection, so a caller may skip just this book.
      throw new PCloudDataError(`${ref.name} is not valid JSON.`, { cause });
    }

    this.jsonCache.set(ref.fileid, { size: ref.size, modified: ref.modified, value });
    return value;
  }

  private buildBook(meta: BookJson, pkg: BookPackageRef, isNsfwFolder: NSFWFolderLookup): Book {
    const book = toBook(meta, pkg.folders, isNsfwFolder(pkg.folders));
    return findCoverFile(pkg, meta) ? { ...book, cover_url: pcloudCoverUrl(meta.id) } : book;
  }

  /**
   * The filter this read is answered through, built from the device setting.
   *
   * Built per call rather than held on the provider: the setting is changed in
   * the settings page while the provider lives for the whole process, and a
   * filter captured once would go on serving what the user has just hidden.
   * Within one read it is fixed, so a listing and the folder tree derived from
   * it cannot disagree.
   */
  private visibility(snapshot: ShelfSnapshot): ShelfVisibility {
    return new ShelfVisibility({
      showNsfw: getShowNsfwOnDevice(),
      isNsfwFolder: (folders) => snapshot.isNsfwFolder(folders) !== undefined
    });
  }

  /**
   * The one lookup by id, so a book this device hides cannot be reached by any
   * route: content, cover, sources and progress all resolve through here, and
   * the existing not-found path is what a hidden book takes — being told it
   * exists but may not be read would disclose it just as well.
   */
  private async findBook(bookId: string): Promise<LoadedBook> {
    const snapshot = await this.ensureSnapshot();
    const entry = snapshot.byID.get(bookId);
    if (!entry || !this.visibility(snapshot).allows(entry.book)) {
      throw new PCloudError(`Book ${bookId} was not found in the pCloud shelf.`);
    }
    return entry;
  }

  private async findSource(bookId: string, sourceId: string): Promise<BookSourceRef> {
    const { pkg } = await this.findBook(bookId);
    const source = pkg.sources.find((candidate) => candidate.id === sourceId);
    if (!source) {
      throw new PCloudError(`Source ${sourceId} was not found for book ${bookId}.`);
    }
    return source;
  }

  private async currentSourceOf(entry: LoadedBook): Promise<BookSourceRef> {
    const source = findCurrentSource(entry.pkg, entry.meta);
    if (!source) {
      throw new PCloudError(`Book ${entry.meta.id} has no source to read.`);
    }
    return source;
  }

  private contentRefOf(source: BookSourceRef, bookId: string): PCloudFileRef {
    if (!source.content) {
      throw new PCloudError(`Source ${source.id} of book ${bookId} has no source.txt.`);
    }
    return source.content;
  }

  /**
   * Rejects rather than throwing synchronously: every method on the provider
   * interface returns a Promise, and a caller attaching `.catch()` to the
   * result would never see a synchronous throw. 403 mirrors how a read-only
   * server answers a mutation (server/app.go).
   */
  private readOnly<T>(): Promise<T> {
    return Promise.reject(new ApiError(READ_ONLY_MESSAGE, { status: 403 }));
  }

  /** Runs a read and restates any pCloud failure for the provider stack. */
  private async guarded<T>(run: () => Promise<T>): Promise<T> {
    try {
      return await run();
    } catch (err) {
      throw toProviderError(err);
    }
  }

  // --- books ---------------------------------------------------------------

  listBooks(page = 1, pageSize = PAGE_SIZE_DEFAULT, options?: ListBooksOptions): Promise<PaginatedBooks> {
    return this.guarded(async () => {
      const snapshot = await this.ensureSnapshot();
      // Filtered before the slice, not after: paging a filtered list is the only
      // way `total` matches what the pages hold, and filtering one page would
      // leave short pages with the hidden books' places still counted.
      const visibility = this.visibility(snapshot);
      const visible = snapshot.books.filter((entry) => visibility.allows(entry.book));
      const start = Math.max(0, (page - 1) * pageSize);
      const pageEntries = visible.slice(start, start + pageSize);

      // char_count lives in the current source's meta.json, so it costs a read
      // per book. It stays opt-in for that reason, and only the requested page
      // is charged for it.
      const items = options?.includeCharCount
        ? await mapWithConcurrency(pageEntries, METADATA_CONCURRENCY, (entry) => this.withCharCount(entry))
        : pageEntries.map((entry) => entry.book);

      return { items, total: visible.length, page, pageSize };
    });
  }

  private async withCharCount(entry: LoadedBook): Promise<Book> {
    try {
      const source = await this.currentSourceOf(entry);
      if (!source.meta) {
        return entry.book;
      }
      const meta = toSourceMeta(await this.readJson(source.meta), source.id);
      return meta.char_count === undefined ? entry.book : { ...entry.book, char_count: meta.char_count };
    } catch (err) {
      // Absent is the documented state for an unavailable char_count, and it
      // must not take the whole listing down with it.
      console.warn(`Could not read char_count for book ${entry.meta.id}.`, err);
      return entry.book;
    }
  }

  getBook(bookId: string): Promise<Book> {
    return this.guarded(async () => (await this.findBook(bookId)).book);
  }

  getBookContent(bookId: string): Promise<BookContent> {
    return this.guarded(async () => {
      const entry = await this.findBook(bookId);
      const source = await this.currentSourceOf(entry);
      return { content: await this.client.downloadText(this.contentRefOf(source, bookId).fileid) };
    });
  }

  downloadBookContent(bookId: string): Promise<Blob> {
    return this.guarded(async () => {
      const entry = await this.findBook(bookId);
      const source = await this.currentSourceOf(entry);
      return await this.client.downloadBlob(this.contentRefOf(source, bookId).fileid);
    });
  }

  getBookCover(bookId: string): Promise<Blob> {
    return this.guarded(async () => {
      const { pkg, meta } = await this.findBook(bookId);
      const cover = findCoverFile(pkg, meta);
      if (!cover) {
        throw new PCloudError(`Book ${bookId} has no cover.`);
      }
      return await this.client.downloadBlob(cover.fileid);
    });
  }

  getBookCoverUrl(bookId: string): string {
    return pcloudCoverUrl(bookId);
  }

  /**
   * pcloudCoverUrl() is an internal reference this provider resolves itself,
   * not an address the browser can fetch. Reaching this backend goes through
   * the mobile shell today, which would ask for a blob anyway; declaring it
   * here keeps the reason attached to the backend that has it.
   */
  coversMustBeFetchedAsBlob(): boolean {
    return true;
  }

  /** Cloud storage, not a PlainShelf server: there is no mode to fetch. */
  supportsServerMode(): boolean {
    return false;
  }

  /**
   * True because there is no server to ask: this provider reads the marks out of
   * the shelf itself and applies the device setting to them. A client that does
   * have a server must answer false, so the server's own `show_nsfw` stays the
   * only filter on that path rather than being doubled here.
   */
  filtersNsfwOnDevice(): boolean {
    return true;
  }

  /**
   * Answering would cost one meta.json download per book, on a metered
   * transport, for a filter the user may not use.
   */
  supportsCharCountListing(): boolean {
    return false;
  }

  // --- folders --------------------------------------------------------------

  listFolders(): Promise<string[]> {
    return this.guarded(async () => {
      const snapshot = await this.ensureSnapshot();
      return this.visibility(snapshot).filterFolders(
        snapshot.folders,
        snapshot.books.map((entry) => entry.book)
      );
    });
  }

  // --- sources -------------------------------------------------------------

  listSources(bookId: string): Promise<SourceMeta[]> {
    return this.guarded(async () => {
      const { pkg } = await this.findBook(bookId);
      return await mapWithConcurrency(pkg.sources, METADATA_CONCURRENCY, async (source) =>
        source.meta ? toSourceMeta(await this.readJson(source.meta), source.id) : toSourceMeta({}, source.id)
      );
    });
  }

  getSource(bookId: string, sourceId: string): Promise<SourceMeta> {
    return this.guarded(async () => {
      const source = await this.findSource(bookId, sourceId);
      return source.meta ? toSourceMeta(await this.readJson(source.meta), source.id) : toSourceMeta({}, source.id);
    });
  }

  getSourceContent(bookId: string, sourceId: string): Promise<string> {
    return this.guarded(async () => {
      const source = await this.findSource(bookId, sourceId);
      return await this.client.downloadText(this.contentRefOf(source, bookId).fileid);
    });
  }

  /**
   * Reads one illustration from a source's `assets/` directory.
   *
   * The file is already located by the recursive listing the shelf is read
   * through, so this costs one download and no extra lookup. A name the
   * listing does not carry is reported as missing rather than requested: the
   * reader shows the alt text, which is what a shelf with no such file should
   * produce.
   */
  getSourceAsset(bookId: string, sourceId: string, name: string): Promise<Blob> {
    return this.guarded(async () => {
      const source = await this.findSource(bookId, sourceId);
      const ref = source.assets[name];
      if (!ref) {
        throw new PCloudError(`Illustration ${name} was not found for source ${sourceId} of book ${bookId}.`);
      }
      return await this.client.downloadBlob(ref.fileid);
    });
  }

  /**
   * Fetches a source's illustrations as one pCloud-built zip, so a chunk costs a
   * single request instead of one `getfilelink`-plus-download per figure — the
   * server's assets.zip win, on the pCloud path.
   *
   * fileids come from the listing the shelf was read through
   * (`BookSourceRef.assets`), so no extra listing is made; a name the listing
   * lacks is dropped (reader shows alt text, as `getSourceAsset` does), and all
   * absent yields an empty archive. The caller bounds and unzips each chunk, so
   * no batching is needed here.
   */
  getSourceAssetsBundle(bookId: string, sourceId: string, names: string[]): Promise<Blob> {
    return this.guarded(async () => {
      const source = await this.findSource(bookId, sourceId);

      const fileids: number[] = [];
      for (const name of names) {
        const ref = source.assets[name];
        if (ref) {
          fileids.push(ref.fileid);
        }
      }

      if (fileids.length === 0) {
        return new Blob([EMPTY_ZIP], { type: 'application/zip' });
      }

      return await this.client.downloadZip(fileids);
    });
  }

  // --- reading progress and history ---------------------------------------

  /**
   * A pCloud shelf stores no progress, so there is nothing to report. Returning
   * a zero offset rather than throwing matters: MobileBookshelfProvider asks
   * the wrapped provider only after its own device-local store came up empty,
   * and an error there would surface as a failure to open the book.
   */
  async getReadProgress(_bookId: string): Promise<ReadingProgress> {
    return { char_offset: 0 };
  }

  /**
   * Never reached in the mobile stack, where progress is saved to the device
   * cache and never forwarded. Refusing rather than silently discarding keeps a
   * future caller from believing progress was stored.
   */
  saveReadProgress(_bookId: string, _progress: BookmarkPayload): Promise<void> {
    return this.readOnly();
  }

  // Reading history is device-local (see storage/readHistory); only the books
  // it points at come from the shelf.
  addReadHistory(bookId: string): Promise<void> {
    return addLocalReadHistory(bookId);
  }

  listReadHistoryBooks(): Promise<Book[]> {
    return collectReadHistoryBooks((page, pageSize) => this.listBooks(page, pageSize));
  }

  clearReadHistory(): Promise<void> {
    return clearLocalReadHistory();
  }

  // --- server-only views, answered empty ------------------------------------
  //
  // The shelf-mutating half of the provider interface is absent rather than
  // refused: BookshelfWriter is optional, and a pCloud shelf does not
  // implement it. These two are reads, so they still have to answer.

  /**
   * Duplicate detection is computed by the server, and the trash lives outside
   * `books/`. Both views are unreachable on mobile (see
   * features/mobile/utils/blockedRoutes.ts), so an empty result is honest
   * rather than a silent stand-in for missing data.
   */
  async getDuplicateBookGroups(): Promise<string[][]> {
    return [];
  }

  // Similarity fingerprints live in the shelf's server-side cache under `app/`,
  // which a pCloud shelf has no server to compute; both reads answer empty
  // rather than reach for data that is not there.
  async getSimilarBookPairs(): Promise<SimilarBookPair[]> {
    return [];
  }

  async getFingerprintStatus(): Promise<FingerprintStatus> {
    return { total: 0, fingerprinted: 0, missing: 0, algo: { normalize: '', shingle: '', hash: '', k: 0 } };
  }

  async listTrashedBooks(): Promise<TrashedBookListing> {
    return { books: [], complete: true };
  }
}
