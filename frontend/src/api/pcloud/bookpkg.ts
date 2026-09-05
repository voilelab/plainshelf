import type { Book, BookFormat } from '@/types/book';
import type { SourceMeta } from '@/types/source';
import { PCloudDataError } from './errors';
import type { PCloudItem } from './types';

// Layout constants mirroring the Go shelf (shelf/shelf.go, shelf/book.go,
// shelf/source.go) and documented in docs/concepts/data-model.md. They are
// duplicated here rather than derived because this client reads the on-disk
// format directly, without the server in between.
export const BOOKS_FOLDER = 'books';
export const BOOK_EXTENSION = '.bookpkg';
const BOOK_META_FILE = 'book.json';
const SOURCES_FOLDER = 'sources';
const SOURCE_META_FILE = 'meta.json';
const SOURCE_FILE = 'source.txt';
const SOURCE_ASSETS_FOLDER = 'assets';

/**
 * The shelf's own settings file. Optional; only ever read, as in
 * shelf/shelf_config.go.
 */
const SHELF_CONFIG_FILE = 'shelf.json';

/**
 * Matches `maxShelfConfigBytes` in shelf/shelf_config.go, so a mis-named large
 * file in the shelf root is skipped rather than downloaded onto a phone.
 */
export const MAX_SHELF_CONFIG_BYTES = 1 << 20;

/** Mirrors `IgnoredDir` in shelf/internal/shelfutil. */
interface IgnoredDir {
  /** As written; matching folds case. */
  name: string;
  /** A short phrase completing "skipped because …". May be empty. */
  reason: string;
}

/**
 * Mirrors `DefaultIgnoredDirs` in shelf/internal/shelfutil, which explains why
 * each name is here. Defaults, not a floor: a shelf.json that lists its own
 * directories replaces them.
 */
export const DEFAULT_IGNORED_DIRS: readonly IgnoredDir[] = [
  { name: '@eaDir', reason: 'Synology index and thumbnail sidecar' },
  { name: '#recycle', reason: 'Synology network recycle bin' },
  { name: '$RECYCLE.BIN', reason: 'Windows recycle bin, visible over SMB' },
  { name: 'lost+found', reason: 'ext filesystem recovery directory' }
];

/**
 * Mirrors `IgnoreRules` in shelf/internal/shelfutil: a name on the shelf's list,
 * or any hidden directory — the one part of the rules a shelf cannot configure
 * away.
 */
type IgnoreRules = (name: string) => boolean;

/** The rules for a shelf that has said nothing about scanning. */
export const DEFAULT_IGNORE_RULES: IgnoreRules = createIgnoreRules(DEFAULT_IGNORED_DIRS);

/** Mirrors `NSFWFolder` in shelf/internal/shelfutil. */
export interface NSFWFolder {
  /** A "/"-separated folder path as written; matching folds case. */
  path: string;
  /** A short phrase completing "marked because …". May be empty. */
  reason: string;
}

/**
 * Mirrors `NSFWRules` in shelf/internal/shelfutil: whether a book's folder path
 * lies in a subtree this shelf marks as adult content. A rule marks its own
 * folder and every folder below it.
 */
type NSFWRules = (folders: readonly string[]) => boolean;

/**
 * Mirrors `NSFWRules.Match`: the listed rule marking a path, rather than only
 * whether one does. A reader that has to *say* where a mark came from needs the
 * entry itself, and asking twice would be two walks down the same path.
 */
export type NSFWFolderLookup = (folders: readonly string[]) => NSFWFolder | undefined;

/** What this client reads out of `shelf.json`. */
export interface ShelfConfig {
  /**
   * The directories this shelf skips, replacing the defaults — including when
   * empty, which means "skip nothing but hidden directories". Undefined when the
   * shelf said nothing, which means the defaults.
   */
  ignoredDirs?: IgnoredDir[];

  /**
   * The folder subtrees this shelf marks as adult content. Unlike `ignoredDirs`
   * there is no built-in list to replace, so undefined and empty mean the same
   * thing: this shelf marks no folder.
   */
  nsfwFolders?: NSFWFolder[];
}

