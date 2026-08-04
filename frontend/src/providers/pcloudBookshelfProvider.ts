import {
  collectBookPackages,
  findBooksFolder,
  findCoverFile,
  findCurrentSource,
  parseBookJson,
  toBook,
  toSourceMeta,
  toSplitConfig
} from '@/api/pcloud/bookpkg';
import type { BookJson, BookPackageRef, BookSourceRef, PCloudFileRef } from '@/api/pcloud/bookpkg';
import { PCloudClient } from '@/api/pcloud/client';
import { PCloudError } from '@/api/pcloud/errors';
import {
  addReadHistory as addLocalReadHistory,
  clearReadHistory as clearLocalReadHistory
} from '@/storage/readHistory';
import { collectReadHistoryBooks } from './readHistoryBooks';
import type {
  BookmarkPayload,
  Book,
  BookCreateRequest,
  BookContent,
  BookUpdateRequest,
  PaginatedBooks,
  ReadingProgress,
  SplitConfig,
  TrashedBook
} from '@/types/book';
import type { SourceMeta } from '@/types/source';
import type { BookBatchRequest, TaskChain } from '@/types/task';
import type { BookshelfProvider, ListBooksOptions } from './bookshelfProvider';

const READ_ONLY_MESSAGE = 'A pCloud shelf is read-only.';

/**
 * How long a shelf listing is reused before the tree is walked again. Mirrors
 * the Go shelf's `scan_interval` default (shelf/shelf.go) and serves the same
 * purpose: on high-latency storage, re-walking on every request costs far more
 * than showing a book a minute late.
 */
const DEFAULT_SCAN_INTERVAL_MS = 60_000;

/**
 * Parallel metadata reads. Each book.json costs two requests (getfilelink plus
 * the download), so some parallelism is essential on a large shelf, but pCloud
 * rate-limits and a phone's connection is finite.
 */
const METADATA_CONCURRENCY = 8;

const PAGE_SIZE_DEFAULT = 8;

/**
 * Marks a book as having a cover without naming a fetchable address.
 *
 * `useCoverSrc` treats `cover_url` as a presence flag on mobile — falsy means
 * "no cover, show the placeholder" — and then resolves the bytes through
 * `getBookCover()`. A real URL cannot be used here: pCloud download links come
 * from a per-file request and expire, so producing one per book during a
 * listing would cost an extra round trip each and be stale by the time it
 * rendered.
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
}

interface CachedJson {
  size: number;
  modified: string;
  value: unknown;
}

export interface PCloudBookshelfProviderOptions {
  client: PCloudClient;
  /** Path of the shelf directory on pCloud, e.g. `/PlainShelf/default-shelf`. */
  shelfRoot: string;
  scanIntervalMs?: number;
  /** Overridable so tests do not depend on wall-clock time. */
  nowImpl?: () => number;
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
 * Intended to be wrapped by MobileBookshelfProvider, which layers the offline
 * cache and device-local progress on top.
 */
export class PCloudBookshelfProvider implements BookshelfProvider {
  private readonly client: PCloudClient;
  private readonly shelfRoot: string;
  private readonly scanIntervalMs: number;
  private readonly now: () => number;

  /**
   * Decoded book.json and meta.json payloads keyed by pCloud file id, valid
   * only while the file's size and modified time are unchanged.
   *
   * This is the same freshness rule the Go shelf applies through `FileStat`
   * (shelf/filestate.go), and it is what keeps a re-scan cheap: the recursive
   * listing already carries size and modified time for every file, so an
   * unchanged book costs no request at all on the second walk.
   */
  private readonly jsonCache = new Map<number, CachedJson>();

  private snapshot: ShelfSnapshot | null = null;
  private inFlight: Promise<ShelfSnapshot> | null = null;

  constructor(options: PCloudBookshelfProviderOptions) {
    this.client = options.client;
    this.shelfRoot = options.shelfRoot.trim();
    this.scanIntervalMs = options.scanIntervalMs ?? DEFAULT_SCAN_INTERVAL_MS;
    this.now = options.nowImpl ?? (() => Date.now());

    if (!this.shelfRoot) {
      throw new PCloudError('A pCloud shelf path is required.');
    }
  }

  // --- shelf loading -------------------------------------------------------

