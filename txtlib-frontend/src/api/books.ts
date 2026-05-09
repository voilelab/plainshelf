/**
 * API client for txtlib-srv backend.
 *
 * Real API endpoints are tried first. On failure, functions fall back to mock
 * data so the frontend can continue to run without a running backend.
 *
 * Backend book shape:  { meta: BackendBookMeta, layer: string[] }
 * Marks/progress:      GET/POST /api/marks/{id}  →  { char_offset: number }
 * Content:             GET /api/books/{id}/content  →  plain text
 */
import type {
  BookmarkPayload,
  Book,
  BookCreateRequest,
  BookContent,
  BookFormat,
  BookUpdateRequest,
  PaginatedBooks,
  ReadingProgress,
} from '../types/book';

interface BackendBookMeta {
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
  current_snapshot?: string;
}

interface BackendBook {
  meta: BackendBookMeta;
  layer?: string[];
  layers?: string[];
}

interface BackendMark {
  char_offset: number;
}

const API_BASE = import.meta.env.VITE_API_BASE ?? '';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  const contentLength = res.headers.get('content-length');
  if (contentLength === '0') {
    return undefined as T;
  }

  const raw = await res.text();
  if (!raw.trim()) {
    return undefined as T;
  }

  return JSON.parse(raw) as T;
}

async function requestText(path: string): Promise<string> {
  const res = await fetch(`${API_BASE}${path}`);
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  }
  return res.text();
}

async function uploadBookCoverInternal(bookID: string, file: File): Promise<void> {
  const res = await fetch(`${API_BASE}/api/books/${encodeURIComponent(bookID)}/cover`, {
    method: 'PUT',
    headers: {
      'Content-Type': file.type || 'application/octet-stream'
    },
    body: file
  });

  if (!res.ok) {
    const msg = (await res.text()).trim();
    throw new Error(msg || `HTTP ${res.status}: ${res.statusText}`);
  }
}

async function deleteBookCoverInternal(bookID: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/books/${encodeURIComponent(bookID)}/cover`, {
    method: 'DELETE'
  });

  if (!res.ok) {
    const msg = (await res.text()).trim();
    throw new Error(msg || `HTTP ${res.status}: ${res.statusText}`);
  }
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
    cover_url: cover ? `${API_BASE}/api/books/${b.meta.id}/cover` : undefined,
    layers,
    created_at: b.meta.created_at,
    updated_at: b.meta.updated_at,
    published_at: b.meta.published_at,
    current_snapshot: b.meta.current_snapshot
  };
}

const PAGE_SIZE_DEFAULT = 8;

export const mockBooks: Book[] = [
  {
    id: 'book-1',
    title: 'The Quiet River',
    authors: ['A. Lin'],
    layers: ['fiction', 'quiet'],
    language: 'en',
    format: 'markdown',
    tags: ['fiction', 'calm'],
    comment: 'Imported from local markdown notes.',
    created_at: '2026-01-07T10:00:00Z',
    updated_at: '2026-04-18T08:30:00Z',
    cover_url: 'https://picsum.photos/seed/txtlib1/120/180'
  },
  {
    id: 'book-2',
    title: 'Go Patterns Notes',
    authors: ['P. Chen'],
    layers: ['programming', 'go'],
    language: 'zh-TW',
    format: 'txt',
    tags: ['programming', 'go'],
    created_at: '2026-02-10T12:00:00Z',
    cover_url: 'https://picsum.photos/seed/txtlib2/120/180'
  },
  {
    id: 'book-3',
    title: 'Mountain Diary',
    authors: ['Y. Wang'],
    layers: ['travel'],
    language: 'zh-TW',
    format: 'markdown',
    tags: ['travel'],
    cover_url: 'https://picsum.photos/seed/txtlib3/120/180'
  },
  {
    id: 'book-4',
    title: 'Designing Small Tools',
    authors: ['N. Hsu'],
    layers: ['design'],
    language: 'en',
    format: 'txt',
    tags: ['design', 'notes'],
    cover_url: 'https://picsum.photos/seed/txtlib4/120/180'
  },
  {
    id: 'book-5',
    title: 'Tea House Stories',
    authors: ['K. Lee'],
    layers: ['fiction'],
    language: 'zh-TW',
    format: 'markdown',
    tags: ['fiction'],
    cover_url: 'https://picsum.photos/seed/txtlib5/120/180'
  },
  {
    id: 'book-6',
    title: 'Minimal Linux Book',
    authors: ['R. Cho'],
    layers: ['ops'],
    language: 'en',
    format: 'txt',
    tags: ['linux', 'ops'],
    cover_url: 'https://picsum.photos/seed/txtlib6/120/180'
  },
  {
    id: 'book-7',
    title: 'Autumn Poems',
    authors: ['S. Yu'],
    layers: ['poetry'],
    language: 'zh-TW',
    format: 'markdown',
    tags: ['poetry'],
    cover_url: 'https://picsum.photos/seed/txtlib7/120/180'
  },
  {
    id: 'book-8',
    title: 'Product Journal 2025',
    authors: ['M. Kao'],
    layers: ['product'],
    language: 'en',
    format: 'txt',
    tags: ['product'],
    cover_url: 'https://picsum.photos/seed/txtlib8/120/180'
  },
  {
    id: 'book-9',
    title: 'Kitchen and Code',
    authors: ['L. Ho'],
    layers: ['essay'],
    language: 'en',
    format: 'markdown',
    tags: ['essay'],
    cover_url: 'https://picsum.photos/seed/txtlib9/120/180'
  },
  {
    id: 'book-10',
    title: 'Reading Machines',
    authors: ['D. Ko'],
    layers: ['tech'],
    language: 'en',
    format: 'txt',
    tags: ['tech', 'history'],
    cover_url: 'https://picsum.photos/seed/txtlib10/120/180'
  }
];

const mockProgress: Record<string, ReadingProgress> = {
  'book-1': { file_path: '/library/book-1.md', char_offset: 240, percent: 15 },
  'book-2': { file_path: '/library/book-2.txt', char_offset: 1200, percent: 42 },
  'book-3': { file_path: '/library/book-3.md', char_offset: 700, percent: 58 }
};

const mockContent: Record<string, string> = {
  'book-1': `# The Quiet River\n\nThe river moved slowly by the old town.\nEach house kept a small lamp lit through the night...`,
  'book-2': `Go Patterns Notes\n\n1. Keep interfaces small.\n2. Prefer composition over inheritance.\n3. Handle errors early and clearly.`,
  'book-3': `# Mountain Diary\n\nDay 1: Clouds under the ridge.\nDay 2: A narrow trail and cold wind.`
};

