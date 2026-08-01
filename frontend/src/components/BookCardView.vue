<template>
  <div class="book-card-grid">
    <ContextMenuRoot v-for="book in books" :key="book.id">
      <ContextMenuTrigger as-child>
        <article
          class="book-card-view panel"
          :class="{ 'is-dragging': interactions.draggingBookId.value === book.id, 'is-selected': selectedIds.has(book.id) }"
          :draggable="selectable && !mobileSelection"
          :aria-selected="selectedIds.has(book.id)"
          @click="interactions.onClick($event, book.id)"
          @pointerdown="interactions.onPointerDown($event, book.id)"
          @pointermove="interactions.onPointerMove"
          @pointerup="interactions.cancelLongPress"
          @pointercancel="interactions.cancelLongPress"
          @dragstart="interactions.onDragStart($event, book)"
          @dragend="interactions.onDragEnd"
        >
          <BookSelectionCheckbox
            v-if="selectable"
            :selected="selectedIds.has(book.id)"
            :label="t('bookCollection.selection.selectBook', { title: book.title })"
            @toggle="emit('toggle-selection', book.id)"
          />
          <BookCoverImg :book-id="book.id" :cover-url="book.cover_url" :alt="book.title" class="book-card-cover" />

          <div class="book-card-body">
            <p class="book-card-layer">{{ layerLabel(book) }}</p>
            <h3 class="book-card-title">{{ book.title }}</h3>
            <p class="book-card-summary">{{ summaryText(book) }}</p>
            <p class="book-card-meta">
              <span v-if="book.authors?.length">{{ book.authors[0] }}</span>
              <span v-if="book.language">{{ book.language.toUpperCase() }}</span>
              <span>{{ primaryDateLabel(book) }}</span>
            </p>
          </div>
        </article>
      </ContextMenuTrigger>
      <ContextMenuPortal>
        <ContextMenuContent class="reka-menu">
          <template v-if="selectedIds.has(book.id) && selectedIds.size > 1">
            <ContextMenuItem class="reka-menu-item" @select="emit('batch-move')">
              {{ t('bookCollection.selection.move') }}
            </ContextMenuItem>
            <ContextMenuItem class="reka-menu-item danger" @select="emit('batch-delete')">
              {{ t('bookCollection.selection.trash') }}
            </ContextMenuItem>
          </template>
          <template v-else>
            <ContextMenuItem class="reka-menu-item" @select="emit('read', book.id)">
              {{ t('bookCollection.contextMenu.read') }}
            </ContextMenuItem>
            <ContextMenuItem class="reka-menu-item" @select="emit('activate', { id: book.id, metaKey: false, ctrlKey: false, shiftKey: false })">
              {{ t('bookCollection.contextMenu.openDetail') }}
            </ContextMenuItem>
            <ContextMenuItem
              v-if="canOpenBookFolder"
              class="reka-menu-item"
              @select="emit('open-book-folder', book.id)"
            >
              {{ t('bookCollection.contextMenu.openBookFolder') }}
            </ContextMenuItem>
            <ContextMenuItem class="reka-menu-item" @select="emit('download', book.id)">
              {{ t('bookCollection.contextMenu.download') }}
            </ContextMenuItem>
            <ContextMenuItem
              v-if="!readOnly"
              class="reka-menu-item"
              @select="emit('edit', book.id)"
            >
              {{ t('bookCollection.contextMenu.edit') }}
            </ContextMenuItem>
            <ContextMenuItem
              v-if="!readOnly"
              class="reka-menu-item danger"
              @select="emit('delete', book.id)"
            >
              {{ t('bookCollection.contextMenu.delete') }}
            </ContextMenuItem>
          </template>
        </ContextMenuContent>
      </ContextMenuPortal>
    </ContextMenuRoot>
  </div>
</template>

<script setup lang="ts">
import {
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuPortal,
  ContextMenuRoot,
  ContextMenuTrigger
} from 'reka-ui';
import BookCoverImg from './BookCoverImg.vue';
import BookSelectionCheckbox from './BookSelectionCheckbox.vue';
import { useBookItemInteractions } from '@/composables/useBookItemInteractions';
import type { Book } from '@/types/book';
import type { BookActivation } from '@/types/bookSelection';
import { getLayerPath, layerPathLabel } from '@/utils/layers';
import { formatDateLabel } from '@/utils/date';
import { useI18n } from '@/i18n';

const props = withDefaults(defineProps<{
  books: Book[];
  canOpenBookFolder?: boolean;
  readOnly?: boolean;
  selectable?: boolean;
  mobileSelection?: boolean;
  selectedIds?: ReadonlySet<string>;
}>(), {
  canOpenBookFolder: false,
  readOnly: false,
  selectable: false,
  mobileSelection: false,
  selectedIds: () => new Set<string>()
});

const emit = defineEmits<{
  (event: 'activate', payload: BookActivation): void;
  (event: 'toggle-selection', id: string): void;
  (event: 'long-press', id: string): void;
  (event: 'batch-move'): void;
  (event: 'batch-delete'): void;
  (event: 'edit', id: string): void;
  (event: 'read', id: string): void;
  (event: 'open-book-folder', id: string): void;
  (event: 'download', id: string): void;
  (event: 'delete', id: string): void;
}>();

const { t } = useI18n();
const interactions = useBookItemInteractions({
  mobile: () => props.mobileSelection,
  selectedIds: () => props.selectedIds,
  onActivate: (payload) => emit('activate', payload),
  onLongPress: (id) => emit('long-press', id)
});

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
  return rawValue ? formatDateLabel(rawValue) : 'No date';
}

</script>

<style scoped>
.book-card-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
}

.book-card-view {
  position: relative;
  cursor: pointer;
  display: grid;
  grid-template-rows: 220px minmax(0, 1fr);
  min-height: 100%;
  overflow: hidden;
  transition: border-color 120ms ease, box-shadow 120ms ease, transform 120ms ease;
}

.book-card-view.is-selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 25%, transparent);
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

</style>
