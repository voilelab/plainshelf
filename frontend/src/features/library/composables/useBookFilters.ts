/**
 * The library's panel conditions as the UI needs them: the active ones (for the
 * badge and the chip row), a label and a remover per active condition, and a
 * "clear all". Everything is derived from `PANEL_BOOK_FILTERS`, so a new panel
 * condition joins the badge, the chips, and clear-all by being registered — no
 * edit here.
 */
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from '@/i18n';
import { PANEL_BOOK_FILTERS } from '@/utils/bookFilters/registry';
import type { ActiveBookFilter } from '@/utils/bookFilters/apply';
import { filterValueLabel } from '../utils/filterLabels';
import { useBooksRouteQuery } from './useBooksRouteQuery';

interface ActiveFilterChip {
  /** The filter key, used as the chip's list key. */
  readonly key: string;
  /** The human-readable "Field: value" label. */
  readonly label: string;
  /** Clears just this condition (resets to page 1). */
  remove(): void;
}

export function useBookFilters() {
  const route = useRoute();
  const { t } = useI18n();
  const { applyPanelFilters } = useBooksRouteQuery();

  // A backend that cannot answer a condition (charCount on pCloud) hides it
  // everywhere, including any stale value still sitting in the URL.
  const activePanelFilters = computed<ActiveBookFilter[]>(() =>
    PANEL_BOOK_FILTERS.map((filter) => ({ filter, value: filter.parse(route.query) })).filter(
      ({ filter, value }) => filter.supported?.() !== false && filter.isActive(value)
    )
  );

  const activeCount = computed(() => activePanelFilters.value.length);
  const hasActive = computed(() => activeCount.value > 0);

  const chips = computed<ActiveFilterChip[]>(() =>
    activePanelFilters.value.map(({ filter, value }) => ({
      key: filter.key,
      label: filterValueLabel(filter, value, t),
      remove: () => {
        void applyPanelFilters({ [filter.key]: filter.clearedValue });
      }
    }))
  );

  function clearAll(): void {
    const overrides: Record<string, unknown> = {};
    for (const filter of PANEL_BOOK_FILTERS) {
      overrides[filter.key] = filter.clearedValue;
    }
    void applyPanelFilters(overrides);
  }

  return { activePanelFilters, activeCount, hasActive, chips, clearAll };
}
