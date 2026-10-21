import { useShelfCapabilities } from './useShelfCapabilities';
import { computed, ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import type { DownloadState } from '@/types/book';
import { t } from '@/i18n';

export function useOfflineDownload(bookId: () => string) {
  const capability = useShelfCapabilities();
  const state = ref<DownloadState>('not_downloaded');
  const error = ref('');

  const supported = computed(() => {
    const provider = getBookshelfProvider();
    return capability.download.value && Boolean(provider.downloadBook && provider.removeDownload && provider.getDownloadState);
  });

  async function refresh(): Promise<void> {
    const provider = getBookshelfProvider();
    if (!supported.value || !provider.getDownloadState) {
      state.value = 'not_downloaded';
      return;
    }

    try {
      state.value = await provider.getDownloadState(bookId());
    } catch {
      state.value = 'not_downloaded';
    }
  }

  async function download(): Promise<void> {
    const provider = getBookshelfProvider();
    if (!supported.value || !provider.downloadBook) {
      return;
    }

    state.value = 'downloading';
    error.value = '';

    try {
      await provider.downloadBook(bookId());
      state.value = 'downloaded';
    } catch (err) {
      state.value = 'failed';
      error.value = err instanceof Error ? err.message : t('downloads.detail.downloadFailed');
    }
  }

  async function remove(): Promise<void> {
    const provider = getBookshelfProvider();
    if (!supported.value || !provider.removeDownload) {
      return;
    }

    error.value = '';

    try {
      await provider.removeDownload(bookId());
      state.value = 'not_downloaded';
    } catch (err) {
      error.value = err instanceof Error ? err.message : t('downloads.removeConfirm.failed');
    }
  }

  return {
    state,
    error,
    supported,
    refresh,
    download,
    remove
  };
}
