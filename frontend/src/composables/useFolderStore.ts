import { ref } from 'vue';
import type { FolderMeta } from '@/api/folders';
import { getBookshelfProvider } from '@/providers';
import { createShelfInitRetry, isShelfInitializing } from './shelfInitRetry';
import { t } from '@/i18n';

const folders = ref<string[]>([]);
const folderMetaError = ref('');
// Keyed by folder path; a folder absent here shows the default icon.
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
    // Wait out the initial scan rather than stranding the sidebar on an error
    // the shelf resolves on its own. See `shelfInitRetry`.
    if (isShelfInitializing(err)) {
      if (initRetry.schedule(() => void run(true), err)) {
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

// Icons are decoration: a listing that fails, or a backend without one, leaves
// every folder on the default icon rather than failing the folder tree.
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

/** Re-reads only the metadata, after a write or a refused one. */
async function fetchFolderMeta(): Promise<void> {
  folderMeta.value = await loadFolderMeta();
}

// Takes no arguments on purpose: it is bound straight to a template click
// handler, which would otherwise pass the event in as the retry flag.
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
