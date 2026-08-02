import { Directory, Encoding, Filesystem } from '@capacitor/filesystem';

import type { Book, BookContent, DownloadState, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import { currentCacheScopeKey } from './cacheScope';
import type { CachedBookManifest, MobileBookCache } from './mobileBookCache';

// Base directory (relative to Capacitor's Directory.Data) that holds every
// downloaded book. Directory.Data is app-private, requires no runtime
// permission, and — unlike IndexedDB in an Android WebView — is not subject
// to silent LRU eviction (see the rationale in providers/index.ts and
// .claude/rules/50-lessons.md). Layout:
//
//   plainshelf-cache/scopes/<enc(scopeKey)>/books/<enc(bookId)>/
//       manifest.json      # CachedBookManifest
//       content.txt        # BookContent.content
//       progress.json      # ReadingProgress (independent of manifest)
//       cover.json         # StoredCover
//       sources/<enc(sourceId)>.txt
//
// `enc` is encodeURIComponent. The real id is nevertheless always read from
// the manifest.json content, never decoded back from the directory name —
// this keeps the read path independent of the encoding scheme and of any
// platform-specific filename quirks.
//
// The scope segment keeps two shelves' downloads apart; see cacheScope.ts for
// why book ids alone cannot. Without it their downloads would share a directory
// and overwrite each other, and listDownloadedBooks would return them mixed
// together.
//
// Commit-point semantics, downgraded from the IndexedDB version:
// `saveDownloadedBook` writes manifest.json, but the caller (see
// mobileBookshelfProvider.ts's downloadBook) writes manifest.json *before*
// content.txt and the source files, not after — so "manifest present" cannot
// mean "fully downloaded" the way a single atomic transaction would. Rather
// than fight that call order, this implementation treats presence of
// manifest.json as the sole signal for "this book is in the downloaded
// list" (matching listDownloadedBooks/getCachedBook/getDownloadState), and
// treats a missing content/source file as a plain cache miss (returns null)
// instead of throwing. A directory with no manifest.json is an orphan and is
// ignored by listDownloadedBooks. A manifest.json that fails to parse is
// treated the same as a missing manifest (ignored, not thrown).
const BASE_DIR = 'plainshelf-cache';
const SCOPES_DIR = `${BASE_DIR}/scopes`;
// Pre-scope layout. Everything under it belongs to whichever connection was
// configured when it was written, which — until the mobile shell can hold more
// than one connection — is the one configured now. See migrateLegacyBooks.
const LEGACY_BOOKS_DIR = `${BASE_DIR}/books`;
const CACHE_DIRECTORY = Directory.Data;

function encode(id: string): string {
  return encodeURIComponent(id);
}

// Directory for a client that has no (server, shelf) identity yet. Unreachable
// from the native shell this class serves — there `getApiBase()` is applied
// before mount and the router holds the app on the connect page until it is, so
// every key contains the `apiBase|shelfID` separator and encodes to a name with
// `%7C` in it. Named rather than left as an empty path segment so the layout has
// no `//` in it.
const UNSCOPED_DIR_NAME = '_unscoped';

function scopeDir(scopeKey: string): string {
  return `${SCOPES_DIR}/${scopeKey ? encode(scopeKey) : UNSCOPED_DIR_NAME}`;
}

function bookDir(booksDir: string, bookId: string): string {
  return `${booksDir}/${encode(bookId)}`;
}

function manifestPath(booksDir: string, bookId: string): string {
  return `${bookDir(booksDir, bookId)}/manifest.json`;
}

function contentPath(booksDir: string, bookId: string): string {
  return `${bookDir(booksDir, bookId)}/content.txt`;
}

function progressPath(booksDir: string, bookId: string): string {
  return `${bookDir(booksDir, bookId)}/progress.json`;
}

function sourcesDir(booksDir: string, bookId: string): string {
  return `${bookDir(booksDir, bookId)}/sources`;
}

function sourcePath(booksDir: string, bookId: string, sourceId: string): string {
  return `${sourcesDir(booksDir, bookId)}/${encode(sourceId)}.txt`;
}

// The cached cover lives alongside the manifest inside the book directory, so
// removeDownloadedBook's recursive rmdir already sweeps it up. Binary can't be
// stored losslessly through a UTF-8 text file (the only shape both the native
// plugin and its IndexedDB-backed web fallback round-trip identically here),
// so the cover is persisted as a small JSON sidecar holding the blob's MIME
// type plus its bytes base64-encoded — text all the way down.
function coverPath(booksDir: string, bookId: string): string {
  return `${bookDir(booksDir, bookId)}/cover.json`;
}

interface StoredCover {
  mime: string;
  data: string; // base64 (no data: prefix)
}

async function blobToBase64(blob: Blob): Promise<string> {
  const bytes = new Uint8Array(await blob.arrayBuffer());
  let binary = '';
  for (let i = 0; i < bytes.length; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function base64ToBlob(base64: string, mime: string): Blob {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return new Blob([bytes], mime ? { type: mime } : undefined);
}

/**
 * Filesystem-backed {@link MobileBookCache}, using @capacitor/filesystem's
 * app-private `Directory.Data`. See the module-level comment above for the
 * on-disk layout and the commit-point semantics this implementation uses.
 *
 * Every read method (`get*`/`list*`) treats "file or directory missing",
 * "underlying plugin error", and "malformed JSON" uniformly as a cache miss
 * (`null` / empty array) rather than throwing — a single book's corruption
 * must never take down the whole library list.
 */
export class FilesystemMobileBookCache implements MobileBookCache {
  private legacyMigration: Promise<void> | null = null;

  async listDownloadedBooks(): Promise<Book[]> {
    const manifests = await this.readAllManifests();
    return manifests.map((manifest) => this.toDownloadedBook(manifest));
  }

  async getCachedBook(bookId: string): Promise<Book | null> {
    const manifest = await this.readManifest(bookId);
    return manifest ? this.toDownloadedBook(manifest) : null;
  }

  async getDownloadState(bookId: string): Promise<DownloadState> {
    const manifest = await this.readManifest(bookId);
    return manifest ? 'downloaded' : 'not_downloaded';
  }

  async saveDownloadedBook(manifest: CachedBookManifest): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    await this.writeJsonFile(manifestPath(booksDir, manifest.book.id), this.copyManifest(manifest));
  }

  async removeDownloadedBook(bookId: string): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    await this.deleteFileIgnoringMissing(manifestPath(booksDir, bookId));
    await this.rmdirIgnoringMissing(bookDir(booksDir, bookId));
  }

  async listCachedSources(bookId: string): Promise<SourceMeta[]> {
    const manifest = await this.readManifest(bookId);
    return manifest ? manifest.sources.map((source) => ({ ...source })) : [];
  }

  async getCachedSource(bookId: string, sourceId: string): Promise<SourceMeta | null> {
    const manifest = await this.readManifest(bookId);
    const source = manifest?.sources.find((item) => item.id === sourceId);
    return source ? { ...source } : null;
  }

  async getCachedBookContent(bookId: string): Promise<BookContent | null> {
    const booksDir = await this.resolveBooksDir();
    const content = await this.readTextFile(contentPath(booksDir, bookId));
    return content !== null ? { content } : null;
  }

  async saveCachedBookContent(bookId: string, content: BookContent): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    await this.writeTextFile(contentPath(booksDir, bookId), content.content);
  }

  async getCachedSourceContent(bookId: string, sourceId: string): Promise<string | null> {
    const booksDir = await this.resolveBooksDir();
    return this.readTextFile(sourcePath(booksDir, bookId, sourceId));
  }

  async saveCachedSourceContent(bookId: string, sourceId: string, content: string): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    await this.writeTextFile(sourcePath(booksDir, bookId, sourceId), content);
  }

  async getReadProgress(bookId: string): Promise<ReadingProgress | null> {
    const booksDir = await this.resolveBooksDir();
    return this.readJsonFile<ReadingProgress>(progressPath(booksDir, bookId));
  }

  async saveReadProgress(bookId: string, progress: ReadingProgress): Promise<void> {
    // Independent of the manifest/download state, matching IndexedDbMobileBookCache:
    // progress can be written (and read back) even for a book that was never
    // (or isn't currently) downloaded.
    const booksDir = await this.resolveBooksDir();
    await this.writeJsonFile(progressPath(booksDir, bookId), { ...progress });
  }

  async getCachedCover(bookId: string): Promise<Blob | null> {
    const booksDir = await this.resolveBooksDir();
    const stored = await this.readJsonFile<Partial<StoredCover>>(coverPath(booksDir, bookId));
    if (!stored || typeof stored.data !== 'string') {
      return null;
    }
    try {
      return base64ToBlob(stored.data, typeof stored.mime === 'string' ? stored.mime : '');
    } catch {
      // Corrupt/undecodable base64 is a cache miss, not a crash — the cover
      // just falls back to the remote URL (or a broken-image placeholder).
      return null;
    }
  }

  async saveCachedCover(bookId: string, blob: Blob): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    const stored: StoredCover = { mime: blob.type, data: await blobToBase64(blob) };
    await this.writeJsonFile(coverPath(booksDir, bookId), stored);
  }

  async deleteCachedCover(bookId: string): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    await this.deleteFileIgnoringMissing(coverPath(booksDir, bookId));
  }

  async listDownloadedManifests(): Promise<CachedBookManifest[]> {
    const manifests = await this.readAllManifests();
    return manifests.map((manifest) => this.copyManifest(manifest));
  }

  /**
   * Directory holding the downloads of the currently connected (server, shelf),
   * after any one-time move of the pre-scope layout has been applied.
   */
  private async resolveBooksDir(): Promise<string> {
    const scopeKey = currentCacheScopeKey();
    await this.ensureLegacyMigrated(scopeKey);
    return `${scopeDir(scopeKey)}/books`;
  }

  private async ensureLegacyMigrated(scopeKey: string): Promise<void> {
    if (!scopeKey) {
      // No connection applied yet, so there is no scope to attribute the legacy
      // downloads to. Skip without latching, so a later call still migrates.
      return;
    }
    if (!this.legacyMigration) {
      this.legacyMigration = this.migrateLegacyBooks(scopeKey);
    }
    await this.legacyMigration;
  }

  /**
   * Moves `plainshelf-cache/books` into the current scope. Runs at most once per
   * app run: the check costs two stats, and after a successful move the source
   * is gone.
   *
   * Every failure path leaves the legacy directory untouched and lets the caller
   * continue against an empty scope. Losing a download to a cache miss is
   * recoverable — the book re-downloads — whereas deleting the directory to
   * "clean up" would not be.
   */
  private async migrateLegacyBooks(scopeKey: string): Promise<void> {
    if (!(await this.isDirectory(LEGACY_BOOKS_DIR))) {
      return;
    }

    const target = `${scopeDir(scopeKey)}/books`;
    if (await this.isDirectory(target)) {
      // The scope already has its own downloads; merging the two could overwrite
      // them with same-id books from a different shelf, which is the very
      // collision the scope segment exists to prevent.
      return;
    }

    try {
      await Filesystem.mkdir({
        path: scopeDir(scopeKey),
        directory: CACHE_DIRECTORY,
        recursive: true
      });
      await Filesystem.rename({
        from: LEGACY_BOOKS_DIR,
        to: target,
        directory: CACHE_DIRECTORY,
        toDirectory: CACHE_DIRECTORY
      });
    } catch {
      // Left in place on purpose; see the doc comment.
    }
  }

  private async isDirectory(path: string): Promise<boolean> {
    try {
      const result = await Filesystem.stat({ path, directory: CACHE_DIRECTORY });
      return result.type === 'directory';
    } catch {
      return false;
    }
  }

  private async readAllManifests(): Promise<CachedBookManifest[]> {
    const booksDir = await this.resolveBooksDir();

    let entries;
    try {
      entries = (await Filesystem.readdir({ path: booksDir, directory: CACHE_DIRECTORY })).files;
    } catch {
      return [];
    }

    const manifests = await Promise.all(
      entries
        .filter((entry) => entry.type === 'directory')
        .map((entry) => this.readManifestByDir(booksDir, entry.name))
    );
    return manifests.filter((manifest): manifest is CachedBookManifest => manifest !== null);
  }

  private copyManifest(manifest: CachedBookManifest): CachedBookManifest {
    return {
      book: { ...manifest.book },
      sources: manifest.sources.map((source) => ({ ...source })),
      downloaded_at: manifest.downloaded_at,
      local_version: manifest.local_version,
      remote_version: manifest.remote_version,
      size_bytes: manifest.size_bytes,
      size_breakdown: manifest.size_breakdown ? { ...manifest.size_breakdown } : undefined
    };
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

  private async readManifest(bookId: string): Promise<CachedBookManifest | null> {
    const booksDir = await this.resolveBooksDir();
    return this.asManifest(await this.readJsonFile<unknown>(manifestPath(booksDir, bookId)));
  }

  private async readManifestByDir(booksDir: string, dirName: string): Promise<CachedBookManifest | null> {
    return this.asManifest(
      await this.readJsonFile<unknown>(`${booksDir}/${dirName}/manifest.json`)
    );
  }

  // JSON that parses but has the wrong shape ({}, [], missing book.id, …)
  // must count as a corrupt manifest, not a cache hit — otherwise
  // toDownloadedBook dereferences undefined and one bad manifest throws out
  // of listDownloadedBooks for the whole library.
  private asManifest(value: unknown): CachedBookManifest | null {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
      return null;
    }
    const manifest = value as Partial<CachedBookManifest>;
    if (typeof manifest.book !== 'object' || manifest.book === null || typeof manifest.book.id !== 'string') {
      return null;
    }
    if (!Array.isArray(manifest.sources)) {
      return null;
    }
    return manifest as CachedBookManifest;
  }

  private async readTextFile(path: string): Promise<string | null> {
    try {
      const result = await Filesystem.readFile({
        path,
        directory: CACHE_DIRECTORY,
        encoding: Encoding.UTF8
      });
      return typeof result.data === 'string' ? result.data : null;
    } catch {
      return null;
    }
  }

  private async readJsonFile<T>(path: string): Promise<T | null> {
    const text = await this.readTextFile(path);
    if (text === null) {
      return null;
    }
    try {
      return JSON.parse(text) as T;
    } catch {
      return null;
    }
  }

  private async writeTextFile(path: string, data: string): Promise<void> {
    await Filesystem.writeFile({
      path,
      data,
      directory: CACHE_DIRECTORY,
      encoding: Encoding.UTF8,
      recursive: true
    });
  }

  private async writeJsonFile(path: string, value: unknown): Promise<void> {
    await this.writeTextFile(path, JSON.stringify(value));
  }

  private async deleteFileIgnoringMissing(path: string): Promise<void> {
    try {
      await Filesystem.deleteFile({ path, directory: CACHE_DIRECTORY });
    } catch (error) {
      if (!this.isMissingError(error)) {
        throw error;
      }
    }
  }

  private async rmdirIgnoringMissing(path: string): Promise<void> {
    try {
      await Filesystem.rmdir({ path, directory: CACHE_DIRECTORY, recursive: true });
    } catch (error) {
      if (!this.isMissingError(error)) {
        throw error;
      }
    }
  }

  private isMissingError(error: unknown): boolean {
    const message = error instanceof Error ? error.message : String(error);
    return /not exist|does not exist|enoent|no such file|not found/i.test(message);
  }
}
