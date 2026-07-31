import type { Book } from '@/types/book';

/**
 * Filters books by a free-text query matched (case-insensitively) against
 * title, authors, tags, and comment. Runs purely client-side so it behaves
 * identically online, offline, and in mock mode — no network round-trip.
 *
 * An empty (or whitespace-only) query returns the original array unchanged.
 */
export function filterBooksBySearch(books: Book[], query: string): Book[] {
  const term = query.trim().toLowerCase();
  if (term === '') {
    return books;
  }

  return books.filter((book) => {
    const haystack = [book.title, ...book.authors, ...book.tags, book.comment ?? '']
      .join(' ')
      .toLowerCase();
    return haystack.includes(term);
  });
}
