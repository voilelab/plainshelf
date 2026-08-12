export type ReadingAction = 'start' | 'continue' | 'reread';

export function normalizeReadingPercent(percent?: number): number {
  if (typeof percent !== 'number' || !Number.isFinite(percent)) {
    return 0;
  }

  return Math.min(100, Math.max(0, Math.round(percent)));
}

export function getReadingAction(percent?: number): ReadingAction {
  const normalized = normalizeReadingPercent(percent);
  if (normalized >= 100) {
    return 'reread';
  }
  if (normalized > 0) {
    return 'continue';
  }
  return 'start';
}
