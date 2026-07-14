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
import { ApiError } from '../api/client';
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

// True when a remote call never received an HTTP response at all — a
// transport-level failure (device is online per navigator.onLine, but the
// self-hosted server is unreachable, e.g. LTE away from the home LAN) or a
// request timeout — as opposed to the server replying with an error status
// (a real ApiError with a status, which must be surfaced, not swallowed).
function isServerUnreachableError(err: unknown): boolean {
  return err instanceof ApiError ? err.isTimeout : true;
}

export class MobileBookshelfProvider implements BookshelfProvider {
  // Memoized per-book object URLs for cached cover blobs, keyed by book id.
  // Created lazily in applyCachedCover and revoked in removeDownload; the
  // provider is a long-lived singleton so this map's lifetime matches the
  // app's, which is fine given the small number of downloaded books.
  private readonly coverUrlCache = new Map<string, string>();

  constructor(
    private readonly remote: BookshelfProvider = new ServerBookshelfProvider(),
    private readonly cache: MobileBookCache = new InMemoryMobileBookCache(),
    private readonly isOnline: () => boolean = defaultIsOnline
  ) {}

  async listBooks(page = 1, pageSize = 20): Promise<PaginatedBooks> {
    if (this.isOnline()) {
      let remoteBooks: PaginatedBooks;
      try {
        remoteBooks = await this.remote.listBooks(page, pageSize);
      } catch (err) {
        if (!isServerUnreachableError(err)) {
          throw err;
        }
        return this.listBooksFromCache(page, pageSize);
      }
      const items = await Promise.all(remoteBooks.items.map((book) => this.annotateDownloadState(book)));
      return { ...remoteBooks, items };
    }

    return this.listBooksFromCache(page, pageSize);
  }

  private async listBooksFromCache(page: number, pageSize: number): Promise<PaginatedBooks> {
    const downloaded = await this.cache.listDownloadedBooks();
    const start = Math.max(0, (page - 1) * pageSize);
    const pageItems = downloaded.slice(start, start + pageSize);
    const items = await Promise.all(pageItems.map((book) => this.applyCachedCover(book)));

    return {
      items,
      total: downloaded.length,
      page,
      pageSize
    };
  }

  async getBook(bookId: string): Promise<Book> {
    if (this.isOnline()) {
      let remoteBook: Book;
      try {
        remoteBook = await this.remote.getBook(bookId);
      } catch (err) {
        if (!isServerUnreachableError(err)) {
          throw err;
        }
        const cached = await this.cache.getCachedBook(bookId);
        if (cached) {
          return this.applyCachedCover(cached);
        }
        throw err;
      }
      return this.annotateDownloadState(remoteBook);
    }

    const cached = await this.cache.getCachedBook(bookId);
    if (cached) {
      return this.applyCachedCover(cached);
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
      try {
        return await this.remote.getReadProgress(bookId);
      } catch (err) {
        if (!isServerUnreachableError(err)) {
          throw err;
        }
        return { char_offset: 0 };
      }
    }

    return { char_offset: 0 };
  }

  saveReadProgress(bookId: string, progress: BookmarkPayload): Promise<void> {
    return this.cache.saveReadProgress(bookId, progress);
  }

  async addReadHistory(bookId: string): Promise<void> {
    if (!this.isOnline()) {
      return;
    }

    try {
      await this.remote.addReadHistory(bookId);
    } catch (err) {
      if (!isServerUnreachableError(err)) {
        throw err;
      }
    }
  }

  listReadHistoryBooks(): Promise<Book[]> {
    return this.remote.listReadHistoryBooks();
  }

  clearReadHistory(): Promise<void> {
    return this.remote.clearReadHistory();
  }

  // No local cache for reading activity: mobile only ever shows what the
  // server knows about. Offline / server-unreachable just means "no data to
  // show yet" rather than an error, matching addReadHistory's tolerance below.
  async getReadingActivity(from: string, to: string): Promise<Record<string, number>> {
    if (!this.isOnline()) {
      return {};
    }

    try {
      return await this.remote.getReadingActivity(from, to);
    } catch (err) {
      if (!isServerUnreachableError(err)) {
        throw err;
      }
      return {};
    }
  }

  async reportReadingActivity(bookId: string, seconds: number, date: string): Promise<void> {
    if (!this.isOnline()) {
      return;
    }

    try {
      await this.remote.reportReadingActivity(bookId, seconds, date);
    } catch (err) {
      if (!isServerUnreachableError(err)) {
        throw err;
      }
    }
  }

  importBook(payload: BookCreateRequest): Promise<Book> {
    return this.remote.importBook(payload);
  }

