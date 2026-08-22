import { describe, expect, it } from 'vitest';
import {
  filterFieldOpKey,
  parseFilterField,
  parseFilterFieldOp,
  serializeFilterField,
  serializeFilterFieldOp,
  type FilterFieldValue
} from './codec';

// A small deterministic string generator, so the property test explores a wide
// range of eq values (colons, commas, the literal sentinels, unicode, empties)
// without a random seed that could pass one run and fail the next.
function pseudoRandom(seed: number): () => number {
  let state = seed >>> 0;
  return () => {
    // Numerical Recipes LCG; only its spread matters here, not its quality.
    state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
    return state / 0x100000000;
  };
}

const ALPHABET = ['a', 'Z', '0', ':', ',', ' ', 'none', 'has', 'eq', 'eq:', '書', '/', '='];

function randomValueString(next: () => number): string {
  const length = Math.floor(next() * 6);
  let out = '';
  for (let i = 0; i < length; i += 1) {
    out += ALPHABET[Math.floor(next() * ALPHABET.length)];
  }
  return out;
}

describe('serializeFilterField / parseFilterField', () => {
  it('round-trips the three understood forms', () => {
    const cases: FilterFieldValue[] = [
      { kind: 'none' },
      { kind: 'has' },
      { kind: 'eq', value: 'Tolkien' }
    ];
    for (const value of cases) {
      expect(parseFilterField(serializeFilterField(value))).toEqual(value);
    }
  });

  it('keeps eq values distinct from the bare sentinels', () => {
    // The literal strings "none" and "has" survive as eq values rather than
    // collapsing into the "absent"/"present" sentinels.
    expect(serializeFilterField({ kind: 'eq', value: 'none' })).toBe('eq:none');
    expect(parseFilterField('eq:none')).toEqual({ kind: 'eq', value: 'none' });
    expect(parseFilterField('eq:has')).toEqual({ kind: 'eq', value: 'has' });
  });

  it('preserves colons, commas, and an empty value in eq round-trips', () => {
    const tricky = ['a:b', 'a:b:c', 'a,b', 'a,b:c,d', '', 'eq:nested', ':leading', 'trailing:'];
    for (const raw of tricky) {
      const value: FilterFieldValue = { kind: 'eq', value: raw };
      expect(parseFilterField(serializeFilterField(value))).toEqual(value);
    }
  });

  it('round-trips any generated value string (property test)', () => {
    const next = pseudoRandom(0x5eed);
    for (let i = 0; i < 2000; i += 1) {
      const value: FilterFieldValue = { kind: 'eq', value: randomValueString(next) };
      expect(parseFilterField(serializeFilterField(value))).toEqual(value);
    }
  });

  it('round-trips a generated value of any kind (property test)', () => {
    const next = pseudoRandom(0x1234);
    for (let i = 0; i < 2000; i += 1) {
      const roll = next();
      const value: FilterFieldValue =
        roll < 0.2
          ? { kind: 'none' }
          : roll < 0.4
            ? { kind: 'has' }
            : { kind: 'eq', value: randomValueString(next) };
      expect(parseFilterField(serializeFilterField(value))).toEqual(value);
    }
  });

  it('ignores an invalid token rather than throwing', () => {
    // `?author=garbage`: an unknown value applies no condition and raises nothing,
    // the same way toBookSort falls back on an unknown sort key.
    for (const raw of ['garbage', 'eq', 'EQ:x', 'None', 'has ', '', ' ', 'author=garbage']) {
      expect(parseFilterField(raw)).toBeUndefined();
    }
    expect(parseFilterField(undefined)).toBeUndefined();
  });
});

describe('serializeFilterFieldOp / parseFilterFieldOp', () => {
  it('round-trips the only supported operator', () => {
    expect(parseFilterFieldOp(serializeFilterFieldOp('all'))).toBe('all');
  });

  it('treats an absent or not-yet-supported operator as undefined', () => {
    for (const raw of [undefined, '', 'any', 'or', 'ALL', 'and']) {
      expect(parseFilterFieldOp(raw)).toBeUndefined();
    }
  });

  it('derives the sibling op key from the field name', () => {
    expect(filterFieldOpKey('tags')).toBe('tagsOp');
    expect(filterFieldOpKey('author')).toBe('authorOp');
  });
});
