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

  it('returns an empty document for a future version', () => {
    const text = JSON.stringify({
      version: READING_PROGRESS_DOCUMENT_VERSION + 1,
      shelves: { main: { 'book-1': 42 } }
    });
    expect(parseReadingProgressDocument(text)).toEqual(createReadingProgressDocument());
  });

  it('drops invalid shelf keys, book ids, and offsets', () => {
    const text = JSON.stringify({
      version: READING_PROGRESS_DOCUMENT_VERSION,
      shelves: {
        main: {
          'book-1': 42,
          'book-zero': 0,
          'book-negative': -2,
          'book-float': 1.5,
          'book-text': '9',
          ' ': 10
        },
        ' ': { 'book-2': 20 },
        broken: []
      }
    });

    expect(parseReadingProgressDocument(text).shelves).toEqual({ main: { 'book-1': 42 } });
  });

  it('round-trips a document it wrote', () => {
    const doc = withBookReadingOffset(createReadingProgressDocument(), 'main', 'book-1', 42);
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

  it('overwrites an offset and removes it when reset to zero', () => {
    let doc = withBookReadingOffset(createReadingProgressDocument(), 'main', 'book-1', 42);
    doc = withBookReadingOffset(doc, 'main', 'book-1', 99);
    expect(getBookReadingOffset(doc, 'main', 'book-1')).toBe(99);

    doc = withBookReadingOffset(doc, 'main', 'book-1', 0);
    expect(getBookReadingOffset(doc, 'main', 'book-1')).toBe(0);
    expect(doc.shelves).toEqual({});
  });

  it('ignores blank keys and treats invalid offsets as reset', () => {
    const empty = createReadingProgressDocument();
    expect(withBookReadingOffset(empty, '', 'book-1', 10)).toBe(empty);
    expect(withBookReadingOffset(empty, 'main', '', 10)).toBe(empty);

    let doc = withBookReadingOffset(empty, 'main', 'book-1', 10);
    doc = withBookReadingOffset(doc, 'main', 'book-1', Number.NaN);
    expect(doc.shelves).toEqual({});
  });
});
