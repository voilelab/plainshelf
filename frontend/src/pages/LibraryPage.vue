<template>
  <div>
    <DeleteModal
      :open="!!deleteTarget"
      :item-name="deleteTarget?.title || ''"
      description="The book will be moved to Trash. You can restore it later."
      :busy="deleting"
      :error="actionError"
      @cancel="cancelDelete"
      @confirm="confirmDelete"
    />
    <p v-if="actionError && !deleteTarget" class="error" role="alert">{{ actionError }}</p>
    <BookCollectionPage
      :title="selectedLayerTitle"
      :books="visibleBooks"
      :loading="loading"
      :shelf-initializing="shelfInitializing"
      :shelf-unreachable="shelfUnreachable"
      :error="error"
      :page="page"
      :page-size="pageSize"
      :total="total"
      :count="total"
      :empty-message="emptyMessage"
      :page-size-options="PAGE_SIZE_OPTIONS"
      :can-open-book-folder="canOpenBookFolder"
      :read-only="readOnly"
      @retry="reloadBooks"
      @select="openBook"
      @edit="goEdit"
      @read="goRead"
      @open-book-folder="onOpenBookFolder"
      @download="onDownloadBook"
      @delete="onRequestDeleteBook"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    >
      <template #title-meta>
        <template v-if="isRootLayerSelected">
          {{ ROOT_LAYER_LABEL }}
        </template>
        <template v-else-if="selectedLayerSegments.length > 0">
          <button type="button" class="breadcrumb-link" @click="onSelectAllBooks">All books</button>
          <span class="breadcrumb-separator" aria-hidden="true">/</span>
          <template v-for="(segment, index) in selectedLayerSegments" :key="`${segment}-${index}`">
            <button
              type="button"
              class="breadcrumb-link"
              @click="onSelectBreadcrumb(index)"
            >
              {{ segment }}
            </button>
            <span
              v-if="index < selectedLayerSegments.length - 1"
              class="breadcrumb-separator"
              aria-hidden="true"
            >
              /
            </span>
          </template>
        </template>
        <template v-else>
          {{ t('library.allBooks') }}
        </template>
      </template>

      <template #toolbar>
        <div class="toolbar-bar search-bar">
          <input
            v-model="searchInputValue"
            class="toolbar-control toolbar-input search-input"
            type="search"
            :placeholder="t('library.searchPlaceholder')"
            @keydown.enter="onSearchEnter"
          />
          <button
            v-if="searchInputValue"
            type="button"
            class="toolbar-control toolbar-button toolbar-small search-clear-btn"
            :aria-label="t('library.clearSearch')"
            @click="clearSearch"
          >✕</button>
          <button
            type="button"
            class="button toolbar-control toolbar-button toolbar-regular search-commit-btn"
            @click="commitSearch"
          >{{ t('library.search') }}</button>
        </div>
        <div class="toolbar-bar sort-bar">
          <label class="toolbar-label sort-label" for="books-sort">{{ t('library.sort') }}</label>
          <select
            id="books-sort"
            class="toolbar-control toolbar-select sort-select"
            :value="sortBy"
            @change="onSortSelectChange"
          >
            <option value="updated_at">{{ t('library.sortBy.updated') }}</option>
            <option value="created_at">{{ t('library.sortBy.created') }}</option>
            <option value="title">{{ t('library.sortBy.title') }}</option>
          </select>
          <button
            type="button"
            class="button toolbar-control toolbar-button toolbar-regular sort-order-btn"
            @click="toggleOrder"
          >
            {{ sortOrder === 'asc' ? t('library.order.asc') : t('library.order.desc') }}
          </button>
        </div>
        <DropdownMenuRoot v-if="!readOnly">
          <DropdownMenuTrigger class="button">{{ t('library.import') }}</DropdownMenuTrigger>
          <DropdownMenuPortal>
            <DropdownMenuContent class="reka-menu" align="end" :side-offset="6">
              <DropdownMenuItem class="reka-menu-item" @select="openImportFromFiles">{{ t('library.importFromFiles') }}</DropdownMenuItem>
              <DropdownMenuItem class="reka-menu-item" @select="openNewEmptyBookModal">{{ t('library.newEmptyBook') }}</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenuPortal>
        </DropdownMenuRoot>
      </template>
    </BookCollectionPage>

    <ImportBookModal
      :open="isImportModalOpen"
      :current-layer-path="selectedLayer"
      :dropped-files="droppedFiles"
      @close="closeImportModal"
      @imported="onImported"
    />
    <NewEmptyBookModal
      :open="isNewEmptyBookModalOpen"
      :current-layer-path="selectedLayer"
      @close="closeNewEmptyBookModal"
      @imported="onImported"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger
} from 'reka-ui';
import { useRouter } from 'vue-router';
import type { Book } from '../types/book';
import BookCollectionPage from '../components/BookCollectionPage.vue';
import DeleteModal from '../components/DeleteModal.vue';
import ImportBookModal from '../components/ImportBookModal.vue';
import NewEmptyBookModal from '../components/NewEmptyBookModal.vue';
import { useBookActions } from '../composables/useBookActions';
import { useBookStore } from '../composables/useBookStore';
import { useDocumentTitle } from '../composables/useDocumentTitle';
import { useBookPagination } from '../composables/useBookPagination';
import { useBooksRouteQuery } from '../composables/useBooksRouteQuery';
import { useBooksSearch } from '../composables/useBooksSearch';
import { useBooksSort, type BookSortKey, type SortOrder } from '../composables/useBooksSort';
import { useServerMode } from '../composables/useServerMode';
import { hasFileTransfer, readDroppedFiles } from '../utils/file';
import { getLayerPath, layerPathEquals, normalizeLayerPath } from '../utils/layers';
import { useI18n } from '../i18n';
import { getBookshelfProvider } from '../providers';
import '../styles/toolbar-controls.css';

