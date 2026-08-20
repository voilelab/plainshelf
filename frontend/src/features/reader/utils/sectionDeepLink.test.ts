import { describe, expect, it } from 'vitest';
import { parseSectionQuery } from './sectionDeepLink';

describe('parseSectionQuery', () => {
  it('reads a whole section index', () => {
    expect(parseSectionQuery('0')).toBe(0);
    expect(parseSectionQuery('3')).toBe(3);
    expect(parseSectionQuery(' 3 ')).toBe(3);
  });

  // Out of range is the reader's business: it clamps to the nearest chapter,
  // so parsing must hand the number through rather than reject it here.
  it('passes an out-of-range index through for the reader to clamp', () => {
    expect(parseSectionQuery('999')).toBe(999);
    expect(parseSectionQuery('-2')).toBe(-2);
  });

  it('treats anything that is not a whole number as no deep link', () => {
    expect(parseSectionQuery(undefined)).toBeNull();
    expect(parseSectionQuery(null)).toBeNull();
    expect(parseSectionQuery('')).toBeNull();
    expect(parseSectionQuery('   ')).toBeNull();
    expect(parseSectionQuery('two')).toBeNull();
    expect(parseSectionQuery('1.5')).toBeNull();
    expect(parseSectionQuery('Infinity')).toBeNull();
  });

  // A repeated query key arrives as an array from vue-router.
  it('uses the first value of a repeated query key', () => {
    expect(parseSectionQuery(['2', '5'])).toBe(2);
    expect(parseSectionQuery([])).toBeNull();
  });
});
