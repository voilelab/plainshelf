import type { Book } from '@/types/book';
import type { t } from '@/i18n';
import type { AnyBookFilterDef, MultiFieldValue } from '@/utils/bookFilters/registry';
import type { FilterFieldValue } from '@/utils/bookFilters/codec';
import type { CharCountRange } from '@/utils/charCountFilter';

type Translator = typeof t;

export interface FacetEntry {
  readonly value: string;
  readonly count: number;
}

export function facetEntries(books: readonly Book[], filter: AnyBookFilterDef): FacetEntry[] {
  const counts = new Map<string, number>();
  for (const book of books) {
    for (const value of filter.facetValues?.(book) ?? []) {
      counts.set(value, (counts.get(value) ?? 0) + 1);
    }
  }
  return [...counts.entries()]
    .map(([value, count]) => ({ value, count }))
    .sort((a, b) => a.value.localeCompare(b.value));
}

export function fieldLabel(filter: AnyBookFilterDef, t: Translator): string {
  return t(`library.filters.fields.${filter.key}.label`);
}

export function fieldEmptyNote(filter: AnyBookFilterDef, t: Translator): string {
  return t(`library.filters.fields.${filter.key}.emptyNote`);
}

function fieldValueLabel(value: FilterFieldValue, t: Translator): string {
  switch (value.kind) {
    case 'none':
      return t('library.filters.value.unset');
    case 'has':
      return t('library.filters.value.has');
    case 'eq':
      return value.value;
  }
}

function charCountRangeLabel(range: CharCountRange): string {
  const { min, max } = range;
  if (min !== undefined && max !== undefined) {
    return `${min}–${max}`;
  }
  if (min !== undefined) {
    return `≥${min}`;
  }
  if (max !== undefined) {
    return `≤${max}`;
  }
  return '';
}

export function filterValueLabel(
  filter: AnyBookFilterDef,
  value: unknown,
  t: Translator
): string {
  const field = fieldLabel(filter, t);
  switch (filter.panelControl) {
    case 'range': {
      return t('library.filters.chip.pair', {
        field,
        value: charCountRangeLabel(value as CharCountRange)
      });
    }
    case 'facetMulti': {
      const multi = value as MultiFieldValue;
      const joined = multi.values
        .map((entry) => fieldValueLabel(entry, t))
        .join(t('library.filters.chip.separator'));
      return t('library.filters.chip.pair', { field, value: joined });
    }
    case 'triState':
    case 'facetSingle': {
      return t('library.filters.chip.pair', {
        field,
        value: fieldValueLabel(value as FilterFieldValue, t)
      });
    }
    default:
      return field;
  }
}
