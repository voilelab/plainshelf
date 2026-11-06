import { ref } from 'vue';
import { getPreference, setPreference, subscribePreference } from '@/storage/preferences';

const STORAGE_KEY = 'import-notifications';

function readStored(): boolean {
  return getPreference(STORAGE_KEY) !== 'false';
}

const enabled = ref<boolean>(readStored());

subscribePreference(STORAGE_KEY, (value) => {
  enabled.value = value !== 'false';
});

function setImportNotifications(value: boolean): void {
  enabled.value = value;
  setPreference(STORAGE_KEY, value ? 'true' : 'false');
}

export function useImportNotificationPreference() {
  return {
    enabled,
    setEnabled: setImportNotifications
  };
}
