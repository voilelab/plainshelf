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
import type {
  BookshelfProvider,
  DesktopImportBookResult,
  DownloadedBookEntry,
  StorageEstimateResult
} from './bookshelfProvider';
import { InMemoryMobileBookCache, type MobileBookCache } from './mobileBookCache';
import { ServerBookshelfProvider } from './serverBookshelfProvider';

export const OFFLINE_BOOK_CACHE_MISS_ERROR = 'Book is not downloaded and the app is offline';
export const OFFLINE_SOURCE_CACHE_MISS_ERROR = 'Source is not downloaded and the app is offline';
export const OFFLINE_DOWNLOAD_UNAVAILABLE_ERROR = 'Cannot download book while offline';

const defaultIsOnline = (): boolean =>
  typeof navigator === 'undefined' ? true : navigator.onLine;

export class MobileBookshelfProvider implements BookshelfProvider {
  // Memoized per-book object URLs for offline cover blobs, keyed by book id.
  // Created lazily in applyOfflineCover and revoked in removeDownload; the
  // provider is a long-lived singleton so this map's lifetime matches the
  // app's, which is fine given the small number of downloaded books.
  private readonly coverUrlCache = new Map<string, string>();

  constructor(
    private readonly remote: BookshelfProvider = new ServerBookshelfProvider(),
    private readonly cache: MobileBookCache = new InMemoryMobileBookCache(),
    private readonly isOnline: () => boolean = defaultIsOnline
  ) {}

  async listBooks(page = 1, pageSize = 20, search?: string): Promise<PaginatedBooks> {
    if (this.isOnline()) {
      const remoteBooks = await this.remote.listBooks(page, pageSize, search);
      const items = await Promise.all(remoteBooks.items.map((book) => this.annotateDownloadState(book)));
      return { ...remoteBooks, items };
    }

    const downloaded = await this.cache.listDownloadedBooks();
    const filtered = this.filterBooks(downloaded, search);
    const start = Math.max(0, (page - 1) * pageSize);
    const pageItems = filtered.slice(start, start + pageSize);
    const items = await Promise.all(pageItems.map((book) => this.applyOfflineCover(book)));

    return {
      items,
      total: filtered.length,
      page,
      pageSize
    };
  }

  async getBook(bookId: string): Promise<Book> {
    if (this.isOnline()) {
      return this.annotateDownloadState(await this.remote.getBook(bookId));
    }

    const cached = await this.cache.getCachedBook(bookId);
    if (cached) {
      return this.applyOfflineCover(cached);
    }

    throw new Error(OFFLINE_BOOK_CACHE_MISS_ERROR);
  }

  updateBook(bookId: string, payload: BookUpdateRequest): Promise<Book> {
    return this.remote.updateBook(bookId, payload);
  }

  updateBookLayer(bookId: string, layer: string): Promise<void> {
    return this.remote.updateBookLayer(bookId, layer);
  }

  deleteBook(bookId: string): Promise<void> {
    return this.remote.deleteBook(bookId);
  }

  async getBookContent(bookId: string): Promise<BookContent> {
    const cached = await this.cache.getCachedBookContent(bookId);
    if (cached) {
      return cached;
    }

    if (this.isOnline()) {
      return this.remote.getBookContent(bookId);
    }

    throw new Error(OFFLINE_BOOK_CACHE_MISS_ERROR);
  }

  async downloadBookContent(bookId: string): Promise<Blob> {
    const cached = await this.cache.getCachedBookContent(bookId);
    if (cached) {
      return new Blob([cached.content], { type: 'text/plain;charset=utf-8' });
    }

    if (this.isOnline()) {
      return this.remote.downloadBookContent(bookId);
    }

    throw new Error(OFFLINE_BOOK_CACHE_MISS_ERROR);
  }

  getBookSplitConfig(bookId: string): Promise<SplitConfig> {
    return this.remote.getBookSplitConfig(bookId);
  }

  updateBookSplitConfig(bookId: string, config: SplitConfig): Promise<SplitConfig> {
    return this.remote.updateBookSplitConfig(bookId, config);
  }

  async getReadProgress(bookId: string): Promise<ReadingProgress> {
    const cached = await this.cache.getReadProgress(bookId);
    if (cached) {
      return cached;
    }

    if (this.isOnline()) {
      return this.remote.getReadProgress(bookId);
    }

    return { char_offset: 0 };
  }

