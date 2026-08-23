import { Capacitor } from '@capacitor/core';
import { Directory, Encoding, Filesystem } from '@capacitor/filesystem';
import { unzipSync } from 'fflate';
import type {
  BookmarkPayload,
  Book,
  BookContent,
  DownloadState,
  PaginatedBooks,
  ReadingProgress,
  TrashedBook
} from '@/types/book';
import type { SourceMeta } from '@/types/source';
import type { FingerprintStatus, SimilarBookPair } from '@/api/books';
import { ApiError, getActiveShelfID } from '@/api/client';
import {
  addReadHistory as addLocalReadHistory,
  clearReadHistory as clearLocalReadHistory
} from '@/storage/readHistory';
import { referencedAssetNames } from '@/utils/markdownLineSyntax';
import { currentCacheScopeKey } from './cacheScope';
import { collectReadHistoryBooks } from './readHistoryBooks';
import type {
  BookshelfReader,
  DownloadedBookEntry,
  ListBooksOptions,
  ShelfRefreshResult,
  StorageEstimateResult
} from './bookshelfProvider';
import {
  downloadedBookFromManifest,
  InMemoryMobileBookCache,
  type MobileBookCache
} from './mobileBookCache';
import { InMemoryMobileCoverCache, type MobileCoverCache } from './mobileCoverCache';
import { ServerBookshelfProvider } from './serverBookshelfProvider';

export const OFFLINE_BOOK_CACHE_MISS_ERROR = 'Book is not downloaded and the app is offline';
export const OFFLINE_SOURCE_CACHE_MISS_ERROR = 'Source is not downloaded and the app is offline';
export const OFFLINE_DOWNLOAD_UNAVAILABLE_ERROR = 'Cannot download book while offline';
export const DOWNLOAD_SHELF_CHANGED_ERROR =
  'The shelf changed while the book was downloading; the download was discarded';

// How many illustrations are fetched and stored at a time. Small on purpose:
// the point of the limit is to bound peak memory on a phone, and a download is
// not a latency-sensitive operation.
const ASSET_DOWNLOAD_CONCURRENCY = 3;

// How many illustrations one bundle request fetches. The archive is held whole
// in memory and the images are uncompressed, so an unbounded bundle would hold
// the whole book's figures at once; chunking keeps at most this many resident
// while still collapsing dozens of per-image requests into a handful. A book
// with no more figures than this stays a single request.
const ASSET_BUNDLE_CHUNK_SIZE = 16;

// Sub-folder of the shared Documents directory that exported books land in, so
// they are grouped and never collide with an unrelated file already named the
// same at the Documents root. Shown to the user as part of the saved location.
const EXPORT_SUBDIR = 'PlainShelf';

const defaultIsOnline = (): boolean =>
  typeof navigator === 'undefined' ? true : navigator.onLine;

// The bundle arrives as one application/zip, so an unpacked figure's MIME is
// derived from its name the way the server's single-asset route derives it. The
// filesystem cache persists blob.type, so this keeps a cached figure's type
// identical whether the per-file or the bundle path stored it.
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

// True when a remote call never received an HTTP response at all — a
// transport-level failure (device is online per navigator.onLine, but the
// self-hosted server is unreachable, e.g. LTE away from the home LAN) or a
// request timeout — as opposed to the server replying with an error status
// (a real ApiError with a status, which must be surfaced, not swallowed).
function isServerUnreachableError(err: unknown): boolean {
  return err instanceof ApiError ? err.isTimeout : true;
}

export class MobileBookshelfProvider implements BookshelfReader {
  // Memoized object URLs for cached cover blobs, keyed by (server, shelf) and
  // book id. Created lazily in applyCachedCover and revoked in removeDownload;
  // the provider is a long-lived singleton so this map's lifetime matches the
  // app's, which is fine given the small number of downloaded books.
  //
  // The scope belongs in the key for the same reason the filesystem cache puts
  // it in the path (see cacheScope.ts): keyed by book id alone, a cover
  // memoized while connected to one shelf would be handed back for a different
  // book with the same id on another, and the memo would short-circuit the
  // scoped filesystem lookup that would otherwise have corrected it.
  private readonly coverUrlCache = new Map<string, string>();

