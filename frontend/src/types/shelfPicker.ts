import type { ComputedRef, Ref } from 'vue';

export interface ShelfPickerItem {
  id: string;
  name: string;
  /** Source type, shown only on the mobile shell; empty everywhere else. */
  typeLabel: string;
}

/**
 * What the sidebar shelf dropdown needs, whichever client builds it.
 *
 * Declared here rather than beside `useShelfPicker` so the composable and
 * `providers/shell.ts` can both name it without importing each other: the
 * shell offers `createShelfPicker`, and the composable asks the shell for one.
 * That was a type-only cycle, which costs nothing today but turns into a real
 * one the moment either import stops being type-only.
 */
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
