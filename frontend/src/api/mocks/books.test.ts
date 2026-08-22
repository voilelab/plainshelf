import { describe, expect, it } from 'vitest';
import { mockBooks, mockCopyBook, mockGetBook, mockGetBookContent } from './books';

describe('mock book listing shape', () => {
  // The cover filter decides presence from `cover` (the on-disk filename a real
  // listing carries), never the derived `cover_url`. A seed book that renders a
  // cover but omits `cover` would be classified as missing one, making
  // ?cover=none / ?incomplete=1 return nearly the whole seed library in mock
  // mode. Keep the two fields in sync.
  it('carries a cover filename wherever it carries a cover_url', () => {
    const inconsistent = mockBooks
      .filter((book) => book.cover_url)
      .filter((book) => !(typeof book.cover === 'string' && book.cover.trim().length > 0))
      .map((book) => book.id);

    expect(inconsistent).toEqual([]);
  });
});

describe('mockCopyBook', () => {
  it('gives the copy a fresh id in the target layer', () => {
    const copy = mockCopyBook('book-1', 'fiction/copies');

    expect(copy.id).not.toBe('book-1');
    expect(copy.layers).toEqual(['fiction', 'copies']);
    // The copy is discoverable as its own book.
    expect(mockGetBook(copy.id).id).toBe(copy.id);
  });

  // Regression: the copy must read the source's body, not "No content yet.",
  // so navigating straight to it after a mock copy shows real text.
  it('carries the source content across to the copy', () => {
    const original = mockGetBookContent('book-1').content;
    const copy = mockCopyBook('book-1', '');

    expect(mockGetBookContent(copy.id).content).toBe(original);
  });
});
