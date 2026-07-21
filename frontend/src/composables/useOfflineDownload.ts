import { computed, ref } from 'vue';
import { getBookshelfProvider } from '../providers';
import type { DownloadState } from '../types/book';

/**
 * Wraps the optional `downloadBook` / `removeDownload` / `getDownloadState`
 * BookshelfProvider methods into a small state machine for a single book's
 * offline-download UI (BookDetailPage's "Download to device" button).
 *
 * `downloading` and `failed` are transient, component-local states — they
 * are never persisted to the cache, only `not_downloaded` / `downloaded` /
 * `update_available` come back from `getDownloadState`. When the provider
 * does not implement these optional methods (desktop, plain server), the
 * state stays `not_downloaded` and `supported` is false so callers can
 * disable/hide the control.
 */
export function useOfflineDownload(bookId: () => string) {
  const state = ref<DownloadState>('not_downloaded');
  const error = ref('');

  const supported = computed(() => {
    const provider = getBookshelfProvider();
    return Boolean(provider.downloadBook && provider.removeDownload && provider.getDownloadState);
  });

  async function refresh(): Promise<void> {
    const provider = getBookshelfProvider();
    if (!provider.getDownloadState) {
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
    if (!provider.downloadBook) {
      return;
    }

    state.value = 'downloading';
    error.value = '';

    try {
      await provider.downloadBook(bookId());
      state.value = 'downloaded';
    } catch (err) {
      state.value = 'failed';
      error.value = err instanceof Error ? err.message : 'Failed to download book';
    }
  }

  async function remove(): Promise<void> {
    const provider = getBookshelfProvider();
    if (!provider.removeDownload) {
      return;
    }

    error.value = '';

    try {
      await provider.removeDownload(bookId());
      state.value = 'not_downloaded';
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to remove download';
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
