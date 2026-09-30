import { describe, expect, it } from 'vitest';
import { t } from '@/i18n';
import {
  authorFilter,
  charCountFilter,
  coverFilter,
  tagsFilter,
  type AnyBookFilterDef
} from '@/utils/bookFilters/registry';
import { facetEntries, filterValueLabel } from './filterLabels';
import type { Book } from '@/types/book';

// These are the words the filter chips and the library's empty state are made
// of — "Cover: Unset" on the chip, "No books match Cover: Present." when that
// condition empties the list. Two end-to-end cases used to read them off a real
// page; the composition is pure, so it is checked here. Which condition gets
// blamed is `apply.test.ts`; the chip row's own wiring is
// `useBookFilters.test.ts`.

function asDef(filter: unknown): AnyBookFilterDef {
  return filter as AnyBookFilterDef;
}

function book(fields: Partial<Book>): Book {
  return { id: 'b1', title: 'Untitled', ...fields } as Book;
}

describe('filterValueLabel', () => {
  it('names the field and the value for a tri-state condition', () => {
    expect(filterValueLabel(asDef(coverFilter), { kind: 'none' }, t)).toBe('Cover: Unset');
    expect(filterValueLabel(asDef(coverFilter), { kind: 'has' }, t)).toBe('Cover: Present');
  });

  it('quotes the picked value itself for a single facet', () => {
    expect(filterValueLabel(asDef(authorFilter), { kind: 'eq', value: '魯迅' }, t)).toBe(
      'Author: 魯迅'
    );
    expect(filterValueLabel(asDef(authorFilter), { kind: 'none' }, t)).toBe('Author: Unset');
  });

  it('joins a repeatable field into one chip rather than one chip per value', () => {
    const label = filterValueLabel(
      asDef(tagsFilter),
      { values: [{ kind: 'eq', value: 'fiction' }, { kind: 'eq', value: 'sci-fi' }], op: 'all' },
      t
    );

    expect(label).toBe('Tags: fiction, sci-fi');
  });

  it('writes a character range as a range, and an open bound as an inequality', () => {
    const label = (value: unknown) => filterValueLabel(asDef(charCountFilter), value, t);

    expect(label({ min: 100, max: 5000 })).toBe('Characters: 100–5000');
    expect(label({ min: 100 })).toBe('Characters: ≥100');
    expect(label({ max: 5000 })).toBe('Characters: ≤5000');
  });

  it('keeps a zero bound, which is a real filter and not an absent one', () => {
    expect(filterValueLabel(asDef(charCountFilter), { max: 0 }, t)).toBe('Characters: ≤0');
  });
});

describe('facetEntries', () => {
  it('de-duplicates and tallies the values a field carries across the books', () => {
    const books = [
      book({ authors: ['Ursula K. Le Guin'] }),
      book({ authors: ['Ursula K. Le Guin'] }),
      book({ authors: ['Italo Calvino'] })
    ];

    expect(facetEntries(books, asDef(authorFilter))).toEqual([
      { value: 'Italo Calvino', count: 1 },
      { value: 'Ursula K. Le Guin', count: 2 }
    ]);
  });

  it('offers nothing to pick when no book carries the field', () => {
    expect(facetEntries([book({})], asDef(authorFilter))).toEqual([]);
  });
});
