import { describe, expect, it } from 'vitest';
import {
  READING_PROGRESS_DOCUMENT_VERSION,
  createReadingProgressDocument,
  getBookReadingOffset,
  parseReadingProgressDocument,
  serializeReadingProgressDocument,
  withBookReadingOffset
} from './document';

describe('parseReadingProgressDocument', () => {
  it('returns an empty document for missing, corrupt, or wrongly shaped input', () => {
    const empty = createReadingProgressDocument();
    expect(parseReadingProgressDocument(null)).toEqual(empty);
    expect(parseReadingProgressDocument('')).toEqual(empty);
    expect(parseReadingProgressDocument('{oops')).toEqual(empty);
    expect(parseReadingProgressDocument('[1,2,3]')).toEqual(empty);
  });

  it('returns an empty document for any other version, new or old', () => {
    const empty = createReadingProgressDocument();
    for (const version of [READING_PROGRESS_DOCUMENT_VERSION + 1, 1]) {
      const text = JSON.stringify({
        version,
        shelves: { main: { 'book-1': { offset: 42, at: 10 } } }
      });
      expect(parseReadingProgressDocument(text)).toEqual(empty);
    }
  });

  it('drops invalid entries but keeps valid offsets and reset tombstones', () => {
    const text = JSON.stringify({
      version: READING_PROGRESS_DOCUMENT_VERSION,
      shelves: {
        main: {
          'book-1': { offset: 42, at: 10 },
          'book-tombstone': { offset: 0, at: 20 },
          'book-bare-zero': { offset: 0, at: 0 },
          'book-negative': { offset: -2, at: 10 },
          'book-float': { offset: 1.5, at: 10 },
          'book-bare-number': 9,
          ' ': { offset: 10, at: 10 }
        },
        ' ': { 'book-2': { offset: 20, at: 10 } },
        broken: []
      }
    });

    expect(parseReadingProgressDocument(text).shelves).toEqual({
      main: {
        'book-1': { offset: 42, at: 10 },
        'book-tombstone': { offset: 0, at: 20 }
      }
    });
  });

  it('round-trips a document it wrote', () => {
    const doc = withBookReadingOffset(createReadingProgressDocument(), 'main', 'book-1', 42, 1700);
    expect(parseReadingProgressDocument(serializeReadingProgressDocument(doc))).toEqual(doc);
  });
});

describe('withBookReadingOffset', () => {
  it('stores offsets independently per shelf and book', () => {
    let doc = createReadingProgressDocument();
    doc = withBookReadingOffset(doc, 'shelf-1', 'book-1', 42);
    doc = withBookReadingOffset(doc, 'shelf-1', 'book-2', 7);
    doc = withBookReadingOffset(doc, 'shelf-2', 'book-1', 99);

    expect(getBookReadingOffset(doc, 'shelf-1', 'book-1')).toBe(42);
    expect(getBookReadingOffset(doc, 'shelf-1', 'book-2')).toBe(7);
    expect(getBookReadingOffset(doc, 'shelf-2', 'book-1')).toBe(99);
    expect(getBookReadingOffset(doc, 'shelf-2', 'missing')).toBe(0);
  });

  it('stamps each write with the provided timestamp', () => {
    const doc = withBookReadingOffset(createReadingProgressDocument(), 'main', 'book-1', 42, 1700);
    expect(doc.shelves.main['book-1']).toEqual({ offset: 42, at: 1700 });
  });

  it('overwrites an offset and leaves a timestamped tombstone when reset to zero', () => {
    let doc = withBookReadingOffset(createReadingProgressDocument(), 'main', 'book-1', 42, 1000);
    doc = withBookReadingOffset(doc, 'main', 'book-1', 99, 2000);
    expect(getBookReadingOffset(doc, 'main', 'book-1')).toBe(99);

    doc = withBookReadingOffset(doc, 'main', 'book-1', 0, 3000);
    expect(getBookReadingOffset(doc, 'main', 'book-1')).toBe(0);
    // A tombstone, not a deletion: it carries the reset time so the reset can
    // win a cross-process merge against an older advance.
    expect(doc.shelves.main['book-1']).toEqual({ offset: 0, at: 3000 });
  });

  it('is a no-op when resetting a book that has no entry', () => {
    const empty = createReadingProgressDocument();
    expect(withBookReadingOffset(empty, 'main', 'book-1', 0)).toBe(empty);
    expect(withBookReadingOffset(empty, 'main', 'book-1', Number.NaN)).toBe(empty);
  });

  it('is a no-op when the offset is unchanged', () => {
    const doc = withBookReadingOffset(createReadingProgressDocument(), 'main', 'book-1', 42, 1000);
    expect(withBookReadingOffset(doc, 'main', 'book-1', 42, 5000)).toBe(doc);
  });

  it('ignores blank keys and treats invalid offsets as reset', () => {
    const empty = createReadingProgressDocument();
    expect(withBookReadingOffset(empty, '', 'book-1', 10)).toBe(empty);
    expect(withBookReadingOffset(empty, 'main', '', 10)).toBe(empty);

    let doc = withBookReadingOffset(empty, 'main', 'book-1', 10, 1000);
    doc = withBookReadingOffset(doc, 'main', 'book-1', Number.NaN, 2000);
    expect(getBookReadingOffset(doc, 'main', 'book-1')).toBe(0);
  });
});
