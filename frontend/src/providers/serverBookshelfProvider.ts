import {
  copyBook,
  deleteBook,
  deleteBookCover,
  deleteTrashedBook,
  downloadBookContent,
  emptyTrash,
  getBook,
  getBookContent,
  getBookCover,
  getBookCoverUrl,
  getDuplicateBookGroups,
  getFingerprintStatus,
  getSimilarBookPairs,
  importBook,
  listBooks,
  listTrashedBooks,
  refreshContentStats,
  startFingerprintSources,
  restoreTrashedBook,
  transferBook,
  updateBook,
  updateBookFolder,
  uploadBookCover,
  uploadBookCoverBlob
} from '@/api/books';
import {
  createSource,
  deleteSource,
  getSource,
  getSourceAsset,
  getSourceContent,
  listSource,
  refreshSourceMeta,
  setCurrentSource,
  updateSourceContent
} from '@/api/sources';
import { getFolders, transferFolder } from '@/api/folders';
import { rescanShelf } from '@/api/shelves';
import { getTaskChain } from '@/api/taskchains';
import { startBookBatch } from '@/api/bookBatches';
import {
  addReadHistory as addLocalReadHistory,
  clearReadHistory as clearLocalReadHistory
} from '@/storage/readHistory';
import {
  getLocalReadingProgress,
  saveLocalReadingProgress
} from '@/storage/readingProgress';
import { collectReadHistoryBooks } from './readHistoryBooks';
import type {
  BookmarkPayload,
  Book,
  BookCreateRequest,
  BookContent,
  BookUpdateRequest,
  PaginatedBooks,
  ReadingProgress,
  TrashedBook
} from '@/types/book';
import type { BookTransferMode, FingerprintStatus, SimilarBookPair } from '@/api/books';
import type { CreateSourceOptions, SourceMeta } from '@/types/source';
import type { BookBatchRequest, TaskChain } from '@/types/task';
import type {
  BookshelfReader,
  BookshelfWriter,
  ListBooksOptions,
  ShelfRefreshResult
} from './bookshelfProvider';

// Declares both halves rather than the loose BookshelfProvider alias, so
// dropping or mistyping any write method fails to compile here.
export class ServerBookshelfProvider implements BookshelfReader, BookshelfWriter {
  readonly writable = true as const;

  listBooks(page?: number, pageSize?: number, options?: ListBooksOptions): Promise<PaginatedBooks> {
    return listBooks(page, pageSize, options);
  }

  getBook(bookId: string): Promise<Book> {
    return getBook(bookId);
  }

  updateBook(bookId: string, payload: BookUpdateRequest): Promise<Book> {
    return updateBook(bookId, payload);
  }

  updateBookFolder(bookId: string, folder: string): Promise<void> {
    return updateBookFolder(bookId, folder);
  }

  copyBook(bookId: string, folder: string): Promise<Book> {
    return copyBook(bookId, folder);
  }

  listShelfFolders(shelfID: string): Promise<string[]> {
    return getFolders(shelfID);
  }

  transferBook(
    bookId: string,
    targetShelfID: string,
    targetFolder: string,
    mode: BookTransferMode
  ): Promise<string> {
    return transferBook(bookId, targetShelfID, targetFolder, mode);
  }

  transferFolder(
    sourceFolder: string,
    targetShelfID: string,
    targetFolder: string,
    mode: BookTransferMode
  ): Promise<string> {
    return transferFolder(sourceFolder, targetShelfID, targetFolder, mode);
  }

  deleteBook(bookId: string): Promise<void> {
    return deleteBook(bookId);
  }

  getBookContent(bookId: string): Promise<BookContent> {
    return getBookContent(bookId);
  }

  downloadBookContent(bookId: string): Promise<Blob> {
    return downloadBookContent(bookId);
  }

  getReadProgress(bookId: string): Promise<ReadingProgress> {
    return getLocalReadingProgress(bookId);
  }

  saveReadProgress(bookId: string, progress: BookmarkPayload): Promise<void> {
    return saveLocalReadingProgress(bookId, progress);
  }