  saveReadProgress(bookId: string, progress: BookmarkPayload): Promise<void> {
    return this.cache.saveReadProgress(bookId, progress);
  }

  addReadHistory(bookId: string): Promise<void> {
    return this.isOnline() ? this.remote.addReadHistory(bookId) : Promise.resolve();
  }

  listReadHistoryBooks(): Promise<Book[]> {
    return this.remote.listReadHistoryBooks();
  }

  clearReadHistory(): Promise<void> {
    return this.remote.clearReadHistory();
  }

  importBook(payload: BookCreateRequest): Promise<Book> {
    return this.remote.importBook(payload);
  }

  uploadBookCover(bookId: string, file: File): Promise<void> {
    return this.remote.uploadBookCover(bookId, file);
  }

  uploadBookCoverBlob(bookId: string, blob: Blob): Promise<void> {
    return this.remote.uploadBookCoverBlob(bookId, blob);
  }

  async getBookCover(bookId: string): Promise<Blob> {
    const cached = await this.cache.getCachedCover(bookId);
    if (cached) {
      return cached;
    }

    return this.remote.getBookCover(bookId);
  }

  getBookCoverUrl(bookId: string, cacheKey?: number): string {
    return this.remote.getBookCoverUrl(bookId, cacheKey);
  }

  deleteBookCover(bookId: string): Promise<void> {
    return this.remote.deleteBookCover(bookId);
  }

  getDuplicateBookGroups(): Promise<string[][]> {
    return this.remote.getDuplicateBookGroups();
  }

  listTrashedBooks(): Promise<TrashedBook[]> {
    return this.remote.listTrashedBooks();
  }

  restoreTrashedBook(bookId: string): Promise<void> {
    return this.remote.restoreTrashedBook(bookId);
  }

  deleteTrashedBook(bookId: string): Promise<void> {
    return this.remote.deleteTrashedBook(bookId);
  }

  async listSources(bookId: string): Promise<SourceMeta[]> {
    if (this.isOnline()) {
      return this.remote.listSources(bookId);
    }

    return this.cache.listCachedSources(bookId);
  }

  async getSource(bookId: string, sourceId: string): Promise<SourceMeta> {
    if (this.isOnline()) {
      return this.remote.getSource(bookId, sourceId);
    }

    const cached = await this.cache.getCachedSource(bookId, sourceId);
    if (cached) {
      return cached;
    }

    throw new Error(OFFLINE_SOURCE_CACHE_MISS_ERROR);
  }

  async getSourceContent(bookId: string, sourceId: string): Promise<string> {
    const cached = await this.cache.getCachedSourceContent(bookId, sourceId);
    if (cached !== null) {
      return cached;
    }

    if (this.isOnline()) {
      return this.remote.getSourceContent(bookId, sourceId);
    }

    throw new Error(OFFLINE_SOURCE_CACHE_MISS_ERROR);
  }

  createSource(bookId: string): Promise<SourceMeta> {
    return this.remote.createSource(bookId);
  }

  deleteSource(bookId: string, sourceId: string): Promise<void> {
    return this.remote.deleteSource(bookId, sourceId);
  }

  setCurrentSource(bookId: string, sourceId: string): Promise<void> {
    return this.remote.setCurrentSource(bookId, sourceId);
  }

  updateSourceContent(bookId: string, sourceId: string, content: string): Promise<void> {
    return this.remote.updateSourceContent(bookId, sourceId, content);
  }

  async getDownloadState(bookId: string): Promise<DownloadState> {
    return this.cache.getDownloadState(bookId);
  }

