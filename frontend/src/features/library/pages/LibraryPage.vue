<template>
  <div>
    <MetaEditorModal
      :open="metadataEditorOpen"
      :book-id="selectedMetadataBookId"
      @close="closeMetadataEditor"
      @saved="onMetadataSaved"
    />
    <MoveBooksModal
      :open="moveBooksModalOpen"
      :count="selection.count.value"
      :options="folderOptions"
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
      :title="selectedFolderTitle"
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
      @edit="openMetadataEditor"
      @read="goRead"
      @open-book-folder="onOpenBookFolder"
      @download="onDownloadBook"
      @delete="onRequestDeleteBook"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    >
      <template #title-meta>
        <template v-if="isRootFolderSelected">
          {{ ROOT_FOLDER_LABEL }}
        </template>
        <template v-else-if="selectedFolderSegments.length > 0">
          <button type="button" class="breadcrumb-link" @click="onSelectAllBooks">{{ t('library.allBooks') }}</button>
          <span class="breadcrumb-separator" aria-hidden="true">/</span>
          <template v-for="(segment, index) in selectedFolderSegments" :key="`${segment}-${index}`">
            <button
              type="button"
              class="breadcrumb-link"
              @click="onSelectBreadcrumb(index)"
            >
              {{ segment }}
            </button>
            <span
              v-if="index < selectedFolderSegments.length - 1"
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
        <div class="toolbar-bar sort-bar">
          <label class="toolbar-label sort-label" for="books-sort">{{ t('library.sort') }}</label>
          <SelectRoot :model-value="sortBy" @update:model-value="onSortSelectChange">
            <SelectTrigger id="books-sort" class="toolbar-control toolbar-select sort-select">
              <SelectValue>{{ sortLabel }}</SelectValue>
            </SelectTrigger>
            <SelectPortal>
              <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
                <SelectViewport>
                  <SelectItem class="reka-menu-item" value="updated_at">
                    <SelectItemText>{{ t('library.sortBy.updated') }}</SelectItemText>
                  </SelectItem>
                  <SelectItem class="reka-menu-item" value="created_at">
                    <SelectItemText>{{ t('library.sortBy.created') }}</SelectItemText>
                  </SelectItem>
                  <SelectItem class="reka-menu-item" value="title">
                    <SelectItemText>{{ t('library.sortBy.title') }}</SelectItemText>
                  </SelectItem>
                </SelectViewport>
              </SelectContent>
            </SelectPortal>
          </SelectRoot>
          <button
            type="button"
            class="button toolbar-control toolbar-button toolbar-regular sort-order-btn"
            @click="toggleOrder"
          >
            {{ sortOrder === 'asc' ? t('library.order.asc') : t('library.order.desc') }}
          </button>
        </div>
        <FilterPanel
          :books="books"
          :read-only="readOnly"
          :char-count-supported="charCountFilterSupported"
          :char-count-unknown-count="unknownCharCountCount"
          :char-count-refresh-running="contentStatsRunning"
          :char-count-refresh-label="contentStatsLabel"
          :char-count-refresh-outcome="contentStatsOutcome"
          :char-count-refresh-error="contentStatsError"
          @refresh-stats="startContentStatsRefresh"
        />
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
        <div class="toolbar-actions-group">
          <div v-if="shelfRefresh.supported" class="toolbar-bar shelf-refresh-bar">
            <button
              type="button"
              class="button toolbar-control toolbar-button toolbar-regular shelf-refresh-button"
              :disabled="shelfRefresh.refreshing.value"
              @click="shelfRefresh.refresh"
            >
              {{ shelfRefresh.refreshing.value ? t('library.refreshingShelf') : t('library.refreshShelf') }}
            </button>
            <span v-if="lastSyncedLabel" class="toolbar-label shelf-refresh-status">{{ lastSyncedLabel }}</span>
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
        </div>
      </template>

      <template #filters>
        <ActiveFilterChips />
      </template>
    </BookCollectionPage>

    <ImportBookModal
      :open="importModalOpen"
      :current-folder-path="selectedFolder"
      :dropped-files="droppedFiles"
      :local-paths="desktopImportPaths"
      @close="closeImportModal"
      @imported="onImported"
    />
    <NewEmptyBookModal
      :open="isNewEmptyBookModalOpen"
      :current-folder-path="selectedFolder"
      @close="closeNewEmptyBookModal"
      @imported="onImported"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport
} from 'reka-ui';
import type { Book } from '@/types/book';
import BookCollectionPage from '@/components/BookCollectionPage.vue';
import DeleteModal from '@/components/DeleteModal.vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import BaseDialog from '@/components/BaseDialog.vue';
import ProgressBar from '@/components/ProgressBar.vue';
import ImportBookModal from '@/features/library/components/ImportBookModal.vue';
import MetaEditorModal from '@/features/library/components/MetaEditorModal.vue';
import NewEmptyBookModal from '@/features/library/components/NewEmptyBookModal.vue';
import MoveBooksModal from '@/features/library/components/MoveBooksModal.vue';
import FilterPanel from '@/features/library/components/FilterPanel.vue';
import ActiveFilterChips from '@/features/library/components/ActiveFilterChips.vue';
import { DELETE_BOOK_DESCRIPTION } from '@/composables/useBookActions';
import { useBookCollectionActions } from '@/composables/useBookCollectionActions';
import { countPages, pageSlice } from '@/composables/useBookCollectionRoute';
import { useBookStore } from '@/composables/useBookStore';
import { useCharCountIndex } from '@/composables/useCharCountIndex';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useBookPagination } from '@/composables/useBookPagination';
import { useFolderStore } from '@/composables/useFolderStore';
import { useShelfRefresh } from '@/composables/useShelfRefresh';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { useBooksRouteQuery } from '@/features/library/composables/useBooksRouteQuery';
import { useBooksSearch } from '@/features/library/composables/useBooksSearch';
import { useBooksSort } from '@/features/library/composables/useBooksSort';
import { useContentStatsRefresh } from '@/features/library/composables/useContentStatsRefresh';
import { useLibraryImport } from '@/features/library/composables/useLibraryImport';
import { ROOT_FOLDER_LABEL, useLibraryNavigation } from '@/features/library/composables/useLibraryNavigation';
import { useLibrarySelection } from '@/features/library/composables/useLibrarySelection';
import { useMetadataEditorModal } from '@/features/library/composables/useMetadataEditorModal';
import {
  BOOK_FILTERS,
  PANEL_BOOK_FILTERS,
  charCountFilter,
  foldersFilter,
  searchFilter
} from '@/utils/bookFilters/registry';
import {
  applyBookFilters,
  soleBlockingFilter,
  type ActiveBookFilter
} from '@/utils/bookFilters/apply';
import { filterValueLabel } from '@/features/library/utils/filterLabels';
import { isCharCountRangeActive } from '@/utils/charCountFilter';
import { useI18n } from '@/i18n';
import { getBookshelfProvider } from '@/providers';
import { isMobileRuntime } from '@/providers/runtime';
import '@/styles/toolbar-controls.css';

