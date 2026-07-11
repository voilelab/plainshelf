import { Directory, Encoding, Filesystem } from '@capacitor/filesystem';

import type { Book, BookContent, DownloadState, ReadingProgress } from '../types/book';
import type { SourceMeta } from '../types/source';
import type { CachedBookManifest, MobileBookCache } from './mobileBookCache';

// Base directory (relative to Capacitor's Directory.Data) that holds every
// downloaded book. Directory.Data is app-private, requires no runtime
// permission, and — unlike IndexedDB in an Android WebView — is not subject
// to silent LRU eviction (see IndexedDbMobileBookCache's doc comment and
// .claude/rules/50-lessons.md 2026-07-10). Layout:
//
//   plainshelf-cache/books/<enc(bookId)>/
//       manifest.json      # CachedBookManifest
//       content.txt        # BookContent.content
//       progress.json      # ReadingProgress (independent of manifest)
//       sources/<enc(sourceId)>.txt
//
// `enc` is encodeURIComponent. The real id is nevertheless always read from
// the manifest.json content, never decoded back from the directory name —
// this keeps the read path independent of the encoding scheme and of any
// platform-specific filename quirks.
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
const BASE_DIR = 'plainshelf-cache/books';
const CACHE_DIRECTORY = Directory.Data;

function encode(id: string): string {
  return encodeURIComponent(id);
}

function bookDir(bookId: string): string {
  return `${BASE_DIR}/${encode(bookId)}`;
}

function manifestPath(bookId: string): string {
  return `${bookDir(bookId)}/manifest.json`;
}

function contentPath(bookId: string): string {
  return `${bookDir(bookId)}/content.txt`;
}

function progressPath(bookId: string): string {
  return `${bookDir(bookId)}/progress.json`;
}

function sourcesDir(bookId: string): string {
  return `${bookDir(bookId)}/sources`;
}

function sourcePath(bookId: string, sourceId: string): string {
  return `${sourcesDir(bookId)}/${encode(sourceId)}.txt`;
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
  async listDownloadedBooks(): Promise<Book[]> {
    let entries;
    try {
      entries = (await Filesystem.readdir({ path: BASE_DIR, directory: CACHE_DIRECTORY })).files;
    } catch {
      return [];
    }

    const manifests = await Promise.all(
      entries
        .filter((entry) => entry.type === 'directory')
        .map((entry) => this.readManifestByDir(entry.name))
    );
    return manifests
      .filter((manifest): manifest is CachedBookManifest => manifest !== null)
      .map((manifest) => this.toDownloadedBook(manifest));
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
    const toStore: CachedBookManifest = {
      book: { ...manifest.book },
      sources: manifest.sources.map((source) => ({ ...source })),
      downloaded_at: manifest.downloaded_at,
      local_version: manifest.local_version,
      remote_version: manifest.remote_version
    };
    await this.writeJsonFile(manifestPath(manifest.book.id), toStore);
  }

  async removeDownloadedBook(bookId: string): Promise<void> {
    await this.deleteFileIgnoringMissing(manifestPath(bookId));
    await this.rmdirIgnoringMissing(bookDir(bookId));
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
    const content = await this.readTextFile(contentPath(bookId));
    return content !== null ? { content } : null;
  }

  async saveCachedBookContent(bookId: string, content: BookContent): Promise<void> {
    await this.writeTextFile(contentPath(bookId), content.content);
  }

  async getCachedSourceContent(bookId: string, sourceId: string): Promise<string | null> {
    return this.readTextFile(sourcePath(bookId, sourceId));
  }

  async saveCachedSourceContent(bookId: string, sourceId: string, content: string): Promise<void> {
    await this.writeTextFile(sourcePath(bookId, sourceId), content);
  }

  async getReadProgress(bookId: string): Promise<ReadingProgress | null> {
    return this.readJsonFile<ReadingProgress>(progressPath(bookId));
  }

  async saveReadProgress(bookId: string, progress: ReadingProgress): Promise<void> {
    // Independent of the manifest/download state, matching IndexedDbMobileBookCache:
    // progress can be written (and read back) even for a book that was never
    // (or isn't currently) downloaded.
    await this.writeJsonFile(progressPath(bookId), { ...progress });
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
    return this.asManifest(await this.readJsonFile<unknown>(manifestPath(bookId)));
  }

  private async readManifestByDir(dirName: string): Promise<CachedBookManifest | null> {
    return this.asManifest(await this.readJsonFile<unknown>(`${BASE_DIR}/${dirName}/manifest.json`));
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
