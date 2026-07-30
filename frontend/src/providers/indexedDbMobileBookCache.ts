import { openDB, type DBSchema, type IDBPDatabase, type IDBPTransaction } from 'idb';

import type { Book, BookContent, DownloadState, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import type { CachedBookManifest, MobileBookCache } from './mobileBookCache';

const DB_NAME = 'plainshelf-mobile';
const DB_VERSION = 2;

const STORE_MANIFESTS = 'manifests';
const STORE_BOOK_CONTENTS = 'bookContents';
const STORE_SOURCE_CONTENTS = 'sourceContents';
const STORE_PROGRESS = 'progress';
const STORE_COVERS = 'covers';

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
  [STORE_COVERS]: {
    key: string;
    value: Blob;
  };
}

function byteSizeOf(content: string | null | undefined): number {
  return content ? new Blob([content]).size : 0;
}

// Matches the transaction type idb hands to `upgrade()`: a versionchange
// transaction scoped to every store in the schema.
type MobileDBUpgradeTransaction = IDBPTransaction<
  PlainShelfMobileDB,
  (typeof STORE_MANIFESTS | typeof STORE_BOOK_CONTENTS | typeof STORE_SOURCE_CONTENTS | typeof STORE_PROGRESS | typeof STORE_COVERS)[],
  'versionchange'
>;

/**
 * v1 manifests were saved without `size_bytes`/`size_breakdown`. This walks
 * every manifest in the versionchange transaction and, for any that are
 * missing size accounting, recomputes it from the already-downloaded book
 * and source contents (cover size is 0 — v1 never cached covers). Uses only
 * get/getAll/put on the transaction's stores; no cursor deletion is needed.
 */
async function backfillManifestSizes(transaction: MobileDBUpgradeTransaction): Promise<void> {
  const manifestStore = transaction.objectStore(STORE_MANIFESTS);
  const bookContentStore = transaction.objectStore(STORE_BOOK_CONTENTS);
  const sourceContentStore = transaction.objectStore(STORE_SOURCE_CONTENTS);

  const keys = await manifestStore.getAllKeys();
  const manifests = await manifestStore.getAll();

  for (let i = 0; i < manifests.length; i += 1) {
    const manifest = manifests[i];
    const key = keys[i];
    if (manifest.size_bytes !== undefined) {
      continue;
    }

    const bookId = manifest.book.id;
    const storedContent = await bookContentStore.get(bookId);
    const contentSize = byteSizeOf(storedContent?.content);

    let sourcesSize = 0;
    for (const source of manifest.sources) {
      const sourceContent = await sourceContentStore.get(`${bookId}${SOURCE_KEY_SEPARATOR}${source.id}`);
      sourcesSize += byteSizeOf(sourceContent);
    }

    await manifestStore.put(
      {
        ...manifest,
        size_bytes: contentSize + sourcesSize,
        size_breakdown: { content: contentSize, sources: sourcesSize, cover: 0 }
      },
      key
    );
  }
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
        // Async upgrade: the `idb` wrapper keeps the native versionchange
        // transaction alive as long as we keep chaining awaited get/put
        // calls on it (each continuation schedules the next IDBRequest
        // before the task queue drains), so it is safe to `await` inside
        // here even though the upstream type declares `void`.
        async upgrade(db, oldVersion, _newVersion, transaction) {
          if (oldVersion < 1) {
            db.createObjectStore(STORE_MANIFESTS);
            db.createObjectStore(STORE_BOOK_CONTENTS);
            db.createObjectStore(STORE_SOURCE_CONTENTS);
            db.createObjectStore(STORE_PROGRESS);
          }
          if (oldVersion < 2) {
            db.createObjectStore(STORE_COVERS);
            // Backfill size_bytes/size_breakdown for manifests written
            // before size accounting existed (v1). Covers did not exist in
            // v1, so cover size is always 0 for backfilled entries; the app
            // will pick up real cover sizes the next time each book is
            // re-downloaded. Only get/getAll/put are used here (lesson:
            // key cursors cannot delete, but we do not need to delete
            // anything in this migration anyway).
            await backfillManifestSizes(transaction);
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
        remote_version: manifest.remote_version,
        size_bytes: manifest.size_bytes,
        size_breakdown: manifest.size_breakdown ? { ...manifest.size_breakdown } : undefined
      },
      manifest.book.id
    );
  }

  async removeDownloadedBook(bookId: string): Promise<void> {
    const db = await this.db();
    const tx = db.transaction(
      [STORE_MANIFESTS, STORE_BOOK_CONTENTS, STORE_SOURCE_CONTENTS, STORE_PROGRESS, STORE_COVERS],
      'readwrite'
    );

    await tx.objectStore(STORE_MANIFESTS).delete(bookId);
    await tx.objectStore(STORE_BOOK_CONTENTS).delete(bookId);
    await tx.objectStore(STORE_PROGRESS).delete(bookId);
    await tx.objectStore(STORE_COVERS).delete(bookId);

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

  async getCachedCover(bookId: string): Promise<Blob | null> {
    const db = await this.db();
    const blob = await db.get(STORE_COVERS, bookId);
    return blob ?? null;
  }

  async saveCachedCover(bookId: string, blob: Blob): Promise<void> {
    const db = await this.db();
    await db.put(STORE_COVERS, blob, bookId);
  }

  async deleteCachedCover(bookId: string): Promise<void> {
    const db = await this.db();
    await db.delete(STORE_COVERS, bookId);
  }

  async listDownloadedManifests(): Promise<CachedBookManifest[]> {
    const db = await this.db();
    const manifests = await db.getAll(STORE_MANIFESTS);
    return manifests.map((manifest) => ({
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
    return `${bookId}${SOURCE_KEY_SEPARATOR}${sourceId}`;
  }
}
