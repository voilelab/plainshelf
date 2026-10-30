import type { Book } from '@/types/book';
import { foldForSearch } from '@/utils/searchFold';

export function bookMatchesSearch(book: Book, query: string): boolean {
  const term = foldForSearch(query);
  if (term === '') {
    return true;
  }

  const haystack = foldForSearch(
    [book.title, ...book.authors, ...book.tags, book.comment ?? ''].join(' ')
  );
  return haystack.includes(term);
}
