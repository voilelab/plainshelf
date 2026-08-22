import type {
  Book,
  BookCreateRequest,
  BookContent,
  BookUpdateRequest,
  PaginatedBooks,
  TrashedBook
} from '@/types/book';
import { registerMockTaskChain } from './taskchains';
import type { BookBatchRequest, BookBatchResult, RefreshContentStatsResult } from '@/types/task';

// In-memory backend for VITE_USE_MOCK_API. The gate that reaches for it lives in
// api/client.ts (isMockApiMode); nothing here is reachable in a real build.
// State is module-level and mutable on purpose: a mock session should behave
// like a tiny server that remembers edits until the page reloads.

export const mockBooks: Book[] = [
  {
    id: 'book-1',
    title: 'The Quiet River',
    authors: ['A. Lin'],
    layers: ['fiction', 'quiet'],
    language: 'en',
    format: 'txt',
    tags: ['fiction', 'calm'],
    comment: 'Imported from local txt file.',
    created_at: '2026-01-07T10:00:00Z',
    updated_at: '2026-04-18T08:30:00Z',
    published_at: '2026-03-15',
    cover_url: 'https://picsum.photos/seed/shelf1/120/180',
    star: 4,
    identifiers: { isbn: '9787020002207' },
    char_count: 182_400
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
    cover_url: 'https://picsum.photos/seed/shelf2/120/180',
    star: 2,
    identifiers: { isbn: '9787115428028', asin: 'B01N5AX61W' },
    char_count: 64_800
  },
  {
    id: 'book-3',
    title: 'Mountain Diary',
    authors: ['Y. Wang'],
    layers: ['travel'],
    language: 'zh-TW',
    format: 'txt',
    tags: ['travel'],
    created_at: '2026-03-01T09:00:00Z',
    cover_url: 'https://picsum.photos/seed/shelf3/120/180',
    star: 5,
    char_count: 41_200
  },
  {
    id: 'book-4',
    title: 'Designing Small Tools',
    authors: ['N. Hsu'],
    layers: ['design'],
    language: 'en',
    format: 'txt',
    tags: ['design', 'notes'],
    created_at: '2026-07-02T09:00:00Z',
    cover_url: 'https://picsum.photos/seed/shelf4/120/180',
    star: 3,
    char_count: 98_500
  },
  {
    id: 'book-5',
    title: 'Tea House Stories',
    authors: ['K. Lee'],
    layers: ['fiction'],
    language: 'zh-TW',
    format: 'txt',
    tags: ['fiction'],
    created_at: '2026-05-20T09:00:00Z',
    cover_url: 'https://picsum.photos/seed/shelf5/120/180',
    star: 4,
    char_count: 235_900
  },
  {
    id: 'book-6',
    title: 'Minimal Linux Book',
    authors: ['R. Cho'],
    layers: ['ops'],
    language: 'en',
    format: 'txt',
    tags: ['linux', 'ops'],
    created_at: '2026-07-08T09:00:00Z',
    cover_url: 'https://picsum.photos/seed/shelf6/120/180',
    char_count: 152_300
  },
  {
    id: 'book-7',
    title: 'Autumn Poems',
    authors: ['S. Yu'],
    layers: ['poetry'],
    language: 'zh-TW',
    format: 'txt',
    tags: ['poetry'],
    created_at: '2025-11-11T09:00:00Z',
    cover_url: 'https://picsum.photos/seed/shelf7/120/180',
    star: 5,
    char_count: 18_600
  },
  {
    id: 'book-8',
    title: 'Product Journal 2025',
    authors: ['M. Kao'],
    layers: ['product'],
    language: 'en',
    format: 'txt',
    tags: ['product'],
    created_at: '2026-06-30T09:00:00Z',
    cover_url: 'https://picsum.photos/seed/shelf8/120/180',
    star: 2,
    char_count: 76_400
  },
  {
    id: 'book-9',
    title: 'Kitchen and Code',
    authors: ['L. Ho'],
    layers: ['essay'],
    language: 'en',
    format: 'txt',
    tags: ['essay'],
    created_at: '2026-07-13T09:00:00Z',
    cover_url: 'https://picsum.photos/seed/shelf9/120/180',
    star: 3,
    char_count: 112_700
  },
  {
    id: 'book-10',
    title: 'Reading Machines',
    authors: ['D. Ko'],
    layers: ['tech'],
    language: 'en',
    format: 'txt',
    tags: ['tech', 'history'],
    created_at: '2025-08-01T09:00:00Z',
    cover_url: 'https://picsum.photos/seed/shelf10/120/180',
    star: 4,
    char_count: 340_100
  },
  {
    id: 'book-11',
    title: 'Markdown Field Notes',
    authors: ['T. Fang'],
    layers: ['notes'],
    language: 'en',
    format: 'md',
    tags: ['notes', 'markdown'],
    created_at: '2026-07-05T09:00:00Z',
    cover_url: 'https://picsum.photos/seed/shelf11/120/180'
    // char_count is deliberately absent: this is the "unknown character count"
    // case the maintenance page reports and the content-stats sweep repairs.
  }
];

