import { effectScope, ref } from 'vue';
import { describe, expect, it, vi } from 'vitest';
import type { BookSortKey, SortOrder } from './useBooksSort';
import { useLibraryNavigation } from './useLibraryNavigation';

function setup(folder: string | undefined, page = 1) {
  const pushBooksQuery = vi.fn(() => Promise.resolve());
  const setPageSize = vi.fn();
  const scope = effectScope();
  const nav = scope.run(() => useLibraryNavigation({
    selectedFolder: ref(folder),
    page: ref(page),
    sortBy: ref<BookSortKey>('updated_at'),
    sortOrder: ref<SortOrder>('desc'),
    pushBooksQuery
  }, setPageSize))!;
  return { nav, pushBooksQuery, setPageSize };
}

describe('useLibraryNavigation', () => {
  it('navigates a breadcrumb to the folder prefix ending at that segment', () => {
    const { nav, pushBooksQuery } = setup('a/b/c', 3);

    expect(nav.selectedFolderSegments.value).toEqual(['a', 'b', 'c']);
    nav.onSelectBreadcrumb(1);
    expect(pushBooksQuery).toHaveBeenCalledWith({ folder: 'a/b', page: 1 });
  });

  it('skips navigation that would land on the current folder and page', () => {
    const { nav, pushBooksQuery } = setup(undefined, 1);

    nav.onSelectAllBooks();
    nav.onPageChange(1);
    expect(pushBooksQuery).not.toHaveBeenCalled();
  });

  it('resets to page 1 when the page size, sort or order changes', () => {
    const { nav, pushBooksQuery, setPageSize } = setup('x', 4);

    nav.onPageSizeChange(50);
    expect(setPageSize).toHaveBeenCalledWith(50);
    expect(pushBooksQuery).toHaveBeenLastCalledWith({ folder: 'x', page: 1 });

    nav.onSortSelectChange('title');
    expect(pushBooksQuery).toHaveBeenLastCalledWith({ folder: 'x', page: 1, sort: 'title', order: 'desc' });

    nav.toggleOrder();
    expect(pushBooksQuery).toHaveBeenLastCalledWith({ folder: 'x', page: 1, sort: 'updated_at', order: 'asc' });
  });

  it('ignores a sort value that is not a known key', () => {
    const { nav, pushBooksQuery } = setup('x', 2);

    nav.onSortSelectChange('nonsense');
    expect(pushBooksQuery).not.toHaveBeenCalled();
  });
});
