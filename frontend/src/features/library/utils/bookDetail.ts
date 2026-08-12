import type { ReadingProgress } from '@/types/book';

export type ReadingAction = 'start' | 'continue' | 'reread';

export function normalizeReadingPercent(percent?: number): number {
  if (typeof percent !== 'number' || !Number.isFinite(percent)) {
    return 0;
  }

  return Math.min(100, Math.max(0, Math.round(percent)));
}

export function resolveReadingPercent(
  progress?: ReadingProgress | null,
  totalCharacters?: number
): number {
  if (typeof progress?.percent === 'number' && Number.isFinite(progress.percent)) {
    return normalizeReadingPercent(progress.percent);
  }

  if (
    typeof progress?.char_offset !== 'number' ||
    !Number.isFinite(progress.char_offset) ||
    typeof totalCharacters !== 'number' ||
    !Number.isFinite(totalCharacters) ||
    totalCharacters <= 0
  ) {
    return 0;
  }

  return normalizeReadingPercent((progress.char_offset / totalCharacters) * 100);
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