  // Present only in a native shell. A browser preview (mobile-shell-preview, the
  // e2e provider) leaves it undefined so useBookActions.downloadBook falls back
  // to its blob `<a download>` path, which works in a real browser but is
  // silently ignored by the Android WebView — the reason "匯出檔案" did nothing
  // in the app. Gated at construction because native-ness is fixed for the page
  // lifetime; see runtime.ts.
  saveBookContentToFile?: (bookId: string, suggestedName: string) => Promise<string>;

  constructor(
    private readonly remote: BookshelfReader = new ServerBookshelfProvider(),
    private readonly cache: MobileBookCache = new InMemoryMobileBookCache(),
    private readonly isOnline: () => boolean = defaultIsOnline,
    private readonly coverCache: MobileCoverCache = new InMemoryMobileCoverCache()
  ) {
    if (Capacitor.isNativePlatform()) {
      this.saveBookContentToFile = (bookId, suggestedName) =>
        this.exportBookContentToDocuments(bookId, suggestedName);
    }
  }

  /**
   * Writes a book's content into the shared Documents folder so the user can
   * reach it from the Files app or hand it to another app, and returns the
   * saved location for a confirmation toast.
   *
   * `suggestedName` is already filesystem-safe (useBookActions sanitizes it),
   * and Documents needs no runtime permission for a file the app itself creates
   * on modern Android. Text is written as UTF-8 directly rather than base64,
   * since an exported source is plain text/Markdown. An existing file of the
   * same name is overwritten, matching a browser download into a folder that
   * already holds one.
   */
  private async exportBookContentToDocuments(
    bookId: string,
    suggestedName: string
  ): Promise<string> {
    const blob = await this.downloadBookContent(bookId);
    const relativePath = `${EXPORT_SUBDIR}/${suggestedName}`;
    await Filesystem.writeFile({
      path: relativePath,
      data: await blob.text(),
      directory: Directory.Documents,
      encoding: Encoding.UTF8,
      recursive: true
    });
    return `Documents/${relativePath}`;
  }

  // `options` only affects the remote call; the offline cache stores whatever
  // the plain /books response carried, so opt-in fields such as char_count are
  // absent from cached results.
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

  // Delegated without an offline branch: the folder store surfaces a failure in
  // the sidebar, which is what the server path already does when unreachable.
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

  // Reading history lives on the device (see storage/readHistory), so it is
  // recorded and cleared regardless of connectivity. Listing goes through this
  // provider's own listBooks, which falls back to the offline cache, so the
  // history page still shows downloaded books when the server is unreachable.
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

  /**
   * Always, on this shell: the WebView issues an `<img src="http://...">`
   * itself rather than through CapacitorHttp, so the request is not the one
   * the native bridge would have made. Going through getBookCover() also
   * reuses the offline cover cache for a downloaded book.
   */
  coversMustBeFetchedAsBlob(): boolean {
    return true;
  }

  /** Whatever this shell is pointed at answers; a wrapper has no mode of its own. */
  supportsServerMode(): boolean {
    return this.remote.supportsServerMode?.() ?? true;
  }

  /** Likewise: the cost belongs to whatever backend this shell is pointed at. */
  supportsCharCountListing(): boolean {
    return this.remote.supportsCharCountListing?.() ?? true;
  }


  getDuplicateBookGroups(): Promise<string[][]> {
    return this.remote.getDuplicateBookGroups();
  }

  getSimilarBookPairs(floor?: number): Promise<SimilarBookPair[]> {
    return this.remote.getSimilarBookPairs(floor);
  }

  getFingerprintStatus(): Promise<FingerprintStatus> {
    return this.remote.getFingerprintStatus();
  }

  listTrashedBooks(): Promise<TrashedBook[]> {
    return this.remote.listTrashedBooks();
  }


  // Shelf refresh is entirely the wrapped backend's business — nothing here
  // caches the listing itself. Reported as unsupported unless the backend says
  // otherwise, and both backends do: pCloud because its stored listing is only
  // ever as fresh as the last update, a server because a book added to the
  // shelf from outside waits out `scan_interval` otherwise.
  supportsShelfRefresh(): boolean {
    return Boolean(this.remote.supportsShelfRefresh?.());
  }

