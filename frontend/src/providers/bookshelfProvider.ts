import type { DesktopShelfDetails } from '@/api/desktop';
import type { ListBooksOptions } from '@/api/books';
import type {
  BookmarkPayload,
  Book,
  BookCreateRequest,
  BookContent,
  BookUpdateRequest,
  DownloadState,
  PaginatedBooks,
  ReadingProgress,
  TrashedBook
} from '@/types/book';
import type { CreateSourceOptions, SourceMeta } from '@/types/source';
import type { BookBatchRequest, TaskChain } from '@/types/task';

export type { DownloadState } from '@/types/book';
export type { DesktopShelfDetails } from '@/api/desktop';
export type { ListBooksOptions } from '@/api/books';

export interface DesktopImportBookResult {
  path?: string;
  id?: string;
  error?: string;
}

export interface DownloadedBookEntry {
  book: Book;
  sizeBytes: number;
  downloadedAt: string;
}

export interface StorageEstimateResult {
  supported: boolean;
  usage?: number;
  quota?: number;
}

/** What a manual shelf update found, when the backend can report it. */
export interface ShelfRefreshResult {
  bookCount: number;
  layerCount: number;
}

/**
 * Everything a reading client needs. Every provider implements all of it.
 *
 * Reading progress and history are here rather than on the write surface
 * because they are per-device state and never sent to the shelf backend.
 */
export interface BookshelfReader {
  /** `options` is opt-in extra work for the backend; omit it unless the caller displays the field. */
  listBooks(page?: number, pageSize?: number, options?: ListBooksOptions): Promise<PaginatedBooks>;
  getBook(bookId: string): Promise<Book>;

  getBookContent(bookId: string): Promise<BookContent>;
  downloadBookContent(bookId: string): Promise<Blob>;
  /** @deprecated Legacy sources only. New Markdown sources derive chapters from H2. */

  getReadProgress(bookId: string): Promise<ReadingProgress>;
  saveReadProgress(bookId: string, progress: BookmarkPayload): Promise<void>;
  addReadHistory(bookId: string): Promise<void>;
  listReadHistoryBooks(): Promise<Book[]>;
  clearReadHistory(): Promise<void>;

  getBookCover(bookId: string): Promise<Blob>;
  getBookCoverUrl(bookId: string, cacheKey?: number): string;

  /**
   * Whether getBookCoverUrl() produces something an `<img src>` cannot load on
   * its own, so the cover has to be fetched through getBookCover() and handed
   * to the element as a `blob:` URL instead.
   *
   * True for a backend whose cover URL is not a plain fetchable address — the
   * pCloud provider returns an internal `pcloud:` reference — and for the
   * mobile shell, where a direct `<img src="http://...">` is issued by the
   * WebView itself, bypassing CapacitorHttp and tripping mixed-content
   * blocking against the `https://localhost` origin.
   */
  coversMustBeFetchedAsBlob?(): boolean;

  /**
   * Whether there is a PlainShelf server behind this backend to report its
   * `read_only` mode.
   *
   * Absent means yes — a server, desktop or mock client all have one, and that
   * is the ordinary case. pCloud answers false: it is storage, not a server, so
   * asking would be a request nothing can answer. Writes stay refused either
   * way, by the provider's own missing write surface.
   */
  supportsServerMode?(): boolean;

  /**
   * Whether asking for character counts alongside a book listing is affordable
   * on this backend.
   *
   * Absent means yes — a PlainShelf server computes them from its own shelf
   * cache. pCloud answers false: it has no server to aggregate, so it would
   * have to fetch every book's source meta.json over the network to answer one
   * listing. That is a property of the backend, not of the device asking, so a
   * phone pointed at a self-hosted server can offer the filter.
   */
  supportsCharCountListing?(): boolean;

  /** A read-only backend may answer these with an empty result rather than refuse. */
  getDuplicateBookGroups(): Promise<string[][]>;
  listTrashedBooks(): Promise<TrashedBook[]>;

  /** Layer paths in the shape `api/layers.ts` returns: '/' for the top level. */
  listLayers(): Promise<string[]>;

  listSources(bookId: string): Promise<SourceMeta[]>;
  getSource(bookId: string, sourceId: string): Promise<SourceMeta>;
  getSourceContent(bookId: string, sourceId: string): Promise<string>;

  /**
   * Reads one illustration from a source's `assets/` directory.
   *
   * Optional because a backend can serve a book's text without being able to
   * reach its images: the pCloud provider does not enumerate `assets/` yet, and
   * an offline mobile cache only holds what it downloaded. The reader falls
   * back to the image's alt text when this is absent or rejects, so a missing
   * illustration never costs the reader the chapter.
   */
  getSourceAsset?(bookId: string, sourceId: string, name: string): Promise<Blob>;

