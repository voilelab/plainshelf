import type { Book, BookContent, DownloadState, ReadingProgress } from '../types/book';
import type { SourceMeta } from '../types/source';

export interface CachedBookSizeBreakdown {
  content: number;
  sources: number;
  cover: number;
}

export interface CachedBookManifest {
  book: Book;
  sources: SourceMeta[];
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

  getCachedBookContent(bookId: string): Promise<BookContent | null>;
  saveCachedBookContent(bookId: string, content: BookContent): Promise<void>;

  getCachedSourceContent(bookId: string, sourceId: string): Promise<string | null>;
  saveCachedSourceContent(bookId: string, sourceId: string, content: string): Promise<void>;

  getReadProgress(bookId: string): Promise<ReadingProgress | null>;
  saveReadProgress(bookId: string, progress: ReadingProgress): Promise<void>;

  getCachedCover(bookId: string): Promise<Blob | null>;
  saveCachedCover(bookId: string, blob: Blob): Promise<void>;
  deleteCachedCover(bookId: string): Promise<void>;
  listDownloadedManifests(): Promise<CachedBookManifest[]>;
}

export class InMemoryMobileBookCache implements MobileBookCache {
  private readonly manifests = new Map<string, CachedBookManifest>();
  private readonly bookContents = new Map<string, BookContent>();
  private readonly sourceContents = new Map<string, string>();
  private readonly progress = new Map<string, ReadingProgress>();
  private readonly covers = new Map<string, Blob>();

  async listDownloadedBooks(): Promise<Book[]> {
    return Array.from(this.manifests.values()).map((manifest) => this.toDownloadedBook(manifest));
  }

  async getCachedBook(bookId: string): Promise<Book | null> {
    const manifest = this.manifests.get(bookId);
    return manifest ? this.toDownloadedBook(manifest) : null;
  }

  async getDownloadState(bookId: string): Promise<DownloadState> {
    return this.manifests.has(bookId) ? 'downloaded' : 'not_downloaded';
  }

  async saveDownloadedBook(manifest: CachedBookManifest): Promise<void> {
    this.manifests.set(manifest.book.id, {
      book: { ...manifest.book },
      sources: manifest.sources.map((source) => ({ ...source })),
      downloaded_at: manifest.downloaded_at,
      local_version: manifest.local_version,
      remote_version: manifest.remote_version,
      size_bytes: manifest.size_bytes,
      size_breakdown: manifest.size_breakdown ? { ...manifest.size_breakdown } : undefined
    });
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

  async listDownloadedManifests(): Promise<CachedBookManifest[]> {
    return Array.from(this.manifests.values()).map((manifest) => ({
      book: { ...manifest.book },
      sources: manifest.sources.map((source) => ({ ...source })),
      downloaded_at: manifest.downloaded_at,
      local_version: manifest.local_version,
      remote_version: manifest.remote_version,
      size_bytes: manifest.size_bytes,
      size_breakdown: manifest.size_breakdown ? { ...manifest.size_breakdown } : undefined
    }));
  }

  private toDownloadedBook(manifest: CachedBookManifest): Book {
    return {
      ...manifest.book,
      download_state: 'downloaded',
      downloaded_at: manifest.downloaded_at,
      local_version: manifest.local_version ?? manifest.book.local_version,
      remote_version: manifest.remote_version ?? manifest.book.remote_version
    };
  }

  private sourceContentKey(bookId: string, sourceId: string): string {
    return `${bookId}:${sourceId}`;
  }
}
