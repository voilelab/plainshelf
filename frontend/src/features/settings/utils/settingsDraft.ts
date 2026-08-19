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
