import type { BookTransferMode, FingerprintStatus, ListBooksOptions, SimilarBookPair } from '@/api/books';
import type { FolderChangeOptions, FolderMetaListing } from '@/api/folders';
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

export interface ShelfRefreshResult {
  bookCount: number;
  folderCount: number;
}

export interface BookshelfReader {
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

  filtersNsfwOnDevice?(): boolean;

  localNsfwMarks?(): Promise<ReadonlyMap<string, NsfwMarks> | null>;

  getDuplicateBookGroups(): Promise<string[][]>;
  getSimilarBookPairs(floor?: number, confirm?: boolean): Promise<SimilarBookPair[]>;
  getFingerprintStatus(): Promise<FingerprintStatus>;
  listTrashedBooks(): Promise<TrashedBookListing>;

  listFolders(): Promise<string[]>;

  listFolderMeta?(): Promise<FolderMetaListing>;

  listSources(bookId: string): Promise<SourceMeta[]>;
  getSource(bookId: string, sourceId: string): Promise<SourceMeta>;
  getSourceContent(bookId: string, sourceId: string): Promise<string>;

  getSourceAsset(bookId: string, sourceId: string, name: string): Promise<Blob>;

  getSourceAssetsBundle(bookId: string, sourceId: string, names: string[]): Promise<Blob>;

  refreshShelf?(): Promise<ShelfRefreshResult | void>;

  getPersistedShelfID?(): string;

  downloadBook?(bookId: string): Promise<void>;
  removeDownload?(bookId: string): Promise<void>;
  getDownloadState?(bookId: string): Promise<DownloadState>;
  listDownloadedBookEntries?(): Promise<DownloadedBookEntry[]>;
}

export interface BookshelfWriter {
  readonly writable: true;

  updateBook(bookId: string, payload: BookUpdateRequest): Promise<Book>;
  updateBookFolder(bookId: string, folder: string): Promise<void>;
  copyBook(bookId: string, folder: string): Promise<Book>;
  deleteBook(bookId: string): Promise<void>;

  listShelfFolders(shelfID: string): Promise<string[]>;
  transferBook(
    bookId: string,
    targetShelfID: string,
    targetFolder: string,
    mode: BookTransferMode
  ): Promise<string>;
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
  refreshContentStats(): Promise<string>;
  startFingerprintSources(force?: boolean): Promise<string>;
  getTaskChain(taskChainId: string): Promise<TaskChain>;

  createSource(bookId: string, options?: CreateSourceOptions): Promise<SourceMeta>;
  deleteSource(bookId: string, sourceId: string): Promise<void>;
  deleteSourceComment(bookId: string, sourceId: string): Promise<void>;
  setCurrentSource(bookId: string, sourceId: string): Promise<void>;
  updateSourceContent(bookId: string, sourceId: string, content: string): Promise<void>;
  refreshSourceMeta(bookId: string, sourceId: string): Promise<SourceMeta>;
}

export type WritableBookshelfProvider = BookshelfReader & BookshelfWriter;

export type BookshelfProvider = BookshelfReader & Partial<BookshelfWriter>;
