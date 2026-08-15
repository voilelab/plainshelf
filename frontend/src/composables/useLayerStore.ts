import { ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { t } from '@/i18n';

const layers = ref<string[]>([]);
const loading = ref(false);
const error = ref('');
const loaded = ref(false);

async function fetchLayers(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    layers.value = await getBookshelfProvider().listLayers();
    loaded.value = true;
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('layout.layerErrors.loadFailed');
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
