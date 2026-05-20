<template>
  <div class="pagination">
    <label v-if="pageSizeOptions && pageSizeOptions.length > 0" class="page-size-label">
      每頁
      <select class="button page-size-select" :value="pageSize" @change="onPageSizeChange">
        <option v-for="opt in pageSizeOptions" :key="opt" :value="opt">{{ opt }} 本</option>
      </select>
    </label>
    <button class="button" :disabled="!hasPrevPage" @click="goTo(page - 1)">Prev</button>
    <span>Page {{ page }} / {{ totalPages }}</span>
    <button class="button" :disabled="!hasNextPage" @click="goTo(page + 1)">Next</button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

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
const hasPrevPage = computed(() => props.page > 1);
const hasNextPage = computed(() => props.page < totalPages.value);

function goTo(targetPage: number): void {
  if (targetPage < 1 || targetPage > totalPages.value) {
    return;
  }
  emit('update:page', targetPage);
}

function onPageSizeChange(event: Event): void {
  const select = event.target as HTMLSelectElement;
  emit('update:pageSize', Number(select.value));
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
