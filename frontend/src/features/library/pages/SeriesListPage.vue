<template>
  <div class="series-page">
    <div v-if="loading || (!loaded && !error && !shelfUnreachable)" class="loading">
      {{ shelfInitializing ? t('bookCollection.shelfInitializing') : t('bookCollection.loadingBooks') }}
    </div>
    <div v-else-if="shelfUnreachable || error" class="error" role="alert">
      <p>{{ shelfUnreachable ? t('bookCollection.shelfUnreachable') : error }}</p>
      <button type="button" class="button" @click="fetchBooks()">{{ t('common.retry') }}</button>
    </div>
    <template v-else>
      <header class="series-header">
        <h2 class="series-title">{{ t('series.title') }}</h2>
        <p v-if="series.length > 0" class="series-count">{{ t('series.count', { count: series.length }) }}</p>
      </header>

      <div v-if="series.length === 0" class="panel empty-state">
        <p>{{ t('series.empty') }}</p>
      </div>

      <ul v-else class="series-list">
        <li v-for="entry in series" :key="entry.name">
          <RouterLink class="series-row" :to="seriesRoute(entry.name)">
            <BookCoverImg
              class="series-cover"
              :book-id="entry.coverBook.id"
              :cover-url="entry.coverBook.cover_url"
              alt=""
            />
            <span class="series-name">{{ entry.name }}</span>
            <span class="series-books">{{ entry.count === 1 ? t('series.bookCountOne') : t('series.bookCountMany', { count: entry.count }) }}</span>
          </RouterLink>
        </li>
      </ul>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import BookCoverImg from '@/components/BookCoverImg.vue';
import { summarizeSeries } from '@/features/library/utils/series';
import { useBookStore } from '@/composables/useBookStore';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useI18n } from '@/i18n';

const { t } = useI18n();
const { books, loaded, loading, error, shelfInitializing, shelfUnreachable, fetchBooks } = useBookStore();

const series = computed(() => summarizeSeries(books.value));

useDocumentTitle(() => [t('series.title'), t('app.name')]);

function seriesRoute(name: string) {
  return { name: 'series-detail', params: { name } };
}
</script>

<style scoped>
.series-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.series-title {
  margin: 0;
}

.series-count {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.series-list {
  display: grid;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.series-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  color: var(--text);
  text-decoration: none;
}

.series-row:hover,
.series-row:focus-visible {
  border-color: var(--accent);
}

.series-cover {
  flex: 0 0 auto;
  width: 44px;
  height: 64px;
  border: 1px solid var(--border);
  border-radius: 8px;
  object-fit: cover;
  background: var(--surface-sunken);
}

.series-name {
  flex: 1 1 auto;
  min-width: 0;
  overflow-wrap: anywhere;
  font-weight: 600;
}

.series-books {
  flex: 0 0 auto;
  color: var(--muted);
  font-size: 13px;
}
</style>
