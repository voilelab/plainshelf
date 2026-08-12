import { describe, expect, it } from 'vitest';
import { getReadingAction, normalizeReadingPercent } from './bookDetail';

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
});
