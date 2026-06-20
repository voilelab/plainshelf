import { ref } from 'vue';
import { ensureActiveShelf, listShelves, type ShelfInfo } from '../api/shelves';
import { ApiError, setActiveShelfID } from '../api/client';

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

async function fetchShelves(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    const nextShelves = await listShelvesWithStartupRetry();
    shelves.value = nextShelves;
    selectedShelfID.value = ensureActiveShelf(nextShelves);
    loaded.value = true;
  } catch (err) {
    selectedShelfID.value = '';
    error.value = err instanceof Error ? err.message : 'Failed to load shelves';
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
