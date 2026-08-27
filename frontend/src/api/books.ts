import type {
  Book,
  BookCreateRequest,
  BookContent,
  BookFormat,
  BookUpdateRequest,
  PaginatedBooks,
  TrashedBook,
} from '@/types/book';
import {
  ApiError,
  buildShelfApiPath,
  buildApiUrl,
  fetchBlob,
  fetchJson,
  fetchText,
  isMockApiMode
} from './client';
import { delay } from './mocks/latency';
import {
  mockDeleteBook,
  mockDeleteTrashedBook,
  mockEmptyTrash,
  mockGetBook,
  mockGetBookContent,
  mockCopyBook,
  mockGetBookCover,
  mockImportBook,
  mockListBooks,
  mockListTrashedBooks,
  mockRefreshContentStats,
  mockStartFingerprintSources,
  mockRestoreTrashedBook,
  mockTransferBook,
  mockUpdateBook,
  mockUpdateBookFolder
} from './mocks/books';

interface BackendBookMeta {
  // On-disk format version of book.json, managed by the server. Documented here
  // for reference; the UI does not consume it yet.
  schema_version?: number;
  id: string;
  title: string;
  authors: string[];
  language: string;
  format: string;
  tags: string[];
  cover: string;
  comment?: string;
  comments?: string;
  created_at?: string;
  updated_at?: string;
  published_at?: string;
  current_source?: string;
  star?: number;
  identifiers?: Record<string, string>;
}

interface BackendBook {
  meta: BackendBookMeta;
  folder?: string[];
  // Sibling of `meta`, not nested inside it — matches server/handle_books.go's
  // `Book` struct, which only populates this when the request was made with
  // `include=char_count` (see ListBooksOptions.includeCharCount below).
  char_count?: number;
}

interface BackendTrashedBook {
  id: string;
  title: string;
  authors?: string[];
  original_path?: string;
  original_folder?: string[];
  deleted_at?: string;
}

async function uploadBookCoverInternal(bookID: string, file: File): Promise<void> {
  // In Wails/WebView runtime, passing `File` directly as fetch body can result in
  // an empty request body (0 bytes) for same-origin custom protocol requests.
  // Sending a concrete byte payload avoids that runtime-specific body stream issue.
  const payload = new Uint8Array(await file.arrayBuffer());
  const contentType = resolveCoverUploadContentType(file);

  await fetchJson<void>(buildShelfApiPath(`/books/${encodeURIComponent(bookID)}/cover`), {
    method: 'PUT',
    headers: {
      'Content-Type': contentType
    },
    body: payload
  });
}

const coverMimeTypeByExt: Record<string, string> = {
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  png: 'image/png',
  webp: 'image/webp',
  gif: 'image/gif'
};

function inferCoverMimeTypeFromFilename(fileName: string): string | undefined {
  const match = /\.([a-z0-9]+)$/i.exec(fileName.trim());
  if (!match) {
    return undefined;
  }
  return coverMimeTypeByExt[match[1].toLowerCase()];
}

function resolveCoverUploadContentType(file: File): string {
  const type = file.type.trim().toLowerCase();
  if (type.length > 0 && type !== 'application/octet-stream') {
    return type;
  }
  const inferred = inferCoverMimeTypeFromFilename(file.name);
  if (inferred) {
    return inferred;
  }
  return type || 'application/octet-stream';
}

async function deleteBookCoverInternal(bookID: string): Promise<void> {
  await fetchJson<void>(buildShelfApiPath(`/books/${encodeURIComponent(bookID)}/cover`), {
    method: 'DELETE'
  });
}

function transformBook(b: BackendBook): Book {
  const folders = b.folder ?? [];
  const cover = b.meta.cover?.trim() ?? '';

  return {
    id: b.meta.id,
    title: b.meta.title,
    authors: b.meta.authors ?? [],
    language: b.meta.language,
    format: (b.meta.format as BookFormat) || 'txt',
    tags: b.meta.tags ?? [],
    comment: b.meta.comment ?? b.meta.comments,
    cover,
    cover_url: cover ? buildApiUrl(buildShelfApiPath(`/books/${encodeURIComponent(b.meta.id)}/cover`)) : undefined,
    folders,
    created_at: b.meta.created_at,
    updated_at: b.meta.updated_at,
    published_at: b.meta.published_at,
    current_source: b.meta.current_source,
    star: b.meta.star ?? 0,
    identifiers: b.meta.identifiers,
    char_count: b.char_count
  };
}

