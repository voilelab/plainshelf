import { describe, expect, it } from 'vitest';

import {
  createReadingStatsDocument,
  getReadingStatsRange,
  MAX_SECONDS_PER_CALL,
  MAX_SECONDS_PER_DAY,
  parseReadingStatsDocument,
  READING_STATS_DOCUMENT_VERSION,
  READING_STATS_RETENTION_DAYS,
  serializeReadingStatsDocument,
  toIsoDate,
  withReadingSeconds
} from './document';

const NOW = new Date('2026-08-02T10:00:00');
const TODAY = '2026-08-02';
const KEY = 'shelf-1';

function daysBefore(reference: Date, days: number): string {
  const date = new Date(reference);
  date.setDate(date.getDate() - days);
  return toIsoDate(date);
}

describe('parseReadingStatsDocument', () => {
  it.each([
    ['null', null],
    ['empty text', '   '],
    ['unparseable text', '{not json'],
    ['an array', '[]'],
    ['a primitive', '42'],
    ['a document from a future build', `{"version":${READING_STATS_DOCUMENT_VERSION + 1}}`]
  ])('falls back to an empty document for %s', (_label, text) => {
    expect(parseReadingStatsDocument(text)).toEqual(createReadingStatsDocument());
  });

  it('drops malformed dates, non-numbers, and non-positive totals', () => {
    const doc = parseReadingStatsDocument(
      JSON.stringify({
        version: READING_STATS_DOCUMENT_VERSION,
        shelves: {
          [KEY]: {
            [TODAY]: 45,
            '2026-8-2': 60,
            '2026-02-31': 60,
            'not-a-date': 60,
            '2026-08-01': '60',
            '2026-07-31': 0,
            '2026-07-30': -5
          }
        }
      })
    );

    expect(doc.shelves[KEY]).toEqual({ [TODAY]: 45 });
  });

  it('drops a shelf whose days are all invalid rather than keeping it empty', () => {
    const doc = parseReadingStatsDocument(
      JSON.stringify({
        version: READING_STATS_DOCUMENT_VERSION,
        shelves: { [KEY]: { 'not-a-date': 60 }, 'shelf-2': { [TODAY]: 10 } }
      })
    );

    expect(Object.keys(doc.shelves)).toEqual(['shelf-2']);
  });

  it('clamps a stored day total above the daily maximum', () => {
    const doc = parseReadingStatsDocument(
      JSON.stringify({
        version: READING_STATS_DOCUMENT_VERSION,
        shelves: { [KEY]: { [TODAY]: MAX_SECONDS_PER_DAY * 10 } }
      })
    );

    expect(doc.shelves[KEY][TODAY]).toBe(MAX_SECONDS_PER_DAY);
  });

  it('round-trips through serialize', () => {
    const doc = withReadingSeconds(createReadingStatsDocument(), KEY, TODAY, 45, NOW);

    expect(parseReadingStatsDocument(serializeReadingStatsDocument(doc))).toEqual(doc);
  });
});

describe('withReadingSeconds', () => {
  it('accumulates across calls on the same day', () => {
    let doc = createReadingStatsDocument();
    doc = withReadingSeconds(doc, KEY, TODAY, 45, NOW);
    doc = withReadingSeconds(doc, KEY, TODAY, 20, NOW);

    expect(doc.shelves[KEY][TODAY]).toBe(65);
  });

  it('clamps a single call to the per-call maximum', () => {
    const doc = withReadingSeconds(createReadingStatsDocument(), KEY, TODAY, 9999, NOW);

    expect(doc.shelves[KEY][TODAY]).toBe(MAX_SECONDS_PER_CALL);
  });

  it('clamps the accumulated day total to the daily maximum', () => {
    let doc = parseReadingStatsDocument(
      JSON.stringify({
        version: READING_STATS_DOCUMENT_VERSION,
        shelves: { [KEY]: { [TODAY]: MAX_SECONDS_PER_DAY - 10 } }
      })
    );
    doc = withReadingSeconds(doc, KEY, TODAY, MAX_SECONDS_PER_CALL, NOW);

    expect(doc.shelves[KEY][TODAY]).toBe(MAX_SECONDS_PER_DAY);
  });

  it.each([
    ['zero seconds', TODAY, 0],
    ['negative seconds', TODAY, -30],
    ['a malformed date', '2026-8-2', 45],
    ['a date that does not exist', '2026-02-31', 45],
    ['a future date', '2026-08-03', 45]
  ])('returns the document unchanged for %s', (_label, date, seconds) => {
    const doc = createReadingStatsDocument();

    expect(withReadingSeconds(doc, KEY, date, seconds, NOW)).toBe(doc);
  });

  it('returns the document unchanged for an empty key', () => {
    const doc = createReadingStatsDocument();

    expect(withReadingSeconds(doc, '', TODAY, 45, NOW)).toBe(doc);
  });

  it('ignores a write older than the retention window', () => {
    const doc = createReadingStatsDocument();
    const tooOld = daysBefore(NOW, READING_STATS_RETENTION_DAYS);

    expect(withReadingSeconds(doc, KEY, tooOld, 45, NOW)).toBe(doc);
  });

  it('trims days that fell out of the retention window on the next write', () => {
    const oldest = daysBefore(NOW, READING_STATS_RETENTION_DAYS - 1);
    const expired = daysBefore(NOW, READING_STATS_RETENTION_DAYS);
    let doc = parseReadingStatsDocument(
      JSON.stringify({
        version: READING_STATS_DOCUMENT_VERSION,
        shelves: { [KEY]: { [expired]: 60, [oldest]: 60 } }
      })
    );

    doc = withReadingSeconds(doc, KEY, TODAY, 45, NOW);

    expect(Object.keys(doc.shelves[KEY]).sort()).toEqual([oldest, TODAY].sort());
  });

  it('keeps other shelves untouched', () => {
    let doc = withReadingSeconds(createReadingStatsDocument(), 'shelf-2', TODAY, 10, NOW);
    doc = withReadingSeconds(doc, KEY, TODAY, 45, NOW);

    expect(doc.shelves['shelf-2']).toEqual({ [TODAY]: 10 });
    expect(doc.shelves[KEY]).toEqual({ [TODAY]: 45 });
  });
});

describe('getReadingStatsRange', () => {
  const doc = parseReadingStatsDocument(
    JSON.stringify({
      version: READING_STATS_DOCUMENT_VERSION,
      shelves: {
        [KEY]: { '2026-07-30': 10, '2026-07-31': 20, '2026-08-01': 30 },
        'shelf-2': { '2026-07-31': 99 }
      }
    })
  );

  it('returns only the requested inclusive range for the requested shelf', () => {
    expect(getReadingStatsRange(doc, KEY, '2026-07-31', '2026-08-01')).toEqual({
      '2026-07-31': 20,
      '2026-08-01': 30
    });
  });

  it('returns an empty map for an unknown shelf', () => {
    expect(getReadingStatsRange(doc, 'shelf-3', '2026-07-01', '2026-08-31')).toEqual({});
  });

  it('returns an empty map for an inverted range', () => {
    expect(getReadingStatsRange(doc, KEY, '2026-08-01', '2026-07-31')).toEqual({});
  });
});
