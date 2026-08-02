import {
  deleteBook,
  deleteBookCover,
  deleteTrashedBook,
  downloadBookContent,
  emptyTrash,
  getBook,
  getBookContent,
  getBookCover,
  getBookCoverUrl,
  getBookSplitConfig,
  getDuplicateBookGroups,
  getReadingProgress,
  importBook,
  listBooks,
  listTrashedBooks,
  restoreTrashedBook,
  saveBookmark,
  updateBook,
  updateBookLayer,
  updateBookSplitConfig,
  uploadBookCover,
  uploadBookCoverBlob
} from '@/api/books';
import {
  createSource,
  deleteSource,
  getSource,
  getSourceContent,
  listSource,
  refreshSourceMeta,
  setCurrentSource,
  updateSourceContent
} from '@/api/sources';
import { getTaskChain } from '@/api/taskchains';
import { startBookBatch } from '@/api/bookBatches';
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

export class ServerBookshelfProvider implements BookshelfProvider {
  listBooks(page?: number, pageSize?: number, options?: ListBooksOptions): Promise<PaginatedBooks> {
    return listBooks(page, pageSize, options);
  }

  getBook(bookId: string): Promise<Book> {
    return getBook(bookId);
  }

  updateBook(bookId: string, payload: BookUpdateRequest): Promise<Book> {
    return updateBook(bookId, payload);
  }

  updateBookLayer(bookId: string, layer: string): Promise<void> {
    return updateBookLayer(bookId, layer);
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

  getBookSplitConfig(bookId: string): Promise<SplitConfig> {
    return getBookSplitConfig(bookId);
  }

  updateBookSplitConfig(bookId: string, config: SplitConfig): Promise<SplitConfig> {
    return updateBookSplitConfig(bookId, config);
  }

  getReadProgress(bookId: string): Promise<ReadingProgress> {
    return getReadingProgress(bookId);
  }

  saveReadProgress(bookId: string, progress: BookmarkPayload): Promise<void> {
    return saveBookmark(bookId, progress);
  }

  // Reading history is device-local (see storage/readHistory); only the books
  // it points at come from the server.
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

  listSources(bookId: string): Promise<SourceMeta[]> {
    return listSource(bookId);
  }

  getSource(bookId: string, sourceId: string): Promise<SourceMeta> {
    return getSource(bookId, sourceId);
  }

  getSourceContent(bookId: string, sourceId: string): Promise<string> {
    return getSourceContent(bookId, sourceId);
  }

  createSource(bookId: string): Promise<SourceMeta> {
    return createSource(bookId);
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
}
