import type {
  BookmarkPayload,
  Book,
  BookContent,
  PaginatedBooks,
  ReadingProgress,
  TrashedBook
} from '@/types/book';
import type { SourceMeta } from '@/types/source';
import type { BookshelfReader, ListBooksOptions } from '@/providers/bookshelfProvider';

/**
 * What the standalone reading binary reads through.
 *
 * It is the server provider with the write surface taken off, and it declares
 * `BookshelfReader` alone rather than the loose alias so that stays true: adding
 * a write method here would fail to compile.
 *
 * A wrapper rather than a flag on ServerBookshelfProvider because "can this
 * client write" is answered by `isWritableProvider()`, which reads the write
 * surface itself. That is what turns the whole editing UI off — the edit
 * buttons, the trash and maintenance nav, the server settings tabs, the logs —
 * for a backend that has no routes behind any of it. A writable provider
 * pointed at a reading server would render all of it and fail on every click.
 *
 * The two shelf-wide reads a reader server does not mount are answered empty
 * rather than forwarded, which BookshelfReader allows for exactly this case. The
 * pages that show them are blocked by the reader shell's route policy, so this
 * is the second line: a stray caller gets nothing rather than a 404.
 *
 * `supportsShelfRefresh` is deliberately absent, which is what hides the manual
 * update button: the rescan endpoint is not mounted either.
 */
export class ReaderBookshelfProvider implements BookshelfReader {
  constructor(private readonly source: BookshelfReader) {}

  listBooks(page?: number, pageSize?: number, options?: ListBooksOptions): Promise<PaginatedBooks> {
    return this.source.listBooks(page, pageSize, options);
  }

  getBook(bookId: string): Promise<Book> {
    return this.source.getBook(bookId);
  }

  getBookContent(bookId: string): Promise<BookContent> {
    return this.source.getBookContent(bookId);
  }

  downloadBookContent(bookId: string): Promise<Blob> {
    return this.source.downloadBookContent(bookId);
  }

  // Reading progress and history are device-local wherever they are kept, so a
  // reader records them exactly as the web client does: in the browser, never
  // against the shelf.
  getReadProgress(bookId: string): Promise<ReadingProgress> {
    return this.source.getReadProgress(bookId);
  }

  saveReadProgress(bookId: string, progress: BookmarkPayload): Promise<void> {
    return this.source.saveReadProgress(bookId, progress);
  }

  addReadHistory(bookId: string): Promise<void> {
    return this.source.addReadHistory(bookId);
  }

  listReadHistoryBooks(): Promise<Book[]> {
    return this.source.listReadHistoryBooks();
  }

  clearReadHistory(): Promise<void> {
    return this.source.clearReadHistory();
  }

  getBookCover(bookId: string): Promise<Blob> {
    return this.source.getBookCover(bookId);
  }

  getBookCoverUrl(bookId: string, cacheKey?: number): string {
    return this.source.getBookCoverUrl(bookId, cacheKey);
  }

  // Not mounted by a reader server; see the class comment.
  getDuplicateBookGroups(): Promise<string[][]> {
    return Promise.resolve([]);
  }

  listTrashedBooks(): Promise<TrashedBook[]> {
    return Promise.resolve([]);
  }

  listLayers(): Promise<string[]> {
    return this.source.listLayers();
  }

  listSources(bookId: string): Promise<SourceMeta[]> {
    return this.source.listSources(bookId);
  }

  getSource(bookId: string, sourceId: string): Promise<SourceMeta> {
    return this.source.getSource(bookId, sourceId);
  }

  getSourceContent(bookId: string, sourceId: string): Promise<string> {
    return this.source.getSourceContent(bookId, sourceId);
  }

  getSourceAsset(bookId: string, sourceId: string, name: string): Promise<Blob> {
    if (!this.source.getSourceAsset) {
      return Promise.reject(new Error('This backend cannot read source assets.'));
    }
    return this.source.getSourceAsset(bookId, sourceId, name);
  }
}
