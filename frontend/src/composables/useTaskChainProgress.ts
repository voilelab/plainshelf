import { computed, onScopeDispose, ref } from 'vue';

import { bookshelfWriter } from '@/providers';
import { isTerminalTaskStatus, type TaskChain, type TaskStatus } from '@/types/task';
import { t } from '@/i18n';

const DEFAULT_TASK_CHAIN_POLL_INTERVAL_MS = 500;

interface UseTaskChainProgressOptions {
  intervalMs?: number;
  onSettled?: (status: TaskStatus) => void | Promise<void>;
  startFailedMessage?: () => string;
  pollFailedMessage?: () => string;
}

export function useTaskChainProgress(options: UseTaskChainProgressOptions = {}) {
  const intervalMs = options.intervalMs ?? DEFAULT_TASK_CHAIN_POLL_INTERVAL_MS;

  const taskChainId = ref<string | null>(null);
  const submitting = ref(false);
  const status = ref<TaskStatus>('pending');
  const percentage = ref(0);
  const error = ref('');
  const chain = ref<TaskChain | null>(null);

  let timer: ReturnType<typeof setTimeout> | null = null;
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
      const nextChain = await bookshelfWriter().getTaskChain(polling);
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
        err instanceof Error ? err.message : (options.pollFailedMessage?.() ?? t('common.taskPollFailed'));
      status.value = 'failed';
    }
  }

  async function start(begin: () => Promise<string>): Promise<void> {
    if (running.value) {
      return;
    }

    reset();
    const submission = generation;
    submitting.value = true;

    try {
      const id = await begin();
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
        err instanceof Error ? err.message : (options.startFailedMessage?.() ?? t('common.taskStartFailed'));
    } finally {
      if (generation === submission) {
        submitting.value = false;
      }
    }
  }

  onScopeDispose(stop);

  return { taskChainId, chain, status, percentage, error, started, running, finished, start, reset, stop };
}
