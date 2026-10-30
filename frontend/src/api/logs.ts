import { fetchJson, fetchText } from './client';

export interface LogFileEntry {
  id: string;
  source?: string;
  filename: string;
  date: string;
  size: number;
}

export const DEFAULT_LOG_TAIL_BYTES = 256 * 1024;

export const LOG_TAIL_STEP = 4;

export function listLogs(): Promise<LogFileEntry[]> {
  return fetchJson<LogFileEntry[]>('/api/logs');
}

export function getLogContent(logId: string, tailBytes = DEFAULT_LOG_TAIL_BYTES): Promise<string> {
  const query = new URLSearchParams({ tail_bytes: String(tailBytes) });
  return fetchText(`/api/logs/${encodeURIComponent(logId)}/content?${query.toString()}`);
}
