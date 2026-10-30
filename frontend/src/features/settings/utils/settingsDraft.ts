export function parseReadHistoryLimit(value: string): number | null {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0) {
    return null;
  }
  return parsed;
}

export const MAX_LOG_RETENTION_DAYS = 3650;

export function parseLogRetentionDays(value: string): number | null {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > MAX_LOG_RETENTION_DAYS) {
    return null;
  }
  return parsed;
}