const mockContent: Record<string, string> = {
  'book-1': `# The Quiet River\n\nThe river moved slowly by the old town.\nEach house kept a small lamp lit through the night...`,
  'book-2': `Go Patterns Notes\n\n1. Keep interfaces small.\n2. Prefer composition over inheritance.\n3. Handle errors early and clearly.`,
  'book-3': `# Mountain Diary\n\nDay 1: Clouds under the ridge.\nDay 2: A narrow trail and cold wind.`,
  'book-11': [
    '# Markdown Field Notes',
    '',
    'This paragraph has **bold text**, *italic text*, and `inline code`.',
    '',
    '## Section One',
    '',
    '### Subsection',
    '',
    '- First unordered item',
    '- Second unordered item',
    '- Third item with **bold** inside',
    '',
    '1. First ordered step',
    '2. Second ordered step',
    '3. Third ordered step',
    '',
    '> A quoted line of field notes.',
    '> Second line of the same quote.',
    '',
    '```',
    'function greet() {',
    '',
    '  return "hello";',
    '}',
    '```',
    '',
    '---',
    '',
    'Unsupported syntax stays literal: [link](https://example.com) and ![alt](image.png).'
  ].join('\n')
};

const mockTrashedBooks: TrashedBook[] = [];

export function findBookOrThrow(id: string): Book {
  const book = mockBooks.find((item) => item.id === id);
  if (!book) {
    throw new Error('Book not found');
  }
  return book;
}

export function mockListBooks(page: number, pageSize: number): PaginatedBooks {
  const start = (page - 1) * pageSize;
  const end = start + pageSize;
  return {
    items: mockBooks.slice(start, end),
    total: mockBooks.length,
    page,
    pageSize
  };
}

export function mockGetBook(id: string): Book {
  return { ...findBookOrThrow(id) };
}

export function mockUpdateBook(id: string, payload: BookUpdateRequest): Book {
  const book = findBookOrThrow(id);
  if (payload.title !== undefined) book.title = payload.title;
  if (payload.authors !== undefined) book.authors = payload.authors;
  if (payload.tags !== undefined) book.tags = payload.tags;
  if (payload.language !== undefined) book.language = payload.language;
  if (payload.comment !== undefined) book.comment = payload.comment;
  if (payload.star !== undefined) book.star = payload.star;
  if (payload.published_at !== undefined) book.published_at = payload.published_at;
  if (payload.identifiers !== undefined) book.identifiers = payload.identifiers;
  if (payload.format !== undefined) book.format = payload.format;
  book.updated_at = new Date().toISOString();
  return { ...book };
}

export function mockUpdateBookLayer(id: string, layerPath: string): Book {
  const book = findBookOrThrow(id);
  const normalized = layerPath
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0);
  book.layers = normalized;
  book.updated_at = new Date().toISOString();
  return { ...book };
}

export function mockCopyBook(id: string, layerPath: string): Book {
  const source = findBookOrThrow(id);
  const normalized = layerPath
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0);
  const now = new Date().toISOString();
  const copy: Book = {
    ...source,
    id: `mock-${Date.now()}`,
    layers: normalized,
    created_at: now,
    updated_at: now
  };
  mockBooks.unshift(copy);
  // The server duplicates the whole package, so the copy reads the same body.
  // Carry the content entry across too, otherwise the mock copy would open empty.
  if (mockContent[id] !== undefined) {
    mockContent[copy.id] = mockContent[id];
  }
  return { ...copy };
}

export function mockGetBookContent(id: string): BookContent {
  const content = mockContent[id] ?? 'No content yet.';
  return { content };
}

export function mockImportBook(payload: BookCreateRequest): Book {
  const now = new Date().toISOString();
  const id = `mock-${Date.now()}`;
  const normalizedLayer = payload.layer
    ?.split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0) ?? [];

  const isEpub = /\.epub$/i.test(payload.file.name);
  const epubFormat = payload.strategy?.preset === 'plain' ? 'txt' : 'md';

  const created: Book = {
    id,
    title: payload.title.trim() || payload.file.name,
    authors: [],
    layers: normalizedLayer,
    language: 'unknown',
    // An EPUB import stores whatever the conversion strategy produces, not the
    // uploaded file's own format.
    format: isEpub ? epubFormat : 'txt',
    tags: [],
    created_at: now,
    updated_at: now
  };

  mockBooks.unshift(created);
  return created;
}

export async function mockGetBookCover(id: string): Promise<Blob> {
  const book = findBookOrThrow(id);
  if (!book.cover_url) {
    throw new Error('Book cover not available');
  }
  const res = await fetch(book.cover_url);
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  }
  return await res.blob();
}

export function mockDeleteBook(id: string): void {
  const idx = mockBooks.findIndex((book) => book.id === id);
  if (idx < 0) {
    return;
  }

  const [book] = mockBooks.splice(idx, 1);
  mockTrashedBooks.unshift({
    id: book.id,
    title: book.title,
    authors: [...book.authors],
    original_layer: [...book.layers],
    original_path: `/books/${book.layers.join('/')}/${book.id}.bookpkg`,
    deleted_at: new Date().toISOString()
  });
}