const PAGE_SIZE_DEFAULT = 8;

export interface ListBooksOptions {
  /** Ask the backend to also compute/return each book's char_count. Opt-in
   *  because the backend may need to do extra work to produce it; omit for
   *  call sites (library grid, folder tree, etc.) that don't display it. */
  includeCharCount?: boolean;
}

export async function listBooks(page = 1, pageSize = PAGE_SIZE_DEFAULT, opts?: ListBooksOptions): Promise<PaginatedBooks> {
  if (isMockApiMode()) {
    return delay(mockListBooks(page, pageSize));
  }

  const path = opts?.includeCharCount ? '/books?include=char_count' : '/books';
  const all = await fetchJson<BackendBook[]>(buildShelfApiPath(path));
  const books = all.map(transformBook);
  const start = (page - 1) * pageSize;
  return { items: books.slice(start, start + pageSize), total: books.length, page, pageSize };
}

export async function getBook(id: string): Promise<Book> {
  if (isMockApiMode()) {
    return delay(mockGetBook(id));
  }

  const b = await fetchJson<BackendBook>(buildShelfApiPath(`/books/${encodeURIComponent(id)}`));
  return transformBook(b);
}

export async function getDuplicateBookGroups(): Promise<string[][]> {
  if (isMockApiMode()) {
    return delay([]);
  }

  return await fetchJson<string[][]>(buildShelfApiPath('/books/duplicate'));
}

/** How two sources are alike, as classified by the server so every client
 *  reads the same vocabulary. `subset` is one book edited down from the other;
 *  `identical_after_normalize` matches after layout is stripped. */
export type SimilarRelation =
  | 'identical_after_normalize'
  | 'subset'
  | 'near_identical'
  | 'same_source';

/** One similar-but-not-identical pair. `a` and `b` are book IDs in a stable
 *  order, and `containment_a` is how much of `a` lies inside `b`. */
export interface SimilarBookPair {
  a: string;
  b: string;
  jaccard: number;
  containment_a: number;
  containment_b: number;
  norm_chars_a: number;
  norm_chars_b: number;
  relation: SimilarRelation;
}

/** Coverage of the fingerprint cache, for the status bar that offers to build
 *  what is missing. `algo` names the ruleset the count was taken under. */
export interface FingerprintStatus {
  total: number;
  fingerprinted: number;
  missing: number;
  algo: {
    normalize: string;
    shingle: string;
    hash: string;
    k: number;
  };
}

/** The body a shelf too costly for the synchronous comparison answers with,
 *  instead of the pair array. `work` is the estimated merge steps the sweep
 *  would spend and `budget` the cap it exceeded — both step counts, not books,
 *  because the cost is the summed sketch length, not the book count. */
interface SimilarTooLarge {
  status: 'too_large';
  total: number;
  fingerprinted: number;
  pairs: number;
  work: number;
  seconds: number;
  budget: number;
}

/**
 * Thrown by {@link getSimilarBookPairs} when the shelf's fingerprints would cost
 * more than the server's work budget and it answers {@link SimilarTooLarge}
 * rather than the pair array. A distinct type — not a bare Error — so the page
 * can explain that the shelf is past the single-pass budget instead of treating
 * it as a failure to load.
 */
export class SimilarTooLargeError extends Error {
  constructor(
    readonly work: number,
    readonly budget: number,
    readonly total = 0,
    readonly fingerprinted = 0,
    readonly pairs = 0,
    readonly seconds = 0
  ) {
    super(`similarity comparison needs confirmation: ${work} merge steps exceed the budget of ${budget}`);
    this.name = 'SimilarTooLargeError';
  }
}

/**
 * Thrown by {@link startFingerprintSources} when a forced rebuild is refused
 * with 409 because a fingerprint sweep already holds the shelf's single
 * fingerprint chain. The forced request cannot adopt that chain — it may be an
 * incremental sweep that never bypasses the cache — so the caller surfaces this
 * and lets the user retry once the running sweep finishes.
 */
export class FingerprintSweepBusyError extends Error {
  constructor() {
    super('a fingerprint sweep is already running');
    this.name = 'FingerprintSweepBusyError';
  }
}

