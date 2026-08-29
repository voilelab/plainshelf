import { describe, expect, it } from 'vitest';
import { MAX_LOG_RETENTION_DAYS, parseLogRetentionDays, parseReadHistoryLimit } from './settingsDraft';

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

describe('parseLogRetentionDays', () => {
  it('accepts a whole number of days the server would take', () => {
    expect(parseLogRetentionDays('30')).toBe(30);
    expect(parseLogRetentionDays(String(MAX_LOG_RETENTION_DAYS))).toBe(MAX_LOG_RETENTION_DAYS);
  });

  // Zero is the one value with a meaning of its own: it turns log deletion off.
  it('accepts zero, which keeps every log file', () => {
    expect(parseLogRetentionDays('0')).toBe(0);
  });

  it('rejects negative, fractional, oversized and non-numeric input', () => {
    expect(parseLogRetentionDays('-1')).toBeNull();
    expect(parseLogRetentionDays('1.5')).toBeNull();
    expect(parseLogRetentionDays(String(MAX_LOG_RETENTION_DAYS + 1))).toBeNull();
    expect(parseLogRetentionDays('abc')).toBeNull();
  });
});
