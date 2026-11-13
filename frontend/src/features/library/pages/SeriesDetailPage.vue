<template>
  <div>
    <MetaEditorModal
      :open="metadataEditorOpen"
      :book-id="selectedMetadataBookId"
      @close="closeMetadataEditor"
      @saved="onMetadataSaved"
    />
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
      :title="seriesName"
      :books="visibleBooks"
      :loading="loading"
      :shelf-initializing="shelfInitializing"
      :shelf-unreachable="shelfUnreachable"
      :error="error"
      :page="page"
      :page-size="pageSize"
      :total="seriesBooks.length"
      :count="seriesBooks.length"
      :empty-message="t('series.detailEmpty')"
      :page-size-options="PAGE_SIZE_OPTIONS"
      :can-open-book-folder="canOpenBookFolder"
      :read-only="readOnly"
      view-mode-storage-key="series"
      @retry="fetchBooks"
      @activate="openDetail($event.id)"
      @edit="openMetadataEditor"
      @read="goRead"
      @open-book-folder="onOpenBookFolder"
      @download="onDownloadBook"
      @delete="onRequestDeleteBook"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    >
      <template #title-meta>
        <RouterLink to="/series">{{ t('series.backToAll') }}</RouterLink>
      </template>
    </BookCollectionPage>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import BookCollectionPage from '@/components/BookCollectionPage.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import MetaEditorModal from '@/features/library/components/MetaEditorModal.vue';
import { booksInSeries } from '@/features/library/utils/series';
import { DELETE_BOOK_DESCRIPTION } from '@/composables/useBookActions';
import { useBookCollectionActions } from '@/composables/useBookCollectionActions';
import { useBookCollectionRoute } from '@/composables/useBookCollectionRoute';
import { useBookStore } from '@/composables/useBookStore';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useMetadataEditorModal } from '@/features/library/composables/useMetadataEditorModal';
import { useI18n } from '@/i18n';

const props = defineProps<{ name: string }>();

const route = useRoute();
const { t } = useI18n();
const { books, loading, error, shelfInitializing, shelfUnreachable, fetchBooks } = useBookStore();

const seriesName = computed(() => props.name.trim());
const seriesBooks = computed(() => booksInSeries(books.value, seriesName.value));

useDocumentTitle(() => [seriesName.value, t('series.title'), t('app.name')]);

const { page, pageSize, visibleBooks, onPageChange, onPageSizeChange, PAGE_SIZE_OPTIONS } =
  useBookCollectionRoute({
    items: seriesBooks,
    buildQuery: (nextPage) => ({ ...route.query, page: String(nextPage) }) as Record<string, string>
  });

const {
  canOpenBookFolder,
  actionError,
  deleteTarget,
  deleting,
  readOnly,
  goRead,
  openDetail,
  cancelDelete,
  confirmDelete,
  onOpenBookFolder,
  onDownloadBook,
  onRequestDeleteBook
} = useBookCollectionActions({ books: seriesBooks, onDeleted: () => void fetchBooks() });

const {
  selectedBookId: selectedMetadataBookId,
  open: metadataEditorOpen,
  openEditor: openMetadataEditor,
  closeEditor: closeMetadataEditor,
  onSaved: onMetadataSaved
} = useMetadataEditorModal({ books, readOnly, refresh: fetchBooks });
</script>
