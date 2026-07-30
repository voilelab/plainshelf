<template>
  <div class="book-list-view">
    <article
      v-for="book in books"
      :key="book.id"
      class="book-list-row panel"
      @click="emit('select', book.id)"
    >
      <BookCoverImg :book-id="book.id" :cover-url="book.cover_url" :alt="book.title" class="book-list-cover" />

      <div class="book-list-main">
        <div class="book-list-head">
          <h3 class="book-list-title">{{ book.title }}</h3>
          <div class="book-list-head-actions">
            <p class="book-list-layer">{{ layerLabel(book) }}</p>
            <button
              v-if="showEditAction"
              type="button"
              class="book-list-edit"
              @click.stop="emit('edit', book.id)"
            >
              Edit
            </button>
          </div>
        </div>

        <p v-if="book.comment?.trim()" class="book-list-comment">{{ book.comment }}</p>

        <p class="book-list-meta">
          <span v-if="book.authors?.length">{{ book.authors.join(', ') }}</span>
          <span v-if="book.language">{{ book.language.toUpperCase() }}</span>
          <span>{{ primaryDateLabel(book) }}</span>
        </p>
      </div>
    </article>
  </div>
</template>

<script setup lang="ts">
import BookCoverImg from './BookCoverImg.vue';
import type { Book } from '@/types/book';
import { getLayerPath, layerPathLabel } from '@/utils/layers';
import { formatDateLabel } from '@/utils/date';

const props = withDefaults(defineProps<{
  books: Book[];
  showEditAction?: boolean;
}>(), {
  showEditAction: false
});

const emit = defineEmits<{
  (event: 'select', id: string): void;
  (event: 'edit', id: string): void;
}>();

function layerLabel(book: Book): string {
  const path = getLayerPath(book);
  return path === '' ? '/' : layerPathLabel(path);
}

function primaryDateLabel(book: Book): string {
  const rawValue = book.updated_at || book.published_at || book.created_at;
  return rawValue ? formatDateLabel(rawValue) : 'No date';
}
</script>

<style scoped>
.book-list-view {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.book-list-row {
  align-items: center;
  cursor: pointer;
  display: grid;
  gap: 14px;
  grid-template-columns: 54px minmax(0, 1fr);
  padding: 10px 12px;
  transition: border-color 120ms ease, background 120ms ease, transform 120ms ease;
}

.book-list-row:hover {
  background: #fbfdff;
  border-color: #cdd8e6;
  transform: translateY(-1px);
}

.book-list-cover {
  width: 54px;
  height: 78px;
  border: 1px solid var(--border);
  border-radius: 10px;
  object-fit: cover;
  background: #edf2f7;
}

.book-list-main {
  min-width: 0;
}

.book-list-head {
  align-items: start;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.book-list-head-actions {
  align-items: center;
  display: inline-flex;
  gap: 8px;
}

.book-list-title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.35;
}

.book-list-layer {
  color: var(--muted);
  flex: 0 0 auto;
  font-size: 12px;
  margin: 1px 0 0;
  text-align: right;
}

.book-list-edit {
  background: #f4f7fb;
  border: 1px solid #d5dfeb;
  border-radius: 8px;
  color: inherit;
  cursor: pointer;
  font-size: 12px;
  padding: 2px 8px;
}

.book-list-edit:hover {
  background: #e9f1fb;
}

.book-list-comment {
  color: var(--muted);
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.45;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.book-list-meta {
  color: var(--muted);
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 8px 0 0;
  font-size: 12px;
}

.book-list-meta span:not(:last-child)::after {
  content: '·';
  margin-left: 8px;
}

@media (max-width: 760px) {
  .book-list-row {
    grid-template-columns: 44px minmax(0, 1fr);
    padding: 10px;
  }

  .book-list-cover {
    width: 44px;
    height: 64px;
  }

  .book-list-head {
    flex-direction: column;
    gap: 4px;
  }

  .book-list-head-actions {
    width: 100%;
    justify-content: space-between;
  }

  .book-list-layer {
    text-align: left;
  }
}
</style>