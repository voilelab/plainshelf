import { currentCacheScopeKey } from './cacheScope';
import type { MobileBookCache, CachedBookManifest } from './mobileBookCache';
import { ShelfVisibility } from './shelfVisibility';
import { getShowNsfwOnDevice } from '@/composables/useDeviceNsfwPreference';
import type { Book, BookContent, DownloadState, NsfwMarks, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';

export class VisibleMobileBookCache implements MobileBookCache {
  private repaired: Promise<void> | null = null;
  private repairedScope: string | null = null;

  constructor(
    private readonly inner: MobileBookCache,
    private readonly filtersOnDevice: () => boolean,
    private readonly localMarks: () => Promise<ReadonlyMap<string, NsfwMarks> | null>
  ) {}


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

  async getReadProgress(bookId: string): Promise<ReadingProgress | null> {
    return (await this.hidden(bookId)) ? null : this.inner.getReadProgress(bookId);
  }


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


  private visibility(): ShelfVisibility | null {
    return this.filtersOnDevice() ? new ShelfVisibility({ showNsfw: getShowNsfwOnDevice() }) : null;
  }

  private allows(book: Book): boolean {
    return this.visibility()?.allows(book) ?? true;
  }

  private async hidden(bookId: string): Promise<boolean> {
    const visibility = this.visibility();
    if (!visibility) {
      return false;
    }
    await this.repair();
    const book = await this.inner.getCachedBook(bookId);
    return book !== null && !visibility.allows(book);
  }


  private repair(): Promise<void> {
    if (!this.filtersOnDevice()) {
      return Promise.resolve();
    }

    const scope = currentCacheScopeKey();
    if (this.repaired && this.repairedScope === scope) {
      return this.repaired;
    }

    this.repairedScope = scope;
    const done = this.runRepair(scope)
      .catch((err) => {
        console.warn('Could not repair the adult-content marks on earlier downloads.', err);
        return false;
      })
      .then((answered) => {
        if (!answered && this.repaired === done) {
          this.repaired = null;
        }
      });
    this.repaired = done;
    return done;
  }

  private async runRepair(scope: string): Promise<boolean> {
    const anyStale = (await this.inner.listDownloadedManifests()).some(
      (manifest) => manifest.book.nsfw === undefined
    );
    if (!anyStale) {
      return true;
    }

    const marks = await this.localMarks();
    if (!marks) {
      return false;
    }

    const stale = (await this.inner.listDownloadedManifests()).filter(
      (manifest) => manifest.book.nsfw === undefined
    );
    for (const manifest of stale) {
      const mark = marks.get(manifest.book.id);
      if (!mark) {
        continue;
      }
      if (currentCacheScopeKey() !== scope) {
        return false;
      }
      await this.inner.saveDownloadedBook({
        ...manifest,
        book: { ...manifest.book, nsfw: mark.nsfw === true, nsfw_folder: mark.nsfw_folder }
      });
    }
    return true;
  }
}
