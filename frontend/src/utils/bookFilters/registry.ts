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
import {
  filterFieldOpKey,
  parseFilterField,
  parseFilterFieldOp,
  serializeFilterField,
  serializeFilterFieldOp,
  type FilterFieldOp,
  type FilterFieldValue
} from './codec';

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
  /**
   * Fills in the datum the listing omits from the loaded data, returning a book
   * the predicate can decide correctly. This is the bridge that keeps
   * `predicate` transport-independent: a consumer evaluates
   * `predicate(dependency.augment(book), value)` so the lazily fetched value is
   * applied without the predicate ever knowing where the value came from.
   */
  augment(book: Book): Book;
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
  // Transport-independent: reads only the book's own count. A consumer that has
  // loaded the dependency first passes `createDependency().augment(book)` here so
  // a count that lives only in the lazy index is applied; the predicate itself
  // never learns where the count came from.
  predicate: (book, value) => isCharCountInRange(book.char_count, value),
  // Hidden on a backend that cannot afford the counts (pCloud would read every
  // book's source over the network to answer includeCharCount).
  supported: () => getBookshelfProvider().supportsCharCountListing?.() !== false,
  createDependency: () => {
    const index = useCharCountIndex();
    return {
      load: () => index.load(),
      ready: () => index.ready.value,
      // The listing omits char_count; a count fetched into the index is the one
      // the predicate must see. book.char_count already present (mock data, or a
      // listing that did include it) is left untouched.
      augment: (book) =>
        book.char_count !== undefined
          ? book
          : { ...book, char_count: index.counts.value.get(book.id) },
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
 * A book's non-blank entries for a field, trimmed. This is the single place
 * "the field is absent" is decided, so a field filter's `none` value, the facet
 * that will count it, and the derived `incomplete` condition can never disagree
 * about what counts as missing. Accepts a bare string (treated as one entry) as
 * well as the string arrays `authors`/`tags` carry.
 */
function nonBlankValues(raw: unknown): string[] {
  if (typeof raw === 'string') {
    const trimmed = raw.trim();
    return trimmed.length > 0 ? [trimmed] : [];
  }
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw
    .filter((item): item is string => typeof item === 'string')
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
}

function authorValues(book: Book): string[] {
  return nonBlankValues(book.authors);
}

function tagValues(book: Book): string[] {
  return nonBlankValues(book.tags);
}

// The cover contract is presence, not a URL. The listing carries `cover` — the
// on-disk filename, '' when the book has none — while `cover_url` is derived
// downstream (api/books.ts) and is deliberately left unset on the pCloud
// backend even when a cover exists, because its links expire. Reading it here
// would report every pCloud book as missing its cover, so both this predicate
// and any future facet read `cover` alone.
function coverValues(book: Book): string[] {
  return nonBlankValues(book.cover);
}

function languageValues(book: Book): string[] {
  return nonBlankValues(book.language);
}

export function isMissingAuthor(book: Book): boolean {
  return authorValues(book).length === 0;
}

export function isMissingCover(book: Book): boolean {
  return coverValues(book).length === 0;
}

export function isMissingLanguage(book: Book): boolean {
  return languageValues(book).length === 0;
}

/** Decides one field value against a book's own non-blank entries for that field. */
function matchesFieldValue(values: readonly string[], value: FilterFieldValue): boolean {
  switch (value.kind) {
    case 'none':
      return values.length === 0;
    case 'has':
      return values.length > 0;
    case 'eq':
      return values.includes(value.value);
  }
}

/**
 * A presence/equality condition over one book field, carried in the URL as a
 * single `none` / `has` / `eq:<value>` token (see codec.ts). All four metadata
 * filters read data the listing already returns, so — unlike charCount — they
 * are free client-side predicates with no `supported`/`createDependency`. Their
 * control lives in the filter panel added in a later change, hence `panel`.
 */
function fieldFilter(
  key: string,
  values: (book: Book) => string[]
): BookFilterDef<FilterFieldValue | undefined> {
  return defineBookFilter<FilterFieldValue | undefined>({
    key,
    queryKeys: [key],
    chrome: 'panel',
    parse: (query) => parseFilterField(toSingleQueryValue(query[key])),
    serialize: (value) => (value ? { [key]: serializeFilterField(value) } : {}),
    isActive: (value) => value !== undefined,
    predicate: (book, value) => value === undefined || matchesFieldValue(values(book), value)
  });
}

// `author` is a single token = one author selectable at a time, even though the
// datum is an array; if multi-select AND is ever wanted, give it a `<field>Op`
// like `tags` below rather than changing the key.
export const authorFilter = fieldFilter('author', authorValues);
export const coverFilter = fieldFilter('cover', coverValues);
export const languageFilter = fieldFilter('language', languageValues);

/** A repeatable field with a sibling `<field>Op` operator. */
export interface MultiFieldValue {
  readonly values: readonly FilterFieldValue[];
  readonly op: FilterFieldOp;
}

export const tagsFilter = defineBookFilter<MultiFieldValue>({
  key: 'tags',
  queryKeys: ['tags', filterFieldOpKey('tags')],
  chrome: 'panel',
  parse: (query) => {
    const raw = query.tags;
    const tokens = Array.isArray(raw) ? raw : raw == null ? [] : [raw];
    const values = tokens
      .map((token) => parseFilterField(token ?? undefined))
      .filter((value): value is FilterFieldValue => value !== undefined);
    const op = parseFilterFieldOp(toSingleQueryValue(query[filterFieldOpKey('tags')])) ?? 'all';
    return { values, op };
  },
  serialize: (value) => {
    const query: Record<string, string | string[]> = {};
    if (value.values.length > 0) {
      query.tags = value.values.map(serializeFilterField);
      query[filterFieldOpKey('tags')] = serializeFilterFieldOp(value.op);
    }
    return query;
  },
  isActive: (value) => value.values.length > 0,
  // `all` (AND) is the only operator today: every listed value must match.
  predicate: (book, value) => {
    const tags = tagValues(book);
    return value.values.every((entry) => matchesFieldValue(tags, entry));
  }
});

export const incompleteFilter = defineBookFilter<boolean>({
  key: 'incomplete',
  queryKeys: ['incomplete'],
  // Reached from the sidebar's "needs tidying" entry, not a panel control, so it
  // is `inline` like the navigation-driven layer filter.
  chrome: 'inline',
  parse: (query) => toSingleQueryValue(query.incomplete) === '1',
  serialize: (value) => {
    const query: Record<string, string> = {};
    if (value) {
      query.incomplete = '1';
    }
    return query;
  },
  isActive: (value) => value,
  // The one derived condition — a book still needing metadata work — kept as a
  // single OR predicate rather than three composed filters, which the registry
  // would AND together. Tags are intentionally excluded: an empty tag set is
  // not "incomplete".
  predicate: (book) => isMissingAuthor(book) || isMissingCover(book) || isMissingLanguage(book)
});

/**
 * The filters in the order the query builder applies them. New conditions are
 * added here (and, when they narrow the list, wired into the page's filtering);
 * nothing in `buildBooksQuery` needs to change.
 */
export const BOOK_FILTERS: readonly AnyBookFilterDef[] = [
  searchFilter,
  layersFilter,
  charCountFilter,
  authorFilter,
  tagsFilter,
  coverFilter,
  languageFilter,
  incompleteFilter
] as unknown as readonly AnyBookFilterDef[];

/**
 * The subset the library page applies itself as plain client-side predicates
 * over the listing. search/layer/charCount are filtered by LibraryPage's own
 * computeds (charCount needs its lazy index, and the empty state distinguishes
 * their causes); these read data already on every book, so the page loops them
 * generically. Ordering is irrelevant — they are ANDed together.
 */
export const METADATA_BOOK_FILTERS: readonly AnyBookFilterDef[] = [
  authorFilter,
  tagsFilter,
  coverFilter,
  languageFilter,
  incompleteFilter
] as unknown as readonly AnyBookFilterDef[];
