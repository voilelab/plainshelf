import { computed } from 'vue';

import { useServerMode } from '@/composables/useServerMode';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { getBookshelfProvider, isWritableProvider } from '@/providers';

type WriteDisabledReason = 'platform' | 'server-read-only' | 'shelf-read-only' | null;

function isLibraryEditingSupported(): boolean {
  return isWritableProvider(getBookshelfProvider());
}

export function useWriteAccess() {
  const { readOnly } = useServerMode();
  const { selectedShelfReadOnly } = useShelvesStore();

  const writesEnabled = computed(
    () => !readOnly.value && !selectedShelfReadOnly.value && isLibraryEditingSupported()
  );

  const writeDisabledReason = computed<WriteDisabledReason>(() => {
    if (!isLibraryEditingSupported()) {
      return 'platform';
    }
    if (readOnly.value) {
      return 'server-read-only';
    }
    if (selectedShelfReadOnly.value) {
      return 'shelf-read-only';
    }
    return null;
  });

  const outgoingCopyEnabled = computed(() => !readOnly.value && isLibraryEditingSupported());

  const writeDisabledMessageKey = computed(() =>
    writeDisabledReason.value === 'shelf-read-only'
      ? ('layout.readOnly.shelfWriteDisabled' as const)
      : ('layout.readOnly.writeDisabled' as const)
  );

  const libraryEditingAvailable = computed(() => isLibraryEditingSupported());

  const serverSettingsEditable = computed(() => isLibraryEditingSupported());

  const serverAdminAvailable = computed(() => isLibraryEditingSupported());

  return {
    writesEnabled,
    writeDisabledReason,
    writeDisabledMessageKey,
    outgoingCopyEnabled,
    libraryEditingAvailable,
    serverSettingsEditable,
    serverAdminAvailable
  };
}
