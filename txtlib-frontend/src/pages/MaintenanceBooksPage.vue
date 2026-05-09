<template>
  <BookCollectionPage
    :title="heading"
    :books="visibleBooks"
    :loading="loading"
    :error="error"
    :page="page"
    :page-size="pageSize"
    :total="filteredBooks.length"
    :count="filteredBooks.length"
    :empty-message="emptyMessage"
    :show-edit-action="true"
    :page-size-options="PAGE_SIZE_OPTIONS"
    @retry="loadBooks"
    @select="openBook"
    @edit="openEdit"
    @update:page="onPageChange"
    @update:page-size="onPageSizeChange"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import BookCollectionPage from '../components/BookCollectionPage.vue';
import { useBookStore } from '../composables/useBookStore';
import { useBookPagination, toSingleQueryValue, toPage } from '../composables/useBookPagination';
import { isMissingAuthor, isMissingCover } from '../utils/maintenance';

type MaintenanceFilter = 'missing-author' | 'missing-cover';

const props = defineProps<{
  filter: MaintenanceFilter;
}>();

const route = useRoute();
const router = useRouter();
const { books, loading, error, fetchBooks } = useBookStore();
const { pageSize, setPageSize, PAGE_SIZE_OPTIONS } = useBookPagination();

const heading = computed(() => {
  return props.filter === 'missing-author' ? 'Missing Author' : 'Missing Cover';
});

const emptyMessage = computed(() => {
  return props.filter === 'missing-author' ? 'No books missing author' : 'No books missing cover';
});

const filteredBooks = computed(() => {
  return books.value.filter((book) => {
    return props.filter === 'missing-author' ? isMissingAuthor(book) : isMissingCover(book);
  });
});

function buildPageQuery(nextPage: number): Record<string, string> {
  const nextQuery = {
    ...route.query
  } as Record<string, string>;

  delete nextQuery.page;
  nextQuery.page = String(nextPage);

  return nextQuery;
}

const page = computed(() => toPage(route.query.page));
const totalPages = computed(() => Math.max(1, Math.ceil(filteredBooks.value.length / pageSize.value)));

const visibleBooks = computed(() => {
  const start = (page.value - 1) * pageSize.value;
  return filteredBooks.value.slice(start, start + pageSize.value);
});

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

function openEdit(id: string): void {
  void router.push(`/books/${id}/edit`);
}

async function loadBooks(): Promise<void> {
  await fetchBooks();
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
  void loadBooks();
});
</script>