function delay<T>(value: T, ms = 240): Promise<T> {
  return new Promise((resolve) => {
    setTimeout(() => resolve(value), ms);
  });
}

function findBookOrThrow(id: string): Book {
  const book = mockBooks.find((item) => item.id === id);
  if (!book) {
    throw new Error('Book not found');
  }
  return book;
}

function mockListBooks(page: number, pageSize: number): PaginatedBooks {
  const start = (page - 1) * pageSize;
  const end = start + pageSize;
  return {
    items: mockBooks.slice(start, end),
    total: mockBooks.length,
    page,
    pageSize
  };
}

function mockGetBook(id: string): Book {
  return { ...findBookOrThrow(id) };
}

function mockUpdateBook(id: string, payload: BookUpdateRequest): Book {
  const book = findBookOrThrow(id);
  if (payload.title !== undefined) book.title = payload.title;
  if (payload.authors !== undefined) book.authors = payload.authors;
  if (payload.tags !== undefined) book.tags = payload.tags;
  if (payload.language !== undefined) book.language = payload.language;
  if (payload.comment !== undefined) book.comment = payload.comment;
  book.updated_at = new Date().toISOString();
  return { ...book };
}

function mockUpdateBookLayer(id: string, layerPath: string): Book {
  const book = findBookOrThrow(id);
  const normalized = layerPath
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0);
  book.layers = normalized;
  book.updated_at = new Date().toISOString();
  return { ...book };
}

function mockGetBookContent(id: string): BookContent {
  const content = mockContent[id] ?? 'No content yet.';
  return { content };
}

function mockGetReadingProgress(id: string): ReadingProgress {
  return mockProgress[id] ?? { file_path: `/library/${id}.txt`, char_offset: 0, percent: 0 };
}

function mockSaveBookmark(id: string, payload: BookmarkPayload): void {
  const prev = mockGetReadingProgress(id);
  const nextPercent = Math.min(
    100,
    Math.max(prev.percent ?? 0, Math.round(payload.char_offset / 20))
  );
  mockProgress[id] = { ...prev, char_offset: payload.char_offset, percent: nextPercent };
}

export async function listBooks(page = 1, pageSize = PAGE_SIZE_DEFAULT): Promise<PaginatedBooks> {
  try {
    const all = await request<BackendBook[]>('/api/books');
    const books = all.map(transformBook);
    const start = (page - 1) * pageSize;
    return { items: books.slice(start, start + pageSize), total: books.length, page, pageSize };
  } catch (err) {
    console.warn('[api] listBooks fell back to mock:', err);
    return delay(mockListBooks(page, pageSize));
  }
}

export async function getBook(id: string): Promise<Book> {
  try {
    const b = await request<BackendBook>(`/api/books/${encodeURIComponent(id)}`);
    return transformBook(b);
  } catch (err) {
    console.warn('[api] getBook fell back to mock:', err);
    return delay(mockGetBook(id));
  }
}

