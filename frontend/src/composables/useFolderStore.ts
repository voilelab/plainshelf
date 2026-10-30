import { ref } from 'vue';
import type { FolderMeta } from '@/api/folders';
import { getBookshelfProvider } from '@/providers';
import { createShelfInitRetry, isShelfInitializing } from './shelfInitRetry';
import { t } from '@/i18n';

const folders = ref<string[]>([]);
const folderMetaError = ref('');
const folderMeta = ref<Record<string, FolderMeta>>({});
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
    const provider = getBookshelfProvider();
    const [listed, meta] = await Promise.all([provider.listFolders(), loadFolderMeta()]);
    folders.value = listed;
    folderMeta.value = meta;
    loaded.value = true;
    initRetry.reset();
  } catch (err) {
    if (isShelfInitializing(err)) {
      if (initRetry.schedule(() => void run(true), err)) {
        return;
      }
      error.value = t('layout.folderErrors.shelfNotReady');
    } else {
      error.value = err instanceof Error ? err.message : t('layout.folderErrors.loadFailed');
    }
  } finally {
    if (!initRetry.pending) {
      loading.value = false;
    }
  }
}

async function loadFolderMeta(): Promise<Record<string, FolderMeta>> {
  folderMetaError.value = '';
  const provider = getBookshelfProvider();
  if (!provider.listFolderMeta) {
    return {};
  }
  try {
    return (await provider.listFolderMeta()).folders;
  } catch {
    folderMetaError.value = t('layout.folderOrder.reloadFailed');
    return {};
  }
}

async function fetchFolderMeta(): Promise<void> {
  folderMeta.value = await loadFolderMeta();
}

async function fetchFolders(): Promise<void> {
  await run(false);
}

export function useFolderStore() {
  return {
    folders,
    folderMeta,
    folderMetaError,
    loading,
    error,
    loaded,
    fetchFolders,
    fetchFolderMeta
  };
}
