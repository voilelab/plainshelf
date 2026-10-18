import type { BookTransferMode, FingerprintStatus, ListBooksOptions, SimilarBookPair } from '@/api/books';
import type { FolderChangeOptions } from '@/api/folders';
import type {
  BookmarkPayload,
  Book,
  BookCreateRequest,
  BookContent,
  BookUpdateRequest,
  DownloadState,
  NsfwMarks,
  PaginatedBooks,
  ReadingProgress,
  TrashedBook,
  TrashedBookListing
} from '@/types/book';
import type { CreateSourceOptions, SourceMeta } from '@/types/source';
import type { BookBatchRequest, TaskChain } from '@/types/task';

export type { DownloadState } from '@/types/book';
export type { ListBooksOptions } from '@/api/books';

export interface DownloadedBookEntry {
  book: Book;
  sizeBytes: number;
  downloadedAt: string;
}

/** What a manual shelf update found, when the backend can report it. */
export interface ShelfRefreshResult {
  bookCount: number;
  folderCount: number;
}

/**
 * Everything a reading client needs; every provider implements all of it.
 * Reading progress and history are here rather than on the write surface
 * because they are per-device state, never sent to the shelf backend.
 */
export interface BookshelfReader {
  /** `options` is opt-in extra work for the backend; omit it unless the caller displays the field. */
  listBooks(page?: number, pageSize?: number, options?: ListBooksOptions): Promise<PaginatedBooks>;
  getBook(bookId: string): Promise<Book>;

  getBookContent(bookId: string): Promise<BookContent>;
  downloadBookContent(bookId: string): Promise<Blob>;

  getReadProgress(bookId: string): Promise<ReadingProgress>;
  saveReadProgress(bookId: string, progress: BookmarkPayload): Promise<void>;
  addReadHistory(bookId: string): Promise<void>;
  listReadHistoryBooks(): Promise<Book[]>;
  clearReadHistory(): Promise<void>;

  getBookCover(bookId: string): Promise<Blob>;
  getBookCoverUrl(bookId: string, cacheKey?: number): string;

  /**
   * Whether this backend applies the device's adult-content setting itself.
   * Absent means no: behind a server, `show_nsfw` has already filtered.
   */
  filtersNsfwOnDevice?(): boolean;

  /**
   * Adult-content marks by book id, from the device alone: no request, and null when it has none.
   */
  localNsfwMarks?(): Promise<ReadonlyMap<string, NsfwMarks> | null>;

  /** A read-only backend may answer these with an empty result rather than refuse. */
  getDuplicateBookGroups(): Promise<string[][]>;
  /**
   * Similar-but-not-identical book pairs, scored by the server in one pass.
   * `floor` is the lowest Jaccard returned; the similar-content page fetches
   * once at the widest floor and narrows in memory. A read-only backend with no
   * fingerprint cache may answer empty. `confirm` releases the server's work
   * budget after the user has reviewed its estimate.
   */
  getSimilarBookPairs(floor?: number, confirm?: boolean): Promise<SimilarBookPair[]>;
  /** Coverage of the fingerprint cache, for the "build what's missing" bar. */
  getFingerprintStatus(): Promise<FingerprintStatus>;
  listTrashedBooks(): Promise<TrashedBookListing>;

  /** Folder paths in the shape `api/folders.ts` returns: '/' for the top level. */
  listFolders(): Promise<string[]>;

  listSources(bookId: string): Promise<SourceMeta[]>;
  getSource(bookId: string, sourceId: string): Promise<SourceMeta>;
  getSourceContent(bookId: string, sourceId: string): Promise<string>;

  /**
   * Rejects for an illustration the backend cannot reach (an offline cache only
   * holds what it downloaded); the reader then shows the alt text, so a missing
   * illustration never costs the chapter.
   */
  getSourceAsset(bookId: string, sourceId: string, name: string): Promise<Blob>;

  /**
   * The named illustrations as one zip, for a download that would otherwise pay
   * a request per figure; only the mobile download path uses it, falling back to
   * per-file fetches. Online reading stays per-image and lazy.
   *
   * Returns the raw archive, not decoded blobs, so the caller can unzip one
   * entry at a time. An absent name is packed as no entry.
   */
  getSourceAssetsBundle(bookId: string, sourceId: string, names: string[]): Promise<Blob>;

  /** Rescan the shelf when capabilities.rescan allows it. */
  refreshShelf?(): Promise<ShelfRefreshResult | void>;

  /**
   * The shelf this client already knows it is pointed at, for when listing
   * shelves fails. Present only where the choice is device-local and outlives a
   * failed request — the mobile shell persists it during connection setup — so
   * offline-cached books stay reachable when the network does not. '' when
   * nothing has been chosen yet.
   */
  getPersistedShelfID?(): string;

  downloadBook?(bookId: string): Promise<void>;
  removeDownload?(bookId: string): Promise<void>;
  getDownloadState?(bookId: string): Promise<DownloadState>;
  listDownloadedBookEntries?(): Promise<DownloadedBookEntry[]>;
}

/**
 * Shelf mutations, absent as a block on a provider that cannot write: the
 * Android client and the pCloud backend are read-only by construction rather
 * than by refusing at runtime. Reach these through `bookshelfWriter()`.
 */
export interface BookshelfWriter {
  /**
   * What `isWritableProvider()` reads. A flag rather than a probe for one of the
   * methods, because a wrapper always has the methods even when the backend it
   * wraps does not; `implements BookshelfWriter` is what makes the claim honest,
   * since TypeScript then demands the whole surface.
   */
  readonly writable: true;

  updateBook(bookId: string, payload: BookUpdateRequest): Promise<Book>;
  updateBookFolder(bookId: string, folder: string): Promise<void>;
  /** Duplicates a book into `folder`, returning the copy with its fresh id. */
  copyBook(bookId: string, folder: string): Promise<Book>;
  deleteBook(bookId: string): Promise<void>;

  /**
   * Folders of another shelf, for the cross-shelf transfer destination picker.
   * A read, but it lives on the writer because only a writable multi-shelf
   * backend (the server and desktop) can reach a shelf other than the active
   * one, which is exactly where the transfer flow that needs it runs.
   */
  listShelfFolders(shelfID: string): Promise<string[]>;
  /**
   * Copies or moves a book from the active shelf to `targetShelfID`, returning
   * the id of the background task chain to poll. `targetFolder` is a '/'-joined
   * path; '' lands the book at the target shelf root.
   */
  transferBook(
    bookId: string,
    targetShelfID: string,
    targetFolder: string,
    mode: BookTransferMode
  ): Promise<string>;
  /**
   * As transferBook, for a whole folder and everything beneath it.
   * `targetFolder` is the folder's full destination path on the target shelf.
   */
  transferFolder(
    sourceFolder: string,
    targetShelfID: string,
    targetFolder: string,
    mode: BookTransferMode,
    options?: FolderChangeOptions
  ): Promise<string>;
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
  /**
   * Builds a similarity fingerprint for every source that lacks one, or — when
   * `force` is set — rebuilds every source's fingerprint, ignoring the cache.
   */
  startFingerprintSources(force?: boolean): Promise<string>;
  /** A GET, but a chain id can only come from a write that schedules a chain. */
  getTaskChain(taskChainId: string): Promise<TaskChain>;

  createSource(bookId: string, options?: CreateSourceOptions): Promise<SourceMeta>;
  deleteSource(bookId: string, sourceId: string): Promise<void>;
  /** Removes a source's import note. There is no counterpart that rewrites it. */
  deleteSourceComment(bookId: string, sourceId: string): Promise<void>;
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
