import type {
  Book,
  BookCreateRequest,
  BookContent,
  BookFormat,
  BookUpdateRequest,
  NsfwFolderRule,
  PaginatedBooks,
  TrashedBook,
  TrashedBookListing,
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
  schema_version?: number;
  id: string;
  title: string;
  authors: string[];
  language: string;
  format: string;
  tags?: string[];
  cover: string;
  comment?: string;
  comments?: string;
  created_at?: string;
  updated_at?: string;
  published_at?: string;
  current_source?: string;
  star?: number;
  identifiers?: Record<string, string>;
  nsfw?: boolean;
}

interface BackendBook {
  download_state?: Book['download_state'];
  downloaded_at?: Book['downloaded_at'];
  meta: BackendBookMeta;
  folder: string[];
  char_count?: number;
  nsfw_folder?: NsfwFolderRule;
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
  const folders = b.folder;
  const cover = b.meta.cover?.trim() ?? '';

  return {
    id: b.meta.id,
    title: b.meta.title,
    authors: b.meta.authors,
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
    char_count: b.char_count,
    nsfw: b.meta.nsfw ?? false,
    download_state: b.download_state,
    downloaded_at: b.downloaded_at,
    nsfw_folder: b.nsfw_folder
  };
}

const PAGE_SIZE_DEFAULT = 8;

export interface ListBooksOptions {
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

export type SimilarRelation =
  | 'identical_after_normalize'
  | 'subset'
  | 'near_identical'
  | 'same_source';

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

interface SimilarTooLarge {
  status: 'too_large';
  total: number;
  fingerprinted: number;
  pairs: number;
  work: number;
  seconds: number;
  budget: number;
}

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

export class FingerprintSweepBusyError extends Error {
  constructor() {
    super('a fingerprint sweep is already running');
    this.name = 'FingerprintSweepBusyError';
  }
}

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

export type BookTransferMode = 'copy' | 'move';

function runningTaskChainID(body: string): string | null {
  try {
    const parsed = JSON.parse(body) as { taskchain_id?: unknown };
    const id = typeof parsed?.taskchain_id === 'string' ? parsed.taskchain_id.trim() : '';
    return id || null;
  } catch {
    return null;
  }
}

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

const TRASH_PARTIAL_HEADER = 'X-PlainShelf-Trash-Partial';

export async function listTrashedBooks(): Promise<TrashedBookListing> {
  if (isMockApiMode()) {
    return delay({ books: mockListTrashedBooks(), complete: true });
  }

  let complete = true;
  const books = await fetchJson<BackendTrashedBook[]>(
    buildShelfApiPath('/trash/books'),
    undefined,
    {
      onResponse: (res) => {
        complete = res.headers.get(TRASH_PARTIAL_HEADER) !== 'true';
      }
    }
  );
  return {
    books: books.map((book) => ({
      id: book.id,
      title: book.title,
      authors: book.authors ?? [],
      original_folder: book.original_folder ?? [],
      original_path: book.original_path,
      deleted_at: book.deleted_at
    })),
    complete
  };
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

export async function downloadOfflineBook(id: string): Promise<void> {
  await fetchJson(buildShelfApiPath(`/books/${encodeURIComponent(id)}/download`),
    { method: 'PUT' }, { readOnlySafe: true, timeoutMs: 300_000 });
}

export async function removeOfflineBook(id: string): Promise<void> {
  await fetchJson(buildShelfApiPath(`/books/${encodeURIComponent(id)}/download`),
    { method: 'DELETE' }, { readOnlySafe: true });
}

export async function listOfflineBooks(): Promise<Array<{ book: Book; sizeBytes: number; downloadedAt: string }>> {
  const result = await fetchJson<{ books: Array<{ book: BackendBook; size_bytes: number; downloaded_at: string }> }>(
    buildShelfApiPath('/downloads'));
  return result.books.map((entry) => ({ book: transformBook(entry.book), sizeBytes: entry.size_bytes, downloadedAt: entry.downloaded_at }));
}
