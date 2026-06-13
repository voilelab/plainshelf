import { ref } from 'vue';
import { ensureActiveShelf, listShelves, type ShelfInfo } from '../api/shelves';
import { setActiveShelfID } from '../api/client';

const shelves = ref<ShelfInfo[]>([]);
const loading = ref(false);
const loaded = ref(false);
const error = ref('');
const selectedShelfID = ref('');

async function fetchShelves(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    const nextShelves = await listShelves();
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
