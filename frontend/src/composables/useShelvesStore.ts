import { computed, ref } from 'vue';
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

let pendingLoad: Promise<void> | null = null;

async function ensureShelvesLoaded(): Promise<void> {
  if (loaded.value) {
    return;
  }
  pendingLoad ??= fetchShelves().finally(() => {
    pendingLoad = null;
  });
  return pendingLoad;
}

const selectedShelfReadOnly = computed(
  () => shelves.value.find((shelf) => shelf.id === selectedShelfID.value)?.readOnly === true
);

const transferDestinationShelves = computed(() =>
  shelves.value.filter((shelf) => shelf.id !== selectedShelfID.value && !shelf.readOnly)
);

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
    selectedShelfReadOnly,
    selectedShelf: computed(() => shelves.value.find((shelf) => shelf.id === selectedShelfID.value)),
    transferDestinationShelves,
    fetchShelves,
    ensureShelvesLoaded,
    selectShelf
  };
}
