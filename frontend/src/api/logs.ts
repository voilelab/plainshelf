import { fetchJson, fetchText } from './client';

export interface LogFileEntry {
  id: string;
  source?: string;
  filename: string;
  date: string;
}

export function listLogs(): Promise<LogFileEntry[]> {
  return fetchJson<LogFileEntry[]>('/api/logs');
}

export function getLogContent(logId: string): Promise<string> {
  return fetchText(`/api/logs/${encodeURIComponent(logId)}/content`);
}
