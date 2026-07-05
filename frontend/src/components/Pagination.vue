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

    <PaginationPrev class="button">{{ t('common.prev') }}</PaginationPrev>
    <span>{{ t('common.page', { page, total: totalPages }) }}</span>
    <PaginationNext class="button">{{ t('common.next') }}</PaginationNext>
  </PaginationRoot>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
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
import { useI18n } from '../i18n';

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
