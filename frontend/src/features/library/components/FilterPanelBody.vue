<template>
  <div class="filter-panel-body">
    <section
      v-for="field in fields"
      :key="field.filter.key"
      class="filter-field"
    >
      <div class="filter-field-head">
        <h3 class="filter-field-label">{{ field.label }}</h3>
        <p class="filter-field-note">{{ field.emptyNote }}</p>
      </div>

      <!-- Numeric range (character count): its own control already carries the
           min/max, clear, unknown-count note and the refresh-stats progress —
           all of which now live here, inside the panel, so they can never resize
           the header. -->
      <CharCountFilterBar
        v-if="field.control === 'range'"
        :range="(field.value as CharCountRange)"
        :unknown-count="charCountUnknownCount"
        :read-only="readOnly"
        hide-label
        :refresh-running="charCountRefreshRunning"
        :refresh-label="charCountRefreshLabel"
        :refresh-outcome="charCountRefreshOutcome"
        :refresh-error="charCountRefreshError"
        @update:range="onRangeChange"
        @refresh-stats="emit('refresh-stats')"
      />

      <!-- Tri-state (cover): presence is all there is, so all / has / none. -->
      <div v-else-if="field.control === 'triState'" class="filter-tristate" role="radiogroup" :aria-label="field.label">
        <label
          v-for="option in TRISTATE_OPTIONS"
          :key="option.mode"
          class="filter-option filter-tristate-option"
        >
          <input
            type="radio"
            :name="`${field.filter.key}-tristate`"
            :checked="triStateMode(field.value) === option.mode"
            @change="setField(field.filter, triStateValue(option.mode))"
          />
          <span>{{ t(option.labelKey) }}</span>
        </label>
      </div>

      <!-- Facet (author, language, tags): the shelf's own values, searchable.
           Single-select is a radio group with (any)/(unset); multi-select is
           checkboxes with an exclusive (unset). -->
      <div v-else class="filter-facet">
        <input
          v-if="field.facet.length > FACET_SEARCH_THRESHOLD"
          class="toolbar-control toolbar-input filter-facet-search"
          type="search"
          :value="facetSearch[field.filter.key] ?? ''"
          :placeholder="t('library.filters.facetSearchPlaceholder')"
          :aria-label="t('library.filters.facetSearchLabel', { field: field.label })"
          @input="onFacetSearch(field.filter.key, $event)"
        />

        <div class="filter-facet-options">
          <template v-if="field.control === 'facetSingle'">
            <label class="filter-option">
              <input
                type="radio"
                :name="`${field.filter.key}-facet`"
                :checked="field.value === undefined"
                @change="setField(field.filter, undefined)"
              />
              <span>{{ t('library.filters.value.any') }}</span>
            </label>
            <label class="filter-option">
              <input
                type="radio"
                :name="`${field.filter.key}-facet`"
                :checked="isFieldKind(field.value, 'none')"
                @change="setField(field.filter, { kind: 'none' })"
              />
              <span>{{ t('library.filters.value.unset') }}</span>
            </label>
            <label
              v-for="entry in visibleFacet(field)"
              :key="entry.value"
              class="filter-option"
            >
              <input
                type="radio"
                :name="`${field.filter.key}-facet`"
                :checked="isFieldEq(field.value, entry.value)"
                @change="setField(field.filter, { kind: 'eq', value: entry.value })"
              />
              <span class="filter-option-value">{{ entry.value }}</span>
              <span class="filter-option-count">{{ entry.count }}</span>
            </label>
          </template>

          <template v-else>
            <label class="filter-option" :for="`${filterOptionId}-${field.filter}-unset`">
              <BaseCheckbox
                :id="`${filterOptionId}-${field.filter}-unset`"
                :model-value="isMultiUnset(field.value)"
                :aria-labelledby="`${filterOptionId}-${field.filter}-unset-label`"
                @update:model-value="toggleMultiUnset(field.filter, field.value)"
              />
              <span :id="`${filterOptionId}-${field.filter}-unset-label`">
                {{ t('library.filters.value.unset') }}
              </span>
            </label>
            <label
              v-for="entry in visibleFacet(field)"
              :key="entry.value"
              class="filter-option"
              :for="`${filterOptionId}-${field.filter}-${entry.value}`"
            >
              <BaseCheckbox
                :id="`${filterOptionId}-${field.filter}-${entry.value}`"
                :model-value="isMultiChecked(field.value, entry.value)"
                :aria-labelledby="`${filterOptionId}-${field.filter}-${entry.value}-label`"
                @update:model-value="toggleMultiValue(field.filter, field.value, entry.value)"
              />
              <!-- The count is deliberately outside the name: a filter called
                   "Tolkien 12" reads as a value nobody has. -->
              <span :id="`${filterOptionId}-${field.filter}-${entry.value}-label`" class="filter-option-value">
                {{ entry.value }}
              </span>
              <span class="filter-option-count">{{ entry.count }}</span>
            </label>
          </template>

          <p v-if="visibleFacet(field).length === 0" class="filter-facet-empty">
            {{ t('library.filters.facetEmpty') }}
          </p>
        </div>
      </div>
    </section>

    <footer class="filter-panel-footer">
      <button
        type="button"
        class="button filter-panel-clear"
        :disabled="!hasActive"
        @click="clearAll()"
      >{{ t('library.filters.clearAll') }}</button>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, useId } from 'vue';
