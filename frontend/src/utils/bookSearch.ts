import type { Book } from '@/types/book';

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
