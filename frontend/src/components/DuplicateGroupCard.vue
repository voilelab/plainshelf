<template>
  <article class="duplicate-group-section">
    <header class="duplicate-group-header">
      <h3 class="duplicate-group-title">Duplicate Group #{{ groupIndex }}</h3>
      <p class="duplicate-group-subtitle">{{ books.length }} books share identical content</p>
    </header>

    <div class="duplicate-group-list">
      <DuplicateBookRow
        v-for="book in books"
        :key="book.id"
        :book="book"
      />
    </div>
  </article>
</template>

<script setup lang="ts">
import type { Book } from '../types/book';
import DuplicateBookRow from './DuplicateBookRow.vue';

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
