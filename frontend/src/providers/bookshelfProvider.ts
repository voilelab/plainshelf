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
} from '../types/book';
import type { SourceMeta } from '../types/source';

export type { DownloadState } from '../types/book';

export interface DesktopImportBookResult {
  path?: string;
  id?: string;
  error?: string;
}

export interface BookshelfProvider {
  listBooks(page?: number, pageSize?: number, search?: string): Promise<PaginatedBooks>;
  getBook(bookId: string): Promise<Book>;
  updateBook(bookId: string, payload: BookUpdateRequest): Promise<Book>;
  updateBookLayer(bookId: string, layer: string): Promise<void>;
  deleteBook(bookId: string): Promise<void>;

  getBookContent(bookId: string): Promise<BookContent>;
  downloadBookContent(bookId: string): Promise<Blob>;
  getBookSplitConfig(bookId: string): Promise<SplitConfig>;
  updateBookSplitConfig(bookId: string, config: SplitConfig): Promise<SplitConfig>;

  getReadProgress(bookId: string): Promise<ReadingProgress>;
  saveReadProgress(bookId: string, progress: BookmarkPayload): Promise<void>;
  addReadHistory(bookId: string): Promise<void>;
  listReadHistoryBooks(): Promise<Book[]>;
  clearReadHistory(): Promise<void>;

  importBook(payload: BookCreateRequest): Promise<Book>;
  uploadBookCover(bookId: string, file: File): Promise<void>;
  uploadBookCoverBlob(bookId: string, blob: Blob): Promise<void>;
  getBookCover(bookId: string): Promise<Blob>;
  getBookCoverUrl(bookId: string, cacheKey?: number): string;
  deleteBookCover(bookId: string): Promise<void>;

  getDuplicateBookGroups(): Promise<string[][]>;
  listTrashedBooks(): Promise<TrashedBook[]>;
  restoreTrashedBook(bookId: string): Promise<void>;
  deleteTrashedBook(bookId: string): Promise<void>;

  listSources(bookId: string): Promise<SourceMeta[]>;
  getSource(bookId: string, sourceId: string): Promise<SourceMeta>;
  getSourceContent(bookId: string, sourceId: string): Promise<string>;
  createSource(bookId: string): Promise<SourceMeta>;
  deleteSource(bookId: string, sourceId: string): Promise<void>;
  setCurrentSource(bookId: string, sourceId: string): Promise<void>;
  updateSourceContent(bookId: string, sourceId: string, content: string): Promise<void>;

  downloadBook?(bookId: string): Promise<void>;
  removeDownload?(bookId: string): Promise<void>;
  getDownloadState?(bookId: string): Promise<DownloadState>;

  openLocalBookFiles?(): Promise<string[] | null>;
  importBooksFromLocalPaths?(localPaths: string[], layerPath: string): Promise<DesktopImportBookResult[] | null>;

  openDesktopShelfDirectory?(): Promise<string | null>;
  addDesktopShelf?(name: string, libRoot: string, scanInterval: string): Promise<void>;
  removeDesktopShelf?(shelfID: string): Promise<void>;
}
