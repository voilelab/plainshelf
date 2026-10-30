import { useShelfCapabilities } from './useShelfCapabilities';
import { useShelvesStore } from './useShelvesStore';
import { computed, ref } from 'vue';
import { ShelfScanInProgressError, ShelfScanRateLimitedError } from '@/api/shelves';
import { getBookshelfProvider } from '@/providers';
import { useBookStore } from './useBookStore';
import { useFolderStore } from './useFolderStore';
import { useToasts } from './useToasts';
import { t } from '@/i18n';

const lastSyncedAt = ref<number | null>(null);
const refreshing = ref(false);
const error = ref('');

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
      const { fetchBooks } = useBookStore();
      const { fetchFolders } = useFolderStore();
      await Promise.all([fetchBooks(), fetchFolders()]);
      await useShelvesStore().fetchShelves();
      await loadLastSyncedAt();
      if (result) {
        showToast(t('library.scanFound', { books: result.bookCount, folders: result.folderCount }));
      }
    } catch (err) {
      if (err instanceof ShelfScanInProgressError) {
        showToast(t('library.scanInProgress'));
      } else if (err instanceof ShelfScanRateLimitedError) {
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