  // Whatever the backend found is passed straight through: it is the only thing
  // that can say, and dropping it would leave the button with nothing to report.
  async refreshShelf(): Promise<ShelfRefreshResult | void> {
    return this.remote.refreshShelf?.();
  }

  async getShelfFetchedAt(): Promise<number | null> {
    return (await this.remote.getShelfFetchedAt?.()) ?? null;
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

  /**
   * The cache answers first, so a downloaded book shows its illustrations
   * without a request even when the server is reachable.
   *
   * An asset the download did not store — a book downloaded before assets were
   * cached, or one whose text changed since — still resolves online. Offline it
   * fails, and the reader shows the alt text rather than losing the chapter.
   */
  async getSourceAsset(bookId: string, sourceId: string, name: string): Promise<Blob> {
    const cached = await this.cache.getCachedAsset(bookId, sourceId, name);
    if (cached) {
      return cached;
    }

    if (this.isOnline() && this.remote.getSourceAsset) {
      return this.remote.getSourceAsset(bookId, sourceId, name);
    }

    throw new Error(OFFLINE_SOURCE_CACHE_MISS_ERROR);
  }


  /**
   * The shelf chosen during connection setup, applied to the API client at
   * bootstrap by initMobileConfig(). Read live rather than captured, for the
   * same reason currentCacheScopeKey() is: the settings UI repoints this
   * process-wide state in place.
   */
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

    // Everything below reads the connected shelf at the moment it runs: the
    // remote calls resolve it per request, and each cache write resolves the
    // scope it lands in. A shelf change while the fetches are in flight would
    // therefore file this book under a shelf it does not belong to, possibly
    // overwriting that shelf's own book of the same id, or split one download
    // across two scopes. Abandon the download instead — the user has left the
    // shelf they asked for it on, and a partial write is worse than none.
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

    if (currentCacheScopeKey() !== scopeAtStart) {
      throw new Error(DOWNLOAD_SHELF_CHANGED_ERROR);
    }

    // Illustrations are found by reading the text that was just downloaded
    // rather than by listing the shelf: the reader only requests what its
    // Markdown renders, so parsing the same text stores exactly that set and
    // skips files no longer referenced. It also needs no listing endpoint and
    // no index on disk, which keeps the shelf the only record of what exists.
    //
    // Only a Markdown book can render an image, so a plain-text one is not
    // scanned at all.
    //
    // This runs before the manifest is written, so a failure leaves files under
    // a book directory carrying no manifest - an orphan this cache already
    // ignores - rather than a book listed as downloaded without its pictures.
    const currentSource = sources.find((source) => source.id === book.current_source);
    const effectiveFormat = currentSource?.format ?? book.format ?? 'txt';
    const currentSourceContent = book.current_source
      ? sourceContents.filter(({ sourceId }) => sourceId === book.current_source)
      : sourceContents;
    const assetsSize = effectiveFormat === 'md'
      ? await this.storeSourceAssets(bookId, currentSourceContent)
      : 0;

    // The asset phase did network I/O of its own, so the window the check above
    // closed has reopened.
    if (currentCacheScopeKey() !== scopeAtStart) {
      throw new Error(DOWNLOAD_SHELF_CHANGED_ERROR);
    }

    const contentSize = new Blob([bookContent.content]).size;
    const sourcesSize = sourceContents.reduce(
      (total, { content }) => total + new Blob([content]).size,
      0
    );
    const coverSize = coverBlob ? coverBlob.size : 0;

    // The manifest is the download's commit point, so it is written *last* —
    // after the content, source files and cover it accounts for are all on
    // disk. getDownloadState reports "downloaded" from the manifest alone, and
    // the mobile shell now gates reading on that, so "manifest present" must
    // mean "content present": a book that streams on-demand when online would
    // otherwise pass the gate mid-write and then fail once offline. A failure
    // before this last write leaves manifest-less files the cache already
    // ignores as an orphan (same as the asset phase above), not a book listed
    // as downloaded without its text.
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

  /**
   * Fetches the illustrations the downloaded text references and stores each.
   *
   * Prefers the bundle path (one request per source) when the backend offers
   * it, falling back to the per-file concurrent path when it does not or a
   * bundle request fails. Either way each figure is best-effort — one that will
   * not download or store is left for the reader's alt text — and the whole
   * download never fails over a picture.
   *
   * Returns the bytes actually stored, for the manifest's size breakdown.
   */
  private async storeSourceAssets(
    bookId: string,
    sourceContents: { sourceId: string; content: string }[]
  ): Promise<number> {
    const fetchBundle = this.remote.getSourceAssetsBundle?.bind(this.remote);
    if (fetchBundle) {
      try {
        return await this.storeSourceAssetsViaBundle(bookId, sourceContents, fetchBundle);
      } catch (err) {
        // A bundle request that failed (network, HTTP error, or a malformed
        // archive) drops to the per-file path below. Anything the bundle
        // already stored is simply overwritten there, which recomputes the
        // total from scratch, so no figure is double-counted.
        console.warn(
          `Asset bundle download failed for book ${bookId}; falling back to per-file fetches.`,
          err
        );
      }
    }

    const fetchAsset = this.remote.getSourceAsset?.bind(this.remote);
    if (!fetchAsset) {
      return 0;
    }
    return this.storeSourceAssetsPerFile(bookId, sourceContents, fetchAsset);
  }

  /**
   * Stores each referenced figure by fetching it on its own, a bounded number
   * at a time. The original download path, kept as the fallback for a backend
   * with no bundle endpoint.
   */
  private async storeSourceAssetsPerFile(
    bookId: string,
    sourceContents: { sourceId: string; content: string }[],
    fetchAsset: (bookId: string, sourceId: string, name: string) => Promise<Blob>
  ): Promise<number> {
    const pending = sourceContents.flatMap(({ sourceId, content }) =>
      referencedAssetNames(content).map((name) => ({ sourceId, name }))
    );

    let stored = 0;
    let next = 0;

    const worker = async (): Promise<void> => {
      while (next < pending.length) {
        const { sourceId, name } = pending[next];
        next += 1;
        try {
          const blob = await fetchAsset(bookId, sourceId, name);
          await this.cache.saveCachedAsset(bookId, sourceId, name, blob);
          stored += blob.size;
        } catch (err) {
          console.warn(`Failed to store illustration ${name} for book ${bookId}.`, err);
        }
      }
    };

    await Promise.all(
      Array.from({ length: Math.min(ASSET_DOWNLOAD_CONCURRENCY, pending.length) }, worker)
    );

    return stored;
  }

  /**
   * Stores each referenced figure out of per-source zips, requested in chunks of
   * ASSET_BUNDLE_CHUNK_SIZE so at most one chunk's archive is resident at a time.
   * A chunk fetch/parse failure throws, dropping storeSourceAssets to the
   * per-file fallback; an absent or unstorable figure is skipped, best-effort.
   */
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

  /**
   * Fetches one chunk of a source's figures as a single archive and stores each,
   * decoding one entry at a time. Returns the bytes stored from this chunk.
   */
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
      // A malformed archive throws here (→ per-file fallback); a filter matching
      // nothing yields undefined — a referenced-but-absent figure, shown as alt
      // text.
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
    // Newline cannot occur in a scope key (it is a URL joined to a shelf id),
    // so it cannot be confused with the separator.
    return `${currentCacheScopeKey()}\n${bookId}`;
  }

  // Revokes and forgets this book's memoized cover object URL, if any, for the
  // connected shelf. Does not touch the cached cover blob itself — callers that
  // also want the blob gone must call cache.deleteCachedCover separately (see
  // evictCachedCover below). Safe to call for a book with no cached URL.
  private invalidateCoverUrl(bookId: string): void {
    const key = this.coverUrlKey(bookId);
    const cachedUrl = this.coverUrlCache.get(key);
    if (cachedUrl) {
      URL.revokeObjectURL(cachedUrl);
      this.coverUrlCache.delete(key);
    }
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
