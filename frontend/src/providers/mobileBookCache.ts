import type { Book, BookContent, DownloadState, ReadingProgress, SplitConfig } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import { normalizeSplitConfig } from '@/utils/splitConfig';

export interface CachedBookSizeBreakdown {
  content: number;
  sources: number;
  cover: number;
  // Optional for the same reason size_bytes is: manifests written before
  // illustrations were downloaded carry no figure for them.
  assets?: number;
}

export interface CachedBookManifest {
  book: Book;
  sources: SourceMeta[];
  // Full config for offline reader startup. SourceMeta only carries the split
  // type, which is insufficient for line_count, regex, and boundary splitting.
  split_config?: SplitConfig;
  downloaded_at: string;
  local_version?: string;
  remote_version?: string;
  // Optional: absent on manifests written before size accounting existed
  // (pre-v2 IndexedDB records). Consumers must fall back to 0.
  size_bytes?: number;
  size_breakdown?: CachedBookSizeBreakdown;
}

export interface MobileBookCache {
  listDownloadedBooks(): Promise<Book[]>;
  getCachedBook(bookId: string): Promise<Book | null>;
  getDownloadState(bookId: string): Promise<DownloadState>;

  saveDownloadedBook(manifest: CachedBookManifest): Promise<void>;
  removeDownloadedBook(bookId: string): Promise<void>;

  listCachedSources(bookId: string): Promise<SourceMeta[]>;
  getCachedSource(bookId: string, sourceId: string): Promise<SourceMeta | null>;
  getCachedBookSplitConfig(bookId: string): Promise<SplitConfig | null>;

  getCachedBookContent(bookId: string): Promise<BookContent | null>;
  saveCachedBookContent(bookId: string, content: BookContent): Promise<void>;

  getCachedSourceContent(bookId: string, sourceId: string): Promise<string | null>;
  saveCachedSourceContent(bookId: string, sourceId: string, content: string): Promise<void>;

  getReadProgress(bookId: string): Promise<ReadingProgress | null>;
  saveReadProgress(bookId: string, progress: ReadingProgress): Promise<void>;

  getCachedCover(bookId: string): Promise<Blob | null>;
  saveCachedCover(bookId: string, blob: Blob): Promise<void>;
  deleteCachedCover(bookId: string): Promise<void>;

  /**
   * Illustrations belonging to a downloaded source, keyed by source and file
   * name because assets live per source on the shelf.
   *
   * A miss is a normal outcome: books downloaded before assets were cached
   * have none, and a download only stores what its text referenced.
   */
  getCachedAsset(bookId: string, sourceId: string, name: string): Promise<Blob | null>;
  saveCachedAsset(bookId: string, sourceId: string, name: string, blob: Blob): Promise<void>;

  listDownloadedManifests(): Promise<CachedBookManifest[]>;
}

/**
 * The Book a downloaded manifest stands for.
 *
 * Every MobileBookCache answers listDownloadedBooks/getCachedBook from a stored
 * manifest, and MobileBookshelfProvider.listDownloadedBookEntries builds the
 * same shape for the downloads page — so the manifest-wins-over-book precedence
 * for the version fields lives here rather than being restated per caller.
 */
export function downloadedBookFromManifest(manifest: CachedBookManifest): Book {
  return {
    ...manifest.book,
    download_state: 'downloaded',
    downloaded_at: manifest.downloaded_at,
    local_version: manifest.local_version ?? manifest.book.local_version,
    remote_version: manifest.remote_version ?? manifest.book.remote_version
  };
}

/**
 * Copy of a manifest that shares none of its own objects with the original:
 * assigning to a field of the returned book, of one of its sources, or of the
 * size breakdown cannot reach the cache's copy, and vice versa.
 *
 * **One level deep, no further.** Values nested inside those objects stay
 * shared by reference — `book.authors`, `book.tags`, `book.layers`,
 * `book.identifiers`, a source's `split_config` — so this does not protect
 * against a caller mutating one of those in place. `split_config` on the
 * manifest itself is the exception, because copySplitConfig also copies its
 * `boundaries` array.
 *
 * That depth is what every cache implementation applied before this was
 * shared, and it is enough for how manifests are actually used: the only one
 * ever handed to saveDownloadedBook is built per download in
 * MobileBookshelfProvider.downloadBook and dropped straight after, the
 * filesystem cache serialises to JSON on the way in, and the downloads page
 * only reads what listDownloadedManifests returns. Deepen it here if a caller
 * ever starts mutating a manifest it did not build.
 */
export function cloneManifest(manifest: CachedBookManifest): CachedBookManifest {
  return {
    book: { ...manifest.book },
    sources: manifest.sources.map((source) => ({ ...source })),
    split_config: copySplitConfig(manifest.split_config),
    downloaded_at: manifest.downloaded_at,
    local_version: manifest.local_version,
    remote_version: manifest.remote_version,
    size_bytes: manifest.size_bytes,
    size_breakdown: manifest.size_breakdown ? { ...manifest.size_breakdown } : undefined
  };
}

