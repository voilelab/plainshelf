import { ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { useBookStore } from './useBookStore';
import { useLayerStore } from './useLayerStore';
import { t } from '@/i18n';

// Module-level singleton, matching useBookStore: the button and any other
// consumer must agree on whether an update is currently running.
const lastSyncedAt = ref<number | null>(null);
const refreshing = ref(false);
const error = ref('');

/**
 * Drives the manual shelf update for backends whose listing is not refreshed on
 * its own — today only a pCloud connection (see PCloudBookshelfProvider).
 *
 * `supported` is asked of the provider rather than inferred from the runtime:
 * the mobile shell can be connected to either a server or pCloud, and only the
 * latter has a listing the user is responsible for updating.
 */
export function useShelfRefresh() {
  const provider = getBookshelfProvider();
  const supported = Boolean(provider.supportsShelfRefresh?.());

  async function loadLastSyncedAt(): Promise<void> {
    if (!supported) {
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
      await provider.refreshShelf?.();
      // Only after the shelf itself succeeded: on failure the previous listing
      // is still the current one, and re-fetching would just redraw it.
      const { fetchBooks } = useBookStore();
      const { fetchLayers } = useLayerStore();
      await Promise.all([fetchBooks(), fetchLayers()]);
      await loadLastSyncedAt();
    } catch (err) {
      error.value = err instanceof Error ? err.message : t('library.refreshFailed');
    } finally {
      refreshing.value = false;
    }
  }

  return { supported, lastSyncedAt, refreshing, error, loadLastSyncedAt, refresh };
}
