import { unzipSync } from 'fflate';
import type {
  BookmarkPayload,
  Book,
  BookContent,
  DownloadState,
  PaginatedBooks,
  ReadingProgress,
  TrashedBook,
  TrashedBookListing
} from '@/types/book';
import type { SourceMeta } from '@/types/source';
import type { FingerprintStatus, SimilarBookPair } from '@/api/books';
import { ApiError, getActiveShelfID } from '@/api/client';
import {
  addReadHistory as addLocalReadHistory,
  clearReadHistory as clearLocalReadHistory
} from '@/storage/readHistory';
import { mapWithConcurrency } from '@/utils/concurrency';
import { referencedAssetNames } from '@/utils/markdownLineSyntax';
import { currentCacheScopeKey } from './cacheScope';
import { collectReadHistoryBooks } from './readHistoryBooks';
import { VisibleMobileBookCache } from './visibleMobileBookCache';
import type {
  BookshelfReader,
  DownloadedBookEntry,
  ListBooksOptions,
  ShelfRefreshResult
} from './bookshelfProvider';
import {
  downloadedBookFromManifest,
  InMemoryMobileBookCache,
  type MobileBookCache
} from './mobileBookCache';
import { InMemoryMobileCoverCache, type MobileCoverCache } from './mobileCoverCache';
import { ServerBookshelfProvider } from './serverBookshelfProvider';

const OFFLINE_BOOK_CACHE_MISS_ERROR = 'Book is not downloaded and the app is offline';
const OFFLINE_SOURCE_CACHE_MISS_ERROR = 'Source is not downloaded and the app is offline';
const OFFLINE_DOWNLOAD_UNAVAILABLE_ERROR = 'Cannot download book while offline';
export const DOWNLOAD_SHELF_CHANGED_ERROR =
  'The shelf changed while the book was downloading; the download was discarded';

const ASSET_DOWNLOAD_CONCURRENCY = 3;

const ASSET_BUNDLE_CHUNK_SIZE = 16;

const defaultIsOnline = (): boolean =>
  typeof navigator === 'undefined' ? true : navigator.onLine;

function imageMimeType(name: string): string {
  const dot = name.lastIndexOf('.');
  switch (dot === -1 ? '' : name.slice(dot).toLowerCase()) {
    case '.png':
      return 'image/png';
    case '.webp':
      return 'image/webp';
    case '.gif':
      return 'image/gif';
    case '.jpg':
    case '.jpeg':
      return 'image/jpeg';
    default:
      return '';
  }
}

const SOURCE_UNREACHABLE_CODES = new Set(['SOURCE_UNAVAILABLE', 'BOOK_NOT_DOWNLOADED']);

function isServerUnreachableError(err: unknown): boolean {
  if (!(err instanceof ApiError)) {
    return true;
  }
  return err.isTimeout || SOURCE_UNREACHABLE_CODES.has(err.code ?? '');
}

export class MobileBookshelfProvider implements BookshelfReader {
  private readonly coverUrlCache = new Map<string, string>();

  private readonly cache: MobileBookCache;

  constructor(
    private readonly remote: BookshelfReader = new ServerBookshelfProvider(),
    cache: MobileBookCache = new InMemoryMobileBookCache(),
    private readonly isOnline: () => boolean = defaultIsOnline,
    private readonly coverCache: MobileCoverCache = new InMemoryMobileCoverCache()
  ) {
    this.cache = new VisibleMobileBookCache(
      cache,
      () => this.filtersNsfwOnDevice(),
      () => this.remote.localNsfwMarks?.() ?? Promise.resolve(null)
    );
  }

