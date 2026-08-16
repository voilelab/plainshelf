<template>
  <article class="duplicate-group-section">
    <header class="duplicate-group-header">
      <h3 class="duplicate-group-title">{{ t('maintenance.duplicates.groupTitle', { index: groupIndex }) }}</h3>
      <p class="duplicate-group-subtitle">{{ t('maintenance.duplicates.groupSummary', { count: books.length }) }}</p>
    </header>

    <div class="duplicate-group-list">
      <DuplicateBookRow
        v-for="book in books"
        :key="book.id"
        :book="book"
        @deleted="(bookId) => emit('deleted', bookId)"
      />
    </div>
  </article>
</template>

<script setup lang="ts">
import type { Book } from '@/types/book';
import DuplicateBookRow from './DuplicateBookRow.vue';
import { useI18n } from '@/i18n';

const { t } = useI18n();

const emit = defineEmits<{
  deleted: [bookId: string];
}>();

defineProps<{
  groupIndex: number;
  books: Book[];
}>();
</script>

<style scoped>
.duplicate-group-section {
  border-top: 1px solid #e6ecf3;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 14px;
}

.duplicate-group-header {
  border-bottom: 1px solid #e6ecf3;
  padding-bottom: 10px;
}

.duplicate-group-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.duplicate-group-subtitle {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.duplicate-group-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
</style>
