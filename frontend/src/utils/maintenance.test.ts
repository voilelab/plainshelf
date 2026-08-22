import { describe, expect, it } from 'vitest';
import en from '@/i18n/locales/en';
import zhHant from '@/i18n/locales/zh-Hant';
import {
  MAINTENANCE_BOOK_FILTERS,
  MAINTENANCE_NAV_ITEMS,
  type MaintenanceBookFilter
} from './maintenance';

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

describe('maintenance registration', () => {
  // duplicate-content and similar-content are dedicated pages, not book-list
  // filters, so they carry no MAINTENANCE_BOOK_FILTERS entry.
  const dedicatedPages = new Set(['duplicate-content', 'similar-content']);
  const bookFilterKeys = MAINTENANCE_NAV_ITEMS
    .map((item) => item.key)
    .filter((key): key is MaintenanceBookFilter => !dedicatedPages.has(key));

  it('gives every book-list nav item a filter config', () => {
    for (const key of bookFilterKeys) {
      expect(MAINTENANCE_BOOK_FILTERS[key], `missing filter config for ${key}`).toBeDefined();
    }
  });

  it('resolves every filter message key in both locales', () => {
    for (const config of Object.values(MAINTENANCE_BOOK_FILTERS)) {
      for (const key of [config.titleKey, config.emptyMessageKey]) {
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
