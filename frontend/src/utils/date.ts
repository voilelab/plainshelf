const DATE_ONLY_RE = /^\d{4}-\d{2}-\d{2}$/;

/**
 * Formats a book timestamp for display. Date-only strings ("YYYY-MM-DD") are
 * parsed as local calendar dates rather than through `new Date("YYYY-MM-DD")`,
 * which the spec parses as UTC midnight and would shift to the previous day in
 * negative-offset timezones. Full timestamps fall back to normal Date parsing.
 */
export function formatDateLabel(value?: string): string {
  if (!value) {
    return '';
  }

  if (DATE_ONLY_RE.test(value)) {
    const [year, month, day] = value.split('-').map(Number);
    return new Date(year, month - 1, day).toLocaleDateString();
  }

  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString();
}

const RELATIVE_UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 60 * 60 * 1000],
  ['month', 30 * 24 * 60 * 60 * 1000],
  ['day', 24 * 60 * 60 * 1000],
  ['hour', 60 * 60 * 1000],
  ['minute', 60 * 1000]
];

/**
 * A localized "3 days ago" / "3 天前" for a past epoch-ms timestamp, using the
 * largest unit that fits. Anything under a minute — including a slightly future
 * timestamp from clock skew — reads as "now". `now` is injectable so callers and
 * tests control the reference clock.
 */
export function formatRelativeTime(at: number, locale: string, now: number = Date.now()): string {
  const diffMs = at - now; // negative in the past
  const abs = Math.abs(diffMs);
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });
  for (const [unit, ms] of RELATIVE_UNITS) {
    if (abs >= ms) {
      return rtf.format(Math.round(diffMs / ms), unit);
    }
  }
  return rtf.format(0, 'second');
}
