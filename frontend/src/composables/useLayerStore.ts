import { ref } from 'vue';
import { getLayers } from '../api/layers';

const layers = ref<string[]>([]);
const loading = ref(false);
const error = ref('');
const loaded = ref(false);

async function fetchLayers(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    layers.value = await getLayers();
    loaded.value = true;
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load layers';
  } finally {
    loading.value = false;
  }
}

export function useLayerStore() {
  return {
    layers,
    loading,
    error,
    loaded,
    fetchLayers
  };
}
