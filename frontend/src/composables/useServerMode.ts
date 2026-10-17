import { ref, watch } from 'vue';
import { getReadOnlyMode } from '@/api/mode';
import { t } from '@/i18n';

const readOnly = ref(false);
const loading = ref(false);
const loaded = ref(false);
const error = ref('');

watch(readOnly, (value) => {
  if (typeof window !== 'undefined') {
    window.__PLAINSHELF_READ_ONLY__ = value;
  }
}, { immediate: true });

export function useServerMode() {
  async function fetchServerMode(): Promise<void> {
    loading.value = true;
    error.value = '';
    try {
      readOnly.value = await getReadOnlyMode();
      loaded.value = true;
    } catch (err) {
      error.value = err instanceof Error ? err.message : t('settings.serverModeLoadFailed');
    } finally {
      loading.value = false;
    }
  }

  return {
    readOnly,
    loading,
    loaded,
    error,
    fetchServerMode
  };
}
