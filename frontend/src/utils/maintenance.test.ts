import { describe, expect, it } from 'vitest';
import type { Book } from '../types/book';
import en from '../i18n/locales/en';
import zhHant from '../i18n/locales/zh-Hant';
import {
  LOW_CHAR_COUNT_THRESHOLD,
  MAINTENANCE_BOOK_FILTERS,
  MAINTENANCE_NAV_ITEMS,
  clampThreshold,
  hasUnknownCharCount,
  isLowCharCount,
  type MaintenanceBookFilter
} from './maintenance';

function makeBook(overrides: Partial<Book> = {}): Book {
  return {
    id: 'book-1',
    title: 'Untitled',
    authors: [],
    tags: [],
    layers: [],
    ...overrides
  };
}

function resolve(messages: unknown, key: string): string | undefined {
  let cursor: unknown = messages;
  for (const part of key.split('.')) {
    if (!cursor || typeof cursor !== 'object' || !(part in cursor)) {
      return undefined;
    }
    cursor = (cursor as Record<string, unknown>)[part];
  }

  return typeof cursor === 'string' ? cursor : undefined;
}

describe('isLowCharCount', () => {
  it('matches a book below the threshold', () => {
    expect(isLowCharCount(makeBook({ char_count: 500 }), { threshold: 1000 })).toBe(true);
  });

  it('matches a book exactly at the threshold', () => {
    expect(isLowCharCount(makeBook({ char_count: 1000 }), { threshold: 1000 })).toBe(true);
  });

  it('does not match a book above the threshold', () => {
    expect(isLowCharCount(makeBook({ char_count: 1001 }), { threshold: 1000 })).toBe(false);
  });

  it('treats a missing char_count as zero', () => {
    expect(isLowCharCount(makeBook(), { threshold: 1000 })).toBe(true);
    expect(isLowCharCount(makeBook(), { threshold: 0 })).toBe(true);
  });

  it('matches a zero-character book at threshold zero', () => {
    expect(isLowCharCount(makeBook({ char_count: 0 }), { threshold: 0 })).toBe(true);
    expect(isLowCharCount(makeBook({ char_count: 1 }), { threshold: 0 })).toBe(false);
  });
});

describe('hasUnknownCharCount', () => {
  it('flags only books without a numeric char_count', () => {
    expect(hasUnknownCharCount(makeBook())).toBe(true);
    expect(hasUnknownCharCount(makeBook({ char_count: 0 }))).toBe(false);
    expect(hasUnknownCharCount(makeBook({ char_count: 42 }))).toBe(false);
  });
});

describe('clampThreshold', () => {
  const config = LOW_CHAR_COUNT_THRESHOLD;

  it('falls back to the default for missing or unusable values', () => {
    expect(clampThreshold(undefined, config)).toBe(config.defaultValue);
    expect(clampThreshold('', config)).toBe(config.defaultValue);
    expect(clampThreshold('   ', config)).toBe(config.defaultValue);
    expect(clampThreshold('abc', config)).toBe(config.defaultValue);
    expect(clampThreshold('1.5', config)).toBe(config.defaultValue);
  });

  it('clamps values below the minimum', () => {
    expect(clampThreshold('-5', config)).toBe(config.min);
  });

  it('accepts valid integers, including zero and surrounding whitespace', () => {
    expect(clampThreshold('0', config)).toBe(0);
    expect(clampThreshold('2500', config)).toBe(2500);
    expect(clampThreshold('  2500  ', config)).toBe(2500);
  });

  it('clamps absurdly large values to a safe integer', () => {
    expect(clampThreshold('1e400', config)).toBe(config.defaultValue);
    expect(clampThreshold('999999999999999999999', config)).toBe(Number.MAX_SAFE_INTEGER);
  });
});

describe('maintenance registration', () => {
  const bookFilterKeys = MAINTENANCE_NAV_ITEMS
    .map((item) => item.key)
    .filter((key): key is MaintenanceBookFilter => key !== 'duplicate-content');

  it('gives every book-list nav item a filter config', () => {
    for (const key of bookFilterKeys) {
      expect(MAINTENANCE_BOOK_FILTERS[key], `missing filter config for ${key}`).toBeDefined();
    }
  });

  it('resolves every filter message key in both locales', () => {
    for (const config of Object.values(MAINTENANCE_BOOK_FILTERS)) {
      const keys = [
        config.titleKey,
        config.emptyMessageKey,
        config.filterDescriptionKey,
        config.threshold?.labelKey
      ].filter((key): key is string => typeof key === 'string');

      for (const key of keys) {
        expect(resolve(en, key), `missing en message for ${key}`).toBeTruthy();
        expect(resolve(zhHant, key), `missing zh-Hant message for ${key}`).toBeTruthy();
      }
    }
  });

  it('resolves every nav label in both locales', () => {
    for (const item of MAINTENANCE_NAV_ITEMS) {
      expect(resolve(en, item.labelKey), `missing en label for ${item.key}`).toBeTruthy();
      expect(resolve(zhHant, item.labelKey), `missing zh-Hant label for ${item.key}`).toBeTruthy();
    }
  });
});
