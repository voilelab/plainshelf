import { describe, expect, it } from 'vitest';
import en from './locales/en';
import zhHant from './locales/zh-Hant';

type Messages = { [key: string]: string | Messages };

// Values that are intentionally identical in every locale: product and brand
// names, language endonyms, and example values shown inside placeholders.
const SHARED_VALUE_KEYS = new Set([
  'app.name',
  'language.en',
  'language.zhHant',
  'libraryForms.editBook.identifierKeyPlaceholder',
  'libraryForms.editBook.identifierValuePlaceholder',
  'settings.shelves.idColumn',
  'settings.shelves.modifyShelfIDLabel',
  'mobileConnect.modePCloud',
  'mobileConnect.pcloud.picker.rootLabel',
  'mobileConnect.pcloud.shelfRootPlaceholder',
  'mobileConnect.serverUrlPlaceholder'
]);

function flatten(messages: Messages, prefix = ''): Record<string, string> {
  const flat: Record<string, string> = {};

  for (const [key, value] of Object.entries(messages)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (typeof value === 'string') {
      flat[path] = value;
    } else {
      Object.assign(flat, flatten(value, path));
    }
  }

  return flat;
}

function placeholders(template: string): string[] {
  return (template.match(/\{\w+\}/g) ?? []).sort();
}

const enFlat = flatten(en as Messages);
const zhFlat = flatten(zhHant as Messages);

describe('locale catalogs', () => {
  it('defines the same keys in every locale', () => {
    expect(Object.keys(zhFlat).sort()).toEqual(Object.keys(enFlat).sort());
  });

  it('uses the same interpolation placeholders in every locale', () => {
    for (const [key, value] of Object.entries(enFlat)) {
      expect({ key, placeholders: placeholders(zhFlat[key] ?? '') }).toEqual({
        key,
        placeholders: placeholders(value)
      });
    }
  });

  it('translates every zh-Hant value that is not deliberately shared', () => {
    const untranslated = Object.keys(enFlat).filter(
      (key) => !SHARED_VALUE_KEYS.has(key) && zhFlat[key] === enFlat[key]
    );

    expect(untranslated).toEqual([]);
  });

  it('keeps the shared-value allowlist free of stale entries', () => {
    const stale = [...SHARED_VALUE_KEYS].filter((key) => zhFlat[key] !== enFlat[key]);

    expect(stale).toEqual([]);
  });
});
