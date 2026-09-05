import { Directory, Encoding, Filesystem, type WriteFileOptions } from '@capacitor/filesystem';

// Shared layout and file access for everything the mobile shell keeps on the
// device. Directory.Data is app-private, requires no runtime permission, and —
// unlike IndexedDB in an Android WebView — is not subject to silent LRU
// eviction (see the rationale in providers/index.ts and
// .claude/rules/50-lessons.md).
//
//   plainshelf-cache/scopes/<enc(scopeKey)>/
//       shelf-snapshot.json    # PersistedShelfSnapshot (shelfSnapshotStore.ts)
//       books/<enc(bookId)>/   # downloaded books (filesystemMobileBookCache.ts)
//
// The scope segment keeps two shelves' data apart; see cacheScope.ts for why
// book ids alone cannot.
export const BASE_DIR = 'plainshelf-cache';
const SCOPES_DIR = `${BASE_DIR}/scopes`;
export const CACHE_DIRECTORY = Directory.Data;

// Directory for a client with no (server, shelf) identity yet. Unreachable from
// the native shell, where getApiBase() is applied before mount. Named rather
// than left as an empty segment so the layout has no `//` in it.
export const UNSCOPED_DIR_NAME = '_unscoped';

export function encode(id: string): string {
  return encodeURIComponent(id);
}

/**
 * A short, fixed-length path component standing for an arbitrary name.
 *
 * `encode` suits ids the shelf generates, but not a name a user chose: the shelf
 * accepts 255 UTF-8 bytes and percent-encoding triples a CJK name, so about 29
 * characters already overflow a filesystem component.
 *
 * Callers must still record the exact name inside the file and check it on read:
 * this is a 64-bit non-cryptographic hash, and a collision has to read as a
 * cache miss rather than as the wrong file.
 */
export function hashComponent(value: string): string {
  const bytes = new TextEncoder().encode(value);
  let low = 0x811c9dc5;
  let high = 0x01000193;

  for (const byte of bytes) {
    low = Math.imul(low ^ byte, 0x01000193) >>> 0;
    high = Math.imul(high ^ byte, 0x85ebca6b) >>> 0;
  }

  return low.toString(16).padStart(8, '0') + high.toString(16).padStart(8, '0');
}

export function scopeDir(scopeKey: string): string {
  return `${SCOPES_DIR}/${scopeKey ? encode(scopeKey) : UNSCOPED_DIR_NAME}`;
}

/**
 * Deletes everything one shelf downloaded. Takes the scope key as an argument
 * rather than reading the active one, so removing a shelf the app is *not*
 * reading does not have to repoint the API client at it first; nothing else
 * enumerates scope directories, so this is the only path that reclaims them.
 *
 * Reading history and stats are untouched: they are shared across shelves and
 * are the only device data that cannot be rebuilt from the library.
 */
export async function removeCacheScope(scopeKey: string): Promise<void> {
  // An empty key is not a shelf identity: scopeDir maps it to the shared
  // pre-scope directory, so this would take unrelated downloads with it. An
  // entry that derives no scope never wrote one anyway.
  if (!scopeKey) {
    return;
  }
  await rmdirIgnoringMissing(scopeDir(scopeKey));
}

/**
 * Every failure — missing file, plugin error, unexpected payload — is a cache
 * miss: this data is always reconstructible from the shelf. Writes deliberately
 * do *not* follow the rule, so a caller that must know whether it committed can.
 */
export async function readTextFile(path: string): Promise<string | null> {
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

/** As {@link readTextFile}, with malformed JSON counting as a miss too. */
export async function readJsonFile<T>(path: string): Promise<T | null> {
  const text = await readTextFile(path);
  if (text === null) {
    return null;
  }
  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
  }
}

/**
 * Retries once when another write has already created the directory this one was
 * about to create.
 *
 * `writeFile({ recursive: true })` checks whether the parent exists and then
 * creates it. In the web fallback those are separate IndexedDB round trips, so
 * two writes into the same not-yet-created directory both see it missing, both
 * create it, and the loser fails with "Current directory does already exist."
 * Downloading a book is exactly that shape.
 *
 * The retry does no mkdir at all, since the directory exists by then, so a
 * second collision cannot happen. Only the web fallback needs this — the native
 * plugin creates parents in one `mkdirs` call.
 */
async function writeFileCreatingParents(options: WriteFileOptions): Promise<void> {
  try {
    await Filesystem.writeFile(options);
    return;
  } catch (error) {
    if (!isDirectoryExistsError(error)) {
      throw error;
    }
  }

  await Filesystem.writeFile(options);
}

export async function writeTextFile(path: string, data: string): Promise<void> {
  await writeFileCreatingParents({
    path,
    data,
    directory: CACHE_DIRECTORY,
    encoding: Encoding.UTF8,
    recursive: true
  });
}

export async function writeJsonFile(path: string, value: unknown): Promise<void> {
  await writeTextFile(path, JSON.stringify(value));
}

/**
 * Omitting `encoding` is what makes the plugin treat the file as bytes; it hands
 * them back base64-encoded, which is the shape base64ToBlob wants. A miss
 * degrades to null for the same reason readTextFile's does.
 */
export async function readBinaryFile(path: string): Promise<string | null> {
  try {
    const result = await Filesystem.readFile({ path, directory: CACHE_DIRECTORY });
    return typeof result.data === 'string' ? result.data : null;
  } catch {
    return null;
  }
}

/**
 * Writes a base64 payload as raw bytes.
 *
 * Without `encoding` the plugin decodes the base64 before writing, so what
 * lands on disk is the image itself rather than its ~33% larger text form.
 */
export async function writeBinaryFile(path: string, base64: string): Promise<void> {
  await writeFileCreatingParents({
    path,
    data: base64,
    directory: CACHE_DIRECTORY,
    recursive: true
  });
}

export async function deleteFileIgnoringMissing(path: string): Promise<void> {
  try {
    await Filesystem.deleteFile({ path, directory: CACHE_DIRECTORY });
  } catch (error) {
    if (!isMissingError(error)) {
      throw error;
    }
  }
}

export async function rmdirIgnoringMissing(path: string): Promise<void> {
  try {
    await Filesystem.rmdir({ path, directory: CACHE_DIRECTORY, recursive: true });
  } catch (error) {
    if (!isMissingError(error)) {
      throw error;
    }
  }
}

/**
 * The plugin and its web fallback report a missing file by message rather than
 * by a code, so this has to match text. Also used by storage/deviceDocument.ts,
 * which persists through the same plugin.
 */
export function isMissingError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error);
  return /not exist|does not exist|enoent|no such file|not found/i.test(message);
}

/**
 * Whether a Capacitor Filesystem failure means "that directory is already
 * there". Reported by message rather than by a code, the same way a missing
 * file is; see {@link writeFileCreatingParents} for when it happens.
 */
function isDirectoryExistsError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error);
  return /already exist/i.test(message);
}

export async function blobToBase64(blob: Blob): Promise<string> {
  const bytes = new Uint8Array(await blob.arrayBuffer());
  let binary = '';
  for (let i = 0; i < bytes.length; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

export function base64ToBlob(base64: string, mime: string): Blob {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return new Blob([bytes], mime ? { type: mime } : undefined);
}
