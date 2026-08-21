import { describe, expect, it } from 'vitest';
import { mockCopyBook, mockGetBook, mockGetBookContent } from './books';

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
