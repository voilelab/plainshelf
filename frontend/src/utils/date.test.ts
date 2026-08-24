import { describe, expect, it } from 'vitest';
import { formatRelativeTime } from './date';

const NOW = Date.UTC(2026, 0, 1, 12, 0, 0);
const MINUTE = 60 * 1000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

describe('formatRelativeTime', () => {
  it('picks the largest fitting unit for English', () => {
    expect(formatRelativeTime(NOW - 3 * DAY, 'en', NOW)).toBe('3 days ago');
    expect(formatRelativeTime(NOW - 5 * HOUR, 'en', NOW)).toBe('5 hours ago');
    expect(formatRelativeTime(NOW - 2 * MINUTE, 'en', NOW)).toBe('2 minutes ago');
  });

  it('localizes to Traditional Chinese', () => {
    expect(formatRelativeTime(NOW - 3 * DAY, 'zh-Hant', NOW)).toBe('3 天前');
  });

  it('reads anything under a minute — including future skew — as "now"', () => {
    expect(formatRelativeTime(NOW - 30 * 1000, 'en', NOW)).toBe('now');
    expect(formatRelativeTime(NOW + 30 * 1000, 'en', NOW)).toBe('now');
  });
});
