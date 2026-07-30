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
