import { computed, ref, type ComputedRef, type Ref } from 'vue';

import { useShelvesStore } from '@/composables/useShelvesStore';
import { isMobileRuntime } from '@/providers/runtime';
import {
  getActiveShelfEntryID,
  getShelfEntries,
  setActiveShelfEntry,
  shelfEntryDisplayName,
  type ShelfEntry
} from '@/providers/mobileConfig';
import { reloadIntoApp } from '@/features/mobile/utils/reloadIntoApp';
import { t } from '@/i18n';

export interface ShelfPickerItem {
  id: string;
  name: string;
  /** Source type, shown only on the mobile shell; empty everywhere else. */
  typeLabel: string;
}

export interface ShelfPicker {
  items: ComputedRef<ShelfPickerItem[]>;
  value: ComputedRef<string>;
  disabled: ComputedRef<boolean>;
  loading: Ref<boolean>;
  error: ComputedRef<string>;
  placeholder: ComputedRef<string>;
  /** Whether the picker offers a link out to shelf management. */
  managed: boolean;
  select: (id: string) => Promise<void>;
}

/**
 * What the sidebar shelf dropdown offers, which differs by client.
 *
 * On the web and desktop the shelves come from the server, and picking one is
 * an in-page change. On the mobile shell the device keeps its own list of
 * shelves — several servers and pCloud folders side by side — so the dropdown
 * lists those instead, and picking one restarts the app (see reloadIntoApp).
 *
 * Both branches are behind one interface so MainLayout renders a single Select
 * rather than two copies of the same portal markup.
 */
export function useShelfPicker(options: {
  onServerShelfSelected: (shelfID: string) => Promise<void>;
}): ShelfPicker {
  const {
    shelves,
    loading: shelvesLoading,
    error: shelvesError,
    selectedShelfID,
    selectShelf
  } = useShelvesStore();

  if (!isMobileRuntime()) {
    return {
      items: computed(() =>
        shelves.value.map((shelf) => ({ id: shelf.id, name: shelf.name, typeLabel: '' }))
      ),
      value: computed(() => selectedShelfID.value),
      disabled: computed(() => shelvesLoading.value || shelves.value.length === 0),
      loading: shelvesLoading,
      error: computed(() => shelvesError.value),
      placeholder: computed(() => {
        if (shelvesLoading.value) {
          return t('layout.shelf.loading');
        }
        return shelves.value.length === 0 ? t('layout.shelf.empty') : '';
      }),
      managed: false,
      select: async (id: string) => {
        selectShelf(id);
        await options.onServerShelfSelected(id);
      }
    };
  }

  // Read once: the list only changes on the shelf-management pages, which are
  // outside this layout, and every change to it lands back here through a full
  // reload anyway.
  const entries = ref<ShelfEntry[]>(getShelfEntries());
  const activeID = ref(getActiveShelfEntryID());

  return {
    items: computed(() =>
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
