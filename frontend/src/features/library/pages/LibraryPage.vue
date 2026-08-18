<template>
  <div>
    <MoveBooksModal
      :open="moveBooksModalOpen"
      :count="selection.count.value"
      :options="layerOptions"
      :busy="batchOperations.running.value"
      @cancel="moveBooksModalOpen = false"
      @submit="submitBatchMove"
    />
    <ConfirmModal
      :open="trashBooksModalOpen"
      :title="t('bookCollection.selection.trashTitle')"
      :confirm-text="t('bookCollection.selection.confirmTrash')"
      :busy-text="t('bookCollection.selection.processing')"
      :busy="batchOperations.running.value"
      variant="danger"
      @cancel="trashBooksModalOpen = false"
      @confirm="submitBatchTrash"
    >
      <p>{{ t('bookCollection.selection.trashQuestion', { count: selection.count.value }) }}</p>
      <p>{{ t('bookCollection.selection.trashDescription') }}</p>
    </ConfirmModal>
    <BaseDialog
      :open="downloadBatchOpen"
      :title="t('bookCollection.selection.download')"
      :dismissible="!downloadBatchRunning"
      :busy="downloadBatchRunning"
      @close="closeDownloadBatch"
    >
      <section class="panel download-batch-modal">
        <h2>{{ t('bookCollection.selection.download') }}</h2>
        <p>{{ downloadBatchStatusText }}</p>
        <ProgressBar :value="downloadBatchPercentage" :label="t('bookCollection.selection.progressLabel')" />
        <p class="progress-value">{{ Math.round(downloadBatchPercentage) }}%</p>
        <ul v-if="downloadBatchFailures.length" class="batch-failures">
          <li v-for="failure in downloadBatchFailures" :key="failure.id">
            <strong>{{ failure.title }}</strong> — {{ failure.message }}
          </li>
        </ul>
        <footer><button type="button" class="button" :disabled="downloadBatchRunning" @click="closeDownloadBatch">{{ t('bookCollection.selection.close') }}</button></footer>
      </section>
    </BaseDialog>
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
    <p v-if="shelfRefresh.error.value" class="error" role="alert">{{ shelfRefresh.error.value }}</p>
    <p v-if="charCountError" class="error" role="alert">{{ charCountError }}</p>
    <BookCollectionPage
      :title="selectedLayerTitle"
      :books="visibleBooks"
      :loading="collectionLoading"
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
      :selection-enabled="selectionEnabled"
      :mobile-selection="isMobileEnv"
      :selection-busy="batchOperations.running.value || downloadBatchRunning"
      :selected-ids="selection.selectedIds.value"
      @retry="reloadBooks"
      @activate="onBookActivate"
      @toggle-selection="onToggleSelection"
      @long-press="onLongPress"
      @clear-selection="selection.clear"
      @select-all="selectVisibleBooks"
      @batch-move="openBatchMove"
      @batch-delete="openBatchTrash"
      @batch-download="startBatchDownload"
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
          <button type="button" class="breadcrumb-link" @click="onSelectAllBooks">{{ t('library.allBooks') }}</button>
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
        <CharCountFilterBar
          v-if="charCountFilterSupported"
          :range="charCountRange"
          :unknown-count="unknownCharCountCount"
          :read-only="readOnly"
          @update:range="onCharCountRangeChange"
          @stats-refreshed="onCharCountStatsRefreshed"
        />
        <div v-if="shelfRefresh.supported" class="toolbar-bar shelf-refresh-bar">
          <button
            type="button"
            class="button toolbar-control toolbar-button toolbar-regular shelf-refresh-button"
            :disabled="shelfRefresh.refreshing.value"
            @click="shelfRefresh.refresh"
          >
            {{ shelfRefresh.refreshing.value ? t('library.refreshingShelf') : t('library.refreshShelf') }}
          </button>
          <span class="toolbar-label shelf-refresh-status">{{ lastSyncedLabel }}</span>
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
      :open="importModalOpen"
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
import type { Book } from '@/types/book';
import BookCollectionPage from '@/components/BookCollectionPage.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import BaseDialog from '@/components/BaseDialog.vue';
import ProgressBar from '@/components/ProgressBar.vue';
import ImportBookModal from '@/features/library/components/ImportBookModal.vue';
import NewEmptyBookModal from '@/features/library/components/NewEmptyBookModal.vue';
import MoveBooksModal from '@/features/library/components/MoveBooksModal.vue';
import CharCountFilterBar from '@/features/library/components/CharCountFilterBar.vue';
import { DELETE_BOOK_DESCRIPTION } from '@/composables/useBookActions';
import { useBookCollectionActions } from '@/composables/useBookCollectionActions';
import { countPages, pageSlice } from '@/composables/useBookCollectionRoute';
import { useBookStore } from '@/composables/useBookStore';
import { useCharCountIndex } from '@/composables/useCharCountIndex';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useBookPagination } from '@/composables/useBookPagination';
import { useBookSelection } from '@/composables/useBookSelection';
import { useBookBatchOperations } from '@/composables/useBookBatchOperations';
import { useLayerStore } from '@/composables/useLayerStore';
import { useShelfRefresh } from '@/composables/useShelfRefresh';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { useBooksRouteQuery } from '@/features/library/composables/useBooksRouteQuery';
import { useBooksSearch } from '@/features/library/composables/useBooksSearch';
import { useBooksSort, type BookSortKey, type SortOrder } from '@/features/library/composables/useBooksSort';
import { handleLibraryMobileBack } from '@/features/library/utils/mobileBack';
import { filterBooksBySearch } from '@/utils/bookSearch';
import {
  isCharCountInRange,
  isCharCountRangeActive,
  type CharCountRange
} from '@/utils/charCountFilter';
import { hasFileTransfer, readDroppedFiles } from '@/utils/file';
import { getLayerPath, layerPathEquals, normalizeLayerPath } from '@/utils/layers';
import { useI18n } from '@/i18n';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import { isMobileRuntime } from '@/providers/runtime';
import type { BookActivation } from '@/types/bookSelection';
import '@/styles/toolbar-controls.css';

