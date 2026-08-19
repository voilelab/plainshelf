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

// Persistent cover cache for books that have not been downloaded for offline
// reading. Downloaded books already have their covers stored inside the book
// directory by FilesystemMobileBookCache; this cache lives beside it:
//
//   plainshelf-cache/scopes/<enc(scopeKey)>/covers/<enc(bookId)>
//
// The scope segment prevents cross-shelf collisions (see cacheScope.ts).
// removeCacheScope's recursive rmdir of the scope directory sweeps this
// directory up together with the downloads.
//
// The file is the image itself. An earlier version wrapped it as
// {mime, data: base64} JSON, which cost about a third of the space again —
// base64 needs no JSON escaping, so the wrapper was cheap but the encoding was
// not. The type is recovered by looking at the leading bytes instead of being
// stored, which keeps this one file per cover and one read per lookup.

function coverPath(bookId: string): string {
  return `${scopeDir(currentCacheScopeKey())}/covers/${encode(bookId)}`;
}

/** The pre-binary layout, deleted opportunistically on write. */
function legacyCoverPath(bookId: string): string {
  return `${coverPath(bookId)}.json`;
}

/**
 * The image type, from the first bytes of the file.
 *
 * Only the formats a shelf accepts for a cover (see shelf/asset.go) are
 * recognised. An unrecognised header yields '', which makes an untyped Blob —
 * still renderable, since the decoder sniffs the bytes anyway.
 */
function sniffImageMime(bytes: Uint8Array): string {
  if (bytes.length >= 3 && bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) {
    return 'image/jpeg';
  }
  if (
    bytes.length >= 8 &&
    bytes[0] === 0x89 &&
    bytes[1] === 0x50 &&
    bytes[2] === 0x4e &&
    bytes[3] === 0x47
  ) {
    return 'image/png';
  }
  if (bytes.length >= 4 && bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46) {
    return 'image/gif';
  }
  // RIFF....WEBP
  if (
    bytes.length >= 12 &&
    String.fromCharCode(bytes[0], bytes[1], bytes[2], bytes[3]) === 'RIFF' &&
    String.fromCharCode(bytes[8], bytes[9], bytes[10], bytes[11]) === 'WEBP'
  ) {
    return 'image/webp';
  }
  return '';
}

/** The first bytes of a base64 payload, without decoding the whole thing. */
function leadingBytes(base64: string, count: number): Uint8Array {
  // 4 base64 characters carry 3 bytes; round up so `count` bytes are covered.
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
    // No migration pass: a cover is rebuildable, so the pre-binary file is
    // simply dropped the first time this book's cover is written again.
    await deleteFileIgnoringMissing(legacyCoverPath(bookId));
  }

  async deleteCover(bookId: string): Promise<void> {
    await deleteFileIgnoringMissing(coverPath(bookId));
    await deleteFileIgnoringMissing(legacyCoverPath(bookId));
  }
}
