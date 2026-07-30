import { describe, expect, it } from 'vitest';
import { countPages, pageSlice } from './useBookCollectionRoute';

describe('countPages', () => {
  it('rounds up a partial last page', () => {
    expect(countPages(101, 50)).toBe(3);
    expect(countPages(100, 50)).toBe(2);
  });

  it('stays at 1 for an empty list, so page 1 is always valid', () => {
    expect(countPages(0, 50)).toBe(1);
  });

  it('gives every item its own page when the page size is 1', () => {
    expect(countPages(7, 1)).toBe(7);
  });
});

describe('pageSlice', () => {
  const items = Array.from({ length: 10 }, (_, index) => index);

  it('returns the window for the requested 1-based page', () => {
    expect(pageSlice(items, 1, 3)).toEqual([0, 1, 2]);
    expect(pageSlice(items, 2, 3)).toEqual([3, 4, 5]);
  });

  it('returns a short final page', () => {
    expect(pageSlice(items, 4, 3)).toEqual([9]);
  });

  it('returns nothing past the end rather than throwing', () => {
    expect(pageSlice(items, 99, 3)).toEqual([]);
  });

  it('does not mutate the source list', () => {
    const source = [...items];
    pageSlice(source, 2, 3);
    expect(source).toEqual(items);
  });
});
