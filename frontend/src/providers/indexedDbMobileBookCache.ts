import { openDB, type DBSchema, type IDBPDatabase } from 'idb';

import type { Book, BookContent, DownloadState, ReadingProgress } from '../types/book';
import type { SourceMeta } from '../types/source';
import type { CachedBookManifest, MobileBookCache } from './mobileBookCache';

const DB_NAME = 'plainshelf-mobile';
const DB_VERSION = 1;

const STORE_MANIFESTS = 'manifests';
const STORE_BOOK_CONTENTS = 'bookContents';
const STORE_SOURCE_CONTENTS = 'sourceContents';
const STORE_PROGRESS = 'progress';

// The in-memory cache keyed source content as `${bookId}:${sourceId}` but
// removed entries by matching the `${bookId}:` prefix, which also nukes any
// other book whose id shares that prefix. We use `::` here and match the exact
// `${bookId}::` prefix on removal to avoid that class of collision.
const SOURCE_KEY_SEPARATOR = '::';

interface StoredBookContent {
  bookId: string;
  content: string;
}

interface PlainShelfMobileDB extends DBSchema {
  [STORE_MANIFESTS]: {
    key: string;
    value: CachedBookManifest;
  };
  [STORE_BOOK_CONTENTS]: {
    key: string;
    value: StoredBookContent;
  };
  [STORE_SOURCE_CONTENTS]: {
    key: string;
    value: string;
  };
  [STORE_PROGRESS]: {
    key: string;
    value: ReadingProgress;
  };
}

/**
 * IndexedDB-backed {@link MobileBookCache}. Unlike {@link InMemoryMobileBookCache}
 * this survives app restarts, so downloaded books and reading progress persist
 * across launches — the core requirement for offline reading on mobile.
 */
export class IndexedDbMobileBookCache implements MobileBookCache {
  private dbPromise: Promise<IDBPDatabase<PlainShelfMobileDB>> | null = null;

  constructor(private readonly dbName: string = DB_NAME) {}

  private db(): Promise<IDBPDatabase<PlainShelfMobileDB>> {
    if (!this.dbPromise) {
      this.dbPromise = openDB<PlainShelfMobileDB>(this.dbName, DB_VERSION, {
        upgrade(db) {
          if (!db.objectStoreNames.contains(STORE_MANIFESTS)) {
            db.createObjectStore(STORE_MANIFESTS);
          }
          if (!db.objectStoreNames.contains(STORE_BOOK_CONTENTS)) {
            db.createObjectStore(STORE_BOOK_CONTENTS);
          }
          if (!db.objectStoreNames.contains(STORE_SOURCE_CONTENTS)) {
            db.createObjectStore(STORE_SOURCE_CONTENTS);
          }
          if (!db.objectStoreNames.contains(STORE_PROGRESS)) {
            db.createObjectStore(STORE_PROGRESS);
          }
        }
      });
    }
    return this.dbPromise;
  }

  async listDownloadedBooks(): Promise<Book[]> {
    const db = await this.db();
    const manifests = await db.getAll(STORE_MANIFESTS);
    return manifests.map((manifest) => this.toDownloadedBook(manifest));
  }

  async getCachedBook(bookId: string): Promise<Book | null> {
    const db = await this.db();
    const manifest = await db.get(STORE_MANIFESTS, bookId);
    return manifest ? this.toDownloadedBook(manifest) : null;
  }

  async getDownloadState(bookId: string): Promise<DownloadState> {
    const db = await this.db();
    const key = await db.getKey(STORE_MANIFESTS, bookId);
    return key !== undefined ? 'downloaded' : 'not_downloaded';
  }

  async saveDownloadedBook(manifest: CachedBookManifest): Promise<void> {
    const db = await this.db();
    await db.put(
      STORE_MANIFESTS,
      {
        book: { ...manifest.book },
        sources: manifest.sources.map((source) => ({ ...source })),
        downloaded_at: manifest.downloaded_at,
        local_version: manifest.local_version,
        remote_version: manifest.remote_version
      },
      manifest.book.id
    );
  }

  async removeDownloadedBook(bookId: string): Promise<void> {
    const db = await this.db();
    const tx = db.transaction(
      [STORE_MANIFESTS, STORE_BOOK_CONTENTS, STORE_SOURCE_CONTENTS, STORE_PROGRESS],
      'readwrite'
    );

    await tx.objectStore(STORE_MANIFESTS).delete(bookId);
    await tx.objectStore(STORE_BOOK_CONTENTS).delete(bookId);
    await tx.objectStore(STORE_PROGRESS).delete(bookId);

    const sourceStore = tx.objectStore(STORE_SOURCE_CONTENTS);
    const prefix = `${bookId}${SOURCE_KEY_SEPARATOR}`;
    // A value cursor (openCursor) is required here: key cursors do not support
    // delete(). We only read the key, but must open a value cursor to remove.
    let cursor = await sourceStore.openCursor();
    while (cursor) {
      if (typeof cursor.key === 'string' && cursor.key.startsWith(prefix)) {
        await cursor.delete();
      }
      cursor = await cursor.continue();
    }

    await tx.done;
  }

  async listCachedSources(bookId: string): Promise<SourceMeta[]> {
    const db = await this.db();
    const manifest = await db.get(STORE_MANIFESTS, bookId);
    return manifest ? manifest.sources.map((source) => ({ ...source })) : [];
  }

  async getCachedSource(bookId: string, sourceId: string): Promise<SourceMeta | null> {
    const db = await this.db();
    const manifest = await db.get(STORE_MANIFESTS, bookId);
    const source = manifest?.sources.find((item) => item.id === sourceId);
    return source ? { ...source } : null;
  }

  async getCachedBookContent(bookId: string): Promise<BookContent | null> {
    const db = await this.db();
    const stored = await db.get(STORE_BOOK_CONTENTS, bookId);
    return stored ? { content: stored.content } : null;
  }

  async saveCachedBookContent(bookId: string, content: BookContent): Promise<void> {
    const db = await this.db();
    await db.put(STORE_BOOK_CONTENTS, { bookId, content: content.content }, bookId);
  }

  async getCachedSourceContent(bookId: string, sourceId: string): Promise<string | null> {
    const db = await this.db();
    const value = await db.get(STORE_SOURCE_CONTENTS, this.sourceContentKey(bookId, sourceId));
    return value ?? null;
  }

  async saveCachedSourceContent(bookId: string, sourceId: string, content: string): Promise<void> {
    const db = await this.db();
    await db.put(STORE_SOURCE_CONTENTS, content, this.sourceContentKey(bookId, sourceId));
  }

  async getReadProgress(bookId: string): Promise<ReadingProgress | null> {
    const db = await this.db();
    const progress = await db.get(STORE_PROGRESS, bookId);
    return progress ? { ...progress } : null;
  }

  async saveReadProgress(bookId: string, progress: ReadingProgress): Promise<void> {
    const db = await this.db();
    await db.put(STORE_PROGRESS, { ...progress }, bookId);
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
    return `${bookId}${SOURCE_KEY_SEPARATOR}${sourceId}`;
  }
}