import { useRoute } from 'vue-router';
import type { Book } from '@/types/book';
import BaseCheckbox from '@/components/BaseCheckbox.vue';
import CharCountFilterBar from './CharCountFilterBar.vue';
import { PANEL_BOOK_FILTERS, type AnyBookFilterDef, type MultiFieldValue } from '@/utils/bookFilters/registry';
import type { FilterFieldValue } from '@/utils/bookFilters/codec';
import type { CharCountRange } from '@/utils/charCountFilter';
import { facetEntries, fieldEmptyNote, fieldLabel, type FacetEntry } from '../utils/filterLabels';
import { useBookFilters } from '../composables/useBookFilters';
import { useBooksRouteQuery } from '../composables/useBooksRouteQuery';
import { useI18n } from '@/i18n';
import '@/styles/toolbar-controls.css';

const props = defineProps<{
  books: Book[];
  readOnly: boolean;
  charCountSupported: boolean;
  charCountUnknownCount: number;
  charCountRefreshRunning: boolean;
  charCountRefreshLabel: string;
  charCountRefreshOutcome: string;
  charCountRefreshError: string;
}>();

const emit = defineEmits<{
  (event: 'refresh-stats'): void;
}>();

const { t } = useI18n();
const route = useRoute();
// One prefix per panel instance keeps each facet option's id unique across
// filters and across a second panel on the page.
const filterOptionId = `filter-option-${useId()}`;
const { applyPanelFilters } = useBooksRouteQuery();
const { hasActive, clearAll } = useBookFilters();

// Above this many distinct values a field grows a search box; below it the list
// is short enough to scan.
const FACET_SEARCH_THRESHOLD = 8;

const TRISTATE_OPTIONS = [
  { mode: 'all', labelKey: 'library.filters.value.any' },
  { mode: 'has', labelKey: 'library.filters.value.has' },
  { mode: 'none', labelKey: 'library.filters.value.unset' }
] as const;

type TriStateMode = (typeof TRISTATE_OPTIONS)[number]['mode'];

interface PanelField {
  filter: AnyBookFilterDef;
  label: string;
  emptyNote: string;
  control: NonNullable<AnyBookFilterDef['panelControl']>;
  value: unknown;
  facet: FacetEntry[];
}

const facetSearch = reactive<Record<string, string>>({});

const fields = computed<PanelField[]>(() =>
  PANEL_BOOK_FILTERS.filter(
    (filter) => filter.key !== 'charCount' || props.charCountSupported
  ).map((filter) => {
    const control = filter.panelControl!;
    const usesFacet = control === 'facetSingle' || control === 'facetMulti';
    return {
      filter,
      label: fieldLabel(filter, t),
      emptyNote: fieldEmptyNote(filter, t),
      control,
      value: filter.parse(route.query),
      facet: usesFacet ? facetEntries(props.books, filter) : []
    };
  })
);

function visibleFacet(field: PanelField): FacetEntry[] {
  const term = (facetSearch[field.filter.key] ?? '').trim().toLowerCase();
  if (!term) {
    return field.facet;
  }
  return field.facet.filter((entry) => entry.value.toLowerCase().includes(term));
}