/** 255 bytes is the per-component limit the Go side validates against. */
const MAX_PATH_SEGMENT_BYTES = 255;

function isUsableDirName(name: string): boolean {
  if (name === '' || name === '.' || name === '..') {
    return false;
  }
  if (name.includes('/') || name.includes('\\')) {
    return false;
  }
  return new TextEncoder().encode(name).length <= MAX_PATH_SEGMENT_BYTES;
}

/**
 * JSON null stands in for an absent member throughout this file: the Go side
 * decodes into a struct, where null leaves the zero value rather than failing.
 */
function isAbsent(value: unknown): boolean {
  return value === undefined || value === null;
}

function isJsonObject(value: unknown): boolean {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * One entry's `reason`, or undefined when the entry has to be dropped over it.
 *
 * A wrong type takes the whole entry with it rather than defaulting to empty:
 * the Go side unmarshals the entry into a {name|path, reason} struct, so a
 * number there fails the entry and the directory is not skipped, or the folder
 * not marked. Defaulting here would skip or mark it, which is the two readers
 * disagreeing about the same file.
 */
function parseReason(reason: unknown): string | undefined {
  if (isAbsent(reason)) {
    return '';
  }
  return typeof reason === 'string' ? reason : undefined;
}

/**
 * An entry is always a {name, reason} object; a bare name is not accepted, for
 * the reason parseIgnoredDir gives in shelf/shelf_config.go.
 */
function parseIgnoredDir(entry: unknown): IgnoredDir | undefined {
  if (!isJsonObject(entry)) {
    return undefined;
  }

  const { name, reason } = entry as { name?: unknown; reason?: unknown };
  const reasonText = parseReason(reason);
  if (typeof name !== 'string' || !isUsableDirName(name) || reasonText === undefined) {
    return undefined;
  }
  return { name, reason: reasonText };
}

/**
 * An entry is always a {path, reason} object, for the same reason parseIgnoredDir
 * refuses a bare name: one entry, one shape.
 */
function parseNSFWFolder(entry: unknown): NSFWFolder | undefined {
  if (!isJsonObject(entry)) {
    return undefined;
  }

  const { path, reason } = entry as { path?: unknown; reason?: unknown };
  const reasonText = parseReason(reason);
  if (typeof path !== 'string' || normalizeFolderPath(path) === undefined || reasonText === undefined) {
    return undefined;
  }
  return { path, reason: reasonText };
}

/**
 * Whether the Go side would read this file at all.
 *
 * `json.UnmarshalRead` fills one struct, so a member of the wrong container type
 * fails the *whole file* and the shelf keeps its defaults — it does not fall
 * back section by section. A reader that dropped only the bad section would
 * apply the other one where Go applies nothing, so a file with a broken `scan`
 * and a valid `content` would mark folders on a phone and nowhere else.
 *
 * Unknown members are accepted here as they are there, so a file from a newer
 * build still reads.
 */
function isReadableShelfConfig(raw: unknown): boolean {
  if (!isJsonObject(raw)) {
    return false;
  }

  const { schema_version: schemaVersion, scan, content } = raw as Record<string, unknown>;
  if (!isAbsent(schemaVersion) && !Number.isInteger(schemaVersion)) {
    return false;
  }
  return isReadableSection(scan, 'ignored_dirs') && isReadableSection(content, 'nsfw_folders');
}

function isReadableSection(section: unknown, member: string): boolean {
  if (isAbsent(section)) {
    return true;
  }
  if (!isJsonObject(section)) {
    return false;
  }
  const listed = (section as Record<string, unknown>)[member];
  return isAbsent(listed) || Array.isArray(listed);
}

/** The listed entries of one section, or undefined when the section has none. */
function listedEntries(section: unknown, member: string): unknown[] | undefined {
  const listed = (section as Record<string, unknown> | null | undefined)?.[member];
  return Array.isArray(listed) ? listed : undefined;
}

/**
 * Never throws: a file from a newer build, or one whose shape this reader cannot
 * use, leaves the defaults in place rather than making the shelf unreadable — as
 * the Go side does with the same file. Within a file it can read, unusable
 * entries are dropped one by one, so one bad line does not cost the rest.
 */
export function parseShelfConfig(raw: unknown): ShelfConfig {
  if (!isReadableShelfConfig(raw)) {
    return {};
  }

  const { scan, content } = raw as Record<string, unknown>;
  const config: ShelfConfig = {};

  const dirs = listedEntries(scan, 'ignored_dirs');
  if (dirs) {
    config.ignoredDirs = dirs.map(parseIgnoredDir).filter((dir) => dir !== undefined);
  }

  const folders = listedEntries(content, 'nsfw_folders');
  if (folders) {
    config.nsfwFolders = folders.map(parseNSFWFolder).filter((folder) => folder !== undefined);
  }

  return config;
}

/**
 * The list is the rules, not an addition to them: an empty list skips nothing
 * but hidden directories. Pass DEFAULT_IGNORED_DIRS for a shelf that said
 * nothing.
 */
export function createIgnoreRules(dirs: readonly IgnoredDir[]): IgnoreRules {
  const names = new Set(dirs.filter((dir) => dir.name !== '').map((dir) => dir.name.toLowerCase()));
  return (name: string) => name.startsWith('.') || names.has(name.toLowerCase());
}

/**
 * Folds one path segment into a map key two spellings differing only in case
 * share — `foldSegment` in shelf/internal/shelfutil, which lowercases and then
 * takes the fold orbit's representative.
 *
 * JavaScript has no `unicode.SimpleFold`, so the orbit is reached by going back
 * up and down again: lowercase, uppercase, lowercase. Neither pass alone is
 * enough. Lowercasing misses Greek final sigma ("Σ" lowercases to "σ" while "ς"
 * lowercases to itself, so a folder rule is silently missed), and folding alone
 * leaves "İ" apart from "istanbul". Two guards keep the round trip from merging
 * more than Unicode does: an uppercase mapping that expands ("ß" to "SS", "ﬁ" to
 * "FI") is not a case pair and is skipped, and FOLD_EXCEPTIONS covers the four
 * code points the round trip still gets wrong.
 *
 * Checked exhaustively against the Go side, not by inspection: over all
 * 1,112,064 code points this produces exactly the equivalence classes
 * `minFold(unicode.ToLower(r))` does. The two sides read their own Unicode
 * tables, so a script added in a newer Unicode than one of the two runtimes
 * carries is the one way they can still disagree.
 *
 * IgnoreRules still matches on the lowercase name alone, as it does in Go: that
 * is shipped behavior deciding which directories a shelf skips.
 */
function foldSegment(segment: string): string {
  let folded = '';
  for (const character of segment) {
    folded += foldCharacter(character);
  }
  return folded;
}

/**
 * Where lowercase-uppercase-lowercase does not land on Go's representative.
 * "ı" is excluded from folding with "i" and the round trip would merge them;
 * the other three are orbits Go folds together but whose uppercase expands, so
 * the guard above would leave them apart. Keyed by the lowercased character.
 */
const FOLD_EXCEPTIONS = new Map<string, string>([
  ['\u0131', '\u0131'], // dotless i, which folds with nothing
  ['\u1FD3', '\u0390'], // ΐ, spelled two ways
  ['\u1FE3', '\u03B0'], // ΰ, spelled two ways
  ['\uFB06', '\uFB05'] // the st ligature, spelled two ways
]);

function foldCharacter(character: string): string {
  const lower = simpleLowerCase(character);
  const exception = FOLD_EXCEPTIONS.get(lower);
  if (exception !== undefined) {
    return exception;
  }

  const upper = [...lower.toUpperCase()];
  return upper.length === 1 ? simpleLowerCase(upper[0]) : lower;
}

/**
 * `unicode.ToLower` for one character: the simple 1:1 mapping, where JavaScript
 * applies Unicode's full one. "İ" is the only character whose unconditional
 * lowercase expands (to "i" plus a combining dot), and Go maps it to the "i".
 */
function simpleLowerCase(character: string): string {
  return [...character.toLowerCase()][0] ?? character;
}

/**
 * Folds a written folder path into the key the rules are stored under, or
 * undefined when it names no folder — `normalizeFolderPath` in
 * shelf/internal/shelfutil. Empty segments are dropped, so a leading slash, a
 * trailing slash and a doubled separator all name the same folder; a path with
 * no segment at all is refused, because "" would mark the whole shelf and is far
 * more likely to be an empty field than a decision.
 */
function normalizeFolderPath(folderPath: string): string | undefined {
  const folded: string[] = [];
  for (const segment of folderPath.split('/')) {
    if (segment === '') {
      continue;
    }
    if (!isUsableDirName(segment)) {
      return undefined;
    }
    folded.push(foldSegment(segment));
  }
  return folded.length > 0 ? folded.join('/') : undefined;
}

/**
 * Mirrors `NewNSFWRules` in shelf/internal/shelfutil: the paths are normalized
 * once, because a listing asks this question once per book.
 *
 * There is no built-in list, so an empty one — the shelf that said nothing —
 * marks nothing. An entry that cannot name a folder is dropped on its own.
 */
export function createNSFWFolderLookup(folders: readonly NSFWFolder[]): NSFWFolderLookup {
  const marked = new Map<string, NSFWFolder>();
  for (const folder of folders) {
    const key = normalizeFolderPath(folder.path);
    if (key !== undefined) {
      // Last entry wins for one path, as assigning into Go's map does.
      marked.set(key, folder);
    }
  }
  if (marked.size === 0) {
    return () => undefined;
  }

  // Walking down from the root asks whether any prefix of the path is listed,
  // since a rule marks everything below it. Comparing folded segment by folded
  // segment is what keeps "Fiction/成人" off "Fiction/成人漫畫", which a plain
  // string prefix test would mark. The shallowest match wins, for the reason
  // `NSFWRules.Match` gives: it is the rule that would still mark the folder if
  // every deeper entry were removed.
  return (path) => {
    let key = '';
    for (const segment of path) {
      key = key === '' ? foldSegment(segment) : `${key}/${foldSegment(segment)}`;
      const folder = marked.get(key);
      if (folder !== undefined) {
        return folder;
      }
    }
    return undefined;
  };
}

/** The same question as {@link createNSFWFolderLookup}, asked as a yes or no. */
export function createNSFWRules(folders: readonly NSFWFolder[]): NSFWRules {
  const lookup = createNSFWFolderLookup(folders);
  return (path) => lookup(path) !== undefined;
}

/**
 * Mirrors `Shelf.IsBookNSFW`: the shelf's answer for one book, assembled from
 * the folder rules in `shelf.json` and the book's own `nsfw`.
 *
 * The two sources add, they do not override. `nsfw: false` on a book does NOT
 * take it out of a marked folder — the failure that matters here is a book that
 * should have been marked and quietly was not.
 */
export function isBookNSFW(isNSFWFolder: NSFWRules, folders: readonly string[], meta: BookJson): boolean {
  return meta.nsfw === true || isNSFWFolder(folders);
}

/** Locates `shelf.json` in a listed shelf root, if the shelf has one. */
export function findShelfConfigFile(shelfRoot: PCloudItem): PCloudFileRef | undefined {
  const item = shelfRoot.contents?.find((entry) => !entry.isfolder && entry.name === SHELF_CONFIG_FILE);
  return item ? toFileRef(item) : undefined;
}

/**
 * The book.json schema version this reader understands, matching
 * `BookMetaSchemaVersion` in shelf/book.go.
 *
 * A newer file is still read: the Go side reads best-effort too and only
 * refuses to *write* it (see `Book.EnsureWritable`). Since this client never
 * writes, the version is carried onto the book as
 * `schema_newer_than_supported` for the UI to warn about, rather than used to
 * hide the book.
 */
export const BOOK_META_SCHEMA_VERSION = 1;

/**
 * A file located in the recursive listing.
 *
 * `size` and `modified` come straight from the listing, so a caller can decide
 * whether a cached copy is still current without spending a request — the same
 * freshness rule the Go shelf applies via `FileStat` (shelf/filestate.go).
 */
export interface PCloudFileRef {
  fileid: number;
  name: string;
  size: number;
  modified: string;
}

export interface BookSourceRef {
  id: string;
  folderid: number;
  meta?: PCloudFileRef;
  content?: PCloudFileRef;
  /**
   * Illustrations in the source's `assets/` directory, keyed by file name.
   *
   * Indexed from the listing that was already fetched rather than looked up on
   * demand: the shelf is read through one recursive listing, so an image costs
   * nothing extra to find and a book with none carries an empty record.
   */
  assets: Record<string, PCloudFileRef>;
}

export interface BookPackageRef {
  /** Directory name including the `.bookpkg` suffix. Not the book's identity. */
  folderName: string;
  folderid: number;
  /** Directory names between `books/` and this package, outermost first. */
  folders: string[];
  /** Absent when the package has no book.json and cannot be read. */
  meta?: PCloudFileRef;
  /** Files directly inside the package, keyed by name (cover, pointer file, …). */
  files: Record<string, PCloudFileRef>;
  sources: BookSourceRef[];
}

/** The on-disk shape of book.json — see `BookMeta` in shelf/book.go. */
export interface BookJson {
  schema_version?: number;
  id: string;
  title: string;
  format?: string;
  tags?: string[];
  identifiers?: Record<string, string>;
  cover?: string;
  authors?: string[];
  language?: string;
  comments?: string;
  star?: number;
  /**
   * The book's own mark. Optional because absent means false in book.json, and
   * because a shelf snapshot persisted by a build before this field existed is
   * read back through this type.
   */
  nsfw?: boolean;
  created_at?: string;
  updated_at?: string;
  published_at?: string;
  current_source?: string;
}

function toFileRef(item: PCloudItem): PCloudFileRef | undefined {
  if (item.isfolder || item.fileid === undefined) {
    return undefined;
  }
  return {
    fileid: item.fileid,
    name: item.name,
    size: item.size ?? 0,
    modified: item.modified ?? ''
  };
}

function findFolder(parent: PCloudItem | undefined, name: string): PCloudItem | undefined {
  return parent?.contents?.find((item) => item.isfolder && item.name === name);
}

/**
 * Locates the `books/` directory inside a listed shelf root.
 *
 * Returns undefined rather than throwing so a caller can tell "this folder is
 * not a shelf" from "the request failed", which are different problems for the
 * user to fix.
 */
export function findBooksFolder(shelfRoot: PCloudItem): PCloudItem | undefined {
  return findFolder(shelfRoot, BOOKS_FOLDER);
}

/**
 * Walks a listed `books/` tree and returns every book package it contains.
 *
 * Directories are folders until one ends in `.bookpkg`; that one is a book and is
 * not descended into further, matching how the Go shelf scans the tree
 * (shelf/shelf_book.go). System directories are skipped before that test, so a
 * package inside one is not a book either.
 */
export function collectBookPackages(
  booksFolder: PCloudItem,
  isIgnored: IgnoreRules = DEFAULT_IGNORE_RULES
): BookPackageRef[] {
  const packages: BookPackageRef[] = [];

  const walk = (folder: PCloudItem, folders: string[]): void => {
    for (const item of folder.contents ?? []) {
      if (!item.isfolder || item.folderid === undefined || isIgnored(item.name)) {
        continue;
      }

      if (item.name.endsWith(BOOK_EXTENSION)) {
        packages.push(readBookPackage(item, folders));
        continue;
      }

      walk(item, [...folders, item.name]);
    }
  };

  walk(booksFolder, []);
  return packages;
}

/**
 * Lists every folder under a listed `books/` tree, in the shape `getFolders()`
 * returns (`'/'` for the top level, `Fiction/Classics` for a nested one).
 *
 * Derived from the directories themselves, not from the books found in them, so
 * a folder holding no books is still listed — the Go side walks real directories
 * too (`iterateFolders` in shelf/shelf_folder.go) and `books/` itself counts as
 * the "no folder" group. System directories are not folders, for the same reason
 * the Go scan refuses to make one (`ErrIgnoredFolderName`).
 */
export function collectFolders(
  booksFolder: PCloudItem,
  isIgnored: IgnoreRules = DEFAULT_IGNORE_RULES
): string[] {
  const paths = new Set<string>(['/']);

  const walk = (folder: PCloudItem, segments: string[]): void => {
    for (const item of folder.contents ?? []) {
      if (
        !item.isfolder ||
        item.folderid === undefined ||
        item.name.endsWith(BOOK_EXTENSION) ||
        isIgnored(item.name)
      ) {
        continue;
      }
      const next = [...segments, item.name];
      paths.add(next.join('/'));
      walk(item, next);
    }
  };

  walk(booksFolder, []);
  return Array.from(paths).sort((a, b) => a.localeCompare(b));
}

function readBookPackage(pkg: PCloudItem, folders: string[]): BookPackageRef {
  const files: Record<string, PCloudFileRef> = {};
  let sources: BookSourceRef[] = [];

  for (const item of pkg.contents ?? []) {
    if (item.isfolder) {
      if (item.name === SOURCES_FOLDER) {
        sources = readSources(item);
      }
      continue;
    }

    const ref = toFileRef(item);
    if (ref) {
      files[ref.name] = ref;
    }
  }

  return {
    folderName: pkg.name,
    folderid: pkg.folderid as number,
    folders,
    meta: files[BOOK_META_FILE],
    files,
    sources
  };
}

function readSources(sourcesFolder: PCloudItem): BookSourceRef[] {
  const sources: BookSourceRef[] = [];

  for (const item of sourcesFolder.contents ?? []) {
    if (!item.isfolder || item.folderid === undefined) {
      continue;
    }

    const entry: BookSourceRef = { id: item.name, folderid: item.folderid, assets: {} };
    for (const child of item.contents ?? []) {
      const ref = toFileRef(child);
      if (!ref) {
        continue;
      }
      if (ref.name === SOURCE_META_FILE) {
        entry.meta = ref;
      } else if (ref.name === SOURCE_FILE) {
        entry.content = ref;
      }
    }

    // Illustrations sit one level deeper, in a flat directory beside the text.
    for (const child of findFolder(item, SOURCE_ASSETS_FOLDER)?.contents ?? []) {
      const ref = toFileRef(child);
      if (ref) {
        entry.assets[ref.name] = ref;
      }
    }

    sources.push(entry);
  }

  // Source ids are `YYYYMMDD-HHMMSS` timestamps, so lexical order is
  // chronological. pCloud does not promise a listing order.
  sources.sort((a, b) => a.id.localeCompare(b.id));
  return sources;
}

/** Resolves the cover file a book.json points at, if it is present. */
export function findCoverFile(pkg: BookPackageRef, meta: BookJson): PCloudFileRef | undefined {
  const name = meta.cover?.trim();
  return name ? pkg.files[name] : undefined;
}

/** Finds the source a book.json marks current, falling back to the newest one. */
export function findCurrentSource(pkg: BookPackageRef, meta: BookJson): BookSourceRef | undefined {
  const current = meta.current_source?.trim();
  if (current) {
    const match = pkg.sources.find((source) => source.id === current);
    if (match) {
      return match;
    }
  }
  return pkg.sources[pkg.sources.length - 1];
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
}

/**
 * Validates a decoded book.json.
 *
 * `id` is required and used verbatim: it is assigned once when the book is
 * created and never recomputed from the folder name or title
 * (docs/concepts/data-model.md). Deriving an id here — from the directory name
 * or from pCloud's fileid — would break reading progress the moment a book is
 * renamed or moved.
 */
export function parseBookJson(raw: unknown): BookJson {
  if (!raw || typeof raw !== 'object') {
    throw new PCloudDataError('book.json is not a JSON object.');
  }

  const data = raw as Record<string, unknown>;
  const id = typeof data.id === 'string' ? data.id.trim() : '';
  if (!id) {
    throw new PCloudDataError('book.json has no id.');
  }

  return {
    schema_version: typeof data.schema_version === 'number' ? data.schema_version : undefined,
    id,
    title: typeof data.title === 'string' ? data.title : '',
    format: typeof data.format === 'string' ? data.format : undefined,
    tags: asStringArray(data.tags),
    identifiers:
      data.identifiers && typeof data.identifiers === 'object'
        ? (data.identifiers as Record<string, string>)
        : undefined,
    cover: typeof data.cover === 'string' ? data.cover : undefined,
    authors: asStringArray(data.authors),
    language: typeof data.language === 'string' ? data.language : undefined,
    comments: typeof data.comments === 'string' ? data.comments : undefined,
    star: typeof data.star === 'number' ? data.star : undefined,
    nsfw: data.nsfw === true,
    created_at: typeof data.created_at === 'string' ? data.created_at : undefined,
    updated_at: typeof data.updated_at === 'string' ? data.updated_at : undefined,
    published_at: typeof data.published_at === 'string' ? data.published_at : undefined,
    current_source: typeof data.current_source === 'string' ? data.current_source : undefined
  };
}

/** Reports a book.json written by a newer build than this reader knows about. */
export function isSchemaNewerThanSupported(meta: BookJson): boolean {
  return (meta.schema_version ?? 0) > BOOK_META_SCHEMA_VERSION;
}

/**
 * Maps a book.json onto the UI's Book type.
 *
 * `cover_url` is left unset here: there is no stable URL to point an `<img>` at,
 * because pCloud download links expire, and this function does not know whether
 * the named cover file actually exists. The provider fills it in with a
 * presence marker once it has resolved the file, and the bytes are fetched as a
 * blob — which is what the mobile runtime requires anyway
 * (frontend/src/composables/useCoverSrc.ts).
 *
 * `schema_newer_than_supported` is set only for a book this reader knows it read
 * incompletely, so a book written by a version this one understands carries the
 * same fields it always did.
 *
 * The adult-content mark has two halves and they are carried separately, as the
 * server carries them: `nsfw` is the book's own, out of its book.json, and
 * `nsfwFolder` is the shelf.json rule reaching it — pass the one
 * `createNSFWFolderLookup` returns for `folders`, and nothing when none does.
 */
export function toBook(meta: BookJson, folders: string[], nsfwFolder?: NSFWFolder): Book {
  const book: Book = {
    id: meta.id,
    title: meta.title,
    authors: meta.authors ?? [],
    language: meta.language,
    format: (meta.format as BookFormat) || 'txt',
    tags: meta.tags ?? [],
    // book.json spells this "comments"; the UI type uses the singular.
    comment: meta.comments,
    cover: meta.cover?.trim() ?? '',
    folders,
    created_at: meta.created_at,
    updated_at: meta.updated_at,
    published_at: meta.published_at,
    current_source: meta.current_source,
    star: meta.star ?? 0,
    identifiers: meta.identifiers,
    nsfw: meta.nsfw === true
  };

  if (nsfwFolder) {
    // `reason` is omitted rather than left empty, matching the server's
    // `omitempty`, so a client with nothing to quote names the path instead.
    book.nsfw_folder = nsfwFolder.reason
      ? { path: nsfwFolder.path, reason: nsfwFolder.reason }
      : { path: nsfwFolder.path };
  }

  return isSchemaNewerThanSupported(meta) ? { ...book, schema_newer_than_supported: true } : book;
}

/** Maps a source's meta.json onto the UI's SourceMeta type. */
export function toSourceMeta(raw: unknown, fallbackID: string): SourceMeta {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    schema_version: typeof data.schema_version === 'number' ? Math.trunc(data.schema_version) : undefined,
    id: typeof data.id === 'string' && data.id.trim() ? data.id : fallbackID,
    created_at: typeof data.created_at === 'string' ? data.created_at : '',
    comment: typeof data.comment === 'string' ? data.comment : '',
    md5_hash: typeof data.md5_hash === 'string' ? data.md5_hash : '',
    format: data.format === 'txt' || data.format === 'md' ? data.format : undefined,
    line_count: typeof data.line_count === 'number' ? data.line_count : undefined,
    char_count: typeof data.char_count === 'number' ? data.char_count : undefined
  };
}