  async downloadBook(bookId: string): Promise<void> {
    if (!this.isOnline()) {
      throw new Error(OFFLINE_DOWNLOAD_UNAVAILABLE_ERROR);
    }

    const [book, sources, bookContent] = await Promise.all([
      this.remote.getBook(bookId),
      this.remote.listSources(bookId),
      this.remote.getBookContent(bookId)
    ]);
    const sourceContents = await Promise.all(
      sources.map(async (source) => ({
        sourceId: source.id,
        content: await this.remote.getSourceContent(bookId, source.id)
      }))
    );

    // Cover download is best-effort: a failure here (network hiccup, no
    // cover uploaded, etc.) must not abort the book download itself.
    let coverBlob: Blob | null = null;
    if (book.cover) {
      try {
        coverBlob = await this.remote.getBookCover(bookId);
      } catch (err) {
        coverBlob = null;
        console.warn(`Failed to download cover for book ${bookId}; continuing without it.`, err);
      }
    }

    const contentSize = new Blob([bookContent.content]).size;
    const sourcesSize = sourceContents.reduce(
      (total, { content }) => total + new Blob([content]).size,
      0
    );
    const coverSize = coverBlob ? coverBlob.size : 0;

    await this.cache.saveDownloadedBook({
      book,
      sources,
      downloaded_at: new Date().toISOString(),
      local_version: book.local_version,
      remote_version: book.remote_version,
      size_bytes: contentSize + sourcesSize + coverSize,
      size_breakdown: { content: contentSize, sources: sourcesSize, cover: coverSize }
    });
    await this.cache.saveCachedBookContent(bookId, bookContent);
    await Promise.all(
      sourceContents.map(({ sourceId, content }) =>
        this.cache.saveCachedSourceContent(bookId, sourceId, content)
      )
    );
    if (coverBlob) {
      await this.cache.saveCachedCover(bookId, coverBlob);
    }
  }

  async removeDownload(bookId: string): Promise<void> {
    const cachedUrl = this.coverUrlCache.get(bookId);
    if (cachedUrl) {
      URL.revokeObjectURL(cachedUrl);
      this.coverUrlCache.delete(bookId);
    }

    await this.cache.removeDownloadedBook(bookId);
  }

  async listDownloadedBookEntries(): Promise<DownloadedBookEntry[]> {
    const manifests = await this.cache.listDownloadedManifests();
    return Promise.all(
      manifests.map(async (manifest) => ({
        book: await this.applyOfflineCover({
          ...manifest.book,
          download_state: 'downloaded',
          downloaded_at: manifest.downloaded_at,
          local_version: manifest.local_version ?? manifest.book.local_version,
          remote_version: manifest.remote_version ?? manifest.book.remote_version
        }),
        sizeBytes: manifest.size_bytes ?? 0,
        downloadedAt: manifest.downloaded_at
      }))
    );
  }

  async getStorageEstimate(): Promise<StorageEstimateResult> {
    if (typeof navigator === 'undefined' || !navigator.storage?.estimate) {
      return { supported: false };
    }

    try {
      const estimate = await navigator.storage.estimate();
      return { supported: true, usage: estimate.usage, quota: estimate.quota };
    } catch {
      return { supported: false };
    }
  }

  openLocalBookFiles?(): Promise<string[] | null> {
    return this.remote.openLocalBookFiles?.() ?? Promise.resolve(null);
  }

  importBooksFromLocalPaths?(localPaths: string[], layerPath: string): Promise<DesktopImportBookResult[] | null> {
    return this.remote.importBooksFromLocalPaths?.(localPaths, layerPath) ?? Promise.resolve(null);
  }

  private async annotateDownloadState(book: Book): Promise<Book> {
    const cached = await this.cache.getCachedBook(book.id);
    if (!cached) {
      return this.applyOfflineCover({ ...book, download_state: 'not_downloaded' });
    }

    return this.applyOfflineCover({
      ...book,
      download_state: await this.cache.getDownloadState(book.id),
      downloaded_at: cached.downloaded_at,
      local_version: cached.local_version,
      remote_version: book.remote_version ?? cached.remote_version
    });
  }

  // Rewrites `cover_url` to a memoized object URL for a cached cover blob
  // when offline. UI components render covers via `book.cover_url` as an
  // <img src>, so this is the only place offline cover display needs to be
  // wired in; no component changes required. No-ops (and returns the book
  // unchanged) while online, since the remote cover URL already works.
  private async applyOfflineCover(book: Book): Promise<Book> {
    if (this.isOnline()) {
      return book;
    }

    let url = this.coverUrlCache.get(book.id);
    if (!url) {
      const blob = await this.cache.getCachedCover(book.id);
      if (!blob) {
        return book;
      }
      url = URL.createObjectURL(blob);
      this.coverUrlCache.set(book.id, url);
    }

    return { ...book, cover_url: url };
  }

  private filterBooks(books: Book[], search?: string): Book[] {
    const term = search?.trim().toLowerCase();
    if (!term) {
      return books;
    }

    return books.filter((book) => {
      const haystack = [book.title, ...book.authors, ...book.tags].join(' ').toLowerCase();
      return haystack.includes(term);
    });
  }
}
