const en = {
  app: {
    name: 'PlainShelf',
    mockModeBadge: 'MOCK API MODE',
    desktopHistoryNavigation: 'Desktop history navigation',
    previousPage: 'Previous page',
    nextPage: 'Next page'
  },
  language: {
    label: 'Language',
    en: 'English',
    zhHant: '繁體中文'
  },
  common: {
    retry: 'Retry',
    prev: 'Prev',
    next: 'Next',
    page: 'Page {page} / {total}',
    inLayer: ' in {layer}'
  },
  layout: {
    expandSidebar: 'Expand sidebar',
    collapseSidebar: 'Collapse sidebar',
    sections: {
      layers: 'LAYERS',
      reading: 'READING',
      maintenance: 'MAINTENANCE'
    },
    createLayer: {
      add: 'Add layer',
      cancel: 'Cancel',
      placeholder: 'e.g. programming/rust',
      creating: 'Creating...',
      create: 'Create',
      created: 'Layer created',
      enter: 'Enter',
      loadingLayers: 'Loading layers...'
    },
    deleteLayer: {
      title: 'Delete layer',
      description: 'This will fail if the layer contains books or child layers.',
      failed: 'Failed to delete layer',
      notEmpty:
        'Cannot delete this layer because it is not empty.\nMove books out and delete child layers first.'
    },
    layerErrors: {
      emptyPath: 'Layer path cannot be empty',
      createFailed: 'Failed to create layer'
    },
    moveBookErrors: {
      notFound: 'Book not found.',
      failed: 'Failed to move book.'
    },
    recentlyRead: 'Recently Read',
    trash: 'Trash'
  },
  maintenance: {
    duplicateContent: 'Duplicate Content',
    missingAuthor: {
      title: 'Missing Author',
      empty: 'No books missing author'
    },
    missingCover: {
      title: 'Missing Cover',
      empty: 'No books missing cover'
    },
    missingLanguage: {
      title: 'Missing Language',
      empty: 'No books with missing language.'
    }
  },
  library: {
    allBooks: 'All books',
    searchPlaceholder: 'Search books...',
    clearSearch: 'Clear search',
    search: 'Search',
    sort: 'Sort',
    sortBy: {
      updated: 'Updated',
      created: 'Created',
      title: 'Title'
    },
    order: {
      asc: 'Asc',
      desc: 'Desc'
    },
    import: 'Import ▾',
    importFromFiles: 'Import from files',
    newEmptyBook: 'New empty book',
    empty: {
      noBooksFound: 'No books found for "{query}"{layerSuffix}.',
      noBooksInLayer: 'No books in {layer}.',
      noBooksYet: 'No books yet.'
    },
    titleSearch: 'Search',
    titleLayer: 'Layer'
  },
  bookCollection: {
    loadingBooks: 'Loading books...',
    booksCount: '{count} books',
    viewMode: {
      list: 'List',
      card: 'Card',
      title: 'Title'
    }
  },
  pagination: {
    perPage: 'Per page',
    booksSuffix: ' books'
  },
  deleteModal: {
    closeLabel: 'Close delete confirmation dialog',
    title: 'Confirm delete',
    description: 'This cannot be undone.',
    confirm: 'Delete',
    cancel: 'Cancel',
    busy: 'Deleting...',
    question: 'Delete {itemName}?'
  },
  readHistory: {
    title: 'Recently Read',
    empty: 'No reading history yet. Open a book in the reader to see it here.',
    clear: 'Clear history',
    clearing: 'Clearing...',
    loadFailed: 'Failed to load reading history',
    clearFailed: 'Failed to clear reading history'
  },
  trash: {
    title: 'Trash',
    loading: 'Loading trashed books...',
    empty: 'Trash is empty.',
    loadFailed: 'Failed to load trashed books',
    restoreFailed: 'Failed to restore book',
    permanentDeleteFailed: 'Failed to permanently delete book',
    columns: {
      title: 'Title',
      authors: 'Authors',
      originalLayer: 'Original layer',
      originalPath: 'Original path',
      deletedAt: 'Deleted at',
      bookId: 'Book ID',
      actions: 'Actions'
    },
    actions: {
      restore: 'Restore',
      permanentDelete: 'Delete permanently'
    },
    permanentDelete: {
      title: 'Delete permanently',
      description: 'This permanently removes all data and cannot be undone.',
      confirm: 'Delete permanently',
      busy: 'Deleting permanently...'
    }
  },
  reader: {
    backToDetail: 'Back to detail',
    title: 'Reader',
    progress: 'Progress: {percent}%',
    loadingContent: 'Loading content...',
    actionsLabel: 'Reader actions',
    decreaseFontSize: 'Decrease font size',
    increaseFontSize: 'Increase font size',
    showChapters: 'Show chapters',
    splitSettings: 'Split settings',
    saveBookmark: 'Save bookmark',
    savingBookmark: 'Saving bookmark'
  }
} as const;

export default en;
