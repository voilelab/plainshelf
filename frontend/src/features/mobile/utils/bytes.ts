const UNITS = ['B', 'KB', 'MB', 'GB'] as const;
const STEP = 1024;

/**
 * Formats a byte count for display (1024-based units). Values under 1KB are
 * shown as whole bytes; larger values get one decimal place.
 */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0 B';
  }

  let value = bytes;
  let unitIndex = 0;

  while (value >= STEP && unitIndex < UNITS.length - 1) {
    value /= STEP;
    unitIndex += 1;
  }

  const formatted = unitIndex === 0 ? String(Math.round(value)) : value.toFixed(1);
  return `${formatted} ${UNITS[unitIndex]}`;
}
