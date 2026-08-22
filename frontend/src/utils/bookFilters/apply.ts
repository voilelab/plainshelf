/**
 * Generic evaluation of the declarative book filters over an already-loaded book
 * list. The library page used to hand-write one computed per layer
 * (`searchedBooks` → `layerFilteredBooks` → `filteredBooks`) purely so the empty
 * state could name which layer emptied the list. That does not scale past a few
 * conditions, and it re-encodes ordering the registry already knows. These
 * helpers loop the registry instead: `applyBookFilters` ANDs every active
 * condition, and `soleBlockingFilter` derives the empty-state cause with no
 * per-condition `if`.
 */
import type { Book } from '@/types/book';
import type { AnyBookFilterDef } from './registry';

/** An active condition paired with the value parsed for it. */
export interface ActiveBookFilter {
  readonly filter: AnyBookFilterDef;
  readonly value: unknown;
}

/**
 * Fills in data the listing omits before a predicate reads it — today only the
 * character count, which lives in a lazily loaded index. Applied to every book
 * before every predicate; a filter that needs nothing extra is unaffected, since
 * the returned book still carries all its own fields.
 */
export type BookAugment = (book: Book) => Book;

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
 * The single condition responsible for an empty result, or `null` when there
 * isn't one. A condition is the sole cause when removing *it alone* makes the
 * result non-empty — i.e. every other active condition together still matches
 * something. When two conditions each independently empty the list (removing
 * either still leaves the list empty) or when the list is non-empty, no single
 * condition can be blamed and this returns `null`; the caller then falls back to
 * a generic "nothing matches" message.
 *
 * This is the generic replacement for the hand-written empty-state cascade: the
 * blame follows from the predicates, not from an ordered list of `if`s.
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
