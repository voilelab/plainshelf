import { describe, expect, it } from 'vitest';
import { parseReadHistoryLimit } from './settingsDraft';

describe('parseReadHistoryLimit', () => {
  it('accepts a whole, non-negative count', () => {
    expect(parseReadHistoryLimit('50')).toBe(50);
    expect(parseReadHistoryLimit('0')).toBe(0);
  });

  it('rejects negative, fractional and non-numeric input', () => {
    expect(parseReadHistoryLimit('-1')).toBeNull();
    expect(parseReadHistoryLimit('1.5')).toBeNull();
    expect(parseReadHistoryLimit('abc')).toBeNull();
  });

  // Number('') is 0, so clearing the field saves "no limit" rather than
  // reporting an invalid value. Documented here because the number input makes
  // an empty string easy to produce.
  it('treats an empty field as zero', () => {
    expect(parseReadHistoryLimit('')).toBe(0);
  });
});
