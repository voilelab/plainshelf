<template>
  <div>
    <DeleteModal
      :open="!!deleteTarget"
      :item-name="deleteTarget?.title || ''"
      :description="DELETE_BOOK_DESCRIPTION"
      :busy="deleting"
      :error="actionError"
      @cancel="cancelDelete"
      @confirm="confirmDelete"
    />
    <p v-if="actionError && !deleteTarget" class="error" role="alert">{{ actionError }}</p>
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
      :can-open-book-folder="canOpenBookFolder"
      :read-only="readOnly"
      :page-size-options="PAGE_SIZE_OPTIONS"
      @retry="loadBooks"
      @activate="openDetail($event.id)"
      @edit="goEdit"
      @read="goRead"
      @open-book-folder="onOpenBookFolder"
      @download="onDownloadBook"
      @delete="onRequestDeleteBook"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import BookCollectionPage from '@/components/BookCollectionPage.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import { DELETE_BOOK_DESCRIPTION } from '@/composables/useBookActions';
import { useBookCollectionActions } from '@/composables/useBookCollectionActions';
import { useBookCollectionRoute } from '@/composables/useBookCollectionRoute';
import { useBookStore } from '@/composables/useBookStore';
import {
  MAINTENANCE_BOOK_FILTERS,
  type MaintenanceBookFilter
} from '@/utils/maintenance';
import { useI18n } from '@/i18n';

const props = defineProps<{
  filter: MaintenanceBookFilter;
}>();

const route = useRoute();
const { t } = useI18n();

const filterConfig = computed(() => MAINTENANCE_BOOK_FILTERS[props.filter]);

const { books, loading, error, fetchBooks } = useBookStore();

const heading = computed(() => {
  return t(filterConfig.value.titleKey);
});

const emptyMessage = computed(() => {
  return t(filterConfig.value.emptyMessageKey);
});

const filteredBooks = computed(() => {
  return books.value.filter((book) => filterConfig.value.predicate(book));
});

function buildQuery(nextPage: number): Record<string, string> {
  const nextQuery = {
    ...route.query
  } as Record<string, string>;

  delete nextQuery.page;
  nextQuery.page = String(nextPage);

  return nextQuery;
}

const { page, pageSize, visibleBooks, onPageChange, onPageSizeChange, PAGE_SIZE_OPTIONS } =
  useBookCollectionRoute({
    items: filteredBooks,
    buildQuery
  });

async function loadBooks(): Promise<void> {
  await fetchBooks();
}

const {
  canOpenBookFolder,
  actionError,
  deleteTarget,
  deleting,
  readOnly,
  goRead,
  openDetail,
  goEdit,
  cancelDelete,
  confirmDelete,
  onOpenBookFolder,
  onDownloadBook,
  onRequestDeleteBook
} = useBookCollectionActions({
  books,
  onDeleted: () => {
    void loadBooks();
  }
});

onMounted(() => {
  void loadBooks();
});
</script>