const ROOT_LAYER_LABEL = '/';
const { t } = useI18n();

const { books, loading, error, shelfInitializing, shelfUnreachable, fetchBooks } = useBookStore();
const { layers } = useLayerStore();
const { selectedShelfID } = useShelvesStore();
const { pageSize, setPageSize, PAGE_SIZE_OPTIONS } = useBookPagination();
const {
  selectedLayer,
  page,
  sortBy,
  sortOrder,
  searchQuery,
  charCountRange,
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
const droppedFiles = ref<File[]>([]);
// Matches the isMobileEnv pattern in MainLayout.vue and SettingsPage.vue: the
// runtime does not change during a session, but a computed keeps it consistent
// with the other environment checks used in the template.
const isMobileEnv = computed(() => isMobileRuntime());
const selection = useBookSelection();
const batchOperations = useBookBatchOperations();
const moveBooksModalOpen = ref(false);
const trashBooksModalOpen = ref(false);
const downloadBatchOpen = ref(false);
const downloadBatchRunning = ref(false);
const downloadBatchPercentage = ref(0);
const downloadBatchSucceeded = ref(0);
const downloadBatchTotal = ref(0);
const downloadBatchFailures = ref<Array<{ id: string; title: string; message: string }>>([]);
const selectionEnabled = computed(() => isMobileEnv.value || !readOnly.value);
const layerOptions = computed(() => [...new Set(layers.value.filter((layer) => layer && layer !== '/'))].sort());
const visibleBookIds = computed(() => visibleBooks.value.map((book) => book.id));
const downloadBatchStatusText = computed(() => {
  if (downloadBatchRunning.value) return t('bookCollection.selection.processing');
  if (downloadBatchFailures.value.length === 0) {
    return t('bookCollection.selection.downloadComplete', { count: downloadBatchSucceeded.value });
  }
  return t('bookCollection.selection.downloadPartial', {
    succeeded: downloadBatchSucceeded.value,
    failed: downloadBatchFailures.value.length
  });
});

// Character counts are not part of the shared listing: asking for them makes
// the backend open every book's current source, so they are fetched lazily and
// only while a character-count range is actually set.
const charCountIndex = useCharCountIndex();

// Hidden on a backend that cannot afford the counts: pCloud would have to read
// each book's source meta.json over the network to answer includeCharCount,
// which is why the maintenance page that used to own this filter was blocked
// there too. A phone pointed at a self-hosted server is not in that position,
// so it keeps the filter.
const charCountFilterSupported = computed(
  () => getBookshelfProvider().supportsCharCountListing?.() !== false
);

async function reloadBooks(): Promise<void> {
  booksLoaded.value = false;
  await fetchBooks();
  booksLoaded.value = true;
  // A no-op once the counts are cached, so navigating between layers does not
  // pay for them again: they are keyed by book ID and do not depend on a layer.
  await loadCharCountsIfNeeded();
}

async function loadCharCountsIfNeeded(): Promise<void> {
  if (!charCountFilterSupported.value || !isCharCountRangeActive(charCountRange.value)) {
    return;
  }
  await charCountIndex.load();
}

// Books added since the counts were cached are absent from the index and read
// as unknown, so an import is the one listing change that invalidates it.
async function reloadBooksAfterImport(): Promise<void> {
  charCountIndex.invalidate();
  await reloadBooks();
}

// Only a backend whose listing the user has to update themselves (pCloud)
// reports support; everywhere else the bar stays hidden.
const shelfRefresh = useShelfRefresh();
const lastSyncedLabel = computed(() =>
  shelfRefresh.lastSyncedAt.value === null
    ? t('library.neverSynced')
    : t('library.lastSynced', { time: new Date(shelfRefresh.lastSyncedAt.value).toLocaleString() })
);

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
    void reloadBooks();
  }
});

