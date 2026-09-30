import { ref } from 'vue';

const STORAGE_KEY = 'show-nsfw-device';

/**
 * Whether this device shows the books the shelf marks as adult content.
 *
 * Device-local because a client reading cloud storage has no server to ask for
 * `show_nsfw`. Deliberately not synced with it: "not on this phone" must not
 * become "on no client at all". Off by default — a device that cannot yet
 * evaluate the marks should hide rather than show.
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

// One shared ref, so the settings panel and the lists cannot disagree.
const showNsfw = ref<boolean>(DEFAULT_SHOW_NSFW_ON_DEVICE);

if (typeof window !== 'undefined') {
  showNsfw.value = readStored();
  // `storage` fires in *other* tabs; the writing tab updates its own ref below.
  window.addEventListener('storage', (event) => {
    if (event.key === STORAGE_KEY) {
      showNsfw.value = event.newValue === 'true';
    }
  });
}

/** Straight from storage, so a change in another tab is honoured immediately. */
export function getShowNsfwOnDevice(): boolean {
  if (typeof window === 'undefined') {
    return showNsfw.value;
  }
  const stored = readStored();
  showNsfw.value = stored;
  return stored;
}

export function setShowNsfwOnDevice(value: boolean): void {
  // Synchronous, so a read right after a change sees the new value.
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
