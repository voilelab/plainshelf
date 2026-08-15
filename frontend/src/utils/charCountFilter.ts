/**
 * A character-count range for the library list. An absent bound means
 * "unlimited on that side", so an empty object is the no-filter state.
 */
export interface CharCountRange {
  min?: number;
  max?: number;
}

export const EMPTY_CHAR_COUNT_RANGE: CharCountRange = {};

/** Step used by the range inputs, matching the old maintenance threshold control. */
export const CHAR_COUNT_STEP = 100;

/**
 * Parses one bound out of a raw query/input value. Anything that is not a
 * non-negative integer - blank, `abc`, `1.5`, `-5` - reads as "no bound"
 * rather than falling back to a number the user never typed.
 */
export function parseCharCountBound(raw: string | undefined): number | undefined {
  if (raw === undefined) {
    return undefined;
  }

  const trimmed = raw.trim();
  if (trimmed.length === 0) {
    return undefined;
  }

  const parsed = Number(trimmed);
  if (!Number.isInteger(parsed) || parsed < 0) {
    return undefined;
  }

  return Math.min(parsed, Number.MAX_SAFE_INTEGER);
}

/**
 * Builds a range from two raw bounds, swapping them when they arrive reversed.
 *
 * Swapping is safe because both inputs commit on `change` (blur, Enter, spinner)
 * rather than on every keystroke, so a half-typed number never triggers it.
 */
export function parseCharCountRange(
  minRaw: string | undefined,
  maxRaw: string | undefined
): CharCountRange {
  const min = parseCharCountBound(minRaw);
  const max = parseCharCountBound(maxRaw);

  if (min !== undefined && max !== undefined && min > max) {
    return { min: max, max: min };
  }

  return { min, max };
}

export function isCharCountRangeActive(range: CharCountRange): boolean {
  return range.min !== undefined || range.max !== undefined;
}

/**
 * The backend omits char_count when the source cannot be read, and `omitempty`
 * also drops a genuine 0, so an unknown count is treated as 0 - the same
 * convention the maintenance "low character count" filter used, which keeps
 * unreadable books visible in the low ranges instead of hiding them.
 */
export function isCharCountInRange(count: number | undefined, range: CharCountRange): boolean {
  const value = count ?? 0;

  if (range.min !== undefined && value < range.min) {
    return false;
  }

  if (range.max !== undefined && value > range.max) {
    return false;
  }

  return true;
}
