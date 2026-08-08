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

export function scopeDir(scopeKey: string): string {
  return `${SCOPES_DIR}/${scopeKey ? encode(scopeKey) : UNSCOPED_DIR_NAME}`;
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
