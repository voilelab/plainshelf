import { ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { createShelfInitRetry, isShelfInitializing } from './shelfInitRetry';
import { t } from '@/i18n';

const folders = ref<string[]>([]);
const loading = ref(false);
const error = ref('');
const loaded = ref(false);

const initRetry = createShelfInitRetry();

async function run(isAutoRetry: boolean): Promise<void> {
  if (isAutoRetry) {
    initRetry.cancel();
  } else {
    initRetry.reset();
  }
  loading.value = true;
  error.value = '';

  try {
    folders.value = await getBookshelfProvider().listFolders();
    loaded.value = true;
    initRetry.reset();
  } catch (err) {
    // Wait out the initial scan rather than stranding the sidebar on an error
    // the shelf resolves on its own. See `shelfInitRetry`.
    if (isShelfInitializing(err)) {
      if (initRetry.schedule(() => void run(true))) {
        return;
      }
      error.value = t('layout.folderErrors.shelfNotReady');
    } else {
      error.value = err instanceof Error ? err.message : t('layout.folderErrors.loadFailed');
    }
  } finally {
    // A pending retry keeps the sidebar in its loading state rather than
    // flashing an error between attempts.
    if (!initRetry.pending) {
      loading.value = false;
    }
  }
}

// Takes no arguments on purpose: it is bound straight to a template click
// handler, which would otherwise pass the event in as the retry flag.
async function fetchFolders(): Promise<void> {
  await run(false);
}

export function useFolderStore() {
  return {
    folders,
    loading,
    error,
    loaded,
    fetchFolders
  };
}
