import { getCurrentInstance, onUnmounted, ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { ApiError } from '@/api/client';
import { createShelfInitRetry, isShelfInitializing } from './shelfInitRetry';
import { t } from '@/i18n';

export function useCharCountIndex() {
  const counts = ref<Map<string, number | undefined>>(new Map());
  const ready = ref(false);
  const loading = ref(false);
  const error = ref('');

  const initRetry = createShelfInitRetry();
  let generation = 0;

  function invalidate(): void {
    generation++;
    initRetry.cancel();
    counts.value = new Map();
    ready.value = false;
    loading.value = false;
    error.value = '';
  }

  async function refresh(): Promise<void> {
    generation++;
    await run(generation, false);
  }

  async function run(token: number, isAutoRetry: boolean): Promise<void> {
    if (isAutoRetry) {
      initRetry.cancel();
    } else {
      initRetry.reset();
    }
    loading.value = true;
    error.value = '';
    try {
      const data = await getBookshelfProvider().listBooks(1, Number.MAX_SAFE_INTEGER, {
        includeCharCount: true
      });
      if (token !== generation) {
        return;
      }
      counts.value = new Map(data.items.map((book) => [book.id, book.char_count]));
      ready.value = true;
      initRetry.reset();
    } catch (err) {
      if (token !== generation) {
        return;
      }
      if (isShelfInitializing(err)) {
        if (initRetry.schedule(() => void run(token, true), err)) {
          return;
        }
        error.value = t('library.shelfNotReady');
      } else {
        error.value = err instanceof ApiError && err.isTimeout
          ? t('library.requestTimeout')
          : err instanceof Error ? err.message : t('library.loadFailed');
      }
    } finally {
      if (token === generation && !initRetry.pending) {
        loading.value = false;
      }
    }
  }

  async function load(): Promise<void> {
    if (ready.value || loading.value) {
      return;
    }
    await refresh();
  }

  if (getCurrentInstance()) {
    onUnmounted(initRetry.cancel);
  }

  return {
    counts,
    ready,
    loading,
    error,
    load,
    refresh,
    invalidate
  };
}
