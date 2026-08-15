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
      :can-open-book-folder="canOpenBookFolder"
      :read-only="readOnly"
      view-mode-storage-key="read-history"
      @retry="loadReadHistory"
      @activate="openDetail($event.id)"
      @edit="goEdit"
      @read="goRead"
      @open-book-folder="onOpenBookFolder"
      @download="onDownloadBook"
      @delete="onRequestDeleteBook"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    >
      <template #toolbar>
        <!-- Clearing only touches this device's stored history, so it stays
             available on read-only servers and on the mobile shell. -->
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
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import BookCollectionPage from '@/components/BookCollectionPage.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import { getBookshelfProvider } from '@/providers';
import { DELETE_BOOK_DESCRIPTION } from '@/composables/useBookActions';
import { useBookCollectionActions } from '@/composables/useBookCollectionActions';
import { useBookCollectionRoute } from '@/composables/useBookCollectionRoute';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import type { Book } from '@/types/book';
import { useI18n } from '@/i18n';

const route = useRoute();
const router = useRouter();
const { t } = useI18n();

const books = ref<Book[]>([]);
const loading = ref(false);
const clearing = ref(false);
const error = ref('');

useDocumentTitle(() => [t('readHistory.title'), t('app.name')]);

function buildPageQuery(nextPage: number): Record<string, string> {
  return {
    ...route.query,
    page: String(nextPage)
  } as Record<string, string>;
}

const { page, pageSize, visibleBooks, onPageChange, onPageSizeChange, PAGE_SIZE_OPTIONS } =
  useBookCollectionRoute({ items: books, buildQuery: buildPageQuery });

async function loadReadHistory(): Promise<void> {
  loading.value = true;
  error.value = '';

  try {
    books.value = await getBookshelfProvider().listReadHistoryBooks();
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
    await getBookshelfProvider().clearReadHistory();
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
    void loadReadHistory();
  }
});

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
