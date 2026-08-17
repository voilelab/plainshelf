import { currentCacheScopeKey } from './cacheScope';
import type { MobileCoverCache } from './mobileCoverCache';
import {
  base64ToBlob,
  blobToBase64,
  deleteFileIgnoringMissing,
  encode,
  readJsonFile,
  scopeDir,
  writeJsonFile
} from './mobileCacheFs';

// Persistent cover cache for books that have not been downloaded for offline
// reading. Downloaded books already have their covers stored inside the book
// directory by FilesystemMobileBookCache; this cache lives beside it:
//
//   plainshelf-cache/scopes/<enc(scopeKey)>/covers/<enc(bookId)>.json
//
// The scope segment prevents cross-shelf collisions (see cacheScope.ts).
// removeCacheScope's recursive rmdir of the scope directory sweeps this
// directory up together with the downloads.

interface StoredCover {
  mime: string;
  data: string;
}

function coverPath(bookId: string): string {
  return `${scopeDir(currentCacheScopeKey())}/covers/${encode(bookId)}.json`;
}

export class FilesystemMobileCoverCache implements MobileCoverCache {
  async getCover(bookId: string): Promise<Blob | null> {
    const stored = await readJsonFile<Partial<StoredCover>>(coverPath(bookId));
    if (!stored || typeof stored.data !== 'string') {
      return null;
    }
    try {
      return base64ToBlob(stored.data, typeof stored.mime === 'string' ? stored.mime : '');
    } catch {
      return null;
    }
  }

  async setCover(bookId: string, blob: Blob): Promise<void> {
    const stored: StoredCover = { mime: blob.type, data: await blobToBase64(blob) };
    await writeJsonFile(coverPath(bookId), stored);
  }

  async deleteCover(bookId: string): Promise<void> {
    await deleteFileIgnoringMissing(coverPath(bookId));
  }
}
