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
    railNavLabel: 'Sidebar navigation',
    openMenu: 'Open menu',
    closeMenu: 'Close menu',
    sections: {
      layers: 'LAYERS',
      reading: 'READING',
      maintenance: 'MAINTENANCE',
      admin: 'ADMIN'
    },
    sectionToggleLabels: {
      layers: 'Toggle sidebar folders',
      reading: 'Toggle sidebar history',
      maintenance: 'Toggle sidebar maintenance',
      admin: 'Toggle sidebar administration'
    },
    createLayer: {
      add: 'Add layer',
      title: 'New layer',
      nameLabel: 'Layer name',
      namePlaceholder: 'Layer name',
      parentLabel: 'Where',
      rootOption: 'All books (top level)',
      closeLabel: 'Close new layer dialog',
      invalidName: 'Layer name cannot be empty or contain /.',
      creating: 'Creating...',
      create: 'Create',
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
    dashboard: 'Dashboard',
    recentlyRead: 'Recently Read',
    trash: 'Trash',
    downloads: 'Downloads',
    adminLogs: 'Logs',
    settings: 'Settings',
    readOnly: {
      banner: 'Read-only mode is enabled. Browsing and reading are available, but write operations are disabled.',
      writeDisabled: 'Server is in read-only mode. Write operations are disabled.'
    }
  },
  dashboard: {
    title: 'Dashboard',
    refresh: 'Refresh',
    loading: 'Loading dashboard...',
    loadFailed: 'Failed to load dashboard data',
    stats: {
      totalBooks: 'Total Books',
      addedThisMonth: 'Added This Month',
      avgStar: 'Average Rating',
      totalChars: 'Total Characters',
      starBar: '{star} star: {count} book(s)',
      currentStreak: 'Current Streak',
      currentStreakValue: '{days} day(s)'
    },
    tags: {
      title: 'Tags',
      empty: 'No tags yet'
    },
    randomBook: {
      title: 'Random Pick',
      empty: 'No books yet',
      shuffle: 'Shuffle',
      viewDetail: 'View Details',
      readNow: 'Read Now'
    },
    heatmap: {
      title: 'Reading Heatmap',
      empty: 'Start reading to see activity build up here.',
      legendLess: 'Less',
      legendMore: 'More',
      cellLabel: '{date}: {minutes} min read'
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
    reader: {
      title: 'Reader'
    },
    defaultSplitConfig: {
      label: 'Default section split rule',
      description: 'Applied automatically when a book has no explicit split configuration.',
      typeLabel: 'Split type',
      typeNone: 'None',
      typeLineCount: 'Line count',
      typeRegex: 'Regex',
      lineCountLabel: 'Lines per section',
      lineCountPlaceholder: 'e.g. 100',
      regexLabel: 'Regex pattern',
      regexPlaceholder: 'e.g. ^Chapter\\s+\\d+',
      regexHelp: 'Lines matching this pattern start a new section.',
      invalidRegex: 'Invalid regular expression.',
      invalidLineCount: 'Line count must be a positive integer.',
      save: 'Save',
      saving: 'Saving...'
    },
    readHistoryLimit: {
      label: 'Reading history limit',
      description: 'Maximum number of recently read books to keep. Use 0 to disable retaining history.',
      invalid: 'Reading history limit must be a non-negative whole number.'
    },
    about: {
      title: 'About',
      description: 'PlainShelf is a local-first personal reading library for lightweight reading content, with a filesystem-first data model and a web-based reading interface.',
      version: 'Version',
      repository: 'Repository'
    },
    mobileConnect: {
      title: 'Connection',
      description: 'Change the PlainShelf server, access token, or shelf used on this device.',
      open: 'Edit connection'
    },
    downloads: {
      title: 'Downloaded books',
      description: 'Manage books saved to this device for offline reading.',
      open: 'Manage downloads'
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
    },
    lowCharCount: {
      title: 'Low Character Count',
      empty: 'No books under this character count.',
      thresholdLabel: 'Max characters',
      filterDescription: 'Books with {threshold} characters or fewer',
      unknownNote: '{count} with an unknown character count'
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
    },
    contextMenu: {
      read: 'Read',
      openDetail: 'Open Detail',
      openBookFolder: 'Open Book Folder',
      download: 'Download',
      edit: 'Edit',
      delete: 'Delete'
    },
    selection: {
      toolbarLabel: 'Selected books actions',
      selectedCount: '{count} selected',
      selectBook: 'Select {title}',
      selectAll: 'Select page',
      move: 'Move',
      trash: 'Move to trash',
      download: 'Download to device',
      downloading: 'Downloading...',
      moveTitle: 'Move selected books',
      moveTarget: 'Destination',
      rootLayer: 'All books (top level)',
      chooseLayer: 'Choose a destination',
      confirmMove: 'Move {count} books',
      moving: 'Moving...',
      trashTitle: 'Move selected books to trash',
      trashQuestion: 'Move {count} selected books to the trash?',
      trashDescription: 'You can restore these books later from Trash.',
      confirmTrash: 'Move to trash',
      processing: 'Processing books...',
      progressLabel: 'Batch operation progress',
      complete: '{count} books completed.',
      partial: '{succeeded} completed; {failed} failed.',
      failed: 'The batch operation failed.',
      close: 'Close',
      startFailed: 'Failed to start the batch operation',
      pollFailed: 'Lost track of the batch operation progress.',
      downloadComplete: '{count} books downloaded.',
      downloadPartial: '{succeeded} downloaded; {failed} failed.',
      failureCodes: {
        not_found: 'Book not found',
        unsupported_schema: 'Book was created by a newer PlainShelf version',
        move_failed: 'Could not move the book',
        trash_failed: 'Could not move the book to trash',
        download_failed: 'Could not download the book'
      }
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
    },
    emptyAll: {
      action: 'Empty trash',
      title: 'Empty trash',
      question: 'Permanently delete all {count} books in the trash?',
      questionUnknownCount: 'Permanently delete everything in the trash? Books whose metadata cannot be read are not listed above, but will also be removed.',
      description: 'This permanently removes all data and cannot be undone.',
      confirm: 'Empty trash',
      busy: 'Emptying...',
      close: 'Close',
      progressLabel: 'Empty trash progress',
      pending: 'Waiting to start...',
      running: 'Deleting books...',
      completed: 'The trash is now empty.',
      partiallyCompleted: 'Some books could not be deleted. Check the logs for details.',
      failed: 'Could not start deleting. The trash may be unreadable or locked.',
      startFailed: 'Failed to start emptying the trash',
      pollFailed: 'Lost track of the progress. Reload the page to see the current state.'
    }
  },
  downloads: {
    title: 'Downloaded Books',
    description: 'Books saved to this device for offline reading.',
    overview: {
      countLabel: 'Downloaded books',
      count: '{count} downloaded',
      totalSizeLabel: 'Total size',
      deviceStorageLabel: 'Device storage',
      storageUsed: '{used} / {quota} used',
      storageUnsupported: 'This device does not support storage estimates.'
    },
    list: {
      loading: 'Loading downloaded books...',
      loadFailed: 'Failed to load downloaded books',
      empty: 'No books downloaded yet.'
    },
    item: {
      remove: 'Remove download'
    },
    detail: {
      download: 'Download to device',
      downloading: 'Downloading...',
      remove: 'Remove download',
      retry: 'Download failed, retry'
    },
    removeConfirm: {
      title: 'Remove download',
      description: 'This removes the offline copy from this device. You can download it again anytime.',
      confirm: 'Remove download',
      busy: 'Removing...',
      failed: 'Failed to remove download'
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
  },
  mobileConnect: {
    title: 'Connect to PlainShelf',
    description: 'Enter your PlainShelf server address to browse and read your library on this device.',
    serverUrlLabel: 'Server URL',
    serverUrlPlaceholder: 'http://192.168.1.10:20000',
    tokenLabel: 'Access token (optional)',
    tokenPlaceholder: 'Required only for editing',
    tokenHint: 'Reading works without a token. Add one to make changes.',
    loadShelves: 'Load library',
    loadingShelves: 'Connecting…',
    shelfLabel: 'Shelf',
    shelfPlaceholder: 'Select a shelf',
    save: 'Save and continue',
    saving: 'Saving…',
    serverUrlRequired: 'Enter a server URL first.',
    shelfRequired: 'Select a shelf to continue.'
  }
} as const;

export default en;
