import { ref } from 'vue';
import { getBookshelfProvider } from '@/providers';
import { ApiError } from '@/api/client';
import { t } from '@/i18n';

const SHELF_INIT_RETRY_DELAY_MS = 3000;
const SHELF_INIT_MAX_AUTO_RETRIES = 10; // ~30s of auto-retry before showing an error

const layers = ref<string[]>([]);
const loading = ref(false);
const error = ref('');
const loaded = ref(false);

let retryTimer: ReturnType<typeof setTimeout> | null = null;
let initRetryCount = 0;

function clearRetry(): void {
  if (retryTimer !== null) {
    clearTimeout(retryTimer);
    retryTimer = null;
  }
}

async function run(isAutoRetry: boolean): Promise<void> {
  clearRetry();
  if (!isAutoRetry) {
    initRetryCount = 0;
  }
  loading.value = true;
  error.value = '';

  try {
    layers.value = await getBookshelfProvider().listLayers();
    loaded.value = true;
    initRetryCount = 0;
  } catch (err) {
    // A shelf still running its initial scan answers 503 for every read. The
    // book listing retries through that, so the layer tree has to as well:
    // otherwise the sidebar strands on an error the shelf resolves on its own
    // while the book list recovers on the next tick.
    if (err instanceof ApiError && err.status === 503) {
      initRetryCount++;
      if (initRetryCount < SHELF_INIT_MAX_AUTO_RETRIES) {
        retryTimer = setTimeout(() => void run(true), SHELF_INIT_RETRY_DELAY_MS);
        return;
      }
      error.value = t('layout.layerErrors.shelfNotReady');
    } else {
      error.value = err instanceof Error ? err.message : t('layout.layerErrors.loadFailed');
    }
  } finally {
    // A pending retry keeps the sidebar in its loading state rather than
    // flashing an error between attempts.
    if (retryTimer === null) {
      loading.value = false;
    }
  }
}

// Takes no arguments on purpose: it is bound straight to a template click
// handler, which would otherwise pass the event in as the retry flag.
async function fetchLayers(): Promise<void> {
  await run(false);
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