/**
 * Similar book pairs scored by the server in a single pass. `floor` is the
 * lowest Jaccard the server returns; the page filters upward from there in
 * memory rather than re-requesting, so this is called once per visit with the
 * widest floor the UI offers.
 *
 * A shelf whose fingerprints exceed the server's work budget answers 200 with a
 * {@link SimilarTooLarge} body rather than a pair array; that is surfaced as a
 * thrown error so a caller never iterates a non-array as if it were the list.
 * Passing `confirm` retries past that gate and gives the comparison a five-minute
 * timeout instead of the normal metadata-request deadline.
 */
const SIMILAR_COMPARISON_TIMEOUT_MS = 300_000;

export async function getSimilarBookPairs(floor?: number, confirm = false): Promise<SimilarBookPair[]> {
  if (isMockApiMode()) {
    return delay([]);
  }

  const params = new URLSearchParams();
  if (floor !== undefined) {
    params.set('floor', String(floor));
  }
  if (confirm) {
    params.set('confirm', '1');
  }
  const query = params.size === 0 ? '' : `?${params.toString()}`;
  const result = await fetchJson<SimilarBookPair[] | SimilarTooLarge>(
    buildShelfApiPath(`/books/similar${query}`),
    undefined,
    confirm ? { timeoutMs: SIMILAR_COMPARISON_TIMEOUT_MS } : undefined
  );
  if (!Array.isArray(result)) {
    throw new SimilarTooLargeError(
      result.work,
      result.budget,
      result.total,
      result.fingerprinted,
      result.pairs,
      result.seconds
    );
  }
  return result;
}

/**
 * How many books already have a fingerprint on record. A pure cache read on the
 * server: it never opens a source file, so it is cheap enough to call on every
 * visit to the similarity page.
 */
export async function getFingerprintStatus(): Promise<FingerprintStatus> {
  if (isMockApiMode()) {
    return delay({ total: 0, fingerprinted: 0, missing: 0, algo: { normalize: '', shingle: '', hash: '', k: 0 } });
  }

  return await fetchJson<FingerprintStatus>(buildShelfApiPath('/fingerprints/status'));
}

export async function updateBook(id: string, payload: BookUpdateRequest): Promise<Book> {
  if (isMockApiMode()) {
    return delay(mockUpdateBook(id, payload));
  }

  const b = await fetchJson<BackendBook>(buildShelfApiPath(`/books/${encodeURIComponent(id)}`), {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(payload)
  });
  return transformBook(b);
}

export async function updateBookFolder(bookId: string, folder: string): Promise<void> {
  const normalized = folder
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0);

  if (isMockApiMode()) {
    await delay(mockUpdateBookFolder(bookId, folder));
    return;
  }

  await fetchJson(buildShelfApiPath(`/books/${encodeURIComponent(bookId)}`), {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      folder: normalized
    })
  });
}

/**
 * Duplicates a book into `folder` and returns the copy, which the server gives a
 * fresh id so it coexists with the original. Copying into the book's own folder
 * is allowed - it is how "duplicate here" works - so unlike updateBookFolder this
 * never treats the current folder as a no-op. An empty `folder` lands the copy at
 * the shelf root.
 */
