import { computed, type Ref } from 'vue';
import { bookshelfWriter } from '@/providers';
import { useTaskChainProgress } from '@/composables/useTaskChainProgress';
import { isRefreshContentStatsResult } from '@/types/task';
import { useI18n } from '@/i18n';

export function useContentStatsRefresh(unknownCount: Ref<number>, onRefreshed: () => void) {
  const { t } = useI18n();

  const {
    percentage,
    error,
    running,
    finished,
    chain,
    status,
    start: startChain
  } = useTaskChainProgress({
    onSettled: () => onRefreshed(),
    startFailedMessage: () => t('library.charCount.refreshStats.failed'),
    pollFailedMessage: () => t('library.charCount.refreshStats.failed')
  });

  const label = computed(() =>
    running.value
      ? t('library.charCount.refreshStats.busy', { percent: Math.round(percentage.value) })
      : t('library.charCount.refreshStats.action', { count: unknownCount.value })
  );

  const result = computed(() => {
    for (const task of chain.value?.tasks ?? []) {
      if (isRefreshContentStatsResult(task.result)) {
        return task.result;
      }
    }
    return null;
  });

  const outcome = computed(() => {
    if (!finished.value || error.value) {
      return '';
    }
    const settled = result.value;
    if (!settled || status.value === 'failed') {
      return t('library.charCount.refreshStats.failed');
    }
    if (settled.failures.length === 0) {
      return t('library.charCount.refreshStats.done', { count: settled.refreshed });
    }
    return t('library.charCount.refreshStats.partial', {
      succeeded: settled.refreshed,
      failed: settled.failures.length
    });
  });

  async function start(): Promise<void> {
    await startChain(() => bookshelfWriter().refreshContentStats());
  }

  return { running, label, outcome, error, start };
}
