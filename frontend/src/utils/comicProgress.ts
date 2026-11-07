export function comicReadingPercent(pageIndex: number, pageCount: number): number {
  if (pageCount <= 0) return 0;
  return Math.round((Math.max(0, Math.min(pageCount - 1, pageIndex)) + 1) / pageCount * 100);
}
