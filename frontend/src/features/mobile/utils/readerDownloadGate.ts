import type { DownloadState } from '@/types/book';


export const DOWNLOAD_REQUIRED_QUERY_KEY = 'downloadRequired';

const READABLE_STATES: ReadonlySet<DownloadState> = new Set<DownloadState>([
  'downloaded',
  'update_available'
]);

export function isReadableWhenDownloadRequired(state: DownloadState): boolean {
  return READABLE_STATES.has(state);
}
