export interface CharCountRange {
  min?: number;
  max?: number;
}

export const CHAR_COUNT_STEP = 100;

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
