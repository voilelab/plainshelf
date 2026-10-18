import { computed } from 'vue';
import { useShelvesStore } from './useShelvesStore';

export function useShelfCapabilities() {
  const { selectedShelf } = useShelvesStore();
  return {
    rescan: computed(() => selectedShelf.value?.capabilities?.rescan ?? true),
    charCount: computed(() => selectedShelf.value?.capabilities?.char_count ?? true),
    fingerprint: computed(() => selectedShelf.value?.capabilities?.fingerprint ?? true),
    download: computed(() => selectedShelf.value?.capabilities?.download ?? false),
    snapshotAt: computed(() => {
      const date = selectedShelf.value?.offline?.snapshot_at;
      const time = date ? Date.parse(date) : NaN;
      return Number.isFinite(time) ? time : null;
    })
  };
}
