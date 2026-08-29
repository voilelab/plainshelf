import { fetchJson, fetchText } from './client';

export interface LogFileEntry {
  id: string;
  source?: string;
  filename: string;
  date: string;
  size: number;
}

/**
 * How much of a log file the viewer asks for on first load.
 *
 * A log file is appended to on every request and is only rotated daily, so it
 * has no useful upper size; reading all of it into one `<pre>` is what makes
 * the page unusable exactly when the log is worth reading. The server applies
 * the same bound by default.
 */
export const DEFAULT_LOG_TAIL_BYTES = 256 * 1024;

/** How much further each "load more" step reaches back. */
export const LOG_TAIL_STEP = 4;

export function listLogs(): Promise<LogFileEntry[]> {
  return fetchJson<LogFileEntry[]>('/api/logs');
}

/**
 * Reads the last `tailBytes` bytes of a log file, cut at a line boundary.
 *
 * Pass 0 to read the whole file.
 */
export function getLogContent(logId: string, tailBytes = DEFAULT_LOG_TAIL_BYTES): Promise<string> {
  const query = new URLSearchParams({ tail_bytes: String(tailBytes) });
  return fetchText(`/api/logs/${encodeURIComponent(logId)}/content?${query.toString()}`);
}
