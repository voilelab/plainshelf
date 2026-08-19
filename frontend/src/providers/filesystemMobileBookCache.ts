import { Filesystem } from '@capacitor/filesystem';

import type { Book, BookContent, DownloadState, ReadingProgress } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import { currentCacheScopeKey } from './cacheScope';
import {
  cloneManifest,
  downloadedBookFromManifest,
  type CachedBookManifest,
  type MobileBookCache
} from './mobileBookCache';
import {
  BASE_DIR,
  base64ToBlob,
  blobToBase64,
  CACHE_DIRECTORY,
  deleteFileIgnoringMissing,
  encode,
  hashComponent,
  readJsonFile,
  readTextFile,
  rmdirIgnoringMissing,
  scopeDir,
  writeJsonFile,
  writeTextFile
} from './mobileCacheFs';

// Downloaded books, inside the shared on-device layout defined by
// mobileCacheFs.ts (which owns the base directory and the scope segment):
//
//   plainshelf-cache/scopes/<enc(scopeKey)>/books/<enc(bookId)>/
//       manifest.json      # CachedBookManifest
//       content.txt        # BookContent.content
//       progress.json      # ReadingProgress (independent of manifest)
//       cover.json         # StoredCover
//       sources/<enc(sourceId)>.txt
//       assets/<enc(sourceId)>/<hash(name)>.json   # StoredAsset
//
// `enc` is encodeURIComponent. The real id is nevertheless always read from
// the manifest.json content, never decoded back from the directory name —
// this keeps the read path independent of the encoding scheme and of any
// platform-specific filename quirks.
//
// Commit-point semantics: `saveDownloadedBook` writes manifest.json, but the
// caller (see mobileBookshelfProvider.ts's downloadBook) writes manifest.json
// *before* content.txt and the source files, not after — and there is no
// transaction spanning the whole set, so "manifest present" cannot mean "fully
// downloaded". Rather than fight that call order, this implementation treats
// presence of manifest.json as the sole signal for "this book is in the
// downloaded list" (matching listDownloadedBooks/getCachedBook/
// getDownloadState), and treats a missing content/source file as a plain cache
// miss (returns null) instead of throwing. A directory with no manifest.json is
// an orphan and is ignored by listDownloadedBooks. A manifest.json that fails
// to parse is treated the same as a missing manifest (ignored, not thrown).

// Pre-scope layout. Everything under it belongs to whichever connection was
// configured when it was written, which — until the mobile shell can hold more
// than one connection — is the one configured now. See migrateLegacyBooks.
const LEGACY_BOOKS_DIR = `${BASE_DIR}/books`;

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

// Illustrations sit under the book directory too, so the same recursive rmdir
// sweeps them, and are nested by source because the shelf stores them per
// source. Each is a sidecar for the reason above: binary does not survive the
// text-only file API these caches share.
//
// The file name is a hash rather than the encoded asset name. An asset name is
// user-chosen and the shelf accepts up to 255 UTF-8 bytes; percent-encoding a
// CJK name triples it, so ~29 characters would overflow the 255-byte component
// limit and the write would fail. The exact name is stored inside the file and
// checked on read, so a hash collision is a cache miss and never the wrong
// picture.
function assetPath(booksDir: string, bookId: string, sourceId: string, name: string): string {
  return `${bookDir(booksDir, bookId)}/assets/${encode(sourceId)}/${hashComponent(name)}.json`;
}

interface StoredAsset extends StoredCover {
  name: string;
}

