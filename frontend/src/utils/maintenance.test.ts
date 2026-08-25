import { describe, expect, it } from 'vitest';
import en from '@/i18n/locales/en';
import zhHant from '@/i18n/locales/zh-Hant';
import { MAINTENANCE_NAV_ITEMS } from './maintenance';

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

describe('maintenance navigation', () => {
  it('lists only the two dedicated maintenance pages', () => {
    expect(MAINTENANCE_NAV_ITEMS.map((item) => item.key)).toEqual([
      'duplicate-content',
      'similar-content'
    ]);
  });

  it('resolves every nav label in both locales', () => {
    for (const item of MAINTENANCE_NAV_ITEMS) {
      expect(resolve(en, item.labelKey), `missing en label for ${item.key}`).toBeTruthy();
      expect(resolve(zhHant, item.labelKey), `missing zh-Hant label for ${item.key}`).toBeTruthy();
    }
  });
});
