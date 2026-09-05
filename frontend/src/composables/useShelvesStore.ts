import { computed, ref } from 'vue';
import { ensureActiveShelf, listShelves, type ShelfInfo } from '@/api/shelves';
import { ApiError, setActiveShelfID } from '@/api/client';
import { getBookshelfProvider } from '@/providers';
import { t } from '@/i18n';

const shelves = ref<ShelfInfo[]>([]);
const loading = ref(false);
const loaded = ref(false);
const error = ref('');
const selectedShelfID = ref('');

// Deliberately not the shared `shelfInitRetry` budget: this is a fast startup
// poll for the shelf list, not a wait for the shelf's initial scan, and
// borrowing that budget would stall the sidebar's first paint.
const STARTUP_MAX_RETRIES = 20;
const STARTUP_RETRY_DELAY_MS = 300;

async function listShelvesWithStartupRetry(): Promise<ShelfInfo[]> {
  for (let attempt = 0; attempt < STARTUP_MAX_RETRIES; attempt++) {
    try {
      return await listShelves();
    } catch (err) {
      if (!(err instanceof ApiError && err.status === 503)) {
        throw err;
      }
      await new Promise<void>((resolve) => setTimeout(resolve, STARTUP_RETRY_DELAY_MS));
    }
  }
  return listShelves();
}

async function fetchShelves(options?: { allowPersistedFallback?: boolean }): Promise<void> {
  const allowPersistedFallback = options?.allowPersistedFallback ?? true;
  loading.value = true;
  error.value = '';

  try {
    const nextShelves = await listShelvesWithStartupRetry();
    shelves.value = nextShelves;
    selectedShelfID.value = ensureActiveShelf(nextShelves);
    loaded.value = true;
  } catch (err) {
    // A backend whose shelf choice is device-local already knows which one it is
    // pointed at, so fall back to it and keep offline-cached books reachable.
    // The connect page opts out: while validating a newly typed server, a failed
    // fetch must not resurrect the previous server's shelf — and asking for the
    // provider is what creates it on first use.
    const persistedShelfID = allowPersistedFallback
      ? (getBookshelfProvider().getPersistedShelfID?.() ?? '')
      : '';
    if (persistedShelfID) {
      selectedShelfID.value = persistedShelfID;
      loaded.value = true;
    } else {
      selectedShelfID.value = '';
    }
    error.value = err instanceof Error ? err.message : t('settings.shelves.loadFailed');
  } finally {
    loading.value = false;
  }
}

let pendingLoad: Promise<void> | null = null;

/**
 * Loads the shelf list once for the page. Two independent places want it on a
 * settings load, and a child's onMounted runs before its parent's, so whether
 * the second saw `loaded` set came down to which round-trip finished first.
 * Sharing the in-flight promise settles it.
 *
 * fetchShelves stays the explicit refresh, for when a shelf was just added or
 * removed and the caller needs the new list rather than the one already loading.
 */
async function ensureShelvesLoaded(): Promise<void> {
  if (loaded.value) {
    return;
  }
  pendingLoad ??= fetchShelves().finally(() => {
    pendingLoad = null;
  });
  return pendingLoad;
}

/**
 * Derived from the list rather than fetched per shelf: `GET /api/shelves`
 * already reports every shelf's state, so switching needs no request and the
 * answer is never a round-trip behind the selection.
 *
 * False until the list loads, and false for a shelf it does not contain. The UI
 * then offers writes and the server still answers 409, which is the safe way
 * round: a shelf wrongly treated as read-only would hide controls that work.
 */
const selectedShelfReadOnly = computed(
  () => shelves.value.find((shelf) => shelf.id === selectedShelfID.value)?.readOnly === true
);

/**
 * The shelves a cross-shelf transfer may land in. The selected one is out
 * because the server rejects one shelf as both ends; a read-only one is out
 * because the transfer writes its target whichever mode was picked. The source
 * being read-only is `selectedShelfReadOnly` above.
 */
const transferDestinationShelves = computed(() =>
  shelves.value.filter((shelf) => shelf.id !== selectedShelfID.value && !shelf.readOnly)
);

function selectShelf(id: string): void {
  setActiveShelfID(id);
  selectedShelfID.value = id;
}

export function useShelvesStore() {
  return {
    shelves,
    loading,
    loaded,
    error,
    selectedShelfID,
    selectedShelfReadOnly,
    transferDestinationShelves,
    fetchShelves,
    ensureShelvesLoaded,
    selectShelf
  };
}
