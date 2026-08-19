import { computed, ref, type ComputedRef, type Ref } from 'vue';

import { useShelvesStore } from '@/composables/useShelvesStore';
import { getShell } from '@/providers/shell';
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
    managed: false,
    select: async (id: string) => {
      selectShelf(id);
      await options.onServerShelfSelected(id);
    }
  };
}
