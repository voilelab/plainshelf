<template>
  <div>
    <BookCollectionPage
      :title="selectedLayerTitle"
      :books="visibleBooks"
      :loading="loading"
      :error="error"
      :page="page"
      :page-size="pageSize"
      :total="total"
      :count="total"
      :empty-message="emptyMessage"
      :page-size-options="PAGE_SIZE_OPTIONS"
      @retry="reloadBooks"
      @select="openBook"
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
          {{ ALL_BOOKS_TITLE }}
        </template>
      </template>

      <template #toolbar>
        <div class="toolbar-bar search-bar">
          <input
            v-model="searchInputValue"
            class="toolbar-control toolbar-input search-input"
            type="search"
            placeholder="Search books..."
            @keydown.enter="onSearchEnter"
          />
          <button
            v-if="searchInputValue"
            type="button"
            class="toolbar-control toolbar-button toolbar-small search-clear-btn"
            aria-label="Clear search"
            @click="clearSearch"
          >✕</button>
          <button
            type="button"
            class="button toolbar-control toolbar-button toolbar-regular search-commit-btn"
            @click="commitSearch"
          >Search</button>
        </div>
        <div class="toolbar-bar sort-bar">
          <label class="toolbar-label sort-label" for="books-sort">Sort</label>
          <select
            id="books-sort"
            class="toolbar-control toolbar-select sort-select"
            :value="sortBy"
            @change="onSortSelectChange"
          >
            <option value="updated_at">Updated</option>
            <option value="created_at">Created</option>
            <option value="title">Title</option>
          </select>
          <button
            type="button"
            class="button toolbar-control toolbar-button toolbar-regular sort-order-btn"
            @click="toggleOrder"
          >
            {{ sortOrder === 'asc' ? 'Asc' : 'Desc' }}
          </button>
        </div>
        <div class="import-dropdown" ref="importDropdown">
          <button class="button" type="button" @click="toggleImportDropdown">Import ▾</button>
          <div v-if="showImportDropdown" class="import-dropdown-menu">
            <button class="import-dropdown-item" type="button" @click="openImportFromFiles">Import from files</button>
            <button class="import-dropdown-item" type="button" @click="openNewEmptyBookModal">New empty book</button>
          </div>
        </div>
      </template>
    </BookCollectionPage>

    <ImportBookModal
      :open="isImportModalOpen"
      :current-layer-path="selectedLayer"
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
import { useRouter } from 'vue-router';
import type { Book } from '../types/book';
import BookCollectionPage from '../components/BookCollectionPage.vue';
import ImportBookModal from '../components/ImportBookModal.vue';
import NewEmptyBookModal from '../components/NewEmptyBookModal.vue';
import { useBookStore } from '../composables/useBookStore';
import { useDocumentTitle } from '../composables/useDocumentTitle';
import { useBookPagination } from '../composables/useBookPagination';
import { useBooksRouteQuery } from '../composables/useBooksRouteQuery';
import { useBooksSearch } from '../composables/useBooksSearch';
import { useBooksSort, type BookSortKey, type SortOrder } from '../composables/useBooksSort';
import { getLayerPath, layerPathEquals, normalizeLayerPath } from '../utils/layers';
import '../styles/toolbar-controls.css';

const ALL_BOOKS_TITLE = 'All books';
const ROOT_LAYER_LABEL = '/';

const router = useRouter();
const { books, loading, error, fetchBooks } = useBookStore();
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
const showImportDropdown = ref(false);
const isNewEmptyBookModalOpen = ref(false);
const importDropdown = ref<HTMLElement | null>(null);
const hasInitializedSearch = ref(false);

async function reloadBooks(): Promise<void> {
  booksLoaded.value = false;
  await fetchBooks(committedSearch.value.trim());
  booksLoaded.value = true;
}

const isRootLayerSelected = computed(() => selectedLayer.value === ROOT_LAYER_LABEL);

const selectedLayerTitle = computed(() => {
  if (!selectedLayer.value) {
    return ALL_BOOKS_TITLE;
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
    return ['Search', query, 'PlainShelf'] as const;
  }

  const layerName = selectedLayer.value?.trim();
  if (layerName && layerName !== ROOT_LAYER_LABEL) {
    return ['Layer', layerName, 'PlainShelf'] as const;
  }

  return ['PlainShelf'] as const;
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
    const layerSuffix = selectedLayer.value ? ` in ${selectedLayerTitle.value}` : '';
    return `No books found for "${q}"${layerSuffix}.`;
  }
  if (showLayerEmptyState.value) {
    return `No books in ${selectedLayerTitle.value}.`;
  }
  return 'No books yet.';
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

function openImportFromFiles(): void {
  showImportDropdown.value = false;

  if (isImportModalOpen.value) {
    return;
  }

  void openImportModalQuery();
}

function openNewEmptyBookModal(): void {
  showImportDropdown.value = false;
  isNewEmptyBookModalOpen.value = true;
}

function closeNewEmptyBookModal(): void {
  isNewEmptyBookModalOpen.value = false;
}

function toggleImportDropdown(): void {
  showImportDropdown.value = !showImportDropdown.value;
}

function onDocumentClick(event: MouseEvent): void {
  const target = event.target;
  if (!(target instanceof Node)) {
    return;
  }

  if (!importDropdown.value?.contains(target)) {
    showImportDropdown.value = false;
  }
}



function closeImportModal(): void {
  if (!isImportModalOpen.value) {
    return;
  }

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
  document.addEventListener('click', onDocumentClick);
});

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick);
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

.import-dropdown {
  position: relative;
}

.import-dropdown-menu {
  background: #fff;
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.14);
  display: grid;
  min-width: 180px;
  padding: 6px;
  position: absolute;
  right: 0;
  top: calc(100% + 6px);
  z-index: 20;
}

.import-dropdown-item {
  background: transparent;
  border: 0;
  border-radius: 6px;
  cursor: pointer;
  padding: 8px 10px;
  text-align: left;
}

.import-dropdown-item:hover {
  background: #f4f7fb;
}

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
