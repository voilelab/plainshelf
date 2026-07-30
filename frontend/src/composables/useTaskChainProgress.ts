import { computed, onScopeDispose, ref } from 'vue';

import { getBookshelfProvider } from '../providers';
import { isTerminalTaskStatus, type TaskStatus } from '../types/task';

export const DEFAULT_TASK_CHAIN_POLL_INTERVAL_MS = 500;

export interface UseTaskChainProgressOptions {
  intervalMs?: number;
  // onSettled runs once the chain reaches a terminal status, for callers that
  // need to refresh whatever the task changed.
  onSettled?: (status: TaskStatus) => void | Promise<void>;
  startFailedMessage?: () => string;
  pollFailedMessage?: () => string;
}

/**
 * useTaskChainProgress tracks a background task chain by polling the read-only
 * task chain API until it settles.
 *
 * The server has no streaming channel, so polling is the only way to observe a
 * chain's progress.
 */
export function useTaskChainProgress(options: UseTaskChainProgressOptions = {}) {
  const intervalMs = options.intervalMs ?? DEFAULT_TASK_CHAIN_POLL_INTERVAL_MS;

  const taskChainId = ref<string | null>(null);
  const status = ref<TaskStatus>('pending');
  const percentage = ref(0);
  const error = ref('');

  let timer: ReturnType<typeof setTimeout> | null = null;

  const started = computed(() => taskChainId.value !== null);
  const finished = computed(() => started.value && isTerminalTaskStatus(status.value));
  const running = computed(() => started.value && !finished.value);

  function stop(): void {
    if (timer !== null) {
      clearTimeout(timer);
      timer = null;
    }
  }

  function reset(): void {
    stop();
    taskChainId.value = null;
    status.value = 'pending';
    percentage.value = 0;
    error.value = '';
  }

  function schedule(): void {
    stop();
    timer = setTimeout(() => {
      void poll();
    }, intervalMs);
  }

  async function poll(): Promise<void> {
    const polling = taskChainId.value;
    if (polling === null) {
      return;
    }

    try {
      const chain = await getBookshelfProvider().getTaskChain(polling);
      // A reset or a new chain during the request makes this response stale.
      if (taskChainId.value !== polling) {
        return;
      }

      status.value = chain.status;
      percentage.value = chain.percentage;

      if (!isTerminalTaskStatus(chain.status)) {
        schedule();
        return;
      }

      await options.onSettled?.(chain.status);
    } catch (err) {
      if (taskChainId.value !== polling) {
        return;
      }
      error.value =
        err instanceof Error ? err.message : (options.pollFailedMessage?.() ?? 'Failed to read task progress');
      // Settle so the caller stops reporting itself as busy even though the
      // progress is no longer observable.
      status.value = 'failed';
    }
  }

  /**
   * start runs `begin`, which schedules the work and resolves to the task chain
   * ID, then polls that chain until it settles.
   */
  async function start(begin: () => Promise<string>): Promise<void> {
    if (running.value) {
      return;
    }

    reset();
    try {
      taskChainId.value = await begin();
      schedule();
    } catch (err) {
      taskChainId.value = null;
      error.value =
        err instanceof Error ? err.message : (options.startFailedMessage?.() ?? 'Failed to start the task');
    }
  }

  onScopeDispose(stop);

  return { taskChainId, status, percentage, error, started, running, finished, start, reset, stop };
}