  // Reading progress and history are device-local; only book data comes from
  // the server.
  addReadHistory(bookId: string): Promise<void> {
    return addLocalReadHistory(bookId);
  }

  listReadHistoryBooks(): Promise<Book[]> {
    return collectReadHistoryBooks((page, pageSize) => this.listBooks(page, pageSize));
  }

  clearReadHistory(): Promise<void> {
    return clearLocalReadHistory();
  }

  importBook(payload: BookCreateRequest): Promise<Book> {
    return importBook(payload);
  }

  uploadBookCover(bookId: string, file: File): Promise<void> {
    return uploadBookCover(bookId, file);
  }

  uploadBookCoverBlob(bookId: string, blob: Blob): Promise<void> {
    return uploadBookCoverBlob(bookId, blob);
  }

  getBookCover(bookId: string): Promise<Blob> {
    return getBookCover(bookId);
  }

  getBookCoverUrl(bookId: string, cacheKey?: number): string {
    return getBookCoverUrl(bookId, cacheKey);
  }

  deleteBookCover(bookId: string): Promise<void> {
    return deleteBookCover(bookId);
  }

  getDuplicateBookGroups(): Promise<string[][]> {
    return getDuplicateBookGroups();
  }

  getSimilarBookPairs(floor?: number): Promise<SimilarBookPair[]> {
    return getSimilarBookPairs(floor);
  }

  getFingerprintStatus(): Promise<FingerprintStatus> {
    return getFingerprintStatus();
  }

  listTrashedBooks(): Promise<TrashedBook[]> {
    return listTrashedBooks();
  }

  restoreTrashedBook(bookId: string): Promise<void> {
    return restoreTrashedBook(bookId);
  }

  deleteTrashedBook(bookId: string): Promise<void> {
    return deleteTrashedBook(bookId);
  }

  emptyTrash(): Promise<string> {
    return emptyTrash();
  }

  getTaskChain(taskChainId: string): Promise<TaskChain> {
    return getTaskChain(taskChainId);
  }

  startBookBatch(request: BookBatchRequest): Promise<string> {
    return startBookBatch(request);
  }

  refreshContentStats(): Promise<string> {
    return refreshContentStats();
  }

  startFingerprintSources(force?: boolean): Promise<string> {
    return startFingerprintSources(force);
  }

  listFolders(): Promise<string[]> {
    return getFolders();
  }

  listSources(bookId: string): Promise<SourceMeta[]> {
    return listSource(bookId);
  }

  getSource(bookId: string, sourceId: string): Promise<SourceMeta> {
    return getSource(bookId, sourceId);
  }

  getSourceContent(bookId: string, sourceId: string): Promise<string> {
    return getSourceContent(bookId, sourceId);
  }

  getSourceAsset(bookId: string, sourceId: string, name: string): Promise<Blob> {
    return getSourceAsset(bookId, sourceId, name);
  }

  createSource(bookId: string, options?: CreateSourceOptions): Promise<SourceMeta> {
    return createSource(bookId, options);
  }

  deleteSource(bookId: string, sourceId: string): Promise<void> {
    return deleteSource(bookId, sourceId);
  }

  setCurrentSource(bookId: string, sourceId: string): Promise<void> {
    return setCurrentSource(bookId, sourceId);
  }

  updateSourceContent(bookId: string, sourceId: string, content: string): Promise<void> {
    return updateSourceContent(bookId, sourceId, content);
  }

  refreshSourceMeta(bookId: string, sourceId: string): Promise<SourceMeta> {
    return refreshSourceMeta(bookId, sourceId);
  }

  // A server discovers external changes only every `scan_interval`, and on an
  // SMB or cloud-mounted shelf nothing notifies it sooner, so the user needs a
  // way to say "look now". There is no stored listing to date, hence no
  // getShelfFetchedAt: the counts refreshShelf reports are what the UI shows.
  supportsShelfRefresh(): boolean {
    return true;
  }

  refreshShelf(): Promise<ShelfRefreshResult> {
    return rescanShelf();
  }
}