export async function copyBook(bookId: string, folder: string): Promise<Book> {
  const normalized = folder
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0);

  if (isMockApiMode()) {
    return delay(mockCopyBook(bookId, folder));
  }

  const b = await fetchJson<BackendBook>(buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/copies`), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      folder: normalized
    })
  });
  return transformBook(b);
}

/** Which side of a cross-shelf transfer the caller wants: `copy` leaves the
 *  source in place and gives the new book a fresh id; `move` keeps the id (and
 *  therefore the reader's saved progress and any external references) and drops
 *  the source once the copy is published on the target. */
export type BookTransferMode = 'copy' | 'move';

/**
 * Reads the running chain's id out of the JSON body the server answers 409 with
 * when the same transfer is already in flight. Every other 409 (target already
 * holds this id, target/source read-only) is a plain-text error, so a parse
 * failure means "not the already-running case" rather than a malformed reply.
 */
function runningTaskChainID(body: string): string | null {
  try {
    const parsed = JSON.parse(body) as { taskchain_id?: unknown };
    const id = typeof parsed?.taskchain_id === 'string' ? parsed.taskchain_id.trim() : '';
    return id || null;
  } catch {
    return null;
  }
}

/**
 * Copies or moves a book from the active shelf to `targetShelfId`, returning the
 * id of the background task chain to poll. The work is asynchronous because the
 * copy can be large and either shelf may be a network mount, so the request must
 * not block on it.
 *
 * `targetFolder` is a '/'-joined path; '' lands the book at the target shelf
 * root. When the same transfer is already running the server answers 409 with
 * that chain's id, which is returned here so the caller attaches to the in-flight
 * work instead of failing. Any other 409 — the target already holds this id, or
 * the target/source is read-only — propagates with the server's message.
 */
export async function transferBook(
  bookId: string,
  targetShelfId: string,
  targetFolder: string,
  mode: BookTransferMode
): Promise<string> {
  if (isMockApiMode()) {
    return mockTransferBook(bookId, targetShelfId, targetFolder, mode);
  }

  const folder = targetFolder
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0);

  try {
    const res = await fetchJson<BackendTaskChainSubmitResponse>(
      buildShelfApiPath(`/books/${encodeURIComponent(bookId)}/transfers`),
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ mode, target_shelf: targetShelfId, target_folder: folder })
      }
    );
    return res.taskchain_id;
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      const running = runningTaskChainID(err.message);
      if (running) {
        return running;
      }
    }
    throw err;
  }
}

export async function getBookContent(id: string): Promise<BookContent> {
  if (isMockApiMode()) {
    return delay(mockGetBookContent(id));
  }

  const text = await fetchText(buildShelfApiPath(`/books/${encodeURIComponent(id)}/content`));
  return { content: text };
}

export async function downloadBookContent(id: string): Promise<Blob> {
  if (isMockApiMode()) {
    const { content } = mockGetBookContent(id);
    return delay(new Blob([content], { type: 'text/plain;charset=utf-8' }));
  }

  return await fetchBlob(buildShelfApiPath(`/books/${encodeURIComponent(id)}/content`));
}

export async function importBook(payload: BookCreateRequest): Promise<Book> {
  if (isMockApiMode()) {
    return delay(mockImportBook(payload));
  }

  const form = new FormData();
  form.append('file', payload.file);

  const trimmedTitle = payload.title.trim();
  if (trimmedTitle.length > 0) {
    form.append('title', trimmedTitle);
  }

  const trimmedFolder = payload.folder?.trim() ?? '';
  if (trimmedFolder.length > 0) {
    form.append('folder', trimmedFolder);
  }

  if (payload.strategy) {
    form.append('strategy', JSON.stringify(payload.strategy));
  }

  const created = transformBook(await fetchJson<BackendBook>(buildShelfApiPath('/books/import'), {
    method: 'POST',
    body: form
  }));

  return created;
}

export async function uploadBookCover(id: string, file: File): Promise<void> {
  await uploadBookCoverInternal(id, file);
}

export async function uploadBookCoverBlob(id: string, blob: Blob): Promise<void> {
  const payload = new Uint8Array(await blob.arrayBuffer());

  await fetchJson<void>(buildShelfApiPath(`/books/${encodeURIComponent(id)}/cover`), {
    method: 'PUT',
    headers: {
      'Content-Type': blob.type || 'image/jpeg'
    },
    body: payload
  });
}

export async function getBookCover(id: string): Promise<Blob> {
  if (isMockApiMode()) {
    return await mockGetBookCover(id);
  }

  return await fetchBlob(buildShelfApiPath(`/books/${encodeURIComponent(id)}/cover`));
}

export async function deleteBookCover(id: string): Promise<void> {
  await deleteBookCoverInternal(id);
}

export async function deleteBook(id: string): Promise<void> {
  if (isMockApiMode()) {
    mockDeleteBook(id);
    await delay(undefined);
    return;
  }

  await fetchJson<void>(buildShelfApiPath(`/books/${encodeURIComponent(id)}/trash`), {
    method: 'POST'
  });
}

export async function listTrashedBooks(): Promise<TrashedBook[]> {
  if (isMockApiMode()) {
    return delay(mockListTrashedBooks());
  }

  const books = await fetchJson<BackendTrashedBook[]>(buildShelfApiPath('/trash/books'));
  return books.map((book) => ({
    id: book.id,
    title: book.title,
    authors: book.authors ?? [],
    original_folder: book.original_folder ?? [],
    original_path: book.original_path,
    deleted_at: book.deleted_at
  }));
}

export async function restoreTrashedBook(id: string): Promise<void> {
  if (isMockApiMode()) {
    mockRestoreTrashedBook(id);
    await delay(undefined);
    return;
  }

  await fetchJson<void>(buildShelfApiPath(`/trash/books/${encodeURIComponent(id)}/restore`), {
    method: 'POST'
  });
}

export async function deleteTrashedBook(id: string): Promise<void> {
  if (isMockApiMode()) {
    mockDeleteTrashedBook(id);
    await delay(undefined);
    return;
  }

  await fetchJson<void>(buildShelfApiPath(`/trash/books/${encodeURIComponent(id)}`), {
    method: 'DELETE'
  });
}

interface BackendTaskChainSubmitResponse {
  taskchain_id: string;
}

/**
 * emptyTrash schedules the background sweep that permanently deletes every
 * trashed book, returning the ID of the task chain to poll for progress.
 *
 * A 409 means a sweep is already in flight for this shelf; the server reports
 * its ID so the caller attaches to the existing progress instead of failing.
 */
export async function emptyTrash(): Promise<string> {
  if (isMockApiMode()) {
    return mockEmptyTrash();
  }

  const res = await fetchJson<BackendTaskChainSubmitResponse>(
    buildShelfApiPath('/trash/empty'),
    { method: 'POST' },
    { acceptStatuses: [409] }
  );
  return res.taskchain_id;
}

/**
 * refreshContentStats schedules the background sweep that recomputes the
 * content statistics of every book whose character count is unknown, returning
 * the ID of the task chain to poll for progress.
 *
 * A 409 means a sweep is already in flight for this shelf; the server reports
 * its ID so the caller attaches to the existing progress instead of failing.
 */
export async function refreshContentStats(): Promise<string> {
  if (isMockApiMode()) {
    return mockRefreshContentStats();
  }

  const res = await fetchJson<BackendTaskChainSubmitResponse>(
    buildShelfApiPath('/content-stat-refreshes'),
    { method: 'POST' },
    { acceptStatuses: [409] }
  );
  return res.taskchain_id;
}

/**
 * startFingerprintSources schedules the background sweep that builds a
 * similarity fingerprint for every source that lacks an up-to-date one,
 * returning the ID of the task chain to poll for progress. This is what the
 * similar-content page's "build fingerprints" button triggers.
 *
 * Passing `force` adds the `force` query flag, which makes the sweep ignore the
 * fingerprint cache and rebuild every source's fingerprint — for a reader who
 * suspects a stat comparison has missed a content change. Without it the sweep
 * stays incremental and skips whatever the cache can already answer.
 *
 * An incremental sweep already in flight answers 409 with that chain's ID,
 * accepted here so the caller attaches to the existing progress instead of
 * failing. A forced request does NOT adopt a 409: the shelf runs a single
 * fingerprint chain (`NewFingerprintSourcesChain` shares its key across modes),
 * so a 409 on force may name an *incremental* sweep, and attaching to it would
 * report a rebuild that never bypassed the cache. It is surfaced as a
 * `FingerprintSweepBusyError` for the caller to show and let the user retry.
 * A read-only shelf also refuses with 409 (from `rejectReadOnlyShelf`, without
 * an ID), but the page hides both buttons in that mode, so this is not reached
 * then.
 */
export async function startFingerprintSources(force = false): Promise<string> {
  if (isMockApiMode()) {
    return mockStartFingerprintSources(force);
  }

  const path = buildShelfApiPath('/source-fingerprints') + (force ? '?force=true' : '');

  if (!force) {
    const res = await fetchJson<BackendTaskChainSubmitResponse>(
      path,
      { method: 'POST' },
      { acceptStatuses: [409] }
    );
    return res.taskchain_id;
  }

  try {
    const res = await fetchJson<BackendTaskChainSubmitResponse>(path, { method: 'POST' });
    return res.taskchain_id;
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      throw new FingerprintSweepBusyError();
    }
    throw err;
  }
}

export function getBookCoverUrl(id: string, cacheKey?: number): string {
  const encodedId = encodeURIComponent(id);
  if (cacheKey === undefined) {
    return buildApiUrl(buildShelfApiPath(`/books/${encodedId}/cover`));
  }
  return buildApiUrl(`${buildShelfApiPath(`/books/${encodedId}/cover`)}?t=${encodeURIComponent(String(cacheKey))}`);
}
