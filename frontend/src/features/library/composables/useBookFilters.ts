import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from '@/i18n';
import { PANEL_BOOK_FILTERS } from '@/utils/bookFilters/registry';
import type { ActiveBookFilter } from '@/utils/bookFilters/apply';
import { filterValueLabel } from '../utils/filterLabels';
import { useBooksRouteQuery } from './useBooksRouteQuery';

interface ActiveFilterChip {
  readonly key: string;
  readonly label: string;
  remove(): void;
}

export function useBookFilters() {
  const route = useRoute();
  const { t } = useI18n();
  const { applyPanelFilters } = useBooksRouteQuery();

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