// isImportModalOpen comes straight off ?import=1 (useBooksRouteQuery.ts), which
// the /import route redirects to. Every other way of opening an import flow is
// already guarded, so without this a direct link would present a full import
// form on a client that cannot write.
const importModalOpen = computed(() => isImportModalOpen.value && !readOnly.value);

function selectedBooks(): Book[] {
  return books.value.filter((book) => selection.selectedIds.value.has(book.id));
}

function selectedTitles(): Record<string, string> {
  return Object.fromEntries(selectedBooks().map((book) => [book.id, book.title]));
}

function onBookActivate(payload: BookActivation): void {
  if (batchOperations.running.value || downloadBatchRunning.value) return;
  if (!selectionEnabled.value) {
    openDetail(payload.id);
    return;
  }
  if (!isMobileEnv.value && payload.shiftKey) {
    selection.selectRange(visibleBookIds.value, payload.id);
    return;
  }
  if (selection.active.value || payload.metaKey || payload.ctrlKey) {
    selection.toggle(payload.id);
    return;
  }
  openDetail(payload.id);
}

function onToggleSelection(id: string): void {
  if (!batchOperations.running.value && !downloadBatchRunning.value) selection.toggle(id);
}

function onLongPress(id: string): void {
  if (isMobileEnv.value && !downloadBatchRunning.value) selection.toggle(id);
}

function selectVisibleBooks(): void {
  if (!batchOperations.running.value && !downloadBatchRunning.value) selection.selectAll(visibleBookIds.value);
}

function openBatchMove(): void {
  if (!isMobileEnv.value && selection.active.value && !readOnly.value) moveBooksModalOpen.value = true;
}

function openBatchTrash(): void {
  if (!isMobileEnv.value && selection.active.value && !readOnly.value) trashBooksModalOpen.value = true;
}

function submitBatchMove(targetLayer: string): void {
  moveBooksModalOpen.value = false;
  const ids = [...selection.selectedIds.value];
  void batchOperations.startMove(ids, targetLayer.split('/').filter(Boolean), selectedTitles());
}

function submitBatchTrash(): void {
  trashBooksModalOpen.value = false;
  const ids = [...selection.selectedIds.value];
  void batchOperations.startTrash(ids, selectedTitles());
}

async function startBatchDownload(): Promise<void> {
  const provider = getBookshelfProvider();
  if (!provider.downloadBook || downloadBatchRunning.value) return;
  const targets = selectedBooks();
  if (targets.length === 0) return;

  downloadBatchOpen.value = true;
  downloadBatchRunning.value = true;
  downloadBatchPercentage.value = 0;
  downloadBatchSucceeded.value = 0;
  downloadBatchTotal.value = targets.length;
  downloadBatchFailures.value = [];

  for (let index = 0; index < targets.length; index += 1) {
    const book = targets[index];
    try {
      if (book.download_state !== 'downloaded') await provider.downloadBook(book.id);
      downloadBatchSucceeded.value += 1;
    } catch {
      downloadBatchFailures.value.push({
        id: book.id,
        title: book.title,
        message: t('bookCollection.selection.failureCodes.download_failed')
      });
    }
    downloadBatchPercentage.value = ((index + 1) / targets.length) * 100;
  }

  downloadBatchRunning.value = false;
  const failedVisible = new Set(downloadBatchFailures.value.map((failure) => failure.id).filter((id) => visibleBookIds.value.includes(id)));
  if (failedVisible.size > 0) selection.replace(failedVisible);
  else selection.clear();
  await reloadBooks();
}

function closeDownloadBatch(): void {
  if (!downloadBatchRunning.value) downloadBatchOpen.value = false;
}

function onSelectionKeydown(event: KeyboardEvent): void {
  if (!selection.active.value || batchOperations.running.value || downloadBatchRunning.value) return;
  const target = event.target;
  if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement || (target instanceof HTMLElement && target.isContentEditable)) return;
  if (event.key === 'Escape') {
    event.preventDefault();
    selection.clear();
  } else if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'a') {
    event.preventDefault();
    selection.selectAll(visibleBookIds.value);
  }
}

