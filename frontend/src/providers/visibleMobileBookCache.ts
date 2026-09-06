import type { MobileBookCache, CachedBookManifest } from './mobileBookCache';
import { ShelfVisibility } from './shelfVisibility';
import { getShowNsfwOnDevice } from '@/composables/useDeviceNsfwPreference';
import type { Book, BookContent, DownloadState, NsfwMarks, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';

/**
 * A MobileBookCache that withholds what this device hides. The cache is the one
 * place a book is served without asking the backend, so wrapping it covers
 * every cache-first path. Guarded reads answer as a cache that never stored the
 * book would; writes pass through, and the wrapper is inert unless the backend
 * answers `filtersNsfwOnDevice`.
 *
 * A download taken before the marks were written into manifests carries
 * neither of them, which reads as unmarked. Rather than hide every such
 * download — which would empty the offline library of an offline user until
 * the next shelf update — the first guarded read repairs those manifests from
 * the marks the backend already holds on the device; see {@link repair}.
 */
export class VisibleMobileBookCache implements MobileBookCache {
  /** The repair pass, kept to one per instance; see {@link repair}. */
  private repaired: Promise<void> | null = null;

  constructor(
    private readonly inner: MobileBookCache,
    /** Asked per call: both the setting and the backend change under this. */
    private readonly filtersOnDevice: () => boolean,
    /** The backend's marks, from the device alone; see `localNsfwMarks`. */
    private readonly localMarks: () => Promise<ReadonlyMap<string, NsfwMarks> | null>
  ) {}

  // --- guarded reads -------------------------------------------------------

  async listDownloadedBooks(): Promise<Book[]> {
    await this.repair();
    const books = await this.inner.listDownloadedBooks();
    return this.visibility()?.keepBooks(books).slice() ?? books;
  }

  async listDownloadedManifests(): Promise<CachedBookManifest[]> {
    await this.repair();
    const manifests = await this.inner.listDownloadedManifests();
    const visibility = this.visibility();
    return visibility ? manifests.filter((manifest) => visibility.allows(manifest.book)) : manifests;
  }

  async getCachedBook(bookId: string): Promise<Book | null> {
    await this.repair();
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

  /** Costs one manifest read, from the same local cache the call was headed for. */
  private async hidden(bookId: string): Promise<boolean> {
    const visibility = this.visibility();
    if (!visibility) {
      return false;
    }
    await this.repair();
    const book = await this.inner.getCachedBook(bookId);
    return book !== null && !visibility.allows(book);
  }

  // --- repairing pre-mark manifests ----------------------------------------

  /**
   * Writes the shelf's marks into the manifests downloaded before they were
   * stored, so those downloads answer to the setting like any other.
   *
   * `nsfw` is written as a boolean for every book the backend knows, so
   * `undefined` means exactly "this manifest predates the marks" and a repaired
   * one is never examined twice. A book the backend cannot place — the shelf
   * has not been read on this device yet, or no longer holds it — is left
   * alone: hiding on doubt would take the whole offline library away from a
   * user who cannot refresh it.
   *
   * Runs once per instance, before the first guarded read, and only where the
   * backend applies the marks itself. Nothing but the manifest's `book` is
   * touched: content, cover and sources stay where they are.
   */
  private repair(): Promise<void> {
    if (!this.filtersOnDevice()) {
      return Promise.resolve();
    }
    this.repaired ??= this.runRepair().catch((err) => {
      // A cache read must still answer: the alternative to a stale mark here is
      // no offline library at all.
      console.warn('Could not repair the adult-content marks on earlier downloads.', err);
    });
    return this.repaired;
  }

  private async runRepair(): Promise<void> {
    const stale = (await this.inner.listDownloadedManifests()).filter(
      (manifest) => manifest.book.nsfw === undefined
    );
    if (stale.length === 0) {
      return;
    }

    const marks = await this.localMarks();
    if (!marks) {
      return;
    }

    for (const manifest of stale) {
      const mark = marks.get(manifest.book.id);
      if (!mark) {
        continue;
      }
      await this.inner.saveDownloadedBook({
        ...manifest,
        book: { ...manifest.book, nsfw: mark.nsfw === true, nsfw_folder: mark.nsfw_folder }
      });
    }
  }
}
