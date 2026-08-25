import type { Book } from '@/types/book';

/**
 * Whether a single book matches a free-text query, compared (case-insensitively)
 * against its title, authors, tags, and comment. A blank query matches every
 * book. Exposed as the per-book predicate so the book-filter registry and the
 * list filter below share one definition of "matches the search".
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

/**
 * Filters books by a free-text query matched (case-insensitively) against
 * title, authors, tags, and comment. Runs purely client-side so it behaves
 * identically online, offline, and in mock mode — no network round-trip.
 *
 * An empty (or whitespace-only) query returns the original array unchanged.
 */
export function filterBooksBySearch(books: Book[], query: string): Book[] {
  if (query.trim() === '') {
    return books;
  }

  return books.filter((book) => bookMatchesSearch(book, query));
}
