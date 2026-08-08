import type {
  BookmarkPayload,
  Book,
  BookCreateRequest,
  BookContent,
  BookFormat,
  SplitConfig,
  BookUpdateRequest,
  PaginatedBooks,
  ReadingProgress,
  TrashedBook,
} from '@/types/book';
import {
  buildShelfApiPath,
  buildApiUrl,
  fetchBlob,
  fetchJson,
  fetchText,
  isMockApiMode
} from './client';
import { normalizeSplitConfig, buildSplitConfigPayload } from '@/utils/splitConfig';
import { delay } from './mocks/latency';
import {
  mockDeleteBook,
  mockDeleteTrashedBook,
  mockEmptyTrash,
  mockGetBook,
  mockGetBookContent,
  mockGetBookCover,
  mockGetReadingProgress,
  mockGetSplitConfig,
  mockImportBook,
  mockListBooks,
  mockListTrashedBooks,
  mockRestoreTrashedBook,
  mockSaveBookmark,
  mockSetSplitConfig,
  mockUpdateBook,
  mockUpdateBookLayer
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
  layer?: string[];
  layers?: string[];
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
  original_layer?: string[];
  deleted_at?: string;
}

interface BackendMark {
  char_offset: number;
}


async function uploadBookCoverInternal(bookID: string, file: File): Promise<void> {
  // NOTE:
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
  const layers = b.layers ?? b.layer ?? [];
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
    layers,
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
   *  call sites (library grid, layer tree, etc.) that don't display it. */
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

export async function updateBookLayer(bookId: string, layer: string): Promise<void> {
  const normalized = layer
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0);

  if (isMockApiMode()) {
    await delay(mockUpdateBookLayer(bookId, layer));
    return;
  }

  await fetchJson(buildShelfApiPath(`/books/${encodeURIComponent(bookId)}`), {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      layer: normalized
    })
  });
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

export async function getBookSplitConfig(id: string): Promise<SplitConfig> {
  if (isMockApiMode()) {
    return delay(mockGetSplitConfig(id));
  }

  const config = await fetchJson<unknown>(buildShelfApiPath(`/books/${encodeURIComponent(id)}/split_config`));
  return normalizeSplitConfig(config);
}

export async function updateBookSplitConfig(id: string, config: SplitConfig): Promise<SplitConfig> {
  const payload = buildSplitConfigPayload(config);

  if (isMockApiMode()) {
    return await delay(mockSetSplitConfig(id, payload));
  }

  const updated = await fetchJson<unknown>(buildShelfApiPath(`/books/${encodeURIComponent(id)}/split_config`), {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(payload)
  });

  if (updated === undefined) {
    return normalizeSplitConfig(payload);
  }

  return normalizeSplitConfig(updated);
}

export async function getReadingProgress(id: string): Promise<ReadingProgress> {
  if (isMockApiMode()) {
    return delay({ ...mockGetReadingProgress(id) });
  }

  const mark = await fetchJson<BackendMark>(buildShelfApiPath(`/marks/${encodeURIComponent(id)}`));
  return { char_offset: mark.char_offset };
}

export async function saveBookmark(id: string, payload: BookmarkPayload): Promise<void> {
  if (isMockApiMode()) {
    mockSaveBookmark(id, payload);
    await delay(undefined);
    return;
  }

  await fetchJson(buildShelfApiPath(`/marks/${encodeURIComponent(id)}`), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ char_offset: payload.char_offset })
  });
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

  const trimmedLayer = payload.layer?.trim() ?? '';
  if (trimmedLayer.length > 0) {
    form.append('layer', trimmedLayer);
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
    original_layer: book.original_layer ?? [],
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

interface BackendEmptyTrashResponse {
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

  const res = await fetchJson<BackendEmptyTrashResponse>(
    buildShelfApiPath('/trash/empty'),
    { method: 'POST' },
    { acceptStatuses: [409] }
  );
  return res.taskchain_id;
}

export function getBookCoverUrl(id: string, cacheKey?: number): string {
  const encodedId = encodeURIComponent(id);
  if (cacheKey === undefined) {
    return buildApiUrl(buildShelfApiPath(`/books/${encodedId}/cover`));
  }
  return buildApiUrl(`${buildShelfApiPath(`/books/${encodedId}/cover`)}?t=${encodeURIComponent(String(cacheKey))}`);
}