const { t } = useI18n();
const route = useRoute();

const { books, loading, error, shelfInitializing, shelfUnreachable, fetchBooks } = useBookStore();
const { folders } = useFolderStore();
const { selectedShelfID } = useShelvesStore();
const { pageSize, setPageSize, PAGE_SIZE_OPTIONS } = useBookPagination();
const {
  selectedFolder,
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
// Matches the isMobileEnv pattern in MainLayout.vue and SettingsPage.vue: the
// runtime does not change during a session, but a computed keeps it consistent
// with the other environment checks used in the template.
const isMobileEnv = computed(() => isMobileRuntime());
const folderOptions = computed(() => [...new Set(folders.value.filter((folder) => folder && folder !== '/'))].sort());
const visibleBookIds = computed(() => visibleBooks.value.map((book) => book.id));

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
  // A no-op once the counts are cached, so navigating between folders does not
  // pay for them again: they are keyed by book ID and do not depend on a folder.
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

// Every backend whose listing can lag the shelf reports support: pCloud, whose
// listing the user updates themselves, and a server, which finds an externally
// added book only on its next `scan_interval` walk.
const shelfRefresh = useShelfRefresh();

// Only durable state sits beside the button. A backend that dates its stored
// listing (pCloud) says when; a server has no such date and reports what its
// walk found in a toast instead, so that a count of five digits cannot widen
// the toolbar.
const lastSyncedLabel = computed(() => {
  if (!shelfRefresh.tracksLastSynced) {
    return '';
  }
  return shelfRefresh.lastSyncedAt.value === null
    ? t('library.neverSynced')
    : t('library.lastSynced', { time: new Date(shelfRefresh.lastSyncedAt.value).toLocaleString() });
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
} = useBookCollectionActions({
  books,
  onDeleted: () => {
    void reloadBooks();
  }
});

const {
  selectedBookId: selectedMetadataBookId,
  open: metadataEditorOpen,
  openEditor: openMetadataEditor,
  closeEditor: closeMetadataEditor,
  onSaved: onMetadataSaved
} = useMetadataEditorModal({ books, readOnly, refresh: reloadBooks });

// isImportModalOpen comes straight off ?import=1 (useBooksRouteQuery.ts), which
// the /import route redirects to. Every other way of opening an import flow is
// already guarded, so without this a direct link would present a full import
// form on a client that cannot write.
const importModalOpen = computed(() => isImportModalOpen.value && !readOnly.value);

const {
  selection,
  batchOperations,
  selectionEnabled,
  moveBooksModalOpen,
  trashBooksModalOpen,
  downloadBatchOpen,
  downloadBatchRunning,
  downloadBatchPercentage,
  downloadBatchFailures,
  downloadBatchStatusText,
  onBookActivate,
  onToggleSelection,
  onLongPress,
  selectVisibleBooks,
  openBatchMove,
  openBatchTrash,
  submitBatchMove,
  submitBatchTrash,
  startBatchDownload,
  closeDownloadBatch
} = useLibrarySelection({ books, visibleBookIds, isMobileEnv, readOnly, openDetail, reloadBooks });

const {
  isNewEmptyBookModalOpen,
  droppedFiles,
  desktopImportPaths,
  openImportFromFiles,
  openNewEmptyBookModal,
  closeNewEmptyBookModal,
  closeImportModal,
  onImported
} = useLibraryImport({
  readOnly,
  isImportModalOpen,
  openImportModalQuery,
  closeImportModalQuery,
  reloadAfterImport: reloadBooksAfterImport
});

const {
  isRootFolderSelected,
  selectedFolderTitle,
  selectedFolderSegments,
  onSelectAllBooks,
  onSelectBreadcrumb,
  onPageChange,
  onPageSizeChange,
  sortLabel,
  onSortSelectChange,
  toggleOrder
} = useLibraryNavigation({ selectedFolder, page, sortBy, sortOrder, pushBooksQuery }, setPageSize);


const pageTitleSegments = computed(() => {
  const query = searchQuery.value.trim();
  if (query) {
    return [t('library.titleSearch'), query, t('app.name')] as const;
  }

  const folderName = selectedFolder.value?.trim();
  if (folderName && folderName !== ROOT_FOLDER_LABEL) {
    return [t('library.titleFolder'), folderName, t('app.name')] as const;
  }

  return [t('app.name')] as const;
});

useDocumentTitle(pageTitleSegments);

const charCountFilterActive = computed(
  () => charCountFilterSupported.value && isCharCountRangeActive(charCountRange.value)
);

// The character count lives only in the lazy index, so a book carries it into a
// predicate through this: `char_count` already present (mock data, or a listing
// that included it) is left as-is; otherwise the index supplies it. Other
// filters ignore the extra field.
function augmentBook(book: Book): Book {
  return book.char_count !== undefined
    ? book
    : { ...book, char_count: charCountIndex.counts.value.get(book.id) };
}

function charCountOf(book: Book): number | undefined {
  return book.char_count ?? charCountIndex.counts.value.get(book.id);
}

// Every condition the page actually applies, parsed once from the registry.
// Search uses the *committed* text (not the transient input box), a condition an
// unsupported backend cannot answer is dropped, and the character range only
// joins once its lazy index is ready to decide books — until then the page is in
// its loading state rather than briefly showing an unfiltered list.
const activeFilters = computed<ActiveBookFilter[]>(() => {
  const result: ActiveBookFilter[] = [];
  for (const filter of BOOK_FILTERS) {
    if (filter.supported?.() === false) {
      continue;
    }
    const value = filter.key === searchFilter.key ? committedSearch.value : filter.parse(route.query);
    if (!filter.isActive(value)) {
      continue;
    }
    if (filter.key === charCountFilter.key && !charCountIndex.ready.value) {
      continue;
    }
    result.push({ filter, value });
  }
  return result;
});

// A stable string that changes only when an active panel condition changes, so
// the selection-clearing watcher fires when the visible set does. Without it,
// changing a panel filter reuses LibraryPage and leaves now-hidden books
// selected — and a batch move/trash would then act on books the user cannot see.
const panelFiltersKey = computed(() =>
  PANEL_BOOK_FILTERS.map((filter) => {
    const value = filter.parse(route.query);
    return filter.isActive(value) ? `${filter.key}=${JSON.stringify(filter.serialize(value))}` : '';
  })
    .filter(Boolean)
    .join('&')
);

const filteredBooks = computed(() => applyBookFilters(books.value, activeFilters.value, augmentBook));
const { sortedBooks } = useBooksSort(filteredBooks, sortBy, sortOrder);

const unknownCharCountCount = computed(() => {
  if (!charCountFilterActive.value || !charCountIndex.ready.value) {
    return 0;
  }
  return filteredBooks.value.filter((book) => charCountOf(book) === undefined).length;
});

// The recompute sweep is tracked here, on the always-mounted page, so closing
// the filter panel mid-sweep does not abandon the poll or the index refresh it
// triggers when the sweep settles.
const {
  running: contentStatsRunning,
  label: contentStatsLabel,
  outcome: contentStatsOutcome,
  error: contentStatsError,
  start: startContentStats
} = useContentStatsRefresh(unknownCharCountCount, () => {
  void charCountIndex.refresh();
});

function startContentStatsRefresh(): void {
  if (readOnly.value) {
    return;
  }
  void startContentStats();
}

// Only the book-list load drives the full loading state. The character counts
// are a second request, but making them tear the content down would unmount the
// filter panel the user is setting the range in (BookCollectionPage swaps the
// whole header/toolbar/list for a loading placeholder). While the counts are in
// flight the character range simply isn't applied yet (see activeFilters), so
// the list shows the other conditions' result until the counts arrive — the
// panel, its chips, and any refresh progress stay mounted throughout.
const collectionLoading = computed(() => loading.value);

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

// The single condition to blame for an empty list, derived generically from the
// predicates (see soleBlockingFilter) rather than a hand-written cascade: it is
// the one condition whose removal alone would bring books back. `null` when the
// shelf itself is empty, when the list is non-empty, or when no single condition
// is solely responsible.
const blamedFilter = computed(() => {
  if (collectionLoading.value || books.value.length === 0 || filteredBooks.value.length > 0) {
    return null;
  }
  return soleBlockingFilter(books.value, activeFilters.value, augmentBook);
});

const emptyMessage = computed(() => {
  if (books.value.length === 0) {
    return t('library.empty.noBooksYet');
  }
  // Blame is generic; only the wording per cause is hand-written, because
  // search, folder, and the character range each read better than a generic line.
  const blamed = blamedFilter.value;
  if (blamed) {
    switch (blamed.filter.key) {
      case searchFilter.key: {
        const folderSuffix = selectedFolder.value
          ? t('common.inFolder', { folder: selectedFolderTitle.value })
          : '';
        return t('library.empty.noBooksFound', { query: String(blamed.value), folderSuffix });
      }
      case foldersFilter.key:
        return t('library.empty.noBooksInFolder', { folder: selectedFolderTitle.value });
      case charCountFilter.key:
        return t('library.empty.noBooksInCharCountRange');
      default:
        return t('library.empty.noBooksForCondition', {
          condition: filterValueLabel(blamed.filter, blamed.value, t)
        });
    }
  }
  // Books exist but several conditions together emptied the list, so no single
  // one can be singled out.
  if (activeFilters.value.length > 0 && filteredBooks.value.length === 0 && !collectionLoading.value) {
    return t('library.empty.noBooksMatchFilters');
  }
  return t('library.empty.noBooksYet');
});

onMounted(() => {
  // Chained, not concurrent: on a first connection the listing itself is what
  // creates the timestamp, so reading it alongside the initial load would find
  // nothing and leave the toolbar saying "never updated" for the whole session.
  void reloadBooks().then(() => shelfRefresh.loadLastSyncedAt());
});


watch(selectedFolder, async () => {
  await reloadBooks();
});

watch(
  [selectedFolder, page, pageSize, sortBy, sortOrder, committedSearch, panelFiltersKey, selectedShelfID],
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


// Watch committed search: keep the URL in sync and reset to page 1.
// Filtering itself is a pure computed (searchedBooks) — no refetch needed.
watch(
  committedSearch,
  (newSearch) => {
    void replaceBooksQuery({
      folder: selectedFolder.value,
      page: 1,
      search: newSearch,
      sort: sortBy.value,
      order: sortOrder.value
    });
  }
);

watch(
  [selectedFolder, page, totalPages, booksLoaded],
  ([folder, currentPage, maxPage, hasLoaded]) => {
    const normalizedPage = hasLoaded ? Math.min(currentPage, maxPage) : currentPage;
    const currentSearch = committedSearch.value.trim();

    // Committing a search changes totalPages in the same tick, before the
    // page-1 replace above lands. Normalizing with the stale page here would
    // override that reset, so wait until the route reflects the new search.
    if (currentSearch !== searchQuery.value) {
      return;
    }

    if (isBooksQueryNormalized({
      folder,
      page: normalizedPage,
      search: currentSearch,
      sort: sortBy.value,
      order: sortOrder.value
    })) {
      return;
    }

    void replaceBooksQuery({
      folder,
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

.sort-bar {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* The Reka SelectTrigger renders a <button>, so it needs the alignment and
   font a native <select> got for free while keeping the toolbar-control sizing. */
.sort-select {
  align-items: center;
  cursor: pointer;
  display: inline-flex;
  font-family: inherit;
  min-width: 100px;
  text-align: left;
}

.sort-order-btn {
  min-width: 64px;
}

/* Import and rescan travel together as one cluster at the end of the row, so a
   wrap breaks between groups rather than splitting these two apart. */
.toolbar-actions-group {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  gap: 10px;
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
