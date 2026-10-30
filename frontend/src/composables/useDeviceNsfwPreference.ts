import { ref } from 'vue';

const STORAGE_KEY = 'show-nsfw-device';

const DEFAULT_SHOW_NSFW_ON_DEVICE = false;

function readStored(): boolean {
  if (typeof window === 'undefined') {
    return DEFAULT_SHOW_NSFW_ON_DEVICE;
  }
  return window.localStorage.getItem(STORAGE_KEY) === 'true';
}

const showNsfw = ref<boolean>(DEFAULT_SHOW_NSFW_ON_DEVICE);

if (typeof window !== 'undefined') {
  showNsfw.value = readStored();
  // HTML Web Storage sends storage events to other documents, excluding the document that performed the write.
  window.addEventListener('storage', (event) => {
    if (event.key === STORAGE_KEY) {
      showNsfw.value = event.newValue === 'true';
    }
  });
}

export function getShowNsfwOnDevice(): boolean {
  if (typeof window === 'undefined') {
    return showNsfw.value;
  }
  const stored = readStored();
  showNsfw.value = stored;
  return stored;
}

export function setShowNsfwOnDevice(value: boolean): void {
  showNsfw.value = value;
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(STORAGE_KEY, value ? 'true' : 'false');
  }
}

export function useDeviceNsfwPreference() {
  return {
    showNsfw,
    setShowNsfw: setShowNsfwOnDevice
  };
}
