import type {
  DesktopAddShelfParams,
  DesktopModifyShelfParams,
  DesktopShelfDetails,
  DesktopShelfIDPreview
} from '@/api/desktop';
import type { BookTransferMode, FingerprintStatus, ListBooksOptions, SimilarBookPair } from '@/api/books';
import type { FolderChangeOptions } from '@/api/folders';
import type {
  BookmarkPayload,
  Book,
  BookCreateRequest,
  BookContent,
  BookUpdateRequest,
  DownloadState,
  PaginatedBooks,
  ReadingProgress,
  TrashedBook,
  TrashedBookListing
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
   * Whether the cover has to be fetched through getBookCover() and handed to the
   * element as a `blob:` URL. True when the cover URL is not a plain fetchable
   * address (pCloud returns an internal `pcloud:` reference) and on the mobile
   * shell, where a direct `<img src="http://...">` is issued by the WebView
   * itself, bypassing CapacitorHttp and tripping mixed-content blocking.
   */
  coversMustBeFetchedAsBlob?(): boolean;

  /**
   * Whether there is a PlainShelf server behind this backend to report its
   * `read_only` mode. Absent means yes. pCloud answers false: it is storage,
   * not a server, so asking would be a request nothing can answer.
   */
  supportsServerMode?(): boolean;

  /**
   * Whether character counts alongside a listing are affordable. Absent means
   * yes — a server computes them from its shelf cache. pCloud answers false: it
   * would fetch every book's meta.json over the network for one listing. A
   * property of the backend, not the device, so a phone pointed at a server can
   * still offer the filter.
   */
  supportsCharCountListing?(): boolean;

  /**
   * Whether this backend applies the device's adult-content setting itself.
   *
   * Absent means no, which is the answer wherever a PlainShelf server is behind
   * the backend: there the server's own `show_nsfw` filters before anything
   * reaches this client, and a second filter here would let a device setting
   * override a decision the server has already made. Only a backend reading the
   * shelf's files directly — pCloud — answers true, because it has no server to
   * ask and would otherwise ignore the marks entirely.
   *
   * Views that filter on the device read this to know whether to; the listings
   * this backend serves are already filtered by the time they are returned.
   */
  filtersNsfwOnDevice?(): boolean;

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
   * Optional because a backend can serve a book's text without reaching its
   * images: an offline cache only holds what it downloaded. The reader falls
   * back to the alt text, so a missing illustration never costs the chapter.
   */
  getSourceAsset?(bookId: string, sourceId: string, name: string): Promise<Blob>;

  /**
   * The named illustrations as one zip, for a download that would otherwise pay
   * a request per figure; only the mobile download path uses it, falling back to
   * per-file fetches. Online reading stays per-image and lazy.
   *
   * Returns the raw archive, not decoded blobs, so the caller can unzip one
   * entry at a time. An absent name is packed as no entry.
   */
  getSourceAssetsBundle?(bookId: string, sourceId: string, names: string[]): Promise<Blob>;

  /**
   * Manual shelf update, for a listing that may not reflect the shelf right now:
   * pCloud scans once and then reads a stored copy, and a server only rescans
   * every `scan_interval` with no change notification from an SMB or cloud
   * mount.
   *
   * `supportsShelfRefresh` is what the UI asks, because a wrapper always has the
   * methods even when the backend it wraps does not. `refreshShelf` reports what
   * the update found; `getShelfFetchedAt` instead dates a stored listing. No
   * backend does both, and neither is required.
   */
  supportsShelfRefresh?(): boolean;
  refreshShelf?(): Promise<ShelfRefreshResult | void>;
  getShelfFetchedAt?(): Promise<number | null>;

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
  getStorageEstimate?(): Promise<StorageEstimateResult>;

  /** Opens the host file picker. Changes nothing; the import itself is a write. */
  openLocalBookFiles?(): Promise<string[] | null>;
  openDesktopFolder?(folderPath: string): Promise<void>;
  openDesktopBookFolder?(bookId: string): Promise<void>;

  /**
   * Opens a book in the standalone reader's own window (desktop only). Present
   * only on the desktop provider; its absence is what keeps web and mobile on
   * the in-app reader. `section` is the reader section index to open at for a
   * chapter "read" action; omit it to open at the book's restored progress.
   */
  openDesktopReader?(bookId: string, section?: number): Promise<void>;

  openDesktopShelfDirectory?(): Promise<string | null>;
  /** Reveals a shelf's lib_root in the host file explorer (desktop only). */
  openDesktopShelfFolder?(shelfID: string): Promise<void>;
  /**
   * The shelf id AddShelf would assign to `name` right now, plus the directory
   * such a shelf would land in when the user picks none, so the add-shelf form
   * can preview both as the user types. Both are '' when unavailable.
   */
  previewDesktopShelfID?(name: string): Promise<DesktopShelfIDPreview>;
  /**
   * Creates a desktop shelf. `params.readOnly` opens it without writing to it at
   * all, so `params.libRoot` must already exist — a read-only shelf is never
   * created.
   */
  addDesktopShelf?(params: DesktopAddShelfParams): Promise<void>;
  removeDesktopShelf?(shelfID: string): Promise<void>;
  getDesktopShelfDetails?(shelfID: string): Promise<DesktopShelfDetails>;
  modifyDesktopShelf?(params: DesktopModifyShelfParams): Promise<void>;
  /**
   * Writes a book's content out as a file the user keeps. Desktop opens a native
   * save dialog and resolves to `void` — the dialog is its own confirmation.
   * Mobile writes to a shared folder with no dialog, so it resolves to a
   * location string the caller surfaces as a toast. A browser build omits the
   * method and falls back to a blob download.
   */
  saveBookContentToFile?(bookId: string, suggestedName: string): Promise<string | void>;
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

  /**
   * Optional because only the desktop shell can reach host paths, but a write
   * either way: one book in the active shelf, as importBook, from a picker
   * result rather than an upload. Called once per selected file so the shared
   * import executor drives the same N/M progress and file-boundary abort.
   */
  importBookFromLocalPath?(localPath: string, folderPath: string): Promise<DesktopImportBookResult | null>;

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
