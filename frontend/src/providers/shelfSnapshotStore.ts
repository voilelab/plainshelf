import type { BookJson, BookPackageRef } from '@/api/pcloud/bookpkg';
import { currentCacheScopeKey } from './cacheScope';
import { deleteFileIgnoringMissing, readJsonFile, scopeDir, writeJsonFile } from './mobileCacheFs';

/**
 * Bumped when the persisted shape changes in a way an older reader would
 * misread. A snapshot is always reconstructible by walking pCloud again, so a
 * mismatch is simply discarded — there is nothing to migrate.
 */
export const SHELF_SNAPSHOT_VERSION = 1;

const SNAPSHOT_FILE = 'shelf-snapshot.json';

/** One book as the shelf listing found it: where its files are, and what its
 * book.json says. `Book` itself is deliberately absent — it is derived from
 * these two, and storing it as well would create a second source of truth. */
export interface PersistedShelfBook {
  pkg: BookPackageRef;
  meta: BookJson;
}

export interface PersistedShelfSnapshot {
  version: number;
  /** The shelf this was taken from, so a snapshot left behind by a different
   * shelf root is recognised and ignored rather than shown as this shelf. */
  shelf_root: string;
  /** When pCloud was last actually walked (epoch ms). Surfaced to the user as
   * "last updated", and the reason the field is stored rather than derived. */
  fetched_at: number;
  layers: string[];
  books: PersistedShelfBook[];
}

export interface ShelfSnapshotStore {
  load(): Promise<PersistedShelfSnapshot | null>;
  save(snapshot: PersistedShelfSnapshot): Promise<void>;
  clear(): Promise<void>;
}

/**
 * Rejects anything that is not a snapshot this reader wrote.
 *
 * Kept strict on purpose: the whole book list is built from this file, so a
 * partially-valid payload would surface as a silently truncated or malformed
 * library rather than as the cache miss it really is.
 */
export function parseShelfSnapshot(value: unknown): PersistedShelfSnapshot | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return null;
  }

  const snapshot = value as Partial<PersistedShelfSnapshot>;
  if (snapshot.version !== SHELF_SNAPSHOT_VERSION) {
    return null;
  }
  if (typeof snapshot.shelf_root !== 'string' || typeof snapshot.fetched_at !== 'number') {
    return null;
  }
  if (!Array.isArray(snapshot.layers) || !Array.isArray(snapshot.books)) {
    return null;
  }
  if (!snapshot.books.every((book) => isPersistedShelfBook(book))) {
    return null;
  }

  return snapshot as PersistedShelfSnapshot;
}

function isPersistedShelfBook(value: unknown): value is PersistedShelfBook {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const book = value as Partial<PersistedShelfBook>;
  if (typeof book.meta !== 'object' || book.meta === null || typeof book.meta.id !== 'string') {
    return false;
  }
  return typeof book.pkg === 'object' && book.pkg !== null && Array.isArray(book.pkg.sources);
}

/** In-memory store, for tests and for any runtime with nowhere to persist. */
export class InMemoryShelfSnapshotStore implements ShelfSnapshotStore {
  private snapshot: PersistedShelfSnapshot | null = null;

  async load(): Promise<PersistedShelfSnapshot | null> {
    return this.snapshot ? structuredClone(this.snapshot) : null;
  }

  async save(snapshot: PersistedShelfSnapshot): Promise<void> {
    this.snapshot = structuredClone(snapshot);
  }

  async clear(): Promise<void> {
    this.snapshot = null;
  }
}

/**
 * Persists the shelf listing to the device, next to the downloaded books
 * (see mobileCacheFs.ts for the layout and why Directory.Data).
 *
 * `load` treats every failure as a cache miss, so a corrupt or foreign file
 * costs one wasted scan rather than an unopenable library. `save` swallows
 * write failures too: the snapshot in memory is already correct, and a shelf
 * update that succeeded must not be reported as failed because the device
 * could not write the file.
 */
export class FilesystemShelfSnapshotStore implements ShelfSnapshotStore {
  async load(): Promise<PersistedShelfSnapshot | null> {
    return parseShelfSnapshot(await readJsonFile<unknown>(this.path()));
  }

  async save(snapshot: PersistedShelfSnapshot): Promise<void> {
    try {
      await writeJsonFile(this.path(), snapshot);
    } catch (err) {
      console.warn('Could not persist the pCloud shelf snapshot; it stays in memory for this run.', err);
    }
  }

  async clear(): Promise<void> {
    try {
      await deleteFileIgnoringMissing(this.path());
    } catch (err) {
      console.warn('Could not remove the persisted pCloud shelf snapshot.', err);
    }
  }

  // Resolved per call rather than captured: the connection settings behind the
  // scope key are module-level state the settings UI updates in place, while
  // this store is held by a process-wide provider. See cacheScope.ts.
  private path(): string {
    return `${scopeDir(currentCacheScopeKey())}/${SNAPSHOT_FILE}`;
  }
}
