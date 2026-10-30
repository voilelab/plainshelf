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

function coverPath(booksDir: string, bookId: string): string {
  return `${bookDir(booksDir, bookId)}/cover.json`;
}

function assetPath(booksDir: string, bookId: string, sourceId: string, name: string): string {
  return `${bookDir(booksDir, bookId)}/assets/${encode(sourceId)}/${hashComponent(name)}.json`;
}

interface StoredAsset extends StoredCover {
  name: string;
}

interface StoredCover {
  mime: string;
  data: string;
}

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
    if (stored.name !== name) {
      return null;
    }
    try {
      return base64ToBlob(stored.data, typeof stored.mime === 'string' ? stored.mime : '');
    } catch {
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

  private async resolveBooksDir(): Promise<string> {
    const scopeKey = currentCacheScopeKey();
    await this.ensureLegacyMigrated(scopeKey);
    return `${scopeDir(scopeKey)}/books`;
  }

  private async ensureLegacyMigrated(scopeKey: string): Promise<void> {
    if (!scopeKey) {
      return;
    }
    if (!this.legacyMigration) {
      this.legacyMigration = this.migrateLegacyBooks(scopeKey);
    }
    await this.legacyMigration;
  }

  private async migrateLegacyBooks(scopeKey: string): Promise<void> {
    if (!(await this.isDirectory(LEGACY_BOOKS_DIR))) {
      return;
    }

    const target = `${scopeDir(scopeKey)}/books`;
    if (await this.isDirectory(target)) {
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
