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

// Deliberately not the shared `shelfInitRetry` budget the book listing, the
// folder tree, the char-count index and the dashboard use. This is a fast poll
// for the shelf list at startup, not a wait for the shelf's initial scan;
// borrowing that 10 x 3000ms budget would stall the sidebar's first paint.
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
    // Listing shelves needs the network, but a backend whose shelf choice is
    // device-local already knows which one it is pointed at. Fall back to it so
    // offline-cached books stay reachable; the error is still surfaced in the
    // sidebar. The connect page opts out: while validating a newly typed
    // server, a failed fetch must not resurrect a shelf that belongs to the
    // previous server — so the provider is asked for only when the fallback is
    // wanted, since reaching for it is what creates it on first use.
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
 * Loads the shelf list once for the page.
 *
 * Two independent places want the list on a settings page load — the layout and
 * the Shelves panel — and a child's onMounted runs before its parent's, so
 * whether the second one saw `loaded` already set came down to which network
 * round-trip finished first. Sharing the in-flight promise settles it: the list
 * is fetched once, whoever asks first.
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
 * Whether the shelf currently being browsed refuses writes.
 *
 * Derived from the list rather than fetched per shelf: `GET /api/shelves`
 * already reports every shelf's state, so switching shelves needs no request
 * and the answer is never a round-trip behind the selection.
 *
 * False until the list loads, and false for a shelf the list does not contain
 * (the persisted-shelf fallback above). The UI defaults to offering writes and
 * the server still answers 409, which is the safe way round: a shelf wrongly
 * treated as read-only would hide controls that work.
 */
const activeShelfReadOnly = computed(
  () => shelves.value.find((shelf) => shelf.id === selectedShelfID.value)?.readOnly === true
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
    activeShelfReadOnly,
    fetchShelves,
    ensureShelvesLoaded,
    selectShelf
  };
}