export async function getDuplicateBookGroups(): Promise<string[][]> {
  try {
    return await request<string[][]>('/api/books/duplicate');
  } catch (err) {
    console.warn('[api] getDuplicateBookGroups fell back to mock:', err);
    return delay([]);
  }
}

export async function updateBook(id: string, payload: BookUpdateRequest): Promise<Book> {
  try {
    const body: BookUpdateRequest = {};
    if (payload.title !== undefined) body.title = payload.title;
    if (payload.tags !== undefined) body.tags = payload.tags;
    if (payload.authors !== undefined) body.authors = payload.authors;
    if (payload.language !== undefined) body.language = payload.language;
    if (payload.comment !== undefined) body.comment = payload.comment;
    const b = await request<BackendBook>(`/api/books/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body: JSON.stringify(body)
    });
    return transformBook(b);
  } catch (err) {
    console.warn('[api] updateBook fell back to mock:', err);
    return delay(mockUpdateBook(id, payload));
  }
}

export async function updateBookLayer(bookId: string, layer: string): Promise<void> {
  const normalized = layer
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0);

  try {
    await request(`/api/books/${encodeURIComponent(bookId)}`, {
      method: 'PATCH',
      body: JSON.stringify({
        layer: normalized
      })
    });
  } catch (err) {
    console.warn('[api] updateBookLayer fell back to mock:', err);
    await delay(mockUpdateBookLayer(bookId, layer));
  }
}

export async function getBookContent(id: string): Promise<BookContent> {
  try {
    const text = await requestText(`/api/books/${encodeURIComponent(id)}/content`);
    return { content: text };
  } catch (err) {
    console.warn('[api] getBookContent fell back to mock:', err);
    return delay(mockGetBookContent(id));
  }
}

export async function getReadingProgress(id: string): Promise<ReadingProgress> {
  try {
    const mark = await request<BackendMark>(`/api/marks/${encodeURIComponent(id)}`);
    return { char_offset: mark.char_offset };
  } catch (err) {
    console.warn('[api] getReadingProgress fell back to mock:', err);
    return delay({ ...mockGetReadingProgress(id) });
  }
}

export async function saveBookmark(id: string, payload: BookmarkPayload): Promise<void> {
  console.log(`[api] saveBookmark called with id=${id} char_offset=${payload.char_offset}`);
  try {
    await request(`/api/marks/${encodeURIComponent(id)}`, {
      method: 'POST',
      body: JSON.stringify({ char_offset: payload.char_offset })
    });
  } catch (err) {
    console.warn('[api] saveBookmark fell back to mock:', err);
    mockSaveBookmark(id, payload);
    await delay(undefined);
  }
}

export async function importBook(payload: BookCreateRequest): Promise<Book> {
  const form = new FormData();
  form.append('file', payload.file);

  const trimmedTitle = payload.title.trim();
  if (trimmedTitle.length > 0) {
    form.append('title', trimmedTitle);
  }

  const trimmedAlias = payload.alias?.trim() ?? '';
  if (trimmedAlias.length > 0) {
    form.append('alias', trimmedAlias);
  }

  const trimmedLayer = payload.layer?.trim() ?? '';
  if (trimmedLayer.length > 0) {
    form.append('layer', trimmedLayer);
  }

  const res = await fetch(`${API_BASE}/api/books/import`, {
    method: 'POST',
    body: form
  });

  if (!res.ok) {
    const msg = (await res.text()).trim();
    throw new Error(msg || `HTTP ${res.status}: ${res.statusText}`);
  }

  const created = transformBook((await res.json()) as BackendBook);

  if (payload.coverFile) {
    await uploadBookCoverInternal(created.id, payload.coverFile);
  }

  return created;
}

export async function uploadBookCover(id: string, file: File): Promise<void> {
  await uploadBookCoverInternal(id, file);
}

export async function getBookCover(id: string): Promise<Blob> {
  const res = await fetch(getBookCoverUrl(id));
  if (!res.ok) {
    const msg = (await res.text()).trim();
    throw new Error(msg || `HTTP ${res.status}: ${res.statusText}`);
  }
  return res.blob();
}

export async function deleteBookCover(id: string): Promise<void> {
  await deleteBookCoverInternal(id);
}

export async function deleteBook(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/books/${encodeURIComponent(id)}`, {
    method: 'DELETE'
  });
  if (!res.ok) {
    const msg = (await res.text()).trim();
    throw new Error(msg || `HTTP ${res.status}: ${res.statusText}`);
  }
}

export function getBookCoverUrl(id: string, cacheKey?: number): string {
  const encodedId = encodeURIComponent(id);
  if (cacheKey === undefined) {
    return `${API_BASE}/api/books/${encodedId}/cover`;
  }
  return `${API_BASE}/api/books/${encodedId}/cover?t=${encodeURIComponent(String(cacheKey))}`;
}
