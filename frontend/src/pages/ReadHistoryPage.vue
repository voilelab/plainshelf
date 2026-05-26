<template>
  <BookCollectionPage
    :title="t('readHistory.title')"
    :books="visibleBooks"
    :loading="loading"
    :error="error"
    :page="page"
    :page-size="pageSize"
    :total="books.length"
    :count="books.length"
    :empty-message="t('readHistory.empty')"
    :page-size-options="PAGE_SIZE_OPTIONS"
    view-mode-storage-key="read-history"
    @retry="loadReadHistory"
    @select="openBook"
    @update:page="onPageChange"
    @update:page-size="onPageSizeChange"
  >
    <template #toolbar>
      <button
        class="button clear-history-button"
        type="button"
        :disabled="books.length === 0 || loading || clearing"
        @click="onClearHistory"
      >
        {{ clearing ? t('readHistory.clearing') : t('readHistory.clear') }}
      </button>
    </template>
  </BookCollectionPage>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import BookCollectionPage from '../components/BookCollectionPage.vue';
import { clearReadHistory, listReadHistoryBooks } from '../api/readHistory';
import { useBookPagination, toPage, toSingleQueryValue } from '../composables/useBookPagination';
import { useDocumentTitle } from '../composables/useDocumentTitle';
import type { Book } from '../types/book';
import { useI18n } from '../i18n';

const route = useRoute();
const router = useRouter();
const { pageSize, setPageSize, PAGE_SIZE_OPTIONS } = useBookPagination();
const { t } = useI18n();

const books = ref<Book[]>([]);
const loading = ref(false);
const clearing = ref(false);
const error = ref('');

const page = computed(() => toPage(route.query.page));
const totalPages = computed(() => Math.max(1, Math.ceil(books.value.length / pageSize.value)));
const visibleBooks = computed(() => {
  const start = (page.value - 1) * pageSize.value;
  return books.value.slice(start, start + pageSize.value);
});

useDocumentTitle(() => [t('readHistory.title'), t('app.name')]);

function buildPageQuery(nextPage: number): Record<string, string> {
  return {
    ...route.query,
    page: String(nextPage)
  } as Record<string, string>;
}

async function loadReadHistory(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    books.value = await listReadHistoryBooks();
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('readHistory.loadFailed');
  } finally {
    loading.value = false;
  }
}

async function onClearHistory(): Promise<void> {
  if (books.value.length === 0 || clearing.value) {
    return;
  }

  clearing.value = true;
  error.value = '';

  try {
    await clearReadHistory();
    books.value = [];
    if (page.value !== 1) {
      await router.replace({ path: route.path, query: buildPageQuery(1) });
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('readHistory.clearFailed');
  } finally {
    clearing.value = false;
  }
}

function onPageChange(nextPage: number): void {
  if (nextPage === page.value) {
    return;
  }

  void router.push({
    path: route.path,
    query: buildPageQuery(nextPage)
  });
}

function onPageSizeChange(newSize: number): void {
  setPageSize(newSize);
  void router.push({
    path: route.path,
    query: buildPageQuery(1)
  });
}

function openBook(id: string): void {
  void router.push(`/books/${id}`);
}

watch(
  [page, totalPages],
  ([currentPage, maxPage]) => {
    const normalizedPage = Math.min(currentPage, maxPage);
    const rawPage = toSingleQueryValue(route.query.page);
    if (rawPage === String(normalizedPage)) {
      return;
    }

    void router.replace({
      path: route.path,
      query: buildPageQuery(normalizedPage)
    });
  },
  { immediate: true }
);

onMounted(() => {
  void loadReadHistory();
});
</script>

<style scoped>
.clear-history-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
</style>
