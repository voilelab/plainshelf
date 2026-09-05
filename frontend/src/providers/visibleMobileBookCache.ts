import type { MobileBookCache, CachedBookManifest } from './mobileBookCache';
import { ShelfVisibility } from './shelfVisibility';
import { getShowNsfwOnDevice } from '@/composables/useDeviceNsfwPreference';
import type { Book, BookContent, DownloadState, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';

/**
 * A MobileBookCache that withholds what this device has been told to hide.
 *
 * The offline cache is the one place a book can be served without the backend
 * being asked, so it is the one place the pCloud provider's own filter cannot
 * reach: `MobileBookshelfProvider` answers content, covers, sources and
 * illustrations from the cache *before* it checks connectivity, and falls back
 * to the cache for the listing and the single book whenever the backend is
 * unreachable. Already having downloaded a book is not a way past the setting —
 * otherwise "hidden on this phone" would hold everywhere except offline, which
 * is where a phone spends much of its time.
 *
 * Wrapping the cache rather than guarding each of those call sites is
 * deliberate, and the same decision `bookVisibility` embodies on the server: a
 * cache-first path added later is covered by construction instead of by
 * whoever adds it remembering. Every read below therefore answers exactly as a
 * cache that never stored the book would — a miss, an empty list,
 * `not_downloaded` — so no caller learns the difference between hidden and
 * absent. Writes pass straight through: they are the user downloading a book
 * that was visible when they asked for it, and hiding must not silently drop
 * what is already stored.
 *
 * Inert unless the wrapped backend applies the setting itself. Behind a
 * PlainShelf server the listing was filtered server-side by that server's
 * `show_nsfw`, and filtering a second time here would let a device preference
 * overrule it — so `filtersNsfwOnDevice` gates every check, and a server-backed
 * shell pays nothing at all.
 */
export class VisibleMobileBookCache implements MobileBookCache {
  constructor(
    private readonly inner: MobileBookCache,
    /**
     * Read per call rather than captured: the same wrapper outlives a change to
     * the setting made in the settings page, and the backend behind the shell
     * is repointed in place when the device switches shelves.
     */
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

  /**
   * Reading position is device-local state the reader itself wrote, not shelf
   * content, but it is still addressed by book id and answering it would say the
   * book is there. A book that cannot be opened has no position to restore.
   */
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
   * Whether the cached copy of this book must not be served.
   *
   * Costs one stored manifest read for the reads keyed by id — and only on a
   * backend that filters on the device. That read hits the same local cache the
   * call was already going to, so it doubles a local file read rather than
   * adding a request; the mark itself is on the stored book, which is why no
   * listing has to be walked to answer it.
   *
   * A book downloaded by a build older than the mark carries neither half of it
   * and reads as unmarked here, so it stays visible until it is downloaded
   * again. Bumping the manifest version instead would delete every existing
   * download.
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
