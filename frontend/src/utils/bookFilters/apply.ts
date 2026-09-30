/**
 * Generic evaluation of the declarative book filters over an already-loaded book
 * list. These loop the registry instead of the hand-written computed-per-filter
 * cascade they replaced, which re-encoded ordering the registry already knows
 * and did not scale past a few conditions.
 */
import type { Book } from '@/types/book';
import type { AnyBookFilterDef } from './registry';

/** An active condition paired with the value parsed for it. */
export interface ActiveBookFilter {
  readonly filter: AnyBookFilterDef;
  readonly value: unknown;
}

/**
 * Fills in data the listing omits — today only the character count, which lives
 * in a lazily loaded index. A filter that needs nothing extra is unaffected.
 */
type BookAugment = (book: Book) => Book;

const identityAugment: BookAugment = (book) => book;

/** The books that satisfy every active condition (their AND). */
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

/**
 * The single condition responsible for an empty result: the one whose removal
 * *alone* makes the result non-empty. When two conditions each empty the list
 * independently, neither can be blamed and this returns `null`, and the caller
 * falls back to a generic "nothing matches" message.
 */
export function soleBlockingFilter(
  books: readonly Book[],
  active: readonly ActiveBookFilter[],
  augment: BookAugment = identityAugment
): ActiveBookFilter | null {
  if (active.length === 0) {
    return null;
  }
  // Only meaningful when the full set is empty; a non-empty result has no cause.
  if (applyBookFilters(books, active, augment).length === 0) {
    let blamed: ActiveBookFilter | null = null;
    for (const candidate of active) {
      const withoutCandidate = active.filter((entry) => entry !== candidate);
      if (applyBookFilters(books, withoutCandidate, augment).length > 0) {
        if (blamed !== null) {
          // A second condition also rescues the list on its own → the two are
          // jointly responsible and neither can be singled out.
          return null;
        }
        blamed = candidate;
      }
    }
    return blamed;
  }
  return null;
}
