/**
 * Declarative registry of the library page's book filters.
 *
 * Each condition states, in one place, how it is carried in the URL
 * (`queryKeys` / `parse` / `serialize`), whether it is currently narrowing the
 * list (`isActive`), how it decides a single book (`predicate`), and — for the
 * conditions that need it — whether the backend can support it (`supported`) and
 * what extra data it must load first (`createDependency`).
 *
 * The point is that adding a condition is adding one entry here, not editing the
 * query builder, hand-writing a computed, and remembering every key to strip.
 * `buildBooksQuery` (useBooksRouteQuery.ts) loops over `BOOK_FILTERS` and touches
 * only the URL parts; `predicate` stays deliberately transport-independent so a
 * future query language can reuse the same leaf checks its AST evaluates to.
 */
import type { LocationQuery } from 'vue-router';
import type { Book } from '@/types/book';
import { toSingleQueryValue } from '@/composables/useBookPagination';
import { useCharCountIndex } from '@/composables/useCharCountIndex';
import { getBookshelfProvider } from '@/providers';
import { getLayerPath, layerPathEquals } from '@/utils/layers';
import { bookMatchesSearch } from '@/utils/bookSearch';
import {
  isCharCountInRange,
  isCharCountRangeActive,
  parseCharCountRange,
  type CharCountRange
} from '@/utils/charCountFilter';

/**
 * Where a filter's control lives. `inline` conditions sit in the toolbar (or are
 * driven by navigation, as the layer is); `panel` conditions belong to the
 * filter panel added in a later change. The current three are all `inline`.
 */
export type FilterChrome = 'inline' | 'panel';

/**
 * Extra, lazily loaded data a filter needs before it can decide books — today
 * only the character-count index. Kept off the generic path: `buildBooksQuery`
 * never looks at it, and only a consumer that actually filters the list creates
 * and reads it.
 */
export interface FilterDependency {
  /** Ensures the extra data is loaded (idempotent). */
  load(): void | Promise<void>;
  /** Whether the extra data has arrived and the filter may be applied. */
  ready(): boolean;
  /** How many of `books` the filter cannot decide because their datum is unknown. */
  unknownCount(books: readonly Book[], value: unknown): number;
}

export interface BookFilterDef<T> {
  /** Stable identity of the condition, independent of its query keys. */
  readonly key: string;
  /** Every query key the condition owns; all are cleared before re-serializing. */
  readonly queryKeys: readonly string[];
  readonly chrome: FilterChrome;
  /** Reads the condition's value out of the current query. */
  parse(query: LocationQuery): T;
  /** Writes an active value back as the query keys it owns. */
  serialize(value: T): Record<string, string | string[]>;
  /** Whether `value` actually narrows the list (an empty value does not). */
  isActive(value: T): boolean;
  /** Decides a single book. Must depend only on `book` and `value`, never on transport. */
  predicate(book: Book, value: T): boolean;
  /** Whether the active backend can answer this condition at all. */
  supported?(): boolean;
  /** Builds this condition's lazy data dependency; call from a component setup. */
  createDependency?(): FilterDependency;
}

/** A filter whose value type has been erased for iteration over the registry. */
export type AnyBookFilterDef = BookFilterDef<unknown>;

/** Identity helper that keeps each definition checked against its own value type. */
function defineBookFilter<T>(def: BookFilterDef<T>): BookFilterDef<T> {
  return def;
}

/** Mirrors `toLayerPath`: a blank layer value is "no layer", not an empty path. */
function normalizeLayerValue(raw: string | undefined): string | undefined {
  if (raw === undefined) {
    return undefined;
  }
  const trimmed = raw.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

export const searchFilter = defineBookFilter<string>({
  key: 'search',
  queryKeys: ['search'],
  chrome: 'inline',
  parse: (query) => (toSingleQueryValue(query.search) ?? '').trim(),
  serialize: (value) => {
    const normalized = value.trim();
    const query: Record<string, string> = {};
    if (normalized) {
      query.search = normalized;
    }
    return query;
  },
  isActive: (value) => value.trim() !== '',
  predicate: (book, value) => bookMatchesSearch(book, value)
});

export const layersFilter = defineBookFilter<string | undefined>({
  key: 'layers',
  // `layer` (singular) is the legacy key: read for back-compat, always rewritten
  // as `layers`, and stripped either way.
  queryKeys: ['layers', 'layer'],
  chrome: 'inline',
  parse: (query) =>
    normalizeLayerValue(toSingleQueryValue(query.layers))
    ?? normalizeLayerValue(toSingleQueryValue(query.layer)),
  serialize: (value) => {
    const normalized = normalizeLayerValue(value);
    const query: Record<string, string> = {};
    if (normalized) {
      query.layers = normalized;
    }
    return query;
  },
  isActive: (value) => normalizeLayerValue(value) !== undefined,
  predicate: (book, value) => {
    const normalized = normalizeLayerValue(value);
    return normalized === undefined || layerPathEquals(getLayerPath(book), normalized);
  }
});

export const charCountFilter = defineBookFilter<CharCountRange>({
  key: 'charCount',
  queryKeys: ['minChars', 'maxChars'],
  chrome: 'inline',
  parse: (query) =>
    parseCharCountRange(toSingleQueryValue(query.minChars), toSingleQueryValue(query.maxChars)),
  serialize: (value) => {
    const query: Record<string, string> = {};
    if (value.min !== undefined) {
      query.minChars = String(value.min);
    }
    if (value.max !== undefined) {
      query.maxChars = String(value.max);
    }
    return query;
  },
  isActive: (value) => isCharCountRangeActive(value),
  // Transport-independent: reads only the listing's own count. The lazily loaded
  // index that fills in books whose count is not in the listing is the
  // dependency's job, not the predicate's.
  predicate: (book, value) => isCharCountInRange(book.char_count, value),
  // Hidden on a backend that cannot afford the counts (pCloud would read every
  // book's source over the network to answer includeCharCount).
  supported: () => getBookshelfProvider().supportsCharCountListing?.() !== false,
  createDependency: () => {
    const index = useCharCountIndex();
    return {
      load: () => index.load(),
      ready: () => index.ready.value,
      unknownCount: (books, value) => {
        const range = value as CharCountRange;
        if (!isCharCountRangeActive(range) || !index.ready.value) {
          return 0;
        }
        return books.filter(
          (book) => (book.char_count ?? index.counts.value.get(book.id)) === undefined
        ).length;
      }
    };
  }
});

/**
 * The filters in the order the query builder applies them. New conditions are
 * added here (and, when they narrow the list, wired into the page's filtering);
 * nothing in `buildBooksQuery` needs to change.
 */
export const BOOK_FILTERS: readonly AnyBookFilterDef[] = [
  searchFilter,
  layersFilter,
  charCountFilter
] as unknown as readonly AnyBookFilterDef[];
