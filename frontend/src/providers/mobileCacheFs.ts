import { Directory, Encoding, Filesystem } from '@capacitor/filesystem';

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
export const SCOPES_DIR = `${BASE_DIR}/scopes`;
export const CACHE_DIRECTORY = Directory.Data;

// Directory for a client that has no (server, shelf) identity yet. Unreachable
// from the native shell: there `getApiBase()` is applied before mount and the
// router holds the app on the connect page until it is, so every key contains
// the `apiBase|shelfID` separator and encodes to a name with `%7C` in it. Named
// rather than left as an empty path segment so the layout has no `//` in it.
export const UNSCOPED_DIR_NAME = '_unscoped';

export function encode(id: string): string {
  return encodeURIComponent(id);
}

/**
 * A short, fixed-length path component standing for an arbitrary name.
 *
 * `encode` suits ids the shelf generates — short and ASCII — but not a name
 * chosen by whoever put a file on the shelf. The shelf accepts up to 255 UTF-8
 * bytes, and percent-encoding triples a CJK name, so about 29 characters
 * already overflow a 255-byte filesystem component and the write fails.
 *
 * The cache is device-private and always looked up by its logical key, so the
 * component has no reason to be readable. Callers must still record the exact
 * name inside the file and check it on read: this is a 64-bit non-cryptographic
 * hash, and a collision has to read as a cache miss rather than as the wrong
 * file.
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
 * Deletes everything one shelf downloaded — book content, covers, per-book
 * progress files and the pCloud shelf snapshot.
 *
 * Takes the scope key as an argument rather than reading the active one, so
 * removing a shelf the app is *not* currently reading does not have to
 * repoint the API client at it first. Nothing else enumerates scope
 * directories, so this is the only path by which that space is reclaimed.
 *
 * Reading history and reading stats are untouched: they live in documents
 * shared across shelves and are the only device data that cannot be rebuilt
 * from the library.
 */
export async function removeCacheScope(scopeKey: string): Promise<void> {
  await rmdirIgnoringMissing(scopeDir(scopeKey));
}

/**
 * Reads a UTF-8 file, treating every failure — missing file, plugin error,
 * unexpected payload type — as a cache miss.
 *
 * Device-local data here is always reconstructible from the shelf, so a read
 * that cannot answer must degrade to "not cached" rather than take down the
 * caller. Writes deliberately do *not* follow this rule: they throw, so a
 * caller that must know whether it committed still can.
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

export async function writeTextFile(path: string, data: string): Promise<void> {
  await Filesystem.writeFile({
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
 * Whether a Capacitor Filesystem failure means "the file is not there".
 *
 * The plugin and its web fallback report it by message rather than by a code,
 * so this has to match text. Also used by storage/deviceDocument.ts, which
 * persists through the same plugin.
 */
export function isMissingError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error);
  return /not exist|does not exist|enoent|no such file|not found/i.test(message);
}
