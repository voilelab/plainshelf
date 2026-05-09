<template>
  <div class="book-title-view panel">
    <button
      v-for="book in books"
      :key="book.id"
      type="button"
      class="book-title-row"
      @click="emit('select', book.id)"
    >
      <span class="book-title-text">{{ book.title }}</span>
      <span class="book-title-meta">{{ compactMeta(book) }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import type { Book } from '../types/book';
import { getLayerPath, layerPathLabel } from '../utils/layers';

defineProps<{
  books: Book[];
}>();

const emit = defineEmits<{
  (event: 'select', id: string): void;
}>();

function compactMeta(book: Book): string {
  const metaParts: string[] = [];
  if (book.authors?.length) {
    metaParts.push(book.authors[0]);
  }
  if (book.language) {
    metaParts.push(book.language.toUpperCase());
  }

  const path = getLayerPath(book);
  const layer = path === '' ? '/' : layerPathLabel(path);
  if (layer) {
    metaParts.push(layer);
  }

  return metaParts.join(' · ') || 'No metadata';
}
</script>

<style scoped>
.book-title-view {
  overflow: hidden;
}

.book-title-row {
  align-items: center;
  background: transparent;
  border: 0;
  border-bottom: 1px solid #edf2f7;
  color: inherit;
  cursor: pointer;
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) auto;
  padding: 12px 14px;
  text-align: left;
  width: 100%;
}

.book-title-row:last-child {
  border-bottom: 0;
}

.book-title-row:hover {
  background: #fbfdff;
}

.book-title-text {
  font-size: 14px;
  font-weight: 600;
  line-height: 1.4;
  min-width: 0;
}

.book-title-meta {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.3;
  text-align: right;
}

@media (max-width: 760px) {
  .book-title-row {
    gap: 4px;
    grid-template-columns: 1fr;
  }

  .book-title-meta {
    text-align: left;
  }
}
</style>