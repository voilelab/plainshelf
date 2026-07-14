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
import BookCoverImg from '../../../components/BookCoverImg.vue';
import { useI18n } from '../../../i18n';
import type { Book } from '../../../types/book';

const props = defineProps<{
  books: Book[];
}>();

const { t } = useI18n();

const currentId = ref<string | null>(null);

function pickRandomId(exclude?: string): string | null {
  if (props.books.length === 0) {
    return null;
  }
  const candidates = props.books.length > 1 && exclude
    ? props.books.filter((book) => book.id !== exclude)
    : props.books;
  const pool = candidates.length > 0 ? candidates : props.books;
  const index = Math.floor(Math.random() * pool.length);
  return pool[index].id;
}

function shuffle(): void {
  currentId.value = pickRandomId(currentId.value ?? undefined);
}

watch(
  () => props.books,
  (books) => {
    if (books.length === 0) {
      currentId.value = null;
      return;
    }
    if (!currentId.value || !books.some((book) => book.id === currentId.value)) {
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
