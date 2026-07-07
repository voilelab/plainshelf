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
    cancel: 'Cancel',
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
      maintenance: 'MAINTENANCE',
      admin: 'ADMIN'
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
      shortAction: 'Delete',
      description: 'This will fail if the layer contains books or child layers.',
      failed: 'Failed to delete layer',
      notEmpty:
        'Cannot delete this layer because it is not empty.\nMove books out and delete child layers first.'
    },
    renameLayer: {
      shortAction: 'Rename',
      title: 'Rename layer',
      nameLabel: 'Layer name',
      placeholder: 'Layer name',
      help: 'Current name: {layerName}',
      confirm: 'Rename',
      renaming: 'Renaming...',
      closeLabel: 'Close rename layer dialog',
      invalid: 'Layer name cannot be empty or contain /.',
      failed: 'Failed to rename layer'
    },
    moveLayer: {
      failed: 'Failed to move layer. Drag a layer onto an existing target layer.'
    },
    openLayerFolder: {
      shortAction: 'Open folder',
      failed: 'Failed to open layer folder.'
    },
    layerErrors: {
      emptyPath: 'Layer path cannot be empty',
      createFailed: 'Failed to create layer'
    },
    moveBookErrors: {
      notFound: 'Book not found.',
      failed: 'Failed to move book.'
    },
    shelf: {
      label: 'Shelf',
      loading: 'Loading shelves...',
      failed: 'Failed to load shelves',
      empty: 'No shelves configured',
      unavailableTitle: 'No shelf selected',
      unavailableDescription: 'Configure at least one shelf to browse and read books.',
      addShelf: 'Add shelf',
      addShelfTitle: 'Create shelf',
      addShelfCloseLabel: 'Close create shelf dialog',
      addShelfCancel: 'Cancel',
      addShelfNamePlaceholder: 'Shelf name',
      addShelfDirectoryPlaceholder: 'Directory',
      addShelfBrowse: 'Browse...',
      addShelfSubmit: 'Add',
      addShelfAdding: 'Adding...',
      addShelfFailed: 'Failed to add shelf'
    },
    recentlyRead: 'Recently Read',
    trash: 'Trash',
    adminLogs: 'Logs',
    settings: 'Settings',
    readOnly: {
      banner: 'Read-only mode is enabled. Browsing and reading are available, but write operations are disabled.',
      writeDisabled: 'Server is in read-only mode. Write operations are disabled.'
    }
  },
  settings: {
    title: 'Settings',
    description: 'Manage application options.',
    loadFailed: 'Failed to load settings',
    saveFailed: 'Failed to save setting',
    cover: {
      title: 'Cover'
    },
    coverToJpg: {
      label: 'Convert uploaded covers to JPG',
      description: 'Enable this to convert cover images to JPEG when uploading.'
    },
    readHistory: {
      title: 'Reading history'
    },
    readHistoryLimit: {
      label: 'Reading history limit',
      description: 'Maximum number of recently read books to keep. Use 0 to disable retaining history.',
      invalid: 'Reading history limit must be a non-negative whole number.'
    },
    about: {
      title: 'About',
      version: 'Version'
    },
    shelves: {
      title: 'Shelves',
      loadFailed: 'Failed to load shelves',
      empty: 'No shelves configured.',
      serverManaged: 'Shelves are managed by the server configuration.',
      name: 'Name',
      directory: 'Directory',
      remove: 'Delete',
      removing: 'Removing...',
      removeFailed: 'Failed to remove shelf',
      removeShelfTitle: 'Delete shelf',
      removeConfirmDescription: 'This only removes the shelf from PlainShelf; the directory will not be deleted.',
      addShelf: 'Add shelf',
      addShelfTitle: 'Create shelf',
      addShelfCloseLabel: 'Close create shelf dialog',
      addShelfCancel: 'Cancel',
      addShelfNamePlaceholder: 'Shelf name',
      addShelfDirectoryPlaceholder: 'Directory path',
      addShelfScanIntervalPlaceholder: 'Scan interval (optional, e.g. 10m)',
      addShelfScanIntervalHelp: 'Leave blank to use the default 1 minute scan interval.',
      addShelfBrowse: 'Browse...',
      addShelfSubmit: 'Add shelf',
      addShelfAdding: 'Adding...',
      addShelfFailed: 'Failed to add shelf',
      removeConfirm: 'Remove shelf "{name}"? This only removes it from PlainShelf; the directory will not be deleted.',
      removeConfirmInline: 'Remove?',
      removeConfirmYes: 'Delete shelf',
      modify: 'Modify',
      modifyShelfTitle: 'Modify shelf',
      modifyShelfCloseLabel: 'Close modify shelf dialog',
      modifyShelfSubmit: 'Save',
      modifyShelfSaving: 'Saving...',
      modifyShelfFailed: 'Failed to modify shelf',
      modifyShelfIDLabel: 'ID',
      modifyShelfPathLabel: 'Path',
      modifyShelfNamePlaceholder: 'Shelf name',
      modifyShelfScanIntervalPlaceholder: 'Scan interval (optional, e.g. 10m)',
      modifyShelfScanIntervalHelp: 'Leave blank to use the default 1 minute scan interval.'
    }
  },
  adminLogs: {
    title: 'Logs',
    description: 'Select a log source and date to inspect the log file content.',
    name: 'Name',
    date: 'Date',
    source: 'Source',
    filename: 'Filename',
    empty: 'No log files are available.',
    emptyContent: 'The selected log file is empty.',
    missingForDate: 'No log file is available for {date}.',
    loadingList: 'Loading log files...',
    loadingContent: 'Loading log content...',
    loadFailed: 'Failed to load log files',
    loadContentFailed: 'Failed to load log content'
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
    shelfInitializing: 'Shelf is loading, please wait...',
    shelfUnreachable: 'The shelf is taking too long to respond. It may be unavailable (e.g. SMB mount disconnected).',
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