  async uploadBookCover(bookId: string, file: File): Promise<void> {
    await this.remote.uploadBookCover(bookId, file);
    await this.refreshCachedCover(bookId);
  }

  async uploadBookCoverBlob(bookId: string, blob: Blob): Promise<void> {
    await this.remote.uploadBookCoverBlob(bookId, blob);
    await this.refreshCachedCover(bookId);
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

  async deleteBookCover(bookId: string): Promise<void> {
    await this.remote.deleteBookCover(bookId);
    await this.evictCachedCover(bookId);
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
      try {
        return await this.remote.listSources(bookId);
      } catch (err) {
        if (!isServerUnreachableError(err)) {
          throw err;
        }
        return this.cache.listCachedSources(bookId);
      }
    }

    return this.cache.listCachedSources(bookId);
  }

  async getSource(bookId: string, sourceId: string): Promise<SourceMeta> {
    if (this.isOnline()) {
      try {
        return await this.remote.getSource(bookId, sourceId);
      } catch (err) {
        if (!isServerUnreachableError(err)) {
          throw err;
        }
        const cached = await this.cache.getCachedSource(bookId, sourceId);
        if (cached) {
          return cached;
        }
        throw err;
      }
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
    this.invalidateCoverUrl(bookId);
    await this.cache.removeDownloadedBook(bookId);
  }

  async listDownloadedBookEntries(): Promise<DownloadedBookEntry[]> {
    const manifests = await this.cache.listDownloadedManifests();
    return Promise.all(
      manifests.map(async (manifest) => ({
        book: await this.applyCachedCover({
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
      return { ...book, download_state: 'not_downloaded' };
    }

    return {
      ...book,
      download_state: await this.cache.getDownloadState(book.id),
      downloaded_at: cached.downloaded_at,
      local_version: cached.local_version,
      remote_version: book.remote_version ?? cached.remote_version
    };
  }

  // Revokes and forgets this book's memoized cover object URL, if any. Does
  // not touch the cached cover blob itself — callers that also want the
  // blob gone must call cache.deleteCachedCover separately (see
  // evictCachedCover below). Safe to call for a book with no cached URL.
  private invalidateCoverUrl(bookId: string): void {
    const cachedUrl = this.coverUrlCache.get(bookId);
    if (cachedUrl) {
      URL.revokeObjectURL(cachedUrl);
      this.coverUrlCache.delete(bookId);
    }
  }

  // Re-fetches the authoritative cover from remote and overwrites the local
  // cache after a successful remote cover write. re-fetching (rather than
  // caching the blob the caller already has in hand) matters because the
  // server may transcode the upload (cover_to_jpg), so the bytes stored
  // server-side can differ from what was uploaded.
  //
  // Gated on the book actually being downloaded: for a book with no cache
  // entry, saving a cover here would create an orphan cover-only cache
  // entry that removeDownloadedBook never cleans up (it keys off the
  // manifest, which would not exist for this book).
  //
  // Best-effort: the remote write already succeeded by the time this runs,
  // so a failure here (network hiccup, offline, etc.) must not surface as
  // an error to the caller — it just means the local cache falls back to
  // remote fetches for this cover until the next successful refresh. On
  // failure the stale cached cover is dropped rather than left inconsistent
  // with the just-updated remote cover.
  private async refreshCachedCover(bookId: string): Promise<void> {
    const state = await this.cache.getDownloadState(bookId);
    if (state !== 'downloaded') {
      return;
    }

    this.invalidateCoverUrl(bookId);
    try {
      const blob = await this.remote.getBookCover(bookId);
      await this.cache.saveCachedCover(bookId, blob);
    } catch (err) {
      console.warn(`Failed to refresh cached cover for book ${bookId}; dropping stale cache entry.`, err);
      await this.cache.deleteCachedCover(bookId);
    }
  }

  // Drops any cached cover (blob + memoized object URL) after a successful
  // remote cover delete. Unlike refreshCachedCover this is not gated on
  // download state — it is idempotent and there is nothing to preserve.
  private async evictCachedCover(bookId: string): Promise<void> {
    this.invalidateCoverUrl(bookId);
    await this.cache.deleteCachedCover(bookId);
  }

  // Rewrites `cover_url` to a memoized object URL for a cached cover blob
  // when one is available locally. Used for offline reads and for the
  // network-online-but-server-unreachable fallback paths, where the remote
  // cover URL would fail to load anyway. UI components render covers via
  // `book.cover_url` as an <img src>, so this is the only place cached
  // cover display needs to be wired in; no component changes required.
  // No-ops (returns the book unchanged) when there is no cached cover blob
  // for this book, leaving whatever cover_url the caller already set.
  private async applyCachedCover(book: Book): Promise<Book> {
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
}
