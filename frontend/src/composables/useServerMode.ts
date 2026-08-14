import { ref, watch } from 'vue';
import { getReadOnlyMode } from '@/api/mode';
import { getMobileConnectionConfig } from '@/providers/mobileConfig';
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
    // A pCloud connection has no server to report a mode. Skipping keeps a
    // doomed request — awaited before the layout renders anything — out of
    // startup. Read-only is still enforced, by the mobile guard in
    // api/client.ts and by the pCloud provider itself.
    if (getMobileConnectionConfig()?.mode === 'pcloud') {
      loaded.value = true;
      return;
    }

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
