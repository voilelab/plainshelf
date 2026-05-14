<template>
  <div class="book-card-grid">
    <article
      v-for="book in books"
      :key="book.id"
      class="book-card-view panel"
      :class="{ 'is-dragging': draggingBookId === book.id }"
      draggable="true"
      @click="emit('select', book.id)"
      @dragstart="onDragStart($event, book)"
      @dragend="onDragEnd"
    >
      <img :src="coverSrc(book)" :alt="book.title" class="book-card-cover" @error="onCoverError(book.id)" />

      <div class="book-card-body">
        <p class="book-card-layer">{{ layerLabel(book) }}</p>
        <h3 class="book-card-title">{{ book.title }}</h3>
        <p class="book-card-summary">{{ summaryText(book) }}</p>
        <p class="book-card-meta">
          <span v-if="book.authors?.length">{{ book.authors[0] }}</span>
          <span v-if="book.language">{{ book.language.toUpperCase() }}</span>
          <span>{{ primaryDateLabel(book) }}</span>
        </p>
        <div v-if="showEditAction" class="book-card-actions">
          <button
            type="button"
            class="book-card-edit"
            @click.stop="emit('edit', book.id)"
          >
            Edit
          </button>
        </div>
      </div>
    </article>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue';
import bookcover from '../assets/bookcover.svg';
import type { Book } from '../types/book';
import { getLayerPath, layerPathLabel } from '../utils/layers';

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

const brokenCoverIds = ref<Record<string, boolean>>({});
const draggingBookId = ref<string | null>(null);
const dragPreviewEl = ref<HTMLElement | null>(null);

function coverSrc(book: Book): string {
  if (brokenCoverIds.value[book.id]) {
    return bookcover;
  }
  return book.cover_url || bookcover;
}

function onCoverError(bookId: string): void {
  brokenCoverIds.value = {
    ...brokenCoverIds.value,
    [bookId]: true
  };
}

function layerLabel(book: Book): string {
  const path = getLayerPath(book);
  return path === '' ? '/' : layerPathLabel(path);
}

function summaryText(book: Book): string {
  if (book.comment?.trim()) {
    return book.comment;
  }
  if (book.authors?.length) {
    return book.authors.join(', ');
  }
  return 'No summary';
}

function primaryDateLabel(book: Book): string {
  const rawValue = book.updated_at || book.published_at || book.created_at;
  if (!rawValue) {
    return 'No date';
  }

  const date = new Date(rawValue);
  if (Number.isNaN(date.getTime())) {
    return rawValue;
  }

  return date.toLocaleDateString();
}

function createDragPreview(book: Book): HTMLElement {
  const el = document.createElement('div');
  const coverEl = document.createElement('img');
  const titleEl = document.createElement('div');
  const authorEl = document.createElement('div');

  el.className = 'book-drag-preview';

  coverEl.src = book.cover_url || bookcover;
  coverEl.alt = '';
  coverEl.className = 'book-drag-preview-cover';

  titleEl.textContent = book.title;
  titleEl.className = 'book-drag-preview-title';

  authorEl.textContent = (book.authors ?? []).join(', ');
  authorEl.className = 'book-drag-preview-author';

  el.append(coverEl, titleEl, authorEl);
  return el;
}

function cleanupDragPreview(): void {
  dragPreviewEl.value?.remove();
  dragPreviewEl.value = null;
}

function onDragStart(event: DragEvent, book: Book): void {
  draggingBookId.value = book.id;
  cleanupDragPreview();

  event.dataTransfer?.setData('application/x-txtlib-book-id', book.id);
  event.dataTransfer?.setData('text/plain', book.id);

  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
    const preview = createDragPreview(book);
    document.body.appendChild(preview);
    dragPreviewEl.value = preview;
    event.dataTransfer.setDragImage(preview, 60, 80);
  }
}

function onDragEnd(): void {
  draggingBookId.value = null;
  cleanupDragPreview();
}

onBeforeUnmount(() => {
  cleanupDragPreview();
});
</script>

<style scoped>
.book-card-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
}

.book-card-view {
  cursor: pointer;
  display: grid;
  grid-template-rows: 220px minmax(0, 1fr);
  min-height: 100%;
  overflow: hidden;
  transition: border-color 120ms ease, box-shadow 120ms ease, transform 120ms ease;
}

.book-card-view:hover {
  border-color: #cdd8e6;
  box-shadow: 0 10px 26px rgba(15, 23, 42, 0.08);
  transform: translateY(-2px);
}

.book-card-view.is-dragging {
  opacity: 0.35;
  transform: scale(0.92);
  transform-origin: center;
  transition: opacity 120ms ease, transform 120ms ease;
}

.book-card-cover {
  width: 100%;
  height: 220px;
  object-fit: cover;
  background: #eef3f8;
}

.book-card-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
}

.book-card-layer {
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  margin: 0;
  text-transform: uppercase;
}

.book-card-title {
  display: -webkit-box;
  font-size: 15px;
  line-height: 1.35;
  margin: 0;
  min-height: 40px;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
}

.book-card-summary {
  color: var(--muted);
  display: -webkit-box;
  flex: 1 1 auto;
  font-size: 13px;
  line-height: 1.45;
  margin: 0;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  line-clamp: 3;
}

.book-card-meta {
  color: var(--muted);
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 0;
  font-size: 12px;
}

.book-card-meta span:not(:last-child)::after {
  content: '·';
  margin-left: 8px;
}

.book-card-actions {
  display: flex;
  justify-content: flex-end;
}

.book-card-edit {
  background: #f4f7fb;
  border: 1px solid #d5dfeb;
  border-radius: 8px;
  color: inherit;
  cursor: pointer;
  font-size: 12px;
  padding: 4px 10px;
}

.book-card-edit:hover {
  background: #e9f1fb;
}

@media (max-width: 760px) {
  .book-card-grid {
    grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  }

  .book-card-view {
    grid-template-rows: 180px minmax(0, 1fr);
  }

  .book-card-cover {
    height: 180px;
  }
}

@media (max-width: 560px) {
  .book-card-grid {
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  }
}

:global(.book-drag-preview) {
  position: fixed;
  top: -1000px;
  left: -1000px;
  width: 120px;
  height: 160px;
  padding: 8px;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.18);
  overflow: hidden;
  pointer-events: none;
  display: flex;
  flex-direction: column;
  gap: 6px;
  box-sizing: border-box;
}

:global(.book-drag-preview-cover) {
  width: 100%;
  height: 96px;
  object-fit: cover;
  border-radius: 8px;
  background: #f2f2f2;
}

:global(.book-drag-preview-title) {
  color: #0f172a;
  display: -webkit-box;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.2;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
}

:global(.book-drag-preview-author) {
  color: #475569;
  font-size: 11px;
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>