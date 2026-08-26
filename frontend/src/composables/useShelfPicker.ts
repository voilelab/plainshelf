import { computed } from 'vue';

import { useShelvesStore } from '@/composables/useShelvesStore';
import { isWailsRuntime } from '@/providers/runtime';
import { getShell } from '@/providers/shell';
import type { ShelfPicker } from '@/types/shelfPicker';
import { t } from '@/i18n';

export type { ShelfPicker, ShelfPickerItem } from '@/types/shelfPicker';

/**
 * What the sidebar shelf dropdown offers, which differs by client.
 *
 * Here the shelves come from the server and picking one is an in-page change.
 * A shell whose shelves are device-local supplies its own picker instead — see
 * RuntimeShell.createShelfPicker.
 *
 * Both are behind one interface so MainLayout renders a single Select rather
 * than two copies of the same portal markup.
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

  const fromShell = getShell()?.createShelfPicker?.();
  if (fromShell) {
    return fromShell;
  }

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
    // The web build has no shelf-management surface to link to; the desktop
    // build manages shelves on the settings shelves tab.
    manageTo: isWailsRuntime() ? { path: '/settings', query: { tab: 'shelves' } } : null,
    select: async (id: string) => {
      selectShelf(id);
      await options.onServerShelfSelected(id);
    }
  };
}
