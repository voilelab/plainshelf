import type { Book } from '@/types/book';
import type { AnyBookFilterDef } from './registry';

export interface ActiveBookFilter {
  readonly filter: AnyBookFilterDef;
  readonly value: unknown;
}

type BookAugment = (book: Book) => Book;

const identityAugment: BookAugment = (book) => book;

export function applyBookFilters(
  books: readonly Book[],
  active: readonly ActiveBookFilter[],
  augment: BookAugment = identityAugment
): Book[] {
  if (active.length === 0) {
    return books.slice();
  }
  return books.filter((book) => {
    const augmented = augment(book);
    return active.every(({ filter, value }) => filter.predicate(augmented, value));
  });
}

export function soleBlockingFilter(
  books: readonly Book[],
  active: readonly ActiveBookFilter[],
  augment: BookAugment = identityAugment
): ActiveBookFilter | null {
  if (active.length === 0) {
    return null;
  }
  if (applyBookFilters(books, active, augment).length === 0) {
    let blamed: ActiveBookFilter | null = null;
    for (const candidate of active) {
      const withoutCandidate = active.filter((entry) => entry !== candidate);
      if (applyBookFilters(books, withoutCandidate, augment).length > 0) {
        if (blamed !== null) {
          return null;
        }
        blamed = candidate;
      }
    }
    return blamed;
  }
  return null;
}