  private async ensureSnapshot(): Promise<ShelfSnapshot> {
    const current = this.snapshot;
    if (current && this.now() - current.fetchedAt < this.scanIntervalMs) {
      return current;
    }

    // Collapse concurrent callers onto one walk: the library grid, the layer
    // tree and the dashboard all list on mount, and three simultaneous
    // recursive listings would triple the cost of opening the app.
    if (!this.inFlight) {
      this.inFlight = this.loadSnapshot().finally(() => {
        this.inFlight = null;
      });
    }

    return await this.inFlight;
  }

  private async loadSnapshot(): Promise<ShelfSnapshot> {
    const root = await this.client.listFolderRecursive({ path: this.shelfRoot });
    const booksFolder = findBooksFolder(root);
    if (!booksFolder) {
      throw new PCloudError(`No books/ folder under ${this.shelfRoot}; this does not look like a PlainShelf shelf.`);
    }

    const packages = collectBookPackages(booksFolder);
    this.pruneJsonCache(packages);

    const loaded = await mapWithConcurrency(packages, METADATA_CONCURRENCY, async (pkg) => {
      if (!pkg.meta) {
        console.warn(`Skipping ${pkg.folderName}: no ${'book.json'} in the package.`);
        return null;
      }

      try {
        const meta = parseBookJson(await this.readJson(pkg.meta));
        return { pkg, meta, book: this.buildBook(meta, pkg) } satisfies LoadedBook;
      } catch (err) {
        // One unreadable book must not cost the user the rest of the shelf.
        console.warn(`Skipping ${pkg.folderName}: its book.json could not be read.`, err);
        return null;
      }
    });

    const books = loaded.filter((entry): entry is LoadedBook => entry !== null);
    const snapshot: ShelfSnapshot = {
      fetchedAt: this.now(),
      books,
      byID: new Map(books.map((entry) => [entry.meta.id, entry]))
    };

    this.snapshot = snapshot;
    return snapshot;
  }

  /** Drops cache entries for files that no longer exist in the shelf. */
  private pruneJsonCache(packages: BookPackageRef[]): void {
    const live = new Set<number>();
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
      throw new PCloudError(`${ref.name} is not valid JSON.`, { cause });
    }