const ROOT_LAYER_LABEL = '/';
const { t } = useI18n();

const router = useRouter();
const { books, loading, error, shelfInitializing, shelfUnreachable, fetchBooks } = useBookStore();
const { pageSize, setPageSize, PAGE_SIZE_OPTIONS } = useBookPagination();
const {
  selectedLayer,
  page,
  sortBy,
  sortOrder,
  searchQuery,
  isImportModalOpen,
  pushBooksQuery,
  replaceBooksQuery,
  isBooksQueryNormalized,
  openImportModalQuery,
  closeImportModalQuery
} = useBooksRouteQuery();
const {
  searchInputValue,
  committedSearch,
  commitSearch,
  onSearchEnter,
  clearSearch
} = useBooksSearch(searchQuery.value);
const booksLoaded = ref<boolean>(false);
const isNewEmptyBookModalOpen = ref(false);
const { readOnly } = useServerMode();
const hasInitializedSearch = ref(false);
const droppedFiles = ref<File[]>([]);

async function reloadBooks(): Promise<void> {
  booksLoaded.value = false;
  await fetchBooks(committedSearch.value.trim());
  booksLoaded.value = true;
}

const {
  canOpenBookFolder,
  actionError,
  deleteTarget,
  deleting,
  goRead,
  goEdit,
  openBookFolder,
  downloadBook,
  requestDelete,
  cancelDelete,
  confirmDelete
} = useBookActions({
  onDeleted: () => {
    void reloadBooks();
  }
});

function findBook(id: string): Book | undefined {
  return books.value.find((candidate) => candidate.id === id);
}

function onOpenBookFolder(id: string): void {
  void openBookFolder(id);
}

function onDownloadBook(id: string): void {
  const book = findBook(id);
  if (book) {
    void downloadBook(book);
  }
}

function onRequestDeleteBook(id: string): void {
  if (readOnly.value) {
    return;
  }
  const book = findBook(id);
  if (book) {
    requestDelete(book);
  }
}

const isRootLayerSelected = computed(() => selectedLayer.value === ROOT_LAYER_LABEL);

const selectedLayerTitle = computed(() => {
  if (!selectedLayer.value) {
    return t('library.allBooks');
  }
  return selectedLayer.value;
});

const selectedLayerSegments = computed(() => {
  if (!selectedLayer.value) {
    return [] as string[];
  }
  return selectedLayer.value.split('/').filter((segment) => segment.length > 0);
});

const pageTitleSegments = computed(() => {
  const query = searchQuery.value.trim();
  if (query) {
    return [t('library.titleSearch'), query, t('app.name')] as const;
  }

  const layerName = selectedLayer.value?.trim();
  if (layerName && layerName !== ROOT_LAYER_LABEL) {
    return [t('library.titleLayer'), layerName, t('app.name')] as const;
  }

  return [t('app.name')] as const;
});

useDocumentTitle(pageTitleSegments);

