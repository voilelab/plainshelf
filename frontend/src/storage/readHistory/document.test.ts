import { describe, expect, it } from 'vitest';
import {
  DEFAULT_READ_HISTORY_LIMIT,
  createReadHistoryDocument,
  getShelfReadHistory,
  parseReadHistoryDocument,
  serializeReadHistoryDocument,
  withClearedShelf,
  withLimit,
  withReadEntry
} from './document';

describe('parseReadHistoryDocument', () => {
  it('returns an empty document for missing or blank input', () => {
    expect(parseReadHistoryDocument(null)).toEqual(createReadHistoryDocument());
    expect(parseReadHistoryDocument('   ')).toEqual(createReadHistoryDocument());
  });

  it('falls back to an empty document for unparseable or wrongly shaped data', () => {
    expect(parseReadHistoryDocument('{oops')).toEqual(createReadHistoryDocument());
    expect(parseReadHistoryDocument('[1,2,3]')).toEqual(createReadHistoryDocument());
    expect(parseReadHistoryDocument('"text"')).toEqual(createReadHistoryDocument());
  });

  it('falls back to an empty document for an unknown version', () => {
    const stored = JSON.stringify({ version: 99, limit: 5, shelves: { a: ['book-1'] } });
    expect(parseReadHistoryDocument(stored)).toEqual(createReadHistoryDocument());
  });

  it('round-trips a document it wrote', () => {
    const doc = withReadEntry(createReadHistoryDocument(), 'shelf-1', 'book-1');
    expect(parseReadHistoryDocument(serializeReadHistoryDocument(doc))).toEqual(doc);
  });

  it('drops non-string, blank and duplicate ids and trims to the stored limit', () => {
    const stored = JSON.stringify({
      version: 1,
      limit: 2,
      shelves: { 'shelf-1': ['book-1', 7, '', ' book-1 ', 'book-2', 'book-3'] }
    });

    expect(parseReadHistoryDocument(stored).shelves['shelf-1']).toEqual(['book-1', 'book-2']);
  });

  it('repairs an invalid limit and drops a non-object shelves field', () => {
    const stored = JSON.stringify({ version: 1, limit: -3, shelves: ['book-1'] });
    const doc = parseReadHistoryDocument(stored);

    expect(doc.limit).toBe(DEFAULT_READ_HISTORY_LIMIT);
    expect(doc.shelves).toEqual({});
  });
});

describe('withReadEntry', () => {
  it('puts the newest book first without duplicating it', () => {
    let doc = createReadHistoryDocument();
    doc = withReadEntry(doc, 'shelf-1', 'book-1');
    doc = withReadEntry(doc, 'shelf-1', 'book-2');
    doc = withReadEntry(doc, 'shelf-1', 'book-1');

    expect(getShelfReadHistory(doc, 'shelf-1')).toEqual(['book-1', 'book-2']);
  });

  it('keeps shelves independent', () => {
    let doc = createReadHistoryDocument();
    doc = withReadEntry(doc, 'shelf-1', 'book-1');
    doc = withReadEntry(doc, 'shelf-2', 'book-2');

    expect(getShelfReadHistory(doc, 'shelf-1')).toEqual(['book-1']);
    expect(getShelfReadHistory(doc, 'shelf-2')).toEqual(['book-2']);
  });

  it('trims to the limit, dropping the oldest entry', () => {
    let doc = withLimit(createReadHistoryDocument(), 2);
    doc = withReadEntry(doc, 'shelf-1', 'book-1');
    doc = withReadEntry(doc, 'shelf-1', 'book-2');
    doc = withReadEntry(doc, 'shelf-1', 'book-3');

    expect(getShelfReadHistory(doc, 'shelf-1')).toEqual(['book-3', 'book-2']);
  });

  it('records nothing when the limit is 0 or the id is blank', () => {
    const disabled = withLimit(createReadHistoryDocument(), 0);
    expect(withReadEntry(disabled, 'shelf-1', 'book-1')).toBe(disabled);

    const doc = createReadHistoryDocument();
    expect(withReadEntry(doc, 'shelf-1', '   ')).toBe(doc);
  });
});

describe('withClearedShelf', () => {
  it('clears one shelf and leaves the others alone', () => {
    let doc = createReadHistoryDocument();
    doc = withReadEntry(doc, 'shelf-1', 'book-1');
    doc = withReadEntry(doc, 'shelf-2', 'book-2');

    const cleared = withClearedShelf(doc, 'shelf-1');

    expect(getShelfReadHistory(cleared, 'shelf-1')).toEqual([]);
    expect(getShelfReadHistory(cleared, 'shelf-2')).toEqual(['book-2']);
  });
});

describe('withLimit', () => {
  it('trims every shelf to the new limit', () => {
    let doc = createReadHistoryDocument();
    doc = withReadEntry(doc, 'shelf-1', 'book-1');
    doc = withReadEntry(doc, 'shelf-1', 'book-2');
    doc = withReadEntry(doc, 'shelf-2', 'book-3');
    doc = withReadEntry(doc, 'shelf-2', 'book-4');

    const limited = withLimit(doc, 1);

    expect(limited.limit).toBe(1);
    expect(getShelfReadHistory(limited, 'shelf-1')).toEqual(['book-2']);
    expect(getShelfReadHistory(limited, 'shelf-2')).toEqual(['book-4']);
  });

  it('discards all history when set to 0', () => {
    let doc = createReadHistoryDocument();
    doc = withReadEntry(doc, 'shelf-1', 'book-1');

    expect(withLimit(doc, 0).shelves).toEqual({});
  });
});