  /**
   * Manual shelf update, for a listing that does not necessarily reflect the
   * shelf right now.
   *
   * Two backends have that problem for different reasons. The pCloud provider
   * scans once and then reads a stored copy, because walking the shelf costs a
   * request per book. A server keeps its own cache warm, but only every
   * `scan_interval`, and no filesystem change notification reaches it from an
   * SMB or cloud mount — so a book added from outside PlainShelf waits out the
   * interval unless the user asks for the walk.
   *
   * `supportsShelfRefresh` is what the UI asks, because a wrapper always has
   * the methods even when the backend it wraps does not.
   *
   * `refreshShelf` returns what the update found when the backend can say;
   * `getShelfFetchedAt` is for a backend that instead dates its stored listing.
   * No backend does both, and neither is required.
   */
  supportsShelfRefresh?(): boolean;
  refreshShelf?(): Promise<ShelfRefreshResult | void>;
  getShelfFetchedAt?(): Promise<number | null>;

  /**
   * The shelf this client already knows it is pointed at, for use when listing
   * shelves fails.
   *
   * Present only on a backend whose shelf choice is device-local and outlives a
   * failed request — the mobile shell persists it during connection setup — so
   * offline-cached books stay reachable when the network does not. A server or
   * desktop client has no such fallback: its shelf list *is* the source of
   * truth, and a failure there means there is nothing to fall back to.
   *
   * Returns '' when nothing has been chosen yet.
   */
  getPersistedShelfID?(): string;

  downloadBook?(bookId: string): Promise<void>;
  removeDownload?(bookId: string): Promise<void>;
  getDownloadState?(bookId: string): Promise<DownloadState>;
  listDownloadedBookEntries?(): Promise<DownloadedBookEntry[]>;
  getStorageEstimate?(): Promise<StorageEstimateResult>;

  /** Opens the host file picker. Changes nothing; the import itself is a write. */
  openLocalBookFiles?(): Promise<string[] | null>;
  openDesktopLayerFolder?(layerPath: string): Promise<void>;
  openDesktopBookFolder?(bookId: string): Promise<void>;

  openDesktopShelfDirectory?(): Promise<string | null>;
  addDesktopShelf?(name: string, libRoot: string, scanInterval: string): Promise<void>;
  removeDesktopShelf?(shelfID: string): Promise<void>;
  getDesktopShelfDetails?(shelfID: string): Promise<DesktopShelfDetails>;
  modifyDesktopShelf?(shelfID: string, name: string, scanInterval: string): Promise<void>;
  saveBookContentToFile?(bookId: string, suggestedName: string): Promise<void>;
}

/**
 * Shelf mutations. Absent as a block on a provider that cannot write, so the
 * Android client and the pCloud backend are read-only by construction rather
 * than by refusing at runtime.
 *
 * Reach these through `bookshelfWriter()`, never off `BookshelfProvider`
 * directly.
 */
export interface BookshelfWriter {
  /**
   * Present only on a provider that implements every member below, which is
   * what `isWritableProvider()` reads.
   *
   * A capability flag rather than a probe for one of the methods, for the same
   * reason `supportsShelfRefresh` exists: a wrapper always has the methods even
   * when the backend it wraps does not. `implements BookshelfWriter` is what
   * makes the claim honest, because TypeScript then demands the whole surface.
   */
  readonly writable: true;

  updateBook(bookId: string, payload: BookUpdateRequest): Promise<Book>;
  updateBookLayer(bookId: string, layer: string): Promise<void>;
  deleteBook(bookId: string): Promise<void>;
  /** @deprecated Legacy sources only. */

  importBook(payload: BookCreateRequest): Promise<Book>;
  uploadBookCover(bookId: string, file: File): Promise<void>;
  uploadBookCoverBlob(bookId: string, blob: Blob): Promise<void>;
  deleteBookCover(bookId: string): Promise<void>;

  restoreTrashedBook(bookId: string): Promise<void>;
  deleteTrashedBook(bookId: string): Promise<void>;
  emptyTrash(): Promise<string>;

  startBookBatch(request: BookBatchRequest): Promise<string>;
  /** Recomputes content statistics for every book with an unknown char_count. */
  refreshContentStats(): Promise<string>;
  /** A GET, but a chain id can only come from a write that schedules a chain. */
  getTaskChain(taskChainId: string): Promise<TaskChain>;

  /**
   * Optional because only the desktop shell can reach host paths, but a write
   * either way: it creates books in the active shelf exactly as importBook
   * does, from a picker result rather than an upload.
   */
  importBooksFromLocalPaths?(localPaths: string[], layerPath: string): Promise<DesktopImportBookResult[] | null>;

  createSource(bookId: string, options?: CreateSourceOptions): Promise<SourceMeta>;
  deleteSource(bookId: string, sourceId: string): Promise<void>;
  setCurrentSource(bookId: string, sourceId: string): Promise<void>;
  updateSourceContent(bookId: string, sourceId: string, content: string): Promise<void>;
  refreshSourceMeta(bookId: string, sourceId: string): Promise<SourceMeta>;
}

/** A provider that has been proven to implement the write surface. */
export type WritableBookshelfProvider = BookshelfReader & BookshelfWriter;

/**
 * What callers hold. Declare `implements BookshelfReader` (and
 * `, BookshelfWriter` when the backend can write) on the class itself — never
 * this alias, which is deliberately loose enough to describe both kinds.
 */
export type BookshelfProvider = BookshelfReader & Partial<BookshelfWriter>;
