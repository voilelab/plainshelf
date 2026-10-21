import { currentCacheScopeKey } from './cacheScope';
import type { MobileCoverCache } from './mobileCoverCache';
import {
  base64ToBlob,
  blobToBase64,
  deleteFileIgnoringMissing,
  encode,
  readBinaryFile,
  scopeDir,
  writeBinaryFile
} from './mobileCacheFs';


function coverPath(bookId: string): string {
  return `${scopeDir(currentCacheScopeKey())}/covers/${encode(bookId)}`;
}

function legacyCoverPath(bookId: string): string {
  return `${coverPath(bookId)}.json`;
}

function sniffImageMime(bytes: Uint8Array): string {
  if (bytes.length >= 3 && bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) {
    return 'image/jpeg';
  }
  if (
    bytes.length >= 8 &&
    bytes[0] === 0x89 &&
    bytes[1] === 0x50 &&
    bytes[2] === 0x4e &&
    bytes[3] === 0x47 &&
    bytes[4] === 0x0d &&
    bytes[5] === 0x0a &&
    bytes[6] === 0x1a &&
    bytes[7] === 0x0a
  ) {
    return 'image/png';
  }
  if (
    bytes.length >= 6 &&
    bytes[0] === 0x47 &&
    bytes[1] === 0x49 &&
    bytes[2] === 0x46 &&
    bytes[3] === 0x38 &&
    (bytes[4] === 0x37 || bytes[4] === 0x39) &&
    bytes[5] === 0x61
  ) {
    return 'image/gif';
  }
  if (
    bytes.length >= 12 &&
    String.fromCharCode(bytes[0], bytes[1], bytes[2], bytes[3]) === 'RIFF' &&
    String.fromCharCode(bytes[8], bytes[9], bytes[10], bytes[11]) === 'WEBP'
  ) {
    return 'image/webp';
  }
  return '';
}

function leadingBytes(base64: string, count: number): Uint8Array {
  const head = base64.slice(0, Math.ceil(count / 3) * 4);
  const binary = atob(head);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

export class FilesystemMobileCoverCache implements MobileCoverCache {
  async getCover(bookId: string): Promise<Blob | null> {
    const base64 = await readBinaryFile(coverPath(bookId));
    if (!base64) {
      return null;
    }
    try {
      return base64ToBlob(base64, sniffImageMime(leadingBytes(base64, 12)));
    } catch {
      return null;
    }
  }

  async setCover(bookId: string, blob: Blob): Promise<void> {
    await writeBinaryFile(coverPath(bookId), await blobToBase64(blob));
    await deleteFileIgnoringMissing(legacyCoverPath(bookId));
  }

  async deleteCover(bookId: string): Promise<void> {
    await deleteFileIgnoringMissing(coverPath(bookId));
    await deleteFileIgnoringMissing(legacyCoverPath(bookId));
  }
}
