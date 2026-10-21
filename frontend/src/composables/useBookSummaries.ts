import { computed, toValue, type ComputedRef, type MaybeRefOrGetter } from 'vue';
import type { Book } from '@/types/book';
import { toPlainSummary } from '@/utils/safeHtml';

export function useBookSummaries(
  books: MaybeRefOrGetter<readonly Book[]>
): ComputedRef<ReadonlyMap<string, string>> {
  return computed(
    () => new Map(toValue(books).map((book) => [book.id, toPlainSummary(book.comment)]))
  );
}
