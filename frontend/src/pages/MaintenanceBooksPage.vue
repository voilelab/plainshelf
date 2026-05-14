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
import { MAINTENANCE_BOOK_FILTERS, type MaintenanceBookFilter } from '../utils/maintenance';

const props = defineProps<{
  filter: MaintenanceBookFilter;
}>();

const route = useRoute();
const router = useRouter();
const { books, loading, error, fetchBooks } = useBookStore();
const { pageSize, setPageSize, PAGE_SIZE_OPTIONS } = useBookPagination();

const filterConfig = computed(() => MAINTENANCE_BOOK_FILTERS[props.filter]);

const heading = computed(() => {
  return filterConfig.value.title;
});

const emptyMessage = computed(() => {
  return filterConfig.value.emptyMessage;
});

const filteredBooks = computed(() => {
  return books.value.filter((book) => filterConfig.value.predicate(book));
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