    this.jsonCache.set(ref.fileid, { size: ref.size, modified: ref.modified, value });
    return value;
  }

  private buildBook(meta: BookJson, pkg: BookPackageRef): Book {
    const book = toBook(meta, pkg.layers);
    return findCoverFile(pkg, meta) ? { ...book, cover_url: pcloudCoverUrl(meta.id) } : book;
  }

  private async findBook(bookId: string): Promise<LoadedBook> {
    const snapshot = await this.ensureSnapshot();
    const entry = snapshot.byID.get(bookId);
    if (!entry) {
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
   * result would never see a synchronous throw.
   */
  private readOnly<T>(): Promise<T> {
    return Promise.reject(new PCloudError(READ_ONLY_MESSAGE));
  }

  // --- books ---------------------------------------------------------------

  async listBooks(page = 1, pageSize = PAGE_SIZE_DEFAULT, options?: ListBooksOptions): Promise<PaginatedBooks> {
    const snapshot = await this.ensureSnapshot();
    const start = Math.max(0, (page - 1) * pageSize);
    const pageEntries = snapshot.books.slice(start, start + pageSize);

    // char_count lives in the current source's meta.json, so it costs a read
    // per book. It stays opt-in for that reason, and only the requested page is
    // charged for it.
    const items = options?.includeCharCount
      ? await mapWithConcurrency(pageEntries, METADATA_CONCURRENCY, (entry) => this.withCharCount(entry))
      : pageEntries.map((entry) => entry.book);

    return { items, total: snapshot.books.length, page, pageSize };
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

  async getBook(bookId: string): Promise<Book> {
    return (await this.findBook(bookId)).book;
  }

  async getBookContent(bookId: string): Promise<BookContent> {
    const entry = await this.findBook(bookId);
    const source = await this.currentSourceOf(entry);
    return { content: await this.client.downloadText(this.contentRefOf(source, bookId).fileid) };
  }

  async downloadBookContent(bookId: string): Promise<Blob> {
    const entry = await this.findBook(bookId);
    const source = await this.currentSourceOf(entry);
    return await this.client.downloadBlob(this.contentRefOf(source, bookId).fileid);
  }

  async getBookSplitConfig(bookId: string): Promise<SplitConfig> {
    const entry = await this.findBook(bookId);
    const source = await this.currentSourceOf(entry);
    if (!source.meta) {
      return { type: 'none' };
    }
    return toSplitConfig(await this.readJson(source.meta));
  }

  async getBookCover(bookId: string): Promise<Blob> {
    const { pkg, meta } = await this.findBook(bookId);
    const cover = findCoverFile(pkg, meta);
    if (!cover) {
      throw new PCloudError(`Book ${bookId} has no cover.`);
    }
    return await this.client.downloadBlob(cover.fileid);
  }

  getBookCoverUrl(bookId: string): string {
    return pcloudCoverUrl(bookId);
  }

  // --- sources -------------------------------------------------------------

  async listSources(bookId: string): Promise<SourceMeta[]> {
    const { pkg } = await this.findBook(bookId);
    return await mapWithConcurrency(pkg.sources, METADATA_CONCURRENCY, async (source) =>
      source.meta ? toSourceMeta(await this.readJson(source.meta), source.id) : toSourceMeta({}, source.id)
    );
  }

  async getSource(bookId: string, sourceId: string): Promise<SourceMeta> {
    const source = await this.findSource(bookId, sourceId);
    return source.meta ? toSourceMeta(await this.readJson(source.meta), source.id) : toSourceMeta({}, source.id);
  }

  async getSourceContent(bookId: string, sourceId: string): Promise<string> {
    const source = await this.findSource(bookId, sourceId);
    return await this.client.downloadText(this.contentRefOf(source, bookId).fileid);
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

  // --- unavailable on a read-only shelf ------------------------------------

  /**
   * Duplicate detection is computed by the server, and the trash lives outside
   * `books/`. Both views are unreachable on mobile (see
   * features/mobile/utils/blockedRoutes.ts), so an empty result is honest
   * rather than a silent stand-in for missing data.
   */
  async getDuplicateBookGroups(): Promise<string[][]> {
    return [];
  }

  async listTrashedBooks(): Promise<TrashedBook[]> {
    return [];
  }

  updateBook(_bookId: string, _payload: BookUpdateRequest): Promise<Book> {
    return this.readOnly();
  }

  updateBookLayer(_bookId: string, _layer: string): Promise<void> {
    return this.readOnly();
  }

  deleteBook(_bookId: string): Promise<void> {
    return this.readOnly();
  }

  updateBookSplitConfig(_bookId: string, _config: SplitConfig): Promise<SplitConfig> {
    return this.readOnly();
  }

  importBook(_payload: BookCreateRequest): Promise<Book> {
    return this.readOnly();
  }

  uploadBookCover(_bookId: string, _file: File): Promise<void> {
    return this.readOnly();
  }

  uploadBookCoverBlob(_bookId: string, _blob: Blob): Promise<void> {
    return this.readOnly();
  }

  deleteBookCover(_bookId: string): Promise<void> {
    return this.readOnly();
  }

  restoreTrashedBook(_bookId: string): Promise<void> {
    return this.readOnly();
  }

  deleteTrashedBook(_bookId: string): Promise<void> {
    return this.readOnly();
  }

  emptyTrash(): Promise<string> {
    return this.readOnly();
  }

  getTaskChain(_taskChainId: string): Promise<TaskChain> {
    return this.readOnly();
  }

  startBookBatch(_request: BookBatchRequest): Promise<string> {
    return this.readOnly();
  }

  createSource(_bookId: string): Promise<SourceMeta> {
    return this.readOnly();
  }

  deleteSource(_bookId: string, _sourceId: string): Promise<void> {
    return this.readOnly();
  }

  setCurrentSource(_bookId: string, _sourceId: string): Promise<void> {
    return this.readOnly();
  }

  updateSourceContent(_bookId: string, _sourceId: string, _content: string): Promise<void> {
    return this.readOnly();
  }

  refreshSourceMeta(_bookId: string, _sourceId: string): Promise<SourceMeta> {
    return this.readOnly();
  }
}
