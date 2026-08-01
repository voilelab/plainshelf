import { computed, onScopeDispose, ref } from 'vue';

import { getBookshelfProvider } from '@/providers';
import { isTerminalTaskStatus, type TaskChain, type TaskStatus } from '@/types/task';

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
  // submitting covers the window between asking for the work and learning its
  // chain ID. The task is already under way then, so the caller must be able to
  // treat it as running before an ID exists.
  const submitting = ref(false);
  const status = ref<TaskStatus>('pending');
  const percentage = ref(0);
  const error = ref('');
  const chain = ref<TaskChain | null>(null);

  let timer: ReturnType<typeof setTimeout> | null = null;
  // generation invalidates an in-flight submission when reset lands first.
  let generation = 0;

  const started = computed(() => submitting.value || taskChainId.value !== null);
  const finished = computed(() => started.value && !submitting.value && isTerminalTaskStatus(status.value));
  const running = computed(() => started.value && !finished.value);

  function stop(): void {
    if (timer !== null) {
      clearTimeout(timer);
      timer = null;
    }
  }

  function reset(): void {
    generation += 1;
    stop();
    taskChainId.value = null;
    submitting.value = false;
    status.value = 'pending';
    percentage.value = 0;
    error.value = '';
    chain.value = null;
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
      const nextChain = await getBookshelfProvider().getTaskChain(polling);
      // A reset or a new chain during the request makes this response stale.
      if (taskChainId.value !== polling) {
        return;
      }

      chain.value = nextChain;
      status.value = nextChain.status;
      percentage.value = nextChain.percentage;

      if (!isTerminalTaskStatus(nextChain.status)) {
        schedule();
        return;
      }

      await options.onSettled?.(nextChain.status);
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
   *
   * It reports itself as running before awaiting `begin`, so a caller that
   * gates its UI on `running` cannot be dismissed — or fire a second
   * submission — during a slow request for work that is already happening.
   */
  async function start(begin: () => Promise<string>): Promise<void> {
    if (running.value) {
      return;
    }

    reset();
    const submission = generation;
    submitting.value = true;

    try {
      const id = await begin();
      // A reset during the request abandoned this submission; adopting its ID
      // now would poll a chain nothing is displaying.
      if (generation !== submission) {
        return;
      }

      taskChainId.value = id;
      schedule();
    } catch (err) {
      if (generation !== submission) {
        return;
      }
      error.value =
        err instanceof Error ? err.message : (options.startFailedMessage?.() ?? 'Failed to start the task');
    } finally {
      if (generation === submission) {
        submitting.value = false;
      }
    }
  }

  onScopeDispose(stop);

  return { taskChainId, chain, status, percentage, error, started, running, finished, start, reset, stop };
}
