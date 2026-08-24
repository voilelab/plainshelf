<template>
  <section v-if="books.length > 0" class="recently-added panel">
    <div class="recently-added-header">
      <h3 class="recently-added-title">{{ t('dashboard.recentlyAdded.title') }}</h3>
      <RouterLink class="recently-added-all" :to="viewAllTo">
        {{ t('dashboard.recentlyAdded.viewAll') }}
      </RouterLink>
    </div>

    <ul class="recently-added-list">
      <li v-for="book in books" :key="book.id" class="recently-added-item">
        <RouterLink class="recently-added-card" :to="`/books/${book.id}`">
          <BookCoverImg
            :book-id="book.id"
            :cover-url="book.cover_url"
            :alt="book.title"
            class="recently-added-cover"
          />
          <p class="recently-added-book-title" :title="book.title">{{ book.title }}</p>
        </RouterLink>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router';
import BookCoverImg from '@/components/BookCoverImg.vue';
import { useI18n } from '@/i18n';
import type { Book } from '@/types/book';

defineProps<{
  books: Book[];
}>();

const { t } = useI18n();

// "View all" lands on the library sorted by newest first — the same ordering the
// row shows — rather than a dedicated route. sort/order are the library's own
// query keys (useBooksRouteQuery), so no filter is applied.
const viewAllTo: RouteLocationRaw = {
  path: '/books',
  query: { sort: 'created_at', order: 'desc' }
};
</script>

<style scoped>
.recently-added {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px 18px;
}

.recently-added-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.recently-added-title {
  color: var(--text);
  font-size: 14px;
  font-weight: 700;
  margin: 0;
}

.recently-added-all {
  color: var(--muted);
  font-size: 13px;
}

.recently-added-list {
  display: grid;
  gap: 14px;
  grid-auto-columns: minmax(120px, 1fr);
  grid-auto-flow: column;
  list-style: none;
  margin: 0;
  overflow-x: auto;
  padding: 0 0 4px;
}

.recently-added-item {
  min-width: 0;
}

.recently-added-card {
  color: inherit;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  text-decoration: none;
}

.recently-added-cover {
  aspect-ratio: 2 / 3;
  background: #f2f2f2;
  border-radius: 6px;
  object-fit: cover;
  width: 100%;
}

.recently-added-book-title {
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
