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
        <div class="search-bar">
          <input
            v-model="searchInputValue"
            class="search-input"
            type="search"
            placeholder="Search books..."
            @keydown.enter="onSearchEnter"
          />
          <button
            v-if="searchInputValue"
            type="button"
            class="search-clear-btn"
            aria-label="Clear search"
            @click="clearSearch"
          >✕</button>
          <button type="button" class="button search-commit-btn" @click="commitSearch">Search</button>
        </div>
        <button class="button" type="button" @click="openImportModal">Import</button>
      </template>
    </BookCollectionPage>

    <ImportBookModal
      :open="isImportModalOpen"
      :current-layer-path="selectedLayer"
      @close="closeImportModal"
      @imported="onImported"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { LocationQueryValue } from 'vue-router';
import type { Book } from '../types/book';
import BookCollectionPage from '../components/BookCollectionPage.vue';
import ImportBookModal from '../components/ImportBookModal.vue';
import { useBookStore } from '../composables/useBookStore';
import { useBookPagination, toSingleQueryValue, toPage } from '../composables/useBookPagination';
import { getLayerPath, layerPathEquals, normalizeLayerPath } from '../utils/layers';

const ALL_BOOKS_TITLE = 'All books';
const ROOT_LAYER_LABEL = '/';

const route = useRoute();
const router = useRouter();
const { books, loading, error, fetchBooks } = useBookStore();
const { pageSize, setPageSize, PAGE_SIZE_OPTIONS } = useBookPagination();
const booksLoaded = ref<boolean>(false);

// ── Search state ──────────────────────────────────────────────────────────────
const searchInputValue = ref<string>(toSingleQueryValue(route.query.search) ?? '');
const committedSearch = ref<string>(searchInputValue.value.trim());

async function reloadBooks(): Promise<void> {
  booksLoaded.value = false;
  await fetchBooks(committedSearch.value.trim());
  booksLoaded.value = true;
}

function commitSearch(): void {
  const nextSearch = searchInputValue.value.trim();
  if (nextSearch === committedSearch.value.trim()) {
    return;
  }
  committedSearch.value = nextSearch;
}

function onSearchEnter(event: KeyboardEvent): void {
  if (event.isComposing) {
    return;
  }

  event.preventDefault();
  commitSearch();
}

function clearSearch(): void {
  searchInputValue.value = '';
  if (committedSearch.value.trim() !== '') {
    committedSearch.value = '';
  }
}
// ─────────────────────────────────────────────────────────────────────────────

function toLayerPath(value: LocationQueryValue | LocationQueryValue[] | undefined): string | undefined {
  const raw = toSingleQueryValue(value);
  if (!raw) {
    return undefined;
  }

  const normalized = raw.trim();
  return normalized.length > 0 ? normalized : undefined;
}


function buildBooksQuery(layer: string | undefined, nextPage: number, searchOverride?: string) {
  const nextQuery = {
    ...route.query
  } as Record<string, LocationQueryValue | LocationQueryValue[]>;

  delete nextQuery.layer;
  delete nextQuery.layers;
  delete nextQuery.page;
  delete nextQuery.search;

  if (layer) {
    nextQuery.layers = layer;
  }
  nextQuery.page = String(nextPage);

  const search = searchOverride !== undefined ? searchOverride : committedSearch.value;
  if (search.trim()) {
    nextQuery.search = search.trim();
  }

  return nextQuery;
}

const selectedLayer = computed(() => toLayerPath(route.query.layers) ?? toLayerPath(route.query.layer));
const page = computed(() => toPage(route.query.page));
const isImportModalOpen = computed(() => toSingleQueryValue(route.query.import) === '1');
const isRootLayerSelected = computed(() => selectedLayer.value === ROOT_LAYER_LABEL);

function buildImportQuery(open: boolean): Record<string, LocationQueryValue | LocationQueryValue[]> {
  const nextQuery = {
    ...route.query
  } as Record<string, LocationQueryValue | LocationQueryValue[]>;

  if (open) {
    nextQuery.import = '1';
  } else {
    delete nextQuery.import;
  }

  return nextQuery;
}

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

