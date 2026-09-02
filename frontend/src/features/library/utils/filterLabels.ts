/**
 * Human-readable descriptions of an active book filter, shared by the filter
 * panel and the chip row. The registry stays transport- and i18n-independent;
 * everything that needs words about a condition goes through here so the chip, an
 * empty-state message, and a panel heading always phrase the same value the same
 * way.
 *
 * The branches switch on `panelControl` (the condition's *shape*), never on its
 * key, so a new `facetSingle` field is described with no change here.
 */
import type { Book } from '@/types/book';
import type { t } from '@/i18n';
import type { AnyBookFilterDef, MultiFieldValue } from '@/utils/bookFilters/registry';
import type { FilterFieldValue } from '@/utils/bookFilters/codec';
import type { CharCountRange } from '@/utils/charCountFilter';

/** The translate function returned by `useI18n()`. */
type Translator = typeof t;

/** One distinct value a field carries across the loaded books, with its tally. */
export interface FacetEntry {
  readonly value: string;
  readonly count: number;
}

/** The field's own non-blank values across `books`, de-duplicated and counted. */
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

/** The i18n heading for a panel field ("Author", "Tags", …). */
export function fieldLabel(filter: AnyBookFilterDef, t: Translator): string {
  return t(`library.filters.fields.${filter.key}.label`);
}

/** The note under a field explaining what counts as "unset" for it (AC: labelled). */
export function fieldEmptyNote(filter: AnyBookFilterDef, t: Translator): string {
  return t(`library.filters.fields.${filter.key}.emptyNote`);
}

/** Words for a single none/has/eq value. */
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

/** A character range as "100–5000", "≥100", or "≤5000". */
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

/**
 * The chip / empty-state label for an active condition, e.g. "Author: 魯迅",
 * "Tags: fiction, sci-fi", or "Characters: 100–5000".
 */
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
