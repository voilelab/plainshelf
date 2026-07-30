import { ref, watch } from 'vue';
import { getReadOnlyMode } from '@/api/mode';

const readOnly = ref(false);
const loading = ref(false);
const loaded = ref(false);
const error = ref('');

watch(readOnly, (value) => {
  if (typeof window !== 'undefined') {
    window.__PLAINSHELF_READ_ONLY__ = value;
  }
}, { immediate: true });

export function isReadOnlyModeEnabled(): boolean {
  return readOnly.value;
}

export function assertWritableMode(): void {
  if (readOnly.value) {
    throw new Error('Server is in read-only mode. Write operations are disabled.');
  }
}

export function useServerMode() {
  async function fetchServerMode(): Promise<void> {
    loading.value = true;
    error.value = '';
    try {
      readOnly.value = await getReadOnlyMode();
      loaded.value = true;
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load server mode';
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