function matchesLayer(book: Book): boolean {
  if (!selectedLayer.value) {
    return true;
  }
  return layerPathEquals(getLayerPath(book), selectedLayer.value);
}

const filteredBooks = computed(() => books.value.filter((book) => matchesLayer(book)));
const {
  SORT_OPTIONS,
  sortedBooks
} = useBooksSort(filteredBooks, sortBy, sortOrder);

const total = computed(() => filteredBooks.value.length);
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)));

const visibleBooks = computed(() => {
  const start = (page.value - 1) * pageSize.value;
  return sortedBooks.value.slice(start, start + pageSize.value);
});

const showLayerEmptyState = computed(() => {
  return books.value.length > 0 && !!selectedLayer.value && filteredBooks.value.length === 0;
});

const emptyMessage = computed(() => {
  const q = committedSearch.value.trim();
  if (q && filteredBooks.value.length === 0 && !loading.value) {
    const layerSuffix = selectedLayer.value
      ? t('common.inLayer', { layer: selectedLayerTitle.value })
      : '';
    return t('library.empty.noBooksFound', { query: q, layerSuffix });
  }
  if (showLayerEmptyState.value) {
    return t('library.empty.noBooksInLayer', { layer: selectedLayerTitle.value });
  }
  return t('library.empty.noBooksYet');
});

function onSelectAllBooks(): void {
  if (!selectedLayer.value && page.value === 1) {
    return;
  }
  void pushBooksQuery({ layer: undefined, page: 1 });
}

function onSelectLayer(layer: string): void {
  const trimmed = layer.trim();
  if (trimmed === '') {
    onSelectAllBooks();
    return;
  }

  const normalized = trimmed === ROOT_LAYER_LABEL ? ROOT_LAYER_LABEL : normalizeLayerPath(trimmed);

  if (selectedLayer.value === normalized && page.value === 1) {
    return;
  }
  void pushBooksQuery({ layer: normalized, page: 1 });
}

function onSelectBreadcrumb(index: number): void {
  const path = selectedLayerSegments.value.slice(0, index + 1).join('/');
  onSelectLayer(path);
}

function onPageChange(nextPage: number): void {
  if (nextPage === page.value) {
    return;
  }
  void pushBooksQuery({ layer: selectedLayer.value, page: nextPage });
}

function onPageSizeChange(newSize: number): void {
  setPageSize(newSize);
  void pushBooksQuery({ layer: selectedLayer.value, page: 1 });
}

function onSortChange(nextSort: BookSortKey): void {
  if (nextSort === sortBy.value && page.value === 1) {
    return;
  }

  void pushBooksQuery({
    layer: selectedLayer.value,
    page: 1,
    sort: nextSort,
    order: sortOrder.value
  });
}

function onSortSelectChange(event: Event): void {
  const target = event.target;
  if (!(target instanceof HTMLSelectElement)) {
    return;
  }

  const value = target.value;
  if (!SORT_OPTIONS.includes(value as BookSortKey)) {
    return;
  }

  onSortChange(value as BookSortKey);
}

function onOrderChange(nextOrder: SortOrder): void {
  if (nextOrder === sortOrder.value && page.value === 1) {
    return;
  }

  void pushBooksQuery({
    layer: selectedLayer.value,
    page: 1,
    sort: sortBy.value,
    order: nextOrder
  });
}

function toggleOrder(): void {
  onOrderChange(sortOrder.value === 'asc' ? 'desc' : 'asc');
}

async function openImportFromFiles(): Promise<void> {
  if (readOnly.value) {
    return;
  }
  droppedFiles.value = [];

  let desktopFiles: string[] | null = null;
  try {
    desktopFiles = await getBookshelfProvider().openLocalBookFiles?.() ?? null;
  } catch {
    desktopFiles = null;
  }

  if (desktopFiles) {
    if (desktopFiles.length === 0) {
      return;
    }

    try {
      const importResult = await getBookshelfProvider().importBooksFromLocalPaths?.(desktopFiles, selectedLayer.value ?? '') ?? null;
      if (importResult) {
        const hasImportedBook = importResult.some((item) => item.id !== undefined && item.id !== '');
        const hasFailedBook = importResult.some((item) => Boolean(item.error));
        if (hasImportedBook) {
          await reloadBooks();
        } else if (hasFailedBook && !isImportModalOpen.value) {
          void openImportModalQuery();
        }
        return;
      }
    } catch {
      // Fall through to browser file-input import modal.
    }
  }

  if (isImportModalOpen.value) {
    return;
  }

  void openImportModalQuery();
}