  async listBooks(page = 1, pageSize = 20, options?: ListBooksOptions): Promise<PaginatedBooks> {
    if (this.isOnline()) {
      let remoteBooks: PaginatedBooks;
      try {
        remoteBooks = await this.remote.listBooks(page, pageSize, options);
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

  listFolderMeta() {
    return this.remote.listFolderMeta?.() ?? Promise.resolve({icons: [], folders: {}});
  }

  listFolders(): Promise<string[]> {
    return this.remote.listFolders();
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

  addReadHistory(bookId: string): Promise<void> {
    return addLocalReadHistory(bookId);
  }

  listReadHistoryBooks(): Promise<Book[]> {
    return collectReadHistoryBooks((page, pageSize) => this.listBooks(page, pageSize));
  }

  clearReadHistory(): Promise<void> {
    return clearLocalReadHistory();
  }

  async getBookCover(bookId: string): Promise<Blob> {
    const cached = await this.cache.getCachedCover(bookId);
    if (cached) {
      return cached;
    }

    const persisted = await this.coverCache.getCover(bookId);
    if (persisted) {
      return persisted;
    }

    const blob = await this.remote.getBookCover(bookId);
    this.coverCache.setCover(bookId, blob).catch((err) => {
      console.warn(`Failed to persist cover cache for book ${bookId}.`, err);
    });
    return blob;
  }

  getBookCoverUrl(bookId: string, cacheKey?: number): string {
    return this.remote.getBookCoverUrl(bookId, cacheKey);
  }

  filtersNsfwOnDevice(): boolean {
    return this.remote.filtersNsfwOnDevice?.() ?? false;
  }

  getDuplicateBookGroups(): Promise<string[][]> {
    return this.remote.getDuplicateBookGroups();
  }

  getSimilarBookPairs(floor?: number, confirm?: boolean): Promise<SimilarBookPair[]> {
    return this.remote.getSimilarBookPairs(floor, confirm);
  }

  getFingerprintStatus(): Promise<FingerprintStatus> {
    return this.remote.getFingerprintStatus();
  }

  listTrashedBooks(): Promise<TrashedBookListing> {
    return this.remote.listTrashedBooks();
  }

  async refreshShelf(): Promise<ShelfRefreshResult | void> {
    return this.remote.refreshShelf?.();
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

  async getSourceAsset(bookId: string, sourceId: string, name: string): Promise<Blob> {
    const cached = await this.cache.getCachedAsset(bookId, sourceId, name);
    if (cached) {
      return cached;
    }

    if (this.isOnline()) {
      return this.remote.getSourceAsset(bookId, sourceId, name);
    }

    throw new Error(OFFLINE_SOURCE_CACHE_MISS_ERROR);
  }

  async getSourceAssetsBundle(bookId: string, sourceId: string, names: string[]): Promise<Blob> {
    if (this.isOnline()) {
      return this.remote.getSourceAssetsBundle(bookId, sourceId, names);
    }

    throw new Error(OFFLINE_SOURCE_CACHE_MISS_ERROR);
  }

  getPersistedShelfID(): string {
    return getActiveShelfID();
  }

  async getDownloadState(bookId: string): Promise<DownloadState> {
    return this.cache.getDownloadState(bookId);
  }

  async downloadBook(bookId: string): Promise<void> {
    if (!this.isOnline()) {
      throw new Error(OFFLINE_DOWNLOAD_UNAVAILABLE_ERROR);
    }

    const scopeAtStart = currentCacheScopeKey();

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

    let coverBlob: Blob | null = null;
    if (book.cover) {
      try {
        coverBlob = await this.remote.getBookCover(bookId);
      } catch (err) {
        coverBlob = null;
        console.warn(`Failed to download cover for book ${bookId}; continuing without it.`, err);
      }
    }

    if (currentCacheScopeKey() !== scopeAtStart) {
      throw new Error(DOWNLOAD_SHELF_CHANGED_ERROR);
    }

    const currentSource = sources.find((source) => source.id === book.current_source);
    const effectiveFormat = currentSource?.format ?? book.format ?? 'txt';
    const currentSourceContent = book.current_source
      ? sourceContents.filter(({ sourceId }) => sourceId === book.current_source)
      : sourceContents;
    const assetsSize = effectiveFormat === 'md'
      ? await this.storeSourceAssets(bookId, currentSourceContent)
      : 0;

    if (currentCacheScopeKey() !== scopeAtStart) {
      throw new Error(DOWNLOAD_SHELF_CHANGED_ERROR);
    }

    const contentSize = new Blob([bookContent.content]).size;
    const sourcesSize = sourceContents.reduce(
      (total, { content }) => total + new Blob([content]).size,
      0
    );
    const coverSize = coverBlob ? coverBlob.size : 0;

    await this.cache.saveCachedBookContent(bookId, bookContent);
    await Promise.all(
      sourceContents.map(({ sourceId, content }) =>
        this.cache.saveCachedSourceContent(bookId, sourceId, content)
      )
    );
    if (coverBlob) {
      await this.cache.saveCachedCover(bookId, coverBlob);
    }
    await this.cache.saveDownloadedBook({
      book,
      sources,
      downloaded_at: new Date().toISOString(),
      local_version: book.local_version,
      remote_version: book.remote_version,
      size_bytes: contentSize + sourcesSize + coverSize + assetsSize,
      size_breakdown: {
        content: contentSize,
        sources: sourcesSize,
        cover: coverSize,
        assets: assetsSize
      }
    });
  }

  private async storeSourceAssets(
    bookId: string,
    sourceContents: { sourceId: string; content: string }[]
  ): Promise<number> {
    try {
      return await this.storeSourceAssetsViaBundle(
        bookId,
        sourceContents,
        this.remote.getSourceAssetsBundle.bind(this.remote)
      );
    } catch (err) {
      console.warn(
        `Asset bundle download failed for book ${bookId}; falling back to per-file fetches.`,
        err
      );
    }

    return this.storeSourceAssetsPerFile(
      bookId,
      sourceContents,
      this.remote.getSourceAsset.bind(this.remote)
    );
  }

  private async storeSourceAssetsPerFile(
    bookId: string,
    sourceContents: { sourceId: string; content: string }[],
    fetchAsset: (bookId: string, sourceId: string, name: string) => Promise<Blob>
  ): Promise<number> {
    const pending = sourceContents.flatMap(({ sourceId, content }) =>
      referencedAssetNames(content).map((name) => ({ sourceId, name }))
    );

    let stored = 0;
    await mapWithConcurrency(pending, ASSET_DOWNLOAD_CONCURRENCY, async ({ sourceId, name }) => {
      try {
        const blob = await fetchAsset(bookId, sourceId, name);
        await this.cache.saveCachedAsset(bookId, sourceId, name, blob);
        stored += blob.size;
      } catch (err) {
        console.warn(`Failed to store illustration ${name} for book ${bookId}.`, err);
      }
    });

    return stored;
  }

  private async storeSourceAssetsViaBundle(
    bookId: string,
    sourceContents: { sourceId: string; content: string }[],
    fetchBundle: (bookId: string, sourceId: string, names: string[]) => Promise<Blob>
  ): Promise<number> {
    let stored = 0;

    for (const { sourceId, content } of sourceContents) {
      const names = referencedAssetNames(content);
      for (let offset = 0; offset < names.length; offset += ASSET_BUNDLE_CHUNK_SIZE) {
        const chunk = names.slice(offset, offset + ASSET_BUNDLE_CHUNK_SIZE);
        stored += await this.storeAssetChunkViaBundle(bookId, sourceId, chunk, fetchBundle);
      }
    }

    return stored;
  }

  private async storeAssetChunkViaBundle(
    bookId: string,
    sourceId: string,
    names: string[],
    fetchBundle: (bookId: string, sourceId: string, names: string[]) => Promise<Blob>
  ): Promise<number> {
    const zipBlob = await fetchBundle(bookId, sourceId, names);
    const archive = new Uint8Array(await zipBlob.arrayBuffer());

    let stored = 0;
    for (const name of names) {
      const bytes = unzipSync(archive, { filter: (file) => file.name === name })[name];
      if (!bytes) {
        continue;
      }
      try {
        const blob = new Blob([bytes], { type: imageMimeType(name) });
        await this.cache.saveCachedAsset(bookId, sourceId, name, blob);
        stored += blob.size;
      } catch (err) {
        console.warn(`Failed to store illustration ${name} for book ${bookId}.`, err);
      }
    }

    return stored;
  }

  async removeDownload(bookId: string): Promise<void> {
    this.invalidateCoverUrl(bookId);
    await this.cache.removeDownloadedBook(bookId);
  }

  async listDownloadedBookEntries(): Promise<DownloadedBookEntry[]> {
    const manifests = await this.cache.listDownloadedManifests();
    return Promise.all(
      manifests.map(async (manifest) => ({
        book: await this.applyCachedCover(downloadedBookFromManifest(manifest)),
        sizeBytes: manifest.size_bytes ?? 0,
        downloadedAt: manifest.downloaded_at
      }))
    );
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

  private coverUrlKey(bookId: string): string {
    return `${currentCacheScopeKey()}\n${bookId}`;
  }

  private invalidateCoverUrl(bookId: string): void {
    const key = this.coverUrlKey(bookId);
    const cachedUrl = this.coverUrlCache.get(key);
    if (cachedUrl) {
      URL.revokeObjectURL(cachedUrl);
      this.coverUrlCache.delete(key);
    }
  }

  private async applyCachedCover(book: Book): Promise<Book> {
    const key = this.coverUrlKey(book.id);
    let url = this.coverUrlCache.get(key);
    if (!url) {
      const blob = await this.cache.getCachedCover(book.id);
      if (!blob) {
        return book;
      }
      url = URL.createObjectURL(blob);
      this.coverUrlCache.set(key, url);
    }

    return { ...book, cover_url: url };
  }
}