function matchesLayer(book: Book): boolean {
  if (!selectedLayer.value) {
    return true;
  }
  return layerPathEquals(getLayerPath(book), selectedLayer.value);
}

const filteredBooks = computed(() => books.value.filter((book) => matchesLayer(book)));

const total = computed(() => filteredBooks.value.length);
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)));

const visibleBooks = computed(() => {
  const start = (page.value - 1) * pageSize.value;
  return filteredBooks.value.slice(start, start + pageSize.value);
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
  void router.push({ path: '/books', query: buildBooksQuery(undefined, 1) });
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
  void router.push({ path: '/books', query: buildBooksQuery(normalized, 1) });
}

function onSelectBreadcrumb(index: number): void {
  const path = selectedLayerSegments.value.slice(0, index + 1).join('/');
  onSelectLayer(path);
}

function onPageChange(nextPage: number): void {
  if (nextPage === page.value) {
    return;
  }
  void router.push({ path: '/books', query: buildBooksQuery(selectedLayer.value, nextPage) });
}

function onPageSizeChange(newSize: number): void {
  setPageSize(newSize);
  void router.push({ path: '/books', query: buildBooksQuery(selectedLayer.value, 1) });
}

function openImportModal(): void {
  if (isImportModalOpen.value) {
    return;
  }

  void router.push({ path: '/books', query: buildImportQuery(true) });
}

function closeImportModal(): void {
  if (!isImportModalOpen.value) {
    return;
  }

  void router.replace({ path: '/books', query: buildImportQuery(false) });
}

async function onImported(result: { successCount: number }): Promise<void> {
  if (result.successCount > 0) {
    await reloadBooks();
  }
}

function openBook(id: string): void {
  void router.push(`/books/${id}`);
}

watch(selectedLayer, async () => {
  await reloadBooks();
});

// Watch committed search: update URL and fetch from backend
watch(
  committedSearch,
  async (newSearch) => {
    void router.replace({
      path: '/books',
      query: buildBooksQuery(selectedLayer.value, 1, newSearch),
    });
    await reloadBooks();
  },
  { immediate: true }
);

watch(
  [selectedLayer, page, totalPages, booksLoaded],
  ([layer, currentPage, maxPage, hasLoaded]) => {
    const normalizedPage = hasLoaded ? Math.min(currentPage, maxPage) : currentPage;
    const rawPage = toSingleQueryValue(route.query.page);
    const rawLayers = toLayerPath(route.query.layers);
    const hasLegacyLayerQuery = toSingleQueryValue(route.query.layer) !== undefined;

    const rawSearch = toSingleQueryValue(route.query.search) ?? '';
    const currentSearch = committedSearch.value.trim();
    if (rawPage === String(normalizedPage) && rawLayers === layer && !hasLegacyLayerQuery && rawSearch === currentSearch) {
      return;
    }

    void router.replace({
      path: '/books',
      query: buildBooksQuery(layer, normalizedPage)
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

.search-bar {
  align-items: center;
  display: flex;
  gap: 6px;
}

.search-input {
  background: var(--bg, #fff);
  border: 1px solid #d0d7e0;
  border-radius: 6px;
  color: inherit;
  font-size: 13px;
  height: 30px;
  padding: 0 28px 0 8px;
  width: 180px;
}

.search-input:focus {
  border-color: #6b9fe4;
  outline: none;
}

.search-clear-btn {
  background: transparent;
  border: 1px solid #d0d7e0;
  border-radius: 6px;
  color: var(--muted, #888);
  cursor: pointer;
  font-size: 11px;
  height: 30px;
  line-height: 1;
  padding: 0 8px;
}

.search-clear-btn:hover {
  color: var(--text, #333);
}

.search-commit-btn {
  height: 30px;
  min-width: auto;
  padding: 0 12px;
}
</style>