let mobileBackHandle: { remove: () => Promise<void> } | null = null;

async function installMobileBackHandler(): Promise<void> {
  if (!isMobileEnv.value) return;
  const { App } = await import('@capacitor/app');
  mobileBackHandle = await App.addListener('backButton', (event) => {
    handleLibraryMobileBack(event, {
      selectionActive: selection.active.value,
      downloadRunning: downloadBatchRunning.value,
      clearSelection: selection.clear,
      goBack: () => window.history.back(),
      exitApp: () => App.exitApp()
    });
  });
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

const charCountFilterActive = computed(
  () => charCountFilterSupported.value && isCharCountRangeActive(charCountRange.value)
);

// charCountRange is a fresh object on every recompute, so watching it directly
// would fire on any unrelated query change. This collapses it to a value that
// only changes when a bound does.
const charCountKey = computed(
  () => `${charCountRange.value.min ?? ''}:${charCountRange.value.max ?? ''}`
);

function charCountOf(book: Book): number | undefined {
  return book.char_count ?? charCountIndex.counts.value.get(book.id);
}

// Only applied once the counts have arrived: filtering against an empty index
// would read every book as zero characters and briefly show the wrong set.
function matchesCharCount(book: Book): boolean {
  if (!charCountFilterActive.value || !charCountIndex.ready.value) {
    return true;
  }
  return isCharCountInRange(charCountOf(book), charCountRange.value);
}

const searchedBooks = computed(() => filterBooksBySearch(books.value, committedSearch.value));

// The set the search and layer alone produce. Kept separate from filteredBooks
// so the empty state can tell "this layer/search has nothing" apart from "the
// character range excluded everything it found".
const layerFilteredBooks = computed(() => searchedBooks.value.filter((book) => matchesLayer(book)));
const filteredBooks = computed(() => layerFilteredBooks.value.filter((book) => matchesCharCount(book)));
const {
  SORT_OPTIONS,
  sortedBooks
} = useBooksSort(filteredBooks, sortBy, sortOrder);

const unknownCharCountCount = computed(() => {
  if (!charCountFilterActive.value || !charCountIndex.ready.value) {
    return 0;
  }
  return filteredBooks.value.filter((book) => charCountOf(book) === undefined).length;
});

// The counts are a second request, so the first one keeps the page reporting
// itself as loading rather than briefly showing an unfiltered list. A later
// refresh keeps the previous counts on screen instead, which is what stops the
// toolbar - and any refresh progress it is showing - from being torn down.
const collectionLoading = computed(
  () => loading.value
    || (charCountFilterActive.value && charCountIndex.loading.value && !charCountIndex.ready.value)
);

// Reported next to the shelf-refresh error rather than through
// BookCollectionPage, which would replace the list with the message: the books
// are still readable when only their character counts failed to load.
const charCountError = computed(() =>
  charCountFilterActive.value ? charCountIndex.error.value : ''
);

const total = computed(() => filteredBooks.value.length);
const totalPages = computed(() => countPages(total.value, pageSize.value));

// Sliced from sortedBooks, counted from filteredBooks — the sort reorders the
// same set, so both lengths agree.
const visibleBooks = computed(() => pageSlice(sortedBooks.value, page.value, pageSize.value));

const showLayerEmptyState = computed(() => {
  return books.value.length > 0 && !!selectedLayer.value && layerFilteredBooks.value.length === 0;
});

// Ordered narrowest cause first: the search and the layer are checked against
// the set that precedes the character range, so the range is only blamed when
// it is what emptied a set the search and layer had already filled.
const emptyMessage = computed(() => {
  const q = committedSearch.value.trim();
  if (q && layerFilteredBooks.value.length === 0 && !loading.value) {
    const layerSuffix = selectedLayer.value
      ? t('common.inLayer', { layer: selectedLayerTitle.value })
      : '';
    return t('library.empty.noBooksFound', { query: q, layerSuffix });
  }
  if (showLayerEmptyState.value) {
    return t('library.empty.noBooksInLayer', { layer: selectedLayerTitle.value });
  }
  if (
    charCountFilterActive.value
    && charCountIndex.ready.value
    && filteredBooks.value.length === 0
    && !collectionLoading.value
  ) {
    return t('library.empty.noBooksInCharCountRange');
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

function onCharCountRangeChange(nextRange: CharCountRange): void {
  if (nextRange.min === charCountRange.value.min && nextRange.max === charCountRange.value.max) {
    return;
  }

  void replaceBooksQuery({
    layer: selectedLayer.value,
    page: 1,
    search: committedSearch.value,
    sort: sortBy.value,
    order: sortOrder.value,
    charCount: nextRange
  });
}

// The sweep rewrites char_count in each source's meta.json, so the cached
// counts - not the book listing - are what went stale.
function onCharCountStatsRefreshed(): void {
  void charCountIndex.refresh();
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
      // A write like any other import: it creates books in the active shelf,
      // it just takes host paths from the desktop picker instead of an upload.
      // openLocalBookFiles above only opens a dialog, so it stays a read.
      const importResult = await bookshelfWriter().importBooksFromLocalPaths?.(desktopFiles, selectedLayer.value ?? '') ?? null;
      if (importResult) {
        const hasImportedBook = importResult.some((item) => item.id !== undefined && item.id !== '');
        const hasFailedBook = importResult.some((item) => Boolean(item.error));
        if (hasImportedBook) {
          await reloadBooksAfterImport();
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
    await reloadBooksAfterImport();
  }
}

onMounted(() => {
  // Chained, not concurrent: on a first connection the listing itself is what
  // creates the timestamp, so reading it alongside the initial load would find
  // nothing and leave the toolbar saying "never updated" for the whole session.
  void reloadBooks().then(() => shelfRefresh.loadLastSyncedAt());
  document.addEventListener('dragover', onDocumentDragOver);
  document.addEventListener('drop', onDocumentDrop);
  document.addEventListener('keydown', onSelectionKeydown);
  void installMobileBackHandler();
});

onBeforeUnmount(() => {
  document.removeEventListener('dragover', onDocumentDragOver);
  document.removeEventListener('drop', onDocumentDrop);
  document.removeEventListener('keydown', onSelectionKeydown);
  void mobileBackHandle?.remove();
});

watch(selectedLayer, async () => {
  await reloadBooks();
});

watch(
  [selectedLayer, page, pageSize, sortBy, sortOrder, committedSearch, charCountKey, selectedShelfID],
  () => selection.clear()
);

// Setting a range is what pays for the counts; clearing it leaves them cached
// so turning the filter back on does not refetch the whole shelf. The initial
// load is left to reloadBooks() in onMounted, which already covers a direct
// link that arrives with a range in the URL.
watch(charCountFilterActive, (active) => {
  if (active) {
    void charCountIndex.load();
  }
});

watch(
  batchOperations.completionVersion,
  () => {
    const result = batchOperations.lastResult.value;
    if (!result) return;
    const failed = new Set(result.failures.map((failure) => failure.book_id).filter((id) => visibleBookIds.value.includes(id)));
    if (failed.size > 0) selection.replace(failed);
    else selection.clear();
  }
);

// Watch committed search: keep the URL in sync and reset to page 1.
// Filtering itself is a pure computed (searchedBooks) — no refetch needed.
watch(
  committedSearch,
  (newSearch) => {
    void replaceBooksQuery({
      layer: selectedLayer.value,
      page: 1,
      search: newSearch,
      sort: sortBy.value,
      order: sortOrder.value
    });
  }
);

watch(
  [selectedLayer, page, totalPages, booksLoaded],
  ([layer, currentPage, maxPage, hasLoaded]) => {
    const normalizedPage = hasLoaded ? Math.min(currentPage, maxPage) : currentPage;
    const currentSearch = committedSearch.value.trim();

    // Committing a search changes totalPages in the same tick, before the
    // page-1 replace above lands. Normalizing with the stale page here would
    // override that reset, so wait until the route reflects the new search.
    if (currentSearch !== searchQuery.value) {
      return;
    }

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

.download-batch-modal {
  display: grid;
  gap: 12px;
  max-width: 520px;
  padding: 18px;
  width: min(100%, 520px);
}

.download-batch-modal h2,
.download-batch-modal p { margin: 0; }
.download-batch-modal footer { display: flex; justify-content: flex-end; }
.batch-failures { margin: 0; max-height: 180px; overflow: auto; padding-left: 20px; }

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

.shelf-refresh-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  /* Never squeezed: shrinking this bar wraps the button's own label into a
     one-character-per-line column before anything else gives way. */
  flex: 0 0 auto;
}

.shelf-refresh-button {
  white-space: nowrap;
}

.shelf-refresh-status {
  color: var(--muted, #888);
  white-space: nowrap;
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

  /* A line of its own, and free to break between the button and the
     timestamp: at 360px the two together are wider than the viewport. */
  .shelf-refresh-bar {
    flex: 1 1 100%;
    flex-wrap: wrap;
  }
}
</style>
