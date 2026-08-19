import { computed, ref } from 'vue';

import type { ShelfPicker, ShelfPickerItem } from '@/types/shelfPicker';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { reloadIntoApp } from '@/features/mobile/utils/reloadIntoApp';
import { t } from '@/i18n';
import {
  getActiveShelfEntryID,
  getShelfEntries,
  setActiveShelfEntry,
  shelfEntryDisplayName,
  type ShelfEntry
} from '@/providers/mobileConfig';

/**
 * The sidebar shelf dropdown on the mobile shell.
 *
 * The device keeps its own list of shelves — several servers and pCloud folders
 * side by side — so the dropdown lists those rather than anything a server
 * enumerates, and picking one restarts the app (see reloadIntoApp) instead of
 * swapping shelves in place.
 *
 * Lives here rather than in the shared composable because everything it needs —
 * the shelf entries, the reload — is mobile-only, and importing it from shared
 * code is what kept @capacitor/preferences and the Keystore wrapper in the web
 * and desktop bundles.
 */
export function createMobileShelfPicker(): ShelfPicker {
  const { loading: shelvesLoading } = useShelvesStore();

  // Read once: the list only changes on the shelf-management pages, which are
  // outside this layout, and every change to it lands back here through a full
  // reload anyway.
  const entries = ref<ShelfEntry[]>(getShelfEntries());
  const activeID = ref(getActiveShelfEntryID());

  return {
    items: computed<ShelfPickerItem[]>(() =>
      entries.value.map((entry) => ({
        id: entry.id,
        name: shelfEntryDisplayName(entry),
        typeLabel:
          entry.type === 'pcloud' ? t('mobileConnect.modePCloud') : t('mobileConnect.modeServer')
      }))
    ),
    value: computed(() => activeID.value),
    disabled: computed(() => entries.value.length === 0),
    loading: shelvesLoading,
    // The shelves store only speaks for a server, and a pCloud shelf is not
    // one. Its errors belong to the library view, not to this list.
    error: computed(() => ''),
    placeholder: computed(() => (entries.value.length === 0 ? t('layout.shelf.empty') : '')),
    managed: true,
    select: async (id: string) => {
      await setActiveShelfEntry(id);
      reloadIntoApp();
    }
  };
}
