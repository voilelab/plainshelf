/**
 * The read-history limit the user typed, or null when it is not a whole,
 * non-negative count. Zero is valid and means "keep no limit".
 */
export function parseReadHistoryLimit(value: string): number | null {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0) {
    return null;
  }
  return parsed;
}

/**
 * The largest retention window the server accepts, restated here so the field
 * rejects an out-of-range value before the request rather than after it.
 */
export const MAX_LOG_RETENTION_DAYS = 3650;

/**
 * The log retention window the user typed, or null when it is not a whole
 * number of days the server would accept. Zero is valid and is how log
 * deletion is turned off.
 */
export function parseLogRetentionDays(value: string): number | null {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > MAX_LOG_RETENTION_DAYS) {
    return null;
  }
  return parsed;
}
