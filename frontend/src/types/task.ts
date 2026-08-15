export type TaskStatus = 'pending' | 'running' | 'partially_completed' | 'completed' | 'failed';

export const TERMINAL_TASK_STATUSES: readonly TaskStatus[] = [
  'partially_completed',
  'completed',
  'failed'
];

export function isTerminalTaskStatus(status: TaskStatus): boolean {
  return TERMINAL_TASK_STATUSES.includes(status);
}

export interface Task {
  name: string;
  title: string;
  description?: string;
  status: TaskStatus;
  percentage: number;
  result?: unknown;
}

export type BookBatchOperation = 'move' | 'trash';

export type BookBatchFailureCode =
  | 'not_found'
  | 'unsupported_schema'
  | 'move_failed'
  | 'trash_failed';

export interface BookBatchFailure {
  book_id: string;
  code: BookBatchFailureCode;
}

export interface BookBatchResult {
  operation: BookBatchOperation;
  total: number;
  succeeded_ids: string[];
  failures: BookBatchFailure[];
}

export interface BookBatchRequest {
  operation: BookBatchOperation;
  book_ids: string[];
  target_layer?: string[];
}

export function isBookBatchResult(value: unknown): value is BookBatchResult {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Partial<BookBatchResult>;
  return (
    (candidate.operation === 'move' || candidate.operation === 'trash') &&
    typeof candidate.total === 'number' &&
    Array.isArray(candidate.succeeded_ids) &&
    Array.isArray(candidate.failures)
  );
}

export type RefreshContentStatsFailureCode = 'not_found' | 'unsupported_schema' | 'refresh_failed';

export interface RefreshContentStatsFailure {
  book_id: string;
  code: RefreshContentStatsFailureCode;
}

/** Result of the shelf-wide sweep that recomputes unknown character counts. */
export interface RefreshContentStatsResult {
  total: number;
  refreshed: number;
  failures: RefreshContentStatsFailure[];
}

export function isRefreshContentStatsResult(value: unknown): value is RefreshContentStatsResult {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Partial<RefreshContentStatsResult>;
  return (
    typeof candidate.total === 'number' &&
    typeof candidate.refreshed === 'number' &&
    Array.isArray(candidate.failures)
  );
}

export interface TaskChain {
  id: string;
  name: string;
  title: string;
  description?: string;
  status: TaskStatus;
  percentage: number;
  created_at?: string;
  tasks: Task[];
}
