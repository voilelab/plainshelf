import type { ComputedRef, Ref } from 'vue';
import type { RouteLocationRaw } from 'vue-router';

export interface ShelfPickerItem {
  id: string;
  name: string;
  typeLabel: string;
}

export interface ShelfPicker {
  items: ComputedRef<ShelfPickerItem[]>;
  value: ComputedRef<string>;
  disabled: ComputedRef<boolean>;
  loading: Ref<boolean>;
  error: ComputedRef<string>;
  placeholder: ComputedRef<string>;
  manageTo: RouteLocationRaw | null;
  select: (id: string) => Promise<void>;
}