export function mockListTrashedBooks(): TrashedBook[] {
  return mockTrashedBooks.map((book) => ({ ...book }));
}

export function mockRestoreTrashedBook(id: string): void {
  const idx = mockTrashedBooks.findIndex((book) => book.id === id);
  if (idx < 0) {
    return;
  }

  const [book] = mockTrashedBooks.splice(idx, 1);
  mockBooks.unshift({
    id: book.id,
    title: book.title,
    authors: [...(book.authors ?? [])],
    tags: [],
    language: 'unknown',
    format: 'txt',
    layers: [...(book.original_layer ?? [])]
  });
}

export function mockDeleteTrashedBook(id: string): void {
  const idx = mockTrashedBooks.findIndex((book) => book.id === id);
  if (idx >= 0) {
    mockTrashedBooks.splice(idx, 1);
  }
}

/**
 * mockEmptyTrash schedules a task chain that drops one trashed book per poll,
 * so mock mode exercises the same progress reporting as the real sweep.
 */
export function mockEmptyTrash(): string {
  const doomed = mockTrashedBooks.map((book) => book.id);
  return registerMockTaskChain({
    name: 'empty_trash',
    title: 'Empty trash',
    total: doomed.length,
    onItem: (index) => {
      const target = mockTrashedBooks.findIndex((book) => book.id === doomed[index]);
      if (target >= 0) {
        mockTrashedBooks.splice(target, 1);
      }
    }
  });
}

/**
 * mockRefreshContentStats schedules a task chain that fills in one missing
 * char_count per poll, so mock mode exercises the same progress reporting as
 * the real sweep.
 */
export function mockRefreshContentStats(): string {
  const pending = mockBooks.filter((book) => typeof book.char_count !== 'number').map((book) => book.id);
  const result: RefreshContentStatsResult = {
    total: pending.length,
    refreshed: 0,
    failures: []
  };

  return registerMockTaskChain({
    name: 'refresh_content_stats',
    title: 'Update content statistics',
    total: pending.length,
    onItem: (index) => {
      const id = pending[index];
      const book = mockBooks.find((candidate) => candidate.id === id);
      if (!book) {
        result.failures.push({ book_id: id, code: 'not_found' });
        return;
      }
      // Spread rather than .length so the count is code points, matching the
      // server's rune-based count.
      book.char_count = [...(mockContent[id] ?? '')].length;
      result.refreshed += 1;
    },
    getResult: () => ({
      ...result,
      failures: result.failures.map((failure) => ({ ...failure }))
    })
  });
}

/**
 * mockStartFingerprintSources schedules a task chain that walks every mock book
 * once, so mock mode exercises the same progress reporting as the real
 * fingerprint sweep. Mock mode reports full fingerprint coverage
 * (getFingerprintStatus → missing: 0), so this is here for completeness rather
 * than to change any mock state.
 */
export function mockStartFingerprintSources(): string {
  return registerMockTaskChain({
    name: 'fingerprint_sources',
    title: 'Build fingerprints',
    total: mockBooks.length
  });
}

/**
 * mockTransferBook schedules a single-item task chain that mirrors the real
 * cross-shelf transfer's progress reporting. Mock mode models only one shelf, so
 * the target is a phantom the fixture set does not track: a `move` drops the
 * source book, and a `copy` leaves the source list unchanged.
 */
export function mockTransferBook(
  bookId: string,
  _targetShelfId: string,
  _targetLayer: string,
  mode: 'copy' | 'move'
): string {
  return registerMockTaskChain({
    name: 'book_transfer',
    title: mode === 'move' ? 'Move book to another shelf' : 'Copy book to another shelf',
    total: 1,
    onItem: () => {
      if (mode !== 'move') {
        return;
      }
      const idx = mockBooks.findIndex((book) => book.id === bookId);
      if (idx >= 0) {
        mockBooks.splice(idx, 1);
      }
    }
  });
}

export function mockStartBookBatch(request: BookBatchRequest): string {
  const ids = [...new Set(request.book_ids)];
  const result: BookBatchResult = {
    operation: request.operation,
    total: ids.length,
    succeeded_ids: [],
    failures: []
  };

  return registerMockTaskChain({
    name: 'book_batch',
    title: request.operation === 'move' ? 'Move books' : 'Move books to trash',
    total: ids.length,
    onItem: (index) => {
      const id = ids[index];
      const book = mockBooks.find((candidate) => candidate.id === id);
      if (!book) {
        result.failures.push({ book_id: id, code: 'not_found' });
        return;
      }
      if (request.operation === 'move') {
        book.layers = [...(request.target_layer ?? [])];
        book.updated_at = new Date().toISOString();
      } else {
        mockDeleteBook(id);
      }
      result.succeeded_ids.push(id);
    },
    getResult: () => ({
      ...result,
      succeeded_ids: [...result.succeeded_ids],
      failures: result.failures.map((failure) => ({ ...failure }))
    })
  });
}