interface StoredCover {
  mime: string;
  data: string; // base64 (no data: prefix)
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
    return manifests.map(downloadedBookFromManifest);
  }

  async getCachedBook(bookId: string): Promise<Book | null> {
    const manifest = await this.readManifest(bookId);
    return manifest ? downloadedBookFromManifest(manifest) : null;
  }

  async getDownloadState(bookId: string): Promise<DownloadState> {
    const manifest = await this.readManifest(bookId);
    return manifest ? 'downloaded' : 'not_downloaded';
  }

  async saveDownloadedBook(manifest: CachedBookManifest): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    await writeJsonFile(manifestPath(booksDir, manifest.book.id), cloneManifest(manifest));
  }

  async removeDownloadedBook(bookId: string): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    await deleteFileIgnoringMissing(manifestPath(booksDir, bookId));
    await rmdirIgnoringMissing(bookDir(booksDir, bookId));
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
    const content = await readTextFile(contentPath(booksDir, bookId));
    return content !== null ? { content } : null;
  }

  async saveCachedBookContent(bookId: string, content: BookContent): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    await writeTextFile(contentPath(booksDir, bookId), content.content);
  }

  async getCachedSourceContent(bookId: string, sourceId: string): Promise<string | null> {
    const booksDir = await this.resolveBooksDir();
    return readTextFile(sourcePath(booksDir, bookId, sourceId));
  }

  async saveCachedSourceContent(bookId: string, sourceId: string, content: string): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    await writeTextFile(sourcePath(booksDir, bookId, sourceId), content);
  }

  async getReadProgress(bookId: string): Promise<ReadingProgress | null> {
    const booksDir = await this.resolveBooksDir();
    return readJsonFile<ReadingProgress>(progressPath(booksDir, bookId));
  }

  async saveReadProgress(bookId: string, progress: ReadingProgress): Promise<void> {
    // Independent of the manifest/download state: progress can be written (and
    // read back) even for a book that was never (or isn't currently)
    // downloaded.
    const booksDir = await this.resolveBooksDir();
    await writeJsonFile(progressPath(booksDir, bookId), { ...progress });
  }

  async getCachedCover(bookId: string): Promise<Blob | null> {
    const booksDir = await this.resolveBooksDir();
    const stored = await readJsonFile<Partial<StoredCover>>(coverPath(booksDir, bookId));
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
    await writeJsonFile(coverPath(booksDir, bookId), stored);
  }

  async deleteCachedCover(bookId: string): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    await deleteFileIgnoringMissing(coverPath(booksDir, bookId));
  }

  async getCachedAsset(bookId: string, sourceId: string, name: string): Promise<Blob | null> {
    const booksDir = await this.resolveBooksDir();
    const stored = await readJsonFile<Partial<StoredAsset>>(assetPath(booksDir, bookId, sourceId, name));
    if (!stored || typeof stored.data !== 'string') {
      return null;
    }
    // The path is a hash, so the file has to confirm which name it holds. A
    // collision must read as a cache miss rather than as another illustration.
    if (stored.name !== name) {
      return null;
    }
    try {
      return base64ToBlob(stored.data, typeof stored.mime === 'string' ? stored.mime : '');
    } catch {
      // Undecodable base64 is a cache miss, not a crash: the reader falls back
      // to the remote copy, or to the illustration's alt text when offline.
      return null;
    }
  }

  async saveCachedAsset(bookId: string, sourceId: string, name: string, blob: Blob): Promise<void> {
    const booksDir = await this.resolveBooksDir();
    const stored: StoredAsset = { name, mime: blob.type, data: await blobToBase64(blob) };
    await writeJsonFile(assetPath(booksDir, bookId, sourceId, name), stored);
  }

  async listDownloadedManifests(): Promise<CachedBookManifest[]> {
    const manifests = await this.readAllManifests();
    return manifests.map(cloneManifest);
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

  private async readManifest(bookId: string): Promise<CachedBookManifest | null> {
    const booksDir = await this.resolveBooksDir();
    return this.asManifest(await readJsonFile<unknown>(manifestPath(booksDir, bookId)));
  }

  private async readManifestByDir(booksDir: string, dirName: string): Promise<CachedBookManifest | null> {
    return this.asManifest(
      await readJsonFile<unknown>(`${booksDir}/${dirName}/manifest.json`)
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

}
