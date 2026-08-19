import { ref } from 'vue';
import { ensureActiveShelf, listShelves, type ShelfInfo } from '@/api/shelves';
import { ApiError, setActiveShelfID } from '@/api/client';
import { getBookshelfProvider } from '@/providers';
import { t } from '@/i18n';

const shelves = ref<ShelfInfo[]>([]);
const loading = ref(false);
const loaded = ref(false);
const error = ref('');
const selectedShelfID = ref('');

const STARTUP_MAX_RETRIES = 20;
const STARTUP_RETRY_DELAY_MS = 300;

async function listShelvesWithStartupRetry(): Promise<ShelfInfo[]> {
  for (let attempt = 0; attempt < STARTUP_MAX_RETRIES; attempt++) {
    try {
      return await listShelves();
    } catch (err) {
      if (!(err instanceof ApiError && err.status === 503)) {
        throw err;
      }
      await new Promise<void>((resolve) => setTimeout(resolve, STARTUP_RETRY_DELAY_MS));
    }
  }
  return listShelves();
}

async function fetchShelves(options?: { allowPersistedFallback?: boolean }): Promise<void> {
  const allowPersistedFallback = options?.allowPersistedFallback ?? true;
  loading.value = true;
  error.value = '';

  try {
    const nextShelves = await listShelvesWithStartupRetry();
    shelves.value = nextShelves;
    selectedShelfID.value = ensureActiveShelf(nextShelves);
    loaded.value = true;
  } catch (err) {
    // Listing shelves needs the network, but a backend whose shelf choice is
    // device-local already knows which one it is pointed at. Fall back to it so
    // offline-cached books stay reachable; the error is still surfaced in the
    // sidebar. The connect page opts out: while validating a newly typed
    // server, a failed fetch must not resurrect a shelf that belongs to the
    // previous server — so the provider is asked for only when the fallback is
    // wanted, since reaching for it is what creates it on first use.
    const persistedShelfID = allowPersistedFallback
      ? (getBookshelfProvider().getPersistedShelfID?.() ?? '')
      : '';
    if (persistedShelfID) {
      selectedShelfID.value = persistedShelfID;
      loaded.value = true;
    } else {
      selectedShelfID.value = '';
    }
    error.value = err instanceof Error ? err.message : t('settings.shelves.loadFailed');
  } finally {
    loading.value = false;
  }
}

function selectShelf(id: string): void {
  setActiveShelfID(id);
  selectedShelfID.value = id;
}

export function useShelvesStore() {
  return {
    shelves,
    loading,
    loaded,
    error,
    selectedShelfID,
    fetchShelves,
    selectShelf
  };
}
