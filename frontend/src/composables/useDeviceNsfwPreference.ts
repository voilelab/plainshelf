import { ref } from 'vue';
import { getPreference, setPreference, subscribePreference } from '@/storage/preferences';

const STORAGE_KEY = 'show-nsfw-device';

function readStored(): boolean {
  return getPreference(STORAGE_KEY) === 'true';
}

const showNsfw = ref<boolean>(readStored());

subscribePreference(STORAGE_KEY, (value) => {
  showNsfw.value = value === 'true';
});

export function getShowNsfwOnDevice(): boolean {
  const stored = readStored();
  showNsfw.value = stored;
  return stored;
}

export function setShowNsfwOnDevice(value: boolean): void {
  showNsfw.value = value;
  setPreference(STORAGE_KEY, value ? 'true' : 'false');
}

export function useDeviceNsfwPreference() {
  return {
    showNsfw,
    setShowNsfw: setShowNsfwOnDevice
  };
}
