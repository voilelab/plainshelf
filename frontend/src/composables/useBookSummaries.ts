import { computed, toValue, type ComputedRef, type MaybeRefOrGetter } from 'vue';
import type { Book } from '@/types/book';
import { toPlainSummary } from '@/utils/safeHtml';

/**
 * The plain-text summary of every book in a collection, keyed by book id.
 *
 * Stripped once per book rather than once per render: a card or a row
 * re-renders on selection and on every pointer move of a drag.
 *
 * A description of nothing but markup - `<br>`, an empty tag - maps to an empty
 * string, which is each view's cue to fall back to something else or to show
 * nothing at all.
 */
export function useBookSummaries(
  books: MaybeRefOrGetter<readonly Book[]>
): ComputedRef<ReadonlyMap<string, string>> {
  return computed(
    () => new Map(toValue(books).map((book) => [book.id, toPlainSummary(book.comment)]))
  );
}
