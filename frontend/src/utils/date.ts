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
