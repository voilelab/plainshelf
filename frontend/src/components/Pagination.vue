<template>
  <PaginationRoot
    class="pagination"
    :total="total"
    :page="page"
    :items-per-page="pageSize"
    @update:page="onPageChange"
  >
    <SelectRoot
      v-if="pageSizeOptions && pageSizeOptions.length > 0"
      :model-value="pageSize"
      @update:model-value="onPageSizeSelect"
    >
      <span class="page-size-label">
        {{ t('pagination.perPage') }}
        <SelectTrigger class="button page-size-select">
          <SelectValue />
        </SelectTrigger>
      </span>
      <SelectPortal>
        <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
          <SelectViewport>
            <SelectItem
              v-for="opt in pageSizeOptions"
              :key="opt"
              class="reka-menu-item"
              :value="opt"
            >
              <SelectItemText>{{ opt }}{{ t('pagination.booksSuffix') }}</SelectItemText>
            </SelectItem>
          </SelectViewport>
        </SelectContent>
      </SelectPortal>
    </SelectRoot>

    <PaginationList v-slot="{ items }" class="pagination-list">
      <PaginationFirst class="button page-button page-edge-button" :aria-label="t('pagination.firstPage')">
        «
      </PaginationFirst>
      <PaginationPrev class="button page-button page-edge-button">{{ t('common.prev') }}</PaginationPrev>

      <template v-for="(item, index) in items" :key="`${item.type}-${index}`">
        <PaginationListItem
          v-if="item.type === 'page'"
          class="button page-button"
          :value="item.value"
        >
          {{ item.value }}
        </PaginationListItem>
        <PaginationEllipsis v-else class="page-ellipsis" />
      </template>

      <PaginationNext class="button page-button page-edge-button">{{ t('common.next') }}</PaginationNext>
      <PaginationLast class="button page-button page-edge-button" :aria-label="t('pagination.lastPage')">
        »
      </PaginationLast>
    </PaginationList>

    <span class="page-summary">{{ t('common.page', { page, total: totalPages }) }}</span>
  </PaginationRoot>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
  PaginationEllipsis,
  PaginationFirst,
  PaginationLast,
  PaginationList,
  PaginationListItem,
  PaginationNext,
  PaginationPrev,
  PaginationRoot,
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
  type AcceptableValue
} from 'reka-ui';
import { useI18n } from '@/i18n';

const props = defineProps<{
  page: number;
  total: number;
  pageSize: number;
  pageSizeOptions?: number[];
}>();

const emit = defineEmits<{
  (event: 'update:page', value: number): void;
  (event: 'update:pageSize', value: number): void;
}>();

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)));
const { t } = useI18n();

function onPageChange(value: number): void {
  emit('update:page', value);
}

function onPageSizeSelect(value: AcceptableValue): void {
  if (typeof value !== 'number') {
    return;
  }
  emit('update:pageSize', value);
}
</script>

<style scoped>
.pagination {
  display: flex;
  align-items: center;
  gap: 10px;
  justify-content: flex-end;
  margin-top: 16px;
}

.pagination-list {
  align-items: center;
  display: flex;
  gap: 6px;
  list-style: none;
  margin: 0;
  padding: 0;
}

.page-button {
  min-width: 34px;
  padding: 6px 10px;
}

.page-button[data-selected] {
  background: var(--accent);
  border-color: var(--accent);
  color: #ffffff;
}

.page-ellipsis {
  color: var(--muted);
  min-width: 24px;
  text-align: center;
}

.page-summary {
  color: var(--muted);
  font-size: 13px;
}

.page-edge-button {
  white-space: nowrap;
}

.page-size-label {
  align-items: center;
  color: var(--muted);
  display: flex;
  font-size: 13px;
  gap: 6px;
  margin-right: auto;
}

.page-size-select {
  padding: 6px 10px;
}
</style>
