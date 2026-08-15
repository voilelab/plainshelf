import { describe, expect, it } from 'vitest';
import {
  isCharCountInRange,
  isCharCountRangeActive,
  parseCharCountBound,
  parseCharCountRange
} from './charCountFilter';

describe('parseCharCountBound', () => {
  it('reads a missing or blank bound as unlimited', () => {
    expect(parseCharCountBound(undefined)).toBeUndefined();
    expect(parseCharCountBound('')).toBeUndefined();
    expect(parseCharCountBound('   ')).toBeUndefined();
  });

  it('rejects values that are not non-negative integers', () => {
    expect(parseCharCountBound('abc')).toBeUndefined();
    expect(parseCharCountBound('1.5')).toBeUndefined();
    expect(parseCharCountBound('-5')).toBeUndefined();
    expect(parseCharCountBound('1e400')).toBeUndefined();
  });

  it('accepts integers, including zero and surrounding whitespace', () => {
    expect(parseCharCountBound('0')).toBe(0);
    expect(parseCharCountBound('2500')).toBe(2500);
    expect(parseCharCountBound('  2500  ')).toBe(2500);
  });

  it('clamps absurdly large values to a safe integer', () => {
    expect(parseCharCountBound('999999999999999999999')).toBe(Number.MAX_SAFE_INTEGER);
  });
});

describe('parseCharCountRange', () => {
  it('keeps each bound independent', () => {
    expect(parseCharCountRange('100', undefined)).toEqual({ min: 100, max: undefined });
    expect(parseCharCountRange(undefined, '900')).toEqual({ min: undefined, max: 900 });
    expect(parseCharCountRange('100', '900')).toEqual({ min: 100, max: 900 });
  });

  it('swaps reversed bounds', () => {
    expect(parseCharCountRange('900', '100')).toEqual({ min: 100, max: 900 });
  });

  it('leaves equal bounds alone', () => {
    expect(parseCharCountRange('500', '500')).toEqual({ min: 500, max: 500 });
  });

  it('drops an unusable bound instead of the whole range', () => {
    expect(parseCharCountRange('abc', '900')).toEqual({ min: undefined, max: 900 });
  });
});

describe('isCharCountRangeActive', () => {
  it('is inactive only when both bounds are absent', () => {
    expect(isCharCountRangeActive({})).toBe(false);
    expect(isCharCountRangeActive({ min: 0 })).toBe(true);
    expect(isCharCountRangeActive({ max: 0 })).toBe(true);
    expect(isCharCountRangeActive({ min: 1, max: 2 })).toBe(true);
  });
});

describe('isCharCountInRange', () => {
  it('accepts everything when no bound is set', () => {
    expect(isCharCountInRange(0, {})).toBe(true);
    expect(isCharCountInRange(12345, {})).toBe(true);
    expect(isCharCountInRange(undefined, {})).toBe(true);
  });

  it('treats both bounds as inclusive', () => {
    expect(isCharCountInRange(100, { min: 100, max: 900 })).toBe(true);
    expect(isCharCountInRange(900, { min: 100, max: 900 })).toBe(true);
    expect(isCharCountInRange(99, { min: 100, max: 900 })).toBe(false);
    expect(isCharCountInRange(901, { min: 100, max: 900 })).toBe(false);
  });

  it('applies a single bound on its own side only', () => {
    expect(isCharCountInRange(1_000_000, { min: 100 })).toBe(true);
    expect(isCharCountInRange(0, { max: 900 })).toBe(true);
  });

  it('treats an unknown count as zero', () => {
    expect(isCharCountInRange(undefined, { max: 1000 })).toBe(true);
    expect(isCharCountInRange(undefined, { min: 1 })).toBe(false);
    expect(isCharCountInRange(undefined, { min: 0, max: 0 })).toBe(true);
  });
});
