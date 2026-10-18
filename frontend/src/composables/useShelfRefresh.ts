import { useShelfCapabilities } from './useShelfCapabilities';
import { useShelvesStore } from './useShelvesStore';
import { computed, ref } from 'vue';
import { ShelfScanInProgressError, ShelfScanRateLimitedError } from '@/api/shelves';
import { getBookshelfProvider } from '@/providers';
import { useBookStore } from './useBookStore';
import { useFolderStore } from './useFolderStore';
import { useToasts } from './useToasts';
import { t } from '@/i18n';

// Module-level singleton, matching useBookStore: the button and any other
// consumer must agree on whether an update is currently running.
const lastSyncedAt = ref<number | null>(null);
const refreshing = ref(false);
const error = ref('');

/** Rescans and dates the listing using the selected shelf's engine metadata. */
export function useShelfRefresh() {
  const provider = getBookshelfProvider();
  const { showToast } = useToasts();
  const capability = useShelfCapabilities();
  const supported = capability.rescan;
  const tracksLastSynced = computed(() => capability.snapshotAt.value !== null);

  async function loadLastSyncedAt(): Promise<void> {
    lastSyncedAt.value = capability.snapshotAt.value;
  }

  async function refresh(): Promise<void> {
    if (!supported.value || refreshing.value) {
      return;
    }
    refreshing.value = true;
    error.value = '';
    try {
      const result = await provider.refreshShelf?.();
      // Only after the shelf itself succeeded: on failure the previous listing
      // is still the current one, and re-fetching would just redraw it.
      const { fetchBooks } = useBookStore();
      const { fetchFolders } = useFolderStore();
      await Promise.all([fetchBooks(), fetchFolders()]);
      await useShelvesStore().fetchShelves();
      await loadLastSyncedAt();
      if (result) {
        showToast(t('library.scanFound', { books: result.bookCount, folders: result.folderCount }));
      }
    } catch (err) {
      // The one refusal that is not a failure: another client is already walking
      // this shelf, so the answer is to wait rather than retry and the previous
      // listing is still correct. A remark about this press, not a state the page
      // must keep showing, so it goes to a toast. Translated here because the api
      // and provider folders hold no strings.
      if (err instanceof ShelfScanInProgressError) {
        showToast(t('library.scanInProgress'));
      } else if (err instanceof ShelfScanRateLimitedError) {
        // A toast for the same reason as the refusal above: nothing failed, and
        // the generic error would park "update failed" on the page.
        showToast(t('library.scanRateLimited', { seconds: err.retryAfterSeconds }));
      } else {
        error.value = err instanceof Error ? err.message : t('library.refreshFailed');
      }
    } finally {
      refreshing.value = false;
    }
  }

  return { supported, tracksLastSynced, lastSyncedAt, refreshing, error, loadLastSyncedAt, refresh };
}
