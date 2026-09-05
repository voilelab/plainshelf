import { ref } from 'vue';

const STORAGE_KEY = 'show-nsfw-device';

/**
 * Whether this device shows the books the shelf marks as adult content.
 *
 * Device-local (localStorage), like the reader-launch preference and for a
 * sharper reason: a client reading a shelf straight out of cloud storage has no
 * server to ask, so `/api/setting/show_nsfw` — the answer PSW-92 filters on —
 * is not reachable from it at all. Without a local answer the mark would have
 * no effect there, which is the wrong direction to fail in.
 *
 * It is deliberately *not* synchronised with the server setting. They are two
 * machines' separate decisions: "not on this phone" must not become "on no
 * client at all". Where a server does answer, it stays the only authority and
 * this preference filters nothing (see `filtersNsfwOnDevice` on the provider).
 *
 * Defaults to off, so a shelf whose marks this device cannot yet evaluate — a
 * snapshot from an older build, a storage read that failed — hides rather than
 * shows.
 */
const DEFAULT_SHOW_NSFW_ON_DEVICE = false;

function readStored(): boolean {
  if (typeof window === 'undefined') {
    return DEFAULT_SHOW_NSFW_ON_DEVICE;
  }
  // Only the exact string this module writes turns it on: never set, or some
  // unknown value from a hand-edit or an older build, means off.
  return window.localStorage.getItem(STORAGE_KEY) === 'true';
}

// One shared preference for the whole app rather than per-component state, so
// the settings panel, the library and the downloads list cannot disagree.
const showNsfw = ref<boolean>(DEFAULT_SHOW_NSFW_ON_DEVICE);

if (typeof window !== 'undefined') {
  showNsfw.value = readStored();
  // `storage` fires in *other* same-origin tabs, so a change made in one reaches
  // every open tab's ref. The writing tab updates its own ref below.
  window.addEventListener('storage', (event) => {
    if (event.key === STORAGE_KEY) {
      showNsfw.value = event.newValue === 'true';
    }
  });
}

/**
 * Read straight from localStorage rather than from the ref, so a filter built
 * during a listing honours a change made in another tab before its `storage`
 * event has necessarily been processed here.
 */
export function getShowNsfwOnDevice(): boolean {
  if (typeof window === 'undefined') {
    return showNsfw.value;
  }
  const stored = readStored();
  showNsfw.value = stored;
  return stored;
}

export function setShowNsfwOnDevice(value: boolean): void {
  // Persisted synchronously, so a getShowNsfwOnDevice() right after a change
  // reads the new value with no watcher flush in between.
  showNsfw.value = value;
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(STORAGE_KEY, value ? 'true' : 'false');
  }
}

/** Reactive access for the settings panel and for lists that re-filter in place. */
export function useDeviceNsfwPreference() {
  return {
    showNsfw,
    setShowNsfw: setShowNsfwOnDevice
  };
}
