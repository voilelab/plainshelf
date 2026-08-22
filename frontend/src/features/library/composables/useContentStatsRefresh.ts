/**
 * Owns the shelf-wide "recompute character counts" sweep for the library page.
 *
 * This deliberately lives on the always-mounted library page rather than inside
 * the filter panel: the panel (a popover/bottom sheet) unmounts when closed, and
 * `useTaskChainProgress` stops its poll on scope disposal. If the tracker lived
 * in the panel, closing it mid-sweep would abandon the poll — the server would
 * finish the work but `onSettled` would never fire, so the character-count index
 * would stay stale until the next manual refresh. Keeping it here means the poll
 * (and the index refresh it triggers on completion) survives the panel closing.
 */
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
    // The sweep rewrites char_count in each source's meta.json, so the cached
    // counts — not the book listing — are what went stale.
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
    // A sweep already in flight returns its own ID, so this attaches to the
    // existing progress instead of scheduling a second one.
    await startChain(() => bookshelfWriter().refreshContentStats());
  }

  return { running, label, outcome, error, start };
}
