import type { MobileBookCache, CachedBookManifest } from './mobileBookCache';
import { ShelfVisibility } from './shelfVisibility';
import { getShowNsfwOnDevice } from '@/composables/useDeviceNsfwPreference';
import type { Book, BookContent, DownloadState, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';

/**
 * A MobileBookCache that withholds what this device hides. The cache is the one
 * place a book is served without asking the backend, so wrapping it covers
 * every cache-first path. Guarded reads answer as a cache that never stored the
 * book would; writes pass through, and the wrapper is inert unless the backend
 * answers `filtersNsfwOnDevice`.
 */
export class VisibleMobileBookCache implements MobileBookCache {
  constructor(
    private readonly inner: MobileBookCache,
    /** Asked per call: both the setting and the backend change under this. */
    private readonly filtersOnDevice: () => boolean
  ) {}

  // --- guarded reads -------------------------------------------------------

  async listDownloadedBooks(): Promise<Book[]> {
    const books = await this.inner.listDownloadedBooks();
    return this.visibility()?.keepBooks(books).slice() ?? books;
  }

  async listDownloadedManifests(): Promise<CachedBookManifest[]> {
    const manifests = await this.inner.listDownloadedManifests();
    const visibility = this.visibility();
    return visibility ? manifests.filter((manifest) => visibility.allows(manifest.book)) : manifests;
  }

  async getCachedBook(bookId: string): Promise<Book | null> {
    const book = await this.inner.getCachedBook(bookId);
    return book && !this.allows(book) ? null : book;
  }

  /** A hidden book reports as never downloaded, so nothing offers to read it. */
  async getDownloadState(bookId: string): Promise<DownloadState> {
    return (await this.hidden(bookId)) ? 'not_downloaded' : this.inner.getDownloadState(bookId);
  }

  async getCachedBookContent(bookId: string): Promise<BookContent | null> {
    return (await this.hidden(bookId)) ? null : this.inner.getCachedBookContent(bookId);
  }

  async getCachedCover(bookId: string): Promise<Blob | null> {
    return (await this.hidden(bookId)) ? null : this.inner.getCachedCover(bookId);
  }

  async listCachedSources(bookId: string): Promise<SourceMeta[]> {
    return (await this.hidden(bookId)) ? [] : this.inner.listCachedSources(bookId);
  }

  async getCachedSource(bookId: string, sourceId: string): Promise<SourceMeta | null> {
    return (await this.hidden(bookId)) ? null : this.inner.getCachedSource(bookId, sourceId);
  }

  async getCachedSourceContent(bookId: string, sourceId: string): Promise<string | null> {
    return (await this.hidden(bookId)) ? null : this.inner.getCachedSourceContent(bookId, sourceId);
  }

  async getCachedAsset(bookId: string, sourceId: string, name: string): Promise<Blob | null> {
    return (await this.hidden(bookId)) ? null : this.inner.getCachedAsset(bookId, sourceId, name);
  }

  /** A book that cannot be opened has no reading position to restore. */
  async getReadProgress(bookId: string): Promise<ReadingProgress | null> {
    return (await this.hidden(bookId)) ? null : this.inner.getReadProgress(bookId);
  }

  // --- writes, unguarded ---------------------------------------------------

  saveDownloadedBook(manifest: CachedBookManifest): Promise<void> {
    return this.inner.saveDownloadedBook(manifest);
  }

  removeDownloadedBook(bookId: string): Promise<void> {
    return this.inner.removeDownloadedBook(bookId);
  }

  saveCachedBookContent(bookId: string, content: BookContent): Promise<void> {
    return this.inner.saveCachedBookContent(bookId, content);
  }

  saveCachedSourceContent(bookId: string, sourceId: string, content: string): Promise<void> {
    return this.inner.saveCachedSourceContent(bookId, sourceId, content);
  }

  saveReadProgress(bookId: string, progress: ReadingProgress): Promise<void> {
    return this.inner.saveReadProgress(bookId, progress);
  }

  saveCachedCover(bookId: string, blob: Blob): Promise<void> {
    return this.inner.saveCachedCover(bookId, blob);
  }

  deleteCachedCover(bookId: string): Promise<void> {
    return this.inner.deleteCachedCover(bookId);
  }

  saveCachedAsset(bookId: string, sourceId: string, name: string, blob: Blob): Promise<void> {
    return this.inner.saveCachedAsset(bookId, sourceId, name, blob);
  }

  // --- the question itself -------------------------------------------------

  /** The filter for this call, or null when nothing on this device filters. */
  private visibility(): ShelfVisibility | null {
    return this.filtersOnDevice() ? new ShelfVisibility({ showNsfw: getShowNsfwOnDevice() }) : null;
  }

  private allows(book: Book): boolean {
    return this.visibility()?.allows(book) ?? true;
  }

  /**
   * Costs one manifest read, from the same local cache the call was headed for.
   * A download taken before the mark existed carries neither half of it and
   * stays visible until it is fetched again; bumping the manifest version
   * instead would delete every existing download.
   */
  private async hidden(bookId: string): Promise<boolean> {
    const visibility = this.visibility();
    if (!visibility) {
      return false;
    }
    const book = await this.inner.getCachedBook(bookId);
    return book !== null && !visibility.allows(book);
  }
}
