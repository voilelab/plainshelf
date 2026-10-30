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
  SOURCE_TYPE_LABEL_KEYS,
  type ShelfEntry
} from '@/providers/mobileConfig';

export function createMobileShelfPicker(): ShelfPicker {
  const { loading: shelvesLoading } = useShelvesStore();

  const entries = ref<ShelfEntry[]>(getShelfEntries());
  const activeID = ref(getActiveShelfEntryID());

  return {
    items: computed<ShelfPickerItem[]>(() =>
      entries.value.map((entry) => ({
        id: entry.id,
        name: shelfEntryDisplayName(entry),
        typeLabel: t(SOURCE_TYPE_LABEL_KEYS[entry.type])
      }))
    ),
    value: computed(() => activeID.value),
    disabled: computed(() => entries.value.length === 0),
    loading: shelvesLoading,
    error: computed(() => ''),
    placeholder: computed(() => (entries.value.length === 0 ? t('layout.shelf.empty') : '')),
    manageTo: { path: '/connect' },
    select: async (id: string) => {
      await setActiveShelfEntry(id);
      reloadIntoApp();
    }
  };
}
