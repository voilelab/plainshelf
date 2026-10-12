import { computed, type Ref } from 'vue';
import type { AcceptableValue } from 'reka-ui';
import { normalizeFolderPath } from '@/utils/folders';
import { useI18n } from '@/i18n';
import type { useBooksRouteQuery } from './useBooksRouteQuery';
import { SORT_OPTIONS, type BookSortKey, type SortOrder } from './useBooksSort';

export const ROOT_FOLDER_LABEL = '/';

interface LibraryRouteQuery {
  selectedFolder: Readonly<Ref<string | undefined>>;
  page: Readonly<Ref<number>>;
  sortBy: Readonly<Ref<BookSortKey>>;
  sortOrder: Readonly<Ref<SortOrder>>;
  pushBooksQuery: ReturnType<typeof useBooksRouteQuery>['pushBooksQuery'];
}

/**
 * Folder, breadcrumb, page and sort navigation on the library page. Every
 * change is a route push; the page reacts to the query, not to these handlers.
 */
export function useLibraryNavigation(query: LibraryRouteQuery, setPageSize: (size: number) => void) {
  const { selectedFolder, page, sortBy, sortOrder, pushBooksQuery } = query;
  const { t } = useI18n();

  const isRootFolderSelected = computed(() => selectedFolder.value === ROOT_FOLDER_LABEL);

  const selectedFolderTitle = computed(() => {
    if (!selectedFolder.value) {
      return t('library.allBooks');
    }
    return selectedFolder.value;
  });

  const selectedFolderSegments = computed(() => {
    if (!selectedFolder.value) {
      return [] as string[];
    }
    return selectedFolder.value.split('/').filter((segment) => segment.length > 0);
  });

  function onSelectAllBooks(): void {
    if (!selectedFolder.value && page.value === 1) {
      return;
    }
    void pushBooksQuery({ folder: undefined, page: 1 });
  }

  function onSelectFolder(folder: string): void {
    const trimmed = folder.trim();
    if (trimmed === '') {
      onSelectAllBooks();
      return;
    }

    const normalized = trimmed === ROOT_FOLDER_LABEL ? ROOT_FOLDER_LABEL : normalizeFolderPath(trimmed);

    if (selectedFolder.value === normalized && page.value === 1) {
      return;
    }
    void pushBooksQuery({ folder: normalized, page: 1 });
  }

  function onSelectBreadcrumb(index: number): void {
    const path = selectedFolderSegments.value.slice(0, index + 1).join('/');
    onSelectFolder(path);
  }

  function onPageChange(nextPage: number): void {
    if (nextPage === page.value) {
      return;
    }
    void pushBooksQuery({ folder: selectedFolder.value, page: nextPage });
  }

  function onPageSizeChange(newSize: number): void {
    setPageSize(newSize);
    void pushBooksQuery({ folder: selectedFolder.value, page: 1 });
  }

  function onSortChange(nextSort: BookSortKey): void {
    if (nextSort === sortBy.value && page.value === 1) {
      return;
    }

    void pushBooksQuery({
      folder: selectedFolder.value,
      page: 1,
      sort: nextSort,
      order: sortOrder.value
    });
  }

  // Rendered into the SelectValue slot so the closed trigger follows a locale
  // change. reka-ui snapshots each SelectItemText's text into an option registry
  // at mount, and a runtime i18n switch does not refresh it — the popup options
  // retranslate but the trigger would stay stale until the list is reopened.
  const sortLabel = computed(() => {
    switch (sortBy.value) {
      case 'created_at':
        return t('library.sortBy.created');
      case 'title':
        return t('library.sortBy.title');
      default:
        return t('library.sortBy.updated');
    }
  });

  function onSortSelectChange(value: AcceptableValue): void {
    if (typeof value !== 'string' || !SORT_OPTIONS.includes(value as BookSortKey)) {
      return;
    }

    onSortChange(value as BookSortKey);
  }

  function onOrderChange(nextOrder: SortOrder): void {
    if (nextOrder === sortOrder.value && page.value === 1) {
      return;
    }

    void pushBooksQuery({
      folder: selectedFolder.value,
      page: 1,
      sort: sortBy.value,
      order: nextOrder
    });
  }

  function toggleOrder(): void {
    onOrderChange(sortOrder.value === 'asc' ? 'desc' : 'asc');
  }

  return {
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
  };
}
