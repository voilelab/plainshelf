import { ref } from 'vue';
import { ShelfScanInProgressError } from '@/api/shelves';
import { getBookshelfProvider } from '@/providers';
import type { ShelfRefreshResult } from '@/providers/bookshelfProvider';
import { useBookStore } from './useBookStore';
import { useLayerStore } from './useLayerStore';
import { t } from '@/i18n';

// Module-level singleton, matching useBookStore: the button and any other
// consumer must agree on whether an update is currently running.
const lastSyncedAt = ref<number | null>(null);
const lastResult = ref<ShelfRefreshResult | null>(null);
const refreshing = ref(false);
const error = ref('');

/**
 * Drives the manual shelf update for a backend whose listing is not guaranteed
 * to reflect the shelf right now — a pCloud connection, which scans once and
 * then reads a stored copy, and a server, which rescans only every
 * `scan_interval` and is told nothing by an SMB or cloud mount in between.
 *
 * `supported` is asked of the provider rather than inferred from the runtime:
 * the mobile shell can be connected to either a server or pCloud, and the two
 * need the update for different reasons.
 *
 * The two backends report their result differently and neither reports both.
 * pCloud dates its stored listing (`lastSyncedAt`); a server has no stored
 * listing to date and instead says what the walk found (`lastResult`).
 */
export function useShelfRefresh() {
  const provider = getBookshelfProvider();
  const supported = Boolean(provider.supportsShelfRefresh?.());
  const tracksLastSynced = supported && typeof provider.getShelfFetchedAt === 'function';

  async function loadLastSyncedAt(): Promise<void> {
    if (!tracksLastSynced) {
      return;
    }
    lastSyncedAt.value = (await provider.getShelfFetchedAt?.()) ?? null;
  }

  async function refresh(): Promise<void> {
    if (!supported || refreshing.value) {
      return;
    }
    refreshing.value = true;
    error.value = '';
    try {
      const result = await provider.refreshShelf?.();
      // Only after the shelf itself succeeded: on failure the previous listing
      // is still the current one, and re-fetching would just redraw it.
      const { fetchBooks } = useBookStore();
      const { fetchLayers } = useLayerStore();
      await Promise.all([fetchBooks(), fetchLayers()]);
      await loadLastSyncedAt();
      lastResult.value = result ?? null;
    } catch (err) {
      // The one refusal that is not a failure: another client is already
      // walking this shelf, so the answer is to wait rather than to retry.
      // Translated here because the api and provider layers hold no strings.
      if (err instanceof ShelfScanInProgressError) {
        error.value = t('library.scanInProgress');
      } else {
        error.value = err instanceof Error ? err.message : t('library.refreshFailed');
      }
    } finally {
      refreshing.value = false;
    }
  }

  return { supported, tracksLastSynced, lastSyncedAt, lastResult, refreshing, error, loadLastSyncedAt, refresh };
}
