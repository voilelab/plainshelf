import { useShelfCapabilities } from '@/composables/useShelfCapabilities';
import type { LocationQuery } from 'vue-router';
import type { Book } from '@/types/book';
import { toSingleQueryValue } from '@/composables/useBookPagination';
import { useCharCountIndex } from '@/composables/useCharCountIndex';
import { getBookshelfProvider } from '@/providers';
import { getFolderPath, folderPathEquals } from '@/utils/folders';
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

type FilterChrome = 'inline' | 'panel';

interface FilterDependency {
  load(): void | Promise<void>;
  ready(): boolean;
  augment(book: Book): Book;
  unknownCount(books: readonly Book[], value: unknown): number;
}

type PanelControlKind = 'range' | 'triState' | 'facetSingle' | 'facetMulti';

interface BookFilterDef<T> {
  readonly key: string;
  readonly queryKeys: readonly string[];
  readonly chrome: FilterChrome;
  parse(query: LocationQuery): T;
  serialize(value: T): Record<string, string | string[]>;
  isActive(value: T): boolean;
  predicate(book: Book, value: T): boolean;
  supported?(): boolean;
  createDependency?(): FilterDependency;
  readonly panelControl?: PanelControlKind;
  readonly clearedValue?: T;
  facetValues?(book: Book): string[];
}

export type AnyBookFilterDef = BookFilterDef<unknown>;

function defineBookFilter<T>(def: BookFilterDef<T>): BookFilterDef<T> {
  return def;
}

function normalizeFolderValue(raw: string | undefined): string | undefined {
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

export const foldersFilter = defineBookFilter<string | undefined>({
  key: 'folders',
  queryKeys: ['folders', 'folder'],
  chrome: 'inline',
  parse: (query) =>
    normalizeFolderValue(toSingleQueryValue(query.folders))
    ?? normalizeFolderValue(toSingleQueryValue(query.folder)),
  serialize: (value) => {
    const normalized = normalizeFolderValue(value);
    const query: Record<string, string> = {};
    if (normalized) {
      query.folders = normalized;
    }
    return query;
  },
  isActive: (value) => normalizeFolderValue(value) !== undefined,
  predicate: (book, value) => {
    const normalized = normalizeFolderValue(value);
    return normalized === undefined || folderPathEquals(getFolderPath(book), normalized);
  }
});

export const charCountFilter = defineBookFilter<CharCountRange>({
  key: 'charCount',
  queryKeys: ['minChars', 'maxChars'],
  chrome: 'panel',
  panelControl: 'range',
  clearedValue: {},
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
  predicate: (book, value) => isCharCountInRange(book.char_count, value),
  supported: () => useShelfCapabilities().charCount.value,
  createDependency: () => {
    const index = useCharCountIndex();
    return {
      load: () => index.load(),
      ready: () => index.ready.value,
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

function coverValues(book: Book): string[] {
  return nonBlankValues(book.cover);
}

function languageValues(book: Book): string[] {
  return nonBlankValues(book.language);
}

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

function fieldFilter(
  key: string,
  values: (book: Book) => string[],
  panelControl: PanelControlKind
): BookFilterDef<FilterFieldValue | undefined> {
  return defineBookFilter<FilterFieldValue | undefined>({
    key,
    queryKeys: [key],
    chrome: 'panel',
    panelControl,
    clearedValue: undefined,
    facetValues: values,
    parse: (query) => parseFilterField(toSingleQueryValue(query[key])),
    serialize: (value) => (value ? { [key]: serializeFilterField(value) } : {}),
    isActive: (value) => value !== undefined,
    predicate: (book, value) => value === undefined || matchesFieldValue(values(book), value)
  });
}

export const authorFilter = fieldFilter('author', authorValues, 'facetSingle');
export const coverFilter = fieldFilter('cover', coverValues, 'triState');
export const languageFilter = fieldFilter('language', languageValues, 'facetSingle');

export interface MultiFieldValue {
  readonly values: readonly FilterFieldValue[];
  readonly op: FilterFieldOp;
}

export const tagsFilter = defineBookFilter<MultiFieldValue>({
  key: 'tags',
  queryKeys: ['tags', filterFieldOpKey('tags')],
  chrome: 'panel',
  panelControl: 'facetMulti',
  clearedValue: { values: [], op: 'all' },
  facetValues: tagValues,
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
  predicate: (book, value) => {
    const tags = tagValues(book);
    return value.values.every((entry) => matchesFieldValue(tags, entry));
  }
});

export const BOOK_FILTERS: readonly AnyBookFilterDef[] = [
  searchFilter,
  foldersFilter,
  charCountFilter,
  authorFilter,
  tagsFilter,
  coverFilter,
  languageFilter
] as unknown as readonly AnyBookFilterDef[];

export const PANEL_BOOK_FILTERS: readonly AnyBookFilterDef[] = [
  authorFilter,
  tagsFilter,
  languageFilter,
  coverFilter,
  charCountFilter
] as unknown as readonly AnyBookFilterDef[];
