<template>
  <div class="random-book panel">
    <div class="random-book-header">
      <h3 class="random-book-title">{{ t('dashboard.randomBook.title') }}</h3>
      <button
        type="button"
        class="button"
        :disabled="books.length <= 1"
        @click="shuffle"
      >
        {{ t('dashboard.randomBook.shuffle') }}
      </button>
    </div>

    <p v-if="!currentBook" class="random-book-empty">{{ t('dashboard.randomBook.empty') }}</p>
    <div v-else class="random-book-body">
      <BookCoverImg
        :book-id="currentBook.id"
        :cover-url="currentBook.cover_url"
        :alt="currentBook.title"
        class="random-book-cover"
      />
      <div class="random-book-info">
        <p class="random-book-book-title">{{ currentBook.title }}</p>
        <p class="random-book-authors">{{ (currentBook.authors ?? []).join(', ') }}</p>
        <div class="random-book-actions">
          <RouterLink class="button" :to="`/books/${currentBook.id}`">
            {{ t('dashboard.randomBook.viewDetail') }}
          </RouterLink>
          <RouterLink class="button primary" :to="`/reader/${currentBook.id}`">
            {{ t('dashboard.randomBook.readNow') }}
          </RouterLink>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import BookCoverImg from '@/components/BookCoverImg.vue';
import { useI18n } from '@/i18n';
import type { Book } from '@/types/book';

const props = withDefaults(
  defineProps<{
    books: Book[];
    /** Ids of books the reader has already opened; the pick prefers the rest. */
    startedIds?: Set<string>;
  }>(),
  { startedIds: () => new Set<string>() }
);

const { t } = useI18n();

const currentId = ref<string | null>(null);

// The books to draw from: those the reader has not opened yet, so the pick keeps
// surfacing something new. Once every book has been started the preference has
// nothing left to offer, so it falls back to the whole shelf rather than
// showing nothing.
function candidatePool(): Book[] {
  const unstarted = props.books.filter((book) => !props.startedIds.has(book.id));
  return unstarted.length > 0 ? unstarted : props.books;
}

function pickRandomId(exclude?: string): string | null {
  const pool = candidatePool();
  if (pool.length === 0) {
    return null;
  }
  const candidates = pool.length > 1 && exclude
    ? pool.filter((book) => book.id !== exclude)
    : pool;
  const from = candidates.length > 0 ? candidates : pool;
  const index = Math.floor(Math.random() * from.length);
  return from[index].id;
}

function shuffle(): void {
  currentId.value = pickRandomId(currentId.value ?? undefined);
}

// Repick when the current choice is gone, or when progress has since loaded and
// the current book turns out to be one the reader already started while an
// unstarted book is available. A choice that is still valid and unstarted is
// left alone so a background refresh does not reshuffle under the reader.
watch(
  [() => props.books, () => props.startedIds],
  () => {
    if (props.books.length === 0) {
      currentId.value = null;
      return;
    }
    const current = currentId.value;
    const valid = current !== null && props.books.some((book) => book.id === current);
    const currentStarted = current !== null && props.startedIds.has(current);
    const hasUnstarted = props.books.some((book) => !props.startedIds.has(book.id));
    if (!valid || (currentStarted && hasUnstarted)) {
      currentId.value = pickRandomId();
    }
  },
  { immediate: true }
);

const currentBook = computed<Book | null>(
  () => props.books.find((book) => book.id === currentId.value) ?? null
);
</script>

<style scoped>
.random-book {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px 18px;
}

.random-book-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.random-book-title {
  color: var(--text);
  font-size: 14px;
  font-weight: 700;
  margin: 0;
}

.random-book-empty {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
}

.random-book-body {
  display: flex;
  gap: 14px;
}

.random-book-cover {
  border-radius: 6px;
  background: #f2f2f2;
  flex: 0 0 auto;
  height: 140px;
  object-fit: cover;
  width: 96px;
}

.random-book-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.random-book-book-title {
  color: var(--text);
  font-size: 16px;
  font-weight: 700;
  margin: 0;
}

.random-book-authors {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
}

.random-book-actions {
  display: flex;
  gap: 8px;
  margin-top: 4px;
}

.random-book-actions .button {
  font-size: 13px;
  padding: 6px 10px;
}
</style>
