import { describe, expect, it } from 'vitest';
import { getReadingAction, normalizeReadingPercent, resolveReadingPercent } from './bookDetail';

describe('book detail reading state', () => {
  it('normalizes missing and out-of-range progress', () => {
    expect(normalizeReadingPercent()).toBe(0);
    expect(normalizeReadingPercent(Number.NaN)).toBe(0);
    expect(normalizeReadingPercent(-5)).toBe(0);
    expect(normalizeReadingPercent(42.4)).toBe(42);
    expect(normalizeReadingPercent(160)).toBe(100);
  });

  it('selects the correct reading action', () => {
    expect(getReadingAction()).toBe('start');
    expect(getReadingAction(0)).toBe('start');
    expect(getReadingAction(42)).toBe('continue');
    expect(getReadingAction(100)).toBe('reread');
  });

  it('derives server progress from the bookmark offset and source character count', () => {
    expect(resolveReadingPercent({ char_offset: 37 }, 74)).toBe(50);
    expect(resolveReadingPercent({ char_offset: 74 }, 74)).toBe(100);
    expect(resolveReadingPercent({ char_offset: 80 }, 74)).toBe(100);
  });

  it('prefers an explicit percentage and handles an unavailable total', () => {
    expect(resolveReadingPercent({ char_offset: 37, percent: 25 }, 74)).toBe(25);
    expect(resolveReadingPercent({ char_offset: 37 })).toBe(0);
    expect(resolveReadingPercent({ char_offset: 37 }, 0)).toBe(0);
  });
});