export class InMemoryMobileBookCache implements MobileBookCache {
  private readonly manifests = new Map<string, CachedBookManifest>();
  private readonly bookContents = new Map<string, BookContent>();
  private readonly sourceContents = new Map<string, string>();
  private readonly progress = new Map<string, ReadingProgress>();
  private readonly covers = new Map<string, Blob>();
  private readonly assets = new Map<string, Blob>();

  async listDownloadedBooks(): Promise<Book[]> {
    return Array.from(this.manifests.values()).map(downloadedBookFromManifest);
  }

  async getCachedBook(bookId: string): Promise<Book | null> {
    const manifest = this.manifests.get(bookId);
    return manifest ? downloadedBookFromManifest(manifest) : null;
  }

  async getDownloadState(bookId: string): Promise<DownloadState> {
    return this.manifests.has(bookId) ? 'downloaded' : 'not_downloaded';
  }

  async saveDownloadedBook(manifest: CachedBookManifest): Promise<void> {
    this.manifests.set(manifest.book.id, cloneManifest(manifest));
  }

  async removeDownloadedBook(bookId: string): Promise<void> {
    this.manifests.delete(bookId);
    this.bookContents.delete(bookId);
    this.progress.delete(bookId);
    this.covers.delete(bookId);

    for (const key of Array.from(this.sourceContents.keys())) {
      if (key.startsWith(`${bookId}:`)) {
        this.sourceContents.delete(key);
      }
    }

    for (const key of Array.from(this.assets.keys())) {
      if (key.startsWith(`${bookId}:`)) {
        this.assets.delete(key);
      }
    }
  }

  async listCachedSources(bookId: string): Promise<SourceMeta[]> {
    const manifest = this.manifests.get(bookId);
    return manifest ? manifest.sources.map((source) => ({ ...source })) : [];
  }

  async getCachedSource(bookId: string, sourceId: string): Promise<SourceMeta | null> {
    const manifest = this.manifests.get(bookId);
    const source = manifest?.sources.find((item) => item.id === sourceId);
    return source ? { ...source } : null;
  }

  async getCachedBookSplitConfig(bookId: string): Promise<SplitConfig | null> {
    return copySplitConfig(this.manifests.get(bookId)?.split_config) ?? null;
  }

  async getCachedBookContent(bookId: string): Promise<BookContent | null> {
    const content = this.bookContents.get(bookId);
    return content ? { ...content } : null;
  }

  async saveCachedBookContent(bookId: string, content: BookContent): Promise<void> {
    this.bookContents.set(bookId, { ...content });
  }

  async getCachedSourceContent(bookId: string, sourceId: string): Promise<string | null> {
    return this.sourceContents.get(this.sourceContentKey(bookId, sourceId)) ?? null;
  }

  async saveCachedSourceContent(bookId: string, sourceId: string, content: string): Promise<void> {
    this.sourceContents.set(this.sourceContentKey(bookId, sourceId), content);
  }

  async getReadProgress(bookId: string): Promise<ReadingProgress | null> {
    const current = this.progress.get(bookId);
    return current ? { ...current } : null;
  }

  async saveReadProgress(bookId: string, progress: ReadingProgress): Promise<void> {
    this.progress.set(bookId, { ...progress });
  }

  async getCachedCover(bookId: string): Promise<Blob | null> {
    return this.covers.get(bookId) ?? null;
  }

  async saveCachedCover(bookId: string, blob: Blob): Promise<void> {
    this.covers.set(bookId, blob);
  }

  async deleteCachedCover(bookId: string): Promise<void> {
    this.covers.delete(bookId);
  }

  async getCachedAsset(bookId: string, sourceId: string, name: string): Promise<Blob | null> {
    return this.assets.get(this.assetKey(bookId, sourceId, name)) ?? null;
  }

  async saveCachedAsset(bookId: string, sourceId: string, name: string, blob: Blob): Promise<void> {
    this.assets.set(this.assetKey(bookId, sourceId, name), blob);
  }

  async listDownloadedManifests(): Promise<CachedBookManifest[]> {
    return Array.from(this.manifests.values()).map(cloneManifest);
  }

  private sourceContentKey(bookId: string, sourceId: string): string {
    return `${bookId}:${sourceId}`;
  }

  private assetKey(bookId: string, sourceId: string, name: string): string {
    return `${bookId}:${sourceId}:${name}`;
  }
}

export function copySplitConfig(config: unknown): SplitConfig | undefined {
  if (config === undefined || config === null) {
    return undefined;
  }
  const normalized = normalizeSplitConfig(config);
  return normalized.boundaries
    ? { ...normalized, boundaries: [...normalized.boundaries] }
    : { ...normalized };
}
