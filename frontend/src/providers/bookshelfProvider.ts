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
  SplitConfig,
  TrashedBook
} from '@/types/book';
import type { SourceMeta } from '@/types/source';
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

/**
 * Everything a reading client needs. Every provider implements all of it.
 *
 * Reading progress and history are here rather than on the write surface
 * because they are not shelf state: on mobile they are written to device
 * storage and never sent, and on a server they happen to be a request. A
 * read-only backend still has to serve them.
 */
export interface BookshelfReader {
  /** `options` is opt-in extra work for the backend; omit it unless the caller displays the field. */
  listBooks(page?: number, pageSize?: number, options?: ListBooksOptions): Promise<PaginatedBooks>;
  getBook(bookId: string): Promise<Book>;

  getBookContent(bookId: string): Promise<BookContent>;
  downloadBookContent(bookId: string): Promise<Blob>;
  getBookSplitConfig(bookId: string): Promise<SplitConfig>;

  getReadProgress(bookId: string): Promise<ReadingProgress>;
  saveReadProgress(bookId: string, progress: BookmarkPayload): Promise<void>;
  addReadHistory(bookId: string): Promise<void>;
  listReadHistoryBooks(): Promise<Book[]>;
  clearReadHistory(): Promise<void>;

  getBookCover(bookId: string): Promise<Blob>;
  getBookCoverUrl(bookId: string, cacheKey?: number): string;

  /** A read-only backend may answer these with an empty result rather than refuse. */
  getDuplicateBookGroups(): Promise<string[][]>;
  listTrashedBooks(): Promise<TrashedBook[]>;

  /** Layer paths in the shape `api/layers.ts` returns: '/' for the top level. */
  listLayers(): Promise<string[]>;

  listSources(bookId: string): Promise<SourceMeta[]>;
  getSource(bookId: string, sourceId: string): Promise<SourceMeta>;
  getSourceContent(bookId: string, sourceId: string): Promise<string>;

  /**
   * Manual shelf update, for backends whose listing is too expensive to refresh
   * on its own. A server keeps its own shelf cache warm (`scan_interval`), so
   * only the pCloud provider implements these; `supportsShelfRefresh` is what
   * the UI asks, because a wrapper always has the methods even when the backend
   * it wraps does not.
   */
  supportsShelfRefresh?(): boolean;
  refreshShelf?(): Promise<void>;
  getShelfFetchedAt?(): Promise<number | null>;

  downloadBook?(bookId: string): Promise<void>;
  removeDownload?(bookId: string): Promise<void>;
  getDownloadState?(bookId: string): Promise<DownloadState>;
  listDownloadedBookEntries?(): Promise<DownloadedBookEntry[]>;
  getStorageEstimate?(): Promise<StorageEstimateResult>;

  openLocalBookFiles?(): Promise<string[] | null>;
  importBooksFromLocalPaths?(localPaths: string[], layerPath: string): Promise<DesktopImportBookResult[] | null>;
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
  updateBookSplitConfig(bookId: string, config: SplitConfig): Promise<SplitConfig>;

  importBook(payload: BookCreateRequest): Promise<Book>;
  uploadBookCover(bookId: string, file: File): Promise<void>;
  uploadBookCoverBlob(bookId: string, blob: Blob): Promise<void>;
  deleteBookCover(bookId: string): Promise<void>;

  restoreTrashedBook(bookId: string): Promise<void>;
  deleteTrashedBook(bookId: string): Promise<void>;
  emptyTrash(): Promise<string>;

  startBookBatch(request: BookBatchRequest): Promise<string>;
  /** A GET, but a chain id can only come from startBookBatch or emptyTrash. */
  getTaskChain(taskChainId: string): Promise<TaskChain>;

  createSource(bookId: string): Promise<SourceMeta>;
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