function openNewEmptyBookModal(): void {
  if (readOnly.value) {
    return;
  }
  isNewEmptyBookModalOpen.value = true;
}

function closeNewEmptyBookModal(): void {
  isNewEmptyBookModalOpen.value = false;
}

function onDocumentDragOver(event: DragEvent): void {
  if (readOnly.value) {
    return;
  }
  if (!hasFileTransfer(event.dataTransfer)) {
    return;
  }

  event.preventDefault();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy';
  }
}

function onDocumentDrop(event: DragEvent): void {
  if (readOnly.value) {
    return;
  }
  if (!hasFileTransfer(event.dataTransfer)) {
    return;
  }

  event.preventDefault();
  const nextDroppedFiles = readDroppedFiles(event);
  if (nextDroppedFiles.length === 0) {
    return;
  }

  droppedFiles.value = nextDroppedFiles;
  if (!isImportModalOpen.value) {
    void openImportModalQuery();
  }
}

function closeImportModal(): void {
  if (!isImportModalOpen.value) {
    return;
  }

  droppedFiles.value = [];
  void closeImportModalQuery();
}

async function onImported(result: { successCount: number }): Promise<void> {
  if (result.successCount > 0) {
    await reloadBooks();
  }
}

function openBook(id: string): void {
  void router.push(`/books/${id}`);
}


onMounted(() => {
  document.addEventListener('dragover', onDocumentDragOver);
  document.addEventListener('drop', onDocumentDrop);
});

onBeforeUnmount(() => {
  document.removeEventListener('dragover', onDocumentDragOver);
  document.removeEventListener('drop', onDocumentDrop);
});

watch(selectedLayer, async () => {
  await reloadBooks();
});

// Watch committed search: update URL and fetch from backend
watch(
  committedSearch,
  async (newSearch) => {
    if (!hasInitializedSearch.value) {
      hasInitializedSearch.value = true;
      await reloadBooks();
      return;
    }

    void replaceBooksQuery({
      layer: selectedLayer.value,
      page: 1,
      search: newSearch,
      sort: sortBy.value,
      order: sortOrder.value
    });
    await reloadBooks();
  },
  { immediate: true }
);

watch(
  [selectedLayer, page, totalPages, booksLoaded],
  ([layer, currentPage, maxPage, hasLoaded]) => {
    const normalizedPage = hasLoaded ? Math.min(currentPage, maxPage) : currentPage;
    const currentSearch = committedSearch.value.trim();

    if (isBooksQueryNormalized({
      layer,
      page: normalizedPage,
      search: currentSearch,
      sort: sortBy.value,
      order: sortOrder.value
    })) {
      return;
    }

    void replaceBooksQuery({
      layer,
      page: normalizedPage,
      search: currentSearch,
      sort: sortBy.value,
      order: sortOrder.value
    });
  },
  { immediate: true }
);
</script>

<style scoped>

.breadcrumb-link {
  background: transparent;
  border: 0;
  border-radius: 4px;
  color: inherit;
  cursor: pointer;
  font-size: inherit;
  padding: 2px 4px;
}

.breadcrumb-link:hover {
  background: #f4f7fb;
  color: color-mix(in srgb, var(--text) 72%, white);
}

.breadcrumb-separator {
  opacity: 0.6;
}

/* Search bar layout adjustments */
.search-bar {
  display: flex;
  align-items: center;
  gap: 6px;
}

.search-input {
  width: 180px;
  padding: 0 28px 0 8px;
}

.search-clear-btn {
  color: var(--muted, #888);
  line-height: 1;
}

.search-clear-btn:hover {
  color: var(--text, #333);
}

/* Sort bar layout adjustments */
.sort-bar {
  display: flex;
  align-items: center;
  gap: 6px;
}

.sort-select {
  min-width: 100px;
}

.sort-order-btn {
  min-width: 64px;
}

/* Responsive layout */
@media (max-width: 760px) {
  .search-bar {
    flex: 1 1 100%;
    min-width: 0;
  }

  .search-input {
    flex: 1 1 auto;
    min-width: 0;
    width: auto;
  }

  .sort-bar {
    flex: 0 0 auto;
  }

  .sort-select {
    min-width: 92px;
  }
}
</style>
