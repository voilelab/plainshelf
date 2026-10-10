<template>
  <div class="identifier-rows">
    <div v-for="(row, index) in rows" :key="index" class="identifier-row">
      <input
        v-model="row.key"
        class="input identifier-key"
        type="text"
        :placeholder="t('libraryForms.editBook.identifierKeyPlaceholder')"
        :aria-label="t('libraryForms.editBook.identifierKeyLabel', { index: index + 1 })"
      />
      <input
        v-model="row.value"
        class="input identifier-value"
        type="text"
        :placeholder="t('libraryForms.editBook.identifierValuePlaceholder')"
        :aria-label="t('libraryForms.editBook.identifierValueLabel', { index: index + 1 })"
      />
      <button
        class="identifier-remove"
        type="button"
        :aria-label="t('libraryForms.editBook.removeIdentifier', { name: row.key || index + 1 })"
        @click="removeRow(index)"
      >
        ×
      </button>
    </div>
  </div>
  <button class="button" type="button" @click="addRow">{{ t('libraryForms.editBook.addIdentifier') }}</button>
</template>

<script setup lang="ts">
import { useI18n } from '@/i18n';

export interface IdentifierRow {
  key: string;
  value: string;
}

// Edited in place, so several edits in one tick never read a stale copy.
const rows = defineModel<IdentifierRow[]>({ required: true });

const { t } = useI18n();

function addRow(): void {
  rows.value.push({ key: '', value: '' });
}

function removeRow(index: number): void {
  rows.value.splice(index, 1);
}
</script>

<style scoped>
.identifier-rows {
  display: grid;
  gap: 8px;
}

.identifier-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.identifier-key {
  flex: 1 1 160px;
  min-width: 0;
}

.identifier-value {
  flex: 2 1 240px;
  min-width: 0;
}

.identifier-remove {
  flex: 0 0 auto;
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  padding: 4px;
}

.identifier-remove:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

@media (max-width: 520px) {
  .identifier-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .identifier-key,
  .identifier-value {
    grid-column: 1;
  }

  .identifier-remove {
    grid-column: 2;
    grid-row: 1 / 3;
  }
}
</style>
