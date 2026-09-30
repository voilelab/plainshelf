<template>
  <div class="book-list-view">
    <article
      v-for="book in books"
      :key="book.id"
      class="book-list-row panel"
      :class="{ 'is-selected': selectedIds.has(book.id), 'is-dragging': interactions.draggingBookId.value === book.id }"
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
      <BookCoverImg :book-id="book.id" :cover-url="book.cover_url" :alt="book.title" class="book-list-cover" />

      <div class="book-list-main">
        <div class="book-list-head">
          <h3 class="book-list-title">{{ book.title }}</h3>
          <div class="book-list-head-actions">
            <BookNsfwBadge :book="book" />
            <BookDownloadBadge :state="book.download_state" />
            <p class="book-list-folder">{{ folderLabel(book) }}</p>
          </div>
        </div>

        <p v-if="summaries.get(book.id)" class="book-list-comment">{{ summaries.get(book.id) }}</p>

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
import BookDownloadBadge from './BookDownloadBadge.vue';
import BookNsfwBadge from './BookNsfwBadge.vue';
import BookSelectionCheckbox from './BookSelectionCheckbox.vue';
import { useBookItemInteractions } from '@/composables/useBookItemInteractions';
import { useBookSummaries } from '@/composables/useBookSummaries';
import type { Book } from '@/types/book';
import type { BookActivation } from '@/types/bookSelection';
import { getFolderPath, folderPathLabel } from '@/utils/folders';
import { formatDateLabel } from '@/utils/date';
import { useI18n } from '@/i18n';

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

const summaries = useBookSummaries(() => props.books);

function folderLabel(book: Book): string {
  const path = getFolderPath(book);
  return path === '' ? '/' : folderPathLabel(path);
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
  position: relative;
  align-items: center;
  cursor: pointer;
  display: grid;
  gap: 14px;
  grid-template-columns: 54px minmax(0, 1fr);
  padding: 10px 12px;
  transition: border-color 120ms ease, background 120ms ease, transform 120ms ease;
}

.book-list-row.is-selected {
  background: color-mix(in srgb, var(--accent) 8%, white);
  border-color: var(--accent);
}

.book-list-row.is-dragging { opacity: 0.4; }

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
  /* Shrinkable, so the folder label below can give way instead of pushing the
     row wider than the card. */
  min-width: 0;
}

.book-list-title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.35;
}

.book-list-folder {
  color: var(--muted);
  /* The download badge is the fixed part of this row and a deep folder path is
     the elastic one: at the narrow breakpoint the two share one line, and an
     unshrinkable path pushed the whole row past the card's right edge. */
  flex: 0 1 auto;
  font-size: 12px;
  margin: 1px 0 0;
  min-width: 0;
  overflow: hidden;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
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

  .book-list-folder {
    text-align: left;
  }
}
</style>