function onFacetSearch(key: string, event: Event): void {
  facetSearch[key] = (event.target as HTMLInputElement).value;
}

function setField(filter: AnyBookFilterDef, value: unknown): void {
  void applyPanelFilters({ [filter.key]: value });
}

function onRangeChange(range: CharCountRange): void {
  const current = charCountFilter().parse(route.query) as CharCountRange;
  if (range.min === current.min && range.max === current.max) {
    return;
  }
  void applyPanelFilters({ charCount: range });
}

function charCountFilter(): AnyBookFilterDef {
  return PANEL_BOOK_FILTERS.find((filter) => filter.key === 'charCount')!;
}

// ── tri-state (cover) ────────────────────────────────────────────────────────
function triStateMode(value: unknown): TriStateMode {
  const field = value as FilterFieldValue | undefined;
  if (field === undefined) {
    return 'all';
  }
  return field.kind === 'has' ? 'has' : 'none';
}

function triStateValue(mode: TriStateMode): FilterFieldValue | undefined {
  if (mode === 'all') {
    return undefined;
  }
  return mode === 'has' ? { kind: 'has' } : { kind: 'none' };
}

// ── single facet (author, language) ──────────────────────────────────────────
function isFieldKind(value: unknown, kind: FilterFieldValue['kind']): boolean {
  return (value as FilterFieldValue | undefined)?.kind === kind;
}

function isFieldEq(value: unknown, eqValue: string): boolean {
  const field = value as FilterFieldValue | undefined;
  return field?.kind === 'eq' && field.value === eqValue;
}

// ── multi facet (tags) ───────────────────────────────────────────────────────
function multiOf(value: unknown): MultiFieldValue {
  return value as MultiFieldValue;
}

function isMultiUnset(value: unknown): boolean {
  return multiOf(value).values.some((entry) => entry.kind === 'none');
}

function isMultiChecked(value: unknown, eqValue: string): boolean {
  return multiOf(value).values.some((entry) => entry.kind === 'eq' && entry.value === eqValue);
}

function toggleMultiUnset(filter: AnyBookFilterDef, value: unknown): void {
  // "(unset)" is exclusive with picking specific values: a book cannot both have
  // no tags and carry tag X, so choosing it drops any eq selections.
  const next: MultiFieldValue = isMultiUnset(value)
    ? { values: [], op: 'all' }
    : { values: [{ kind: 'none' }], op: 'all' };
  void applyPanelFilters({ [filter.key]: next });
}

function toggleMultiValue(filter: AnyBookFilterDef, value: unknown, eqValue: string): void {
  const eqs = multiOf(value).values.filter(
    (entry): entry is Extract<FilterFieldValue, { kind: 'eq' }> => entry.kind === 'eq'
  );
  const without = eqs.filter((entry) => entry.value !== eqValue);
  const next: MultiFieldValue =
    without.length === eqs.length
      ? { values: [...eqs, { kind: 'eq', value: eqValue }], op: 'all' }
      : { values: without, op: 'all' };
  void applyPanelFilters({ [filter.key]: next });
}
</script>

<style scoped>
.filter-panel-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.filter-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.filter-field-head {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.filter-field-label {
  font-size: 13px;
  font-weight: 600;
  margin: 0;
}

.filter-field-note {
  color: var(--muted, #888);
  font-size: 11px;
  margin: 0;
}

.filter-tristate {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.filter-facet-search {
  width: 100%;
}

.filter-facet-options {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 200px;
  overflow-y: auto;
}

.filter-option {
  align-items: center;
  cursor: pointer;
  display: flex;
  font-size: 13px;
  gap: 8px;
}

.filter-tristate-option {
  gap: 6px;
}

.filter-option-value {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.filter-option-count {
  color: var(--muted, #888);
  font-size: 11px;
  flex: 0 0 auto;
}

.filter-facet-empty {
  color: var(--muted, #888);
  font-size: 12px;
  margin: 0;
}

.filter-panel-footer {
  border-top: 1px solid var(--border, #e6ecf3);
  display: flex;
  justify-content: flex-end;
  padding-top: 12px;
}

.filter-panel-clear:disabled {
  cursor: default;
  opacity: 0.5;
}
</style>
