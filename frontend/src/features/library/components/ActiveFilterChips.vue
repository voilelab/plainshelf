<template>
  <div v-if="chips.length > 0" class="active-filter-chips" role="group" :aria-label="t('library.filters.chipsLabel')">
    <span
      v-for="chip in chips"
      :key="chip.key"
      class="filter-chip"
    >
      <span class="filter-chip-label">{{ chip.label }}</span>
      <button
        type="button"
        class="filter-chip-remove"
        :aria-label="t('library.filters.removeChip', { filter: chip.label })"
        @click="chip.remove()"
      >✕</button>
    </span>
    <button
      type="button"
      class="filter-chip-clear-all"
      @click="clearAll()"
    >{{ t('library.filters.clearAll') }}</button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from '@/i18n';
import { useBookFilters } from '../composables/useBookFilters';

const { t } = useI18n();
const { chips, clearAll } = useBookFilters();
</script>

<style scoped>
.active-filter-chips {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.filter-chip {
  align-items: center;
  background: #f4f7fb;
  border: 1px solid var(--border, #d0d7e0);
  border-radius: 999px;
  color: var(--text, #333);
  display: inline-flex;
  font-size: 12px;
  gap: 6px;
  padding: 3px 6px 3px 10px;
}

.filter-chip-label {
  white-space: nowrap;
}

.filter-chip-remove {
  align-items: center;
  background: transparent;
  border: 0;
  border-radius: 999px;
  color: var(--muted, #888);
  cursor: pointer;
  display: inline-flex;
  font-size: 11px;
  height: 18px;
  justify-content: center;
  line-height: 1;
  padding: 0;
  width: 18px;
}

.filter-chip-remove:hover {
  background: color-mix(in srgb, var(--text, #333) 12%, transparent);
  color: var(--text, #333);
}

.filter-chip-clear-all {
  background: transparent;
  border: 0;
  color: var(--accent, #3b82f6);
  cursor: pointer;
  font-size: 12px;
  padding: 3px 6px;
}

.filter-chip-clear-all:hover {
  text-decoration: underline;
}
</style>
