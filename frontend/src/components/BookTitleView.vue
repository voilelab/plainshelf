<template>
  <div class="book-title-view panel">
    <div
      v-for="book in books"
      :key="book.id"
      class="book-title-row-container"
      :class="{ 'is-selected': selectedIds.has(book.id), 'is-dragging': interactions.draggingBookId.value === book.id }"
    >
      <BookSelectionCheckbox
        v-if="selectable"
        :selected="selectedIds.has(book.id)"
        :label="t('bookCollection.selection.selectBook', { title: book.title })"
        @toggle="emit('toggle-selection', book.id)"
      />
      <button
        type="button"
        class="book-title-row"
        :class="{ 'has-selection': selectable }"
        :draggable="selectable && !mobileSelection"
        :aria-pressed="selectable ? selectedIds.has(book.id) : undefined"
        @click="interactions.onClick($event, book.id)"
        @pointerdown="interactions.onPointerDown($event, book.id)"
        @pointermove="interactions.onPointerMove"
        @pointerup="interactions.cancelLongPress"
        @pointercancel="interactions.cancelLongPress"
        @dragstart="interactions.onDragStart($event, book)"
        @dragend="interactions.onDragEnd"
      >
        <span class="book-title-text">{{ book.title }}</span>
        <span class="book-title-meta">{{ compactMeta(book) }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Book } from '@/types/book';
import type { BookActivation } from '@/types/bookSelection';
import { useI18n } from '@/i18n';
import BookSelectionCheckbox from './BookSelectionCheckbox.vue';
import { useBookItemInteractions } from '@/composables/useBookItemInteractions';
import { getLayerPath, layerPathLabel } from '@/utils/layers';

const props = withDefaults(defineProps<{
  books: Book[];
  selectable?: boolean;
  mobileSelection?: boolean;
  selectedIds?: ReadonlySet<string>;
}>(), {
  selectable: false,
  mobileSelection: false,
  selectedIds: () => new Set<string>()
});

const emit = defineEmits<{
  (event: 'activate', payload: BookActivation): void;
  (event: 'toggle-selection', id: string): void;
  (event: 'long-press', id: string): void;
}>();

const { t } = useI18n();

const interactions = useBookItemInteractions({
  mobile: () => props.mobileSelection,
  selectedIds: () => props.selectedIds,
  onActivate: (payload) => emit('activate', payload),
  onLongPress: (id) => emit('long-press', id)
});

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

.book-title-row-container {
  border-bottom: 1px solid #edf2f7;
  position: relative;
}

.book-title-row {
  align-items: center;
  background: transparent;
  border: 0;
  color: inherit;
  cursor: pointer;
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) auto;
  padding: 12px 14px;
  text-align: left;
  width: 100%;
}

.book-title-row.has-selection {
  padding-left: 42px;
}

.book-title-row-container.is-selected .book-title-row {
  background: color-mix(in srgb, var(--accent) 8%, white);
}

.book-title-row-container.is-dragging { opacity: 0.4; }

.book-title-row-container:last-child {
  border-bottom: 0;
}

.book-title-row-container:hover .book-title-row {
  background: #fbfdff;
}

.book-title-row-container.is-selected:hover .book-title-row {
  background: color-mix(in srgb, var(--accent) 8%, white);
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
