import type { ComputedRef, Ref } from 'vue';
import type { RouteLocationRaw } from 'vue-router';

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
  /**
   * Where the "manage shelves" link points, or null when this shell offers no
   * such link. The mobile shell sends it to the connection page; the desktop
   * build sends it to the settings shelves tab so a first-time user reaches
   * shelf management without opening settings and hunting for the tab. The
   * layout merges the current route query into it (preserving the mobile
   * shell-preview flag).
   */
  manageTo: RouteLocationRaw | null;
  select: (id: string) => Promise<void>;
}
