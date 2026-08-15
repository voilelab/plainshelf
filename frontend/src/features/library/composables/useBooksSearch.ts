import { ref } from 'vue';

export function useBooksSearch(initialSearchValue = '') {
  const searchInputValue = ref<string>(initialSearchValue);
  const committedSearch = ref<string>(searchInputValue.value.trim());

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

  return {
    searchInputValue,
    committedSearch,
    commitSearch,
    onSearchEnter,
    clearSearch
  };
}
