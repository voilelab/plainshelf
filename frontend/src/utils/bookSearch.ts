import type { Book } from '@/types/book';

/**
 * Whether a single book matches a free-text query, compared (case-insensitively)
 * against its title, authors, tags, and comment. A blank query matches every
 * book. Matching runs purely client-side, so it behaves identically online,
 * offline, and in mock mode — no network round-trip.
 */
export function bookMatchesSearch(book: Book, query: string): boolean {
  const term = query.trim().toLowerCase();
  if (term === '') {
    return true;
  }

  const haystack = [book.title, ...book.authors, ...book.tags, book.comment ?? '']
    .join(' ')
    .toLowerCase();
  return haystack.includes(term);
}
