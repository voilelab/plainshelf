const en = {
  app: {
    name: 'PlainShelf',
    mockModeBadge: 'MOCK API MODE',
    desktopHistoryNavigation: 'Desktop history navigation',
    previousPage: 'Previous page',
    nextPage: 'Next page',
    renderError: {
      title: 'This page stopped responding',
      description: 'Something went wrong while drawing this page. Reloading usually clears it.',
      reload: 'Reload'
    }
  },
  toast: {
    label: 'Notifications',
    dismiss: 'Dismiss notification'
  },
  language: {
    label: 'Language',
    en: 'English',
    zhHant: '繁體中文',
    // The language a book is written in, distinct from the two keys above,
    // which name the UI locale and stay in their own endonym either way.
    book: {
      unspecified: 'Unspecified',
      zhHant: 'Chinese (Traditional)',
      zhHans: 'Chinese (Simplified)',
      ja: 'Japanese',
      ko: 'Korean',
      en: 'English',
      custom: 'Custom...',
      customPlaceholder: 'e.g. zh-TW, zh-HK, fr, de',
      help: 'Use en, ja, ko, zh-Hant or zh-Hans; any BCP 47 language tag such as zh-TW also works.',
      invalidTag: 'That is not a valid language tag. Use a form like en, ja, zh-Hant or zh-TW.'
    }
  },
  common: {
    retry: 'Retry',
    cancel: 'Cancel',
    confirm: 'Confirm',
    working: 'Working...',
    closeDialog: 'Close confirmation dialog',
    prev: 'Prev',
    next: 'Next',
    page: 'Page {page} / {total}',
    inLayer: ' in {layer}',
    taskStartFailed: 'Failed to start the task',
    taskPollFailed: 'Failed to read task progress'
  },
  layout: {
    expandSidebar: 'Expand sidebar',
    collapseSidebar: 'Collapse sidebar',
    railNavLabel: 'Sidebar navigation',
    layersNavLabel: 'Layers',
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
      createFailed: 'Failed to create layer',
      loadFailed: 'Failed to load layers',
      shelfNotReady: 'The shelf is still starting up and did not become ready.'
    },
    moveBookErrors: {
      notFound: 'Book not found.',
      failed: 'Failed to move book.'
    },
    shelf: {
      label: 'Shelf',
      loading: 'Loading shelves...',
      empty: 'No shelves configured',
      unavailableTitle: 'No shelf selected',
      unavailableDescription: 'Configure at least one shelf to browse and read books.',
      manage: 'Manage shelves'
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
    serverModeLoadFailed: 'Failed to load server mode',
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
    import: {
      title: 'Import'
    },
    epubImport: {
      title: 'EPUB conversion',
      description:
        'EPUB files are converted into a source when imported. The original file is not kept; supported illustrations can be retained by Markdown. These defaults can be overridden for one batch.',
      presetLabel: 'Convert to',
      presetHelp:
        'Markdown stores chapters as H2 headings and keeps supported illustrations. Plain text has no chapter navigation or Markdown images.',
      presetMarkdown: 'Markdown (chapter headings)',
      presetPlain: 'Plain text',
      includeDescriptionLabel: 'Description in the text',
      includeDescriptionHelp:
        'Put the book description at the start of the text as well. It is always saved to the book metadata either way.',
      save: 'Save',
      saving: 'Saving...'
    },
    readHistoryLimit: {
      label: 'Reading history limit',
      description:
        'Maximum number of recently read books to keep on this device. Reading history is stored locally and never sent to the server. Use 0 to disable retaining history.',
      invalid: 'Reading history limit must be a non-negative whole number.'
    },
    about: {
      title: 'About',
      description: 'PlainShelf is a local-first personal reading library for lightweight reading content, with a filesystem-first data model and a web-based reading interface.',
      version: 'Version',
      repository: 'Repository',
      thirdPartyFonts: 'Third-party font licenses',
      fontAttribution: 'Google Inc. · SIL Open Font License 1.1 · package version 5.3.0',
      source: 'Source',
      license: 'Full license text',
      licenseTitle: '{font} license',
      licenseLoading: 'Loading license text…',
      licenseLoadFailed: 'The bundled license text could not be loaded.',
      licenseClose: 'Close'
    },
    mobileConnect: {
      title: 'Shelves',
      description: 'Every library this device can open, from PlainShelf servers or from pCloud.',
      open: 'Manage shelves'
    },
    downloads: {
      title: 'Downloaded books',
      description: 'Manage books saved to this device for offline reading.',
      open: 'Manage downloads'
    },
    bookCache: {
      title: 'Mobile book cache',
      description:
        'Each shelf keeps a listing of its books in its app folder so a phone reading the same shelf from cloud storage does not have to scan it. It is refreshed automatically; update it now if a phone should see recent changes right away.',
      export: 'Update now',
      exporting: 'Updating…',
      success: 'Book cache updated.',
      failed: 'Failed to update the book cache.'
    },
    shelves: {
      title: 'Shelves',
      loadFailed: 'Failed to load shelves',
      empty: 'No shelves configured.',
      serverManaged: 'Shelves are managed by the server configuration.',
      name: 'Name',
      idColumn: 'ID',
      remove: 'Delete',
      removing: 'Removing...',
      removeFailed: 'Failed to remove shelf',
      removeShelfTitle: 'Delete shelf',
      removeConfirmDescription: 'This only removes the shelf from PlainShelf; the directory will not be deleted.',
      addShelf: 'Add shelf',
      addShelfTitle: 'Create shelf',
      addShelfCloseLabel: 'Close create shelf dialog',
      addShelfNamePlaceholder: 'Shelf name',
      addShelfDirectoryPlaceholder: 'Directory path',
      addShelfScanIntervalPlaceholder: 'Scan interval (optional, e.g. 10m)',
      addShelfScanIntervalHelp: 'Leave blank to use the default 1 minute scan interval.',
      addShelfBrowse: 'Browse...',
      addShelfSubmit: 'Add shelf',
      addShelfAdding: 'Adding...',
      addShelfFailed: 'Failed to add shelf',
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
    duplicates: {
      description: 'Maintenance view for books with identical content.',
      scanning: 'Scanning duplicate content groups...',
      empty: 'No duplicate content found.',
      emptyHint: 'Your library looks clean.',
      loadFailed: 'Failed to load duplicate content',
      groupTitle: 'Duplicate Group #{index}',
      groupSummary: '{count} books share identical content',
      open: 'Open',
      delete: 'Delete',
      deleting: 'Deleting...',
      deleteLabel: 'Delete {title}',
      untitledBook: 'book',
      deleteFailed: 'Failed to delete book'
    },
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
    charCount: {
      label: 'Characters',
      minLabel: 'Minimum characters',
      maxLabel: 'Maximum characters',
      minPlaceholder: 'Min',
      maxPlaceholder: 'Max',
      clear: 'Clear character range',
      unknownNote: '{count} with an unknown character count',
      refreshStats: {
        action: 'Update statistics for {count} books',
        busy: 'Updating… {percent}%',
        done: 'Updated {count} books',
        partial: 'Updated {succeeded} books, {failed} failed',
        failed: 'Could not update content statistics.'
      }
    },
    import: 'Import ▾',
    importFromFiles: 'Import from files',
    newEmptyBook: 'New empty book',
    empty: {
      noBooksFound: 'No books found for "{query}"{layerSuffix}.',
      noBooksInLayer: 'No books in {layer}.',
      noBooksYet: 'No books yet.',
      noBooksInCharCountRange: 'No books in this character range.'
    },
    titleSearch: 'Search',
    titleLayer: 'Layer',
    refreshShelf: 'Update book list',
    refreshingShelf: 'Updating…',
    lastSynced: 'Updated {time}',
    neverSynced: 'Never updated',
    scanFound: 'Found {books} books in {layers} folders',
    scanInProgress: 'This shelf is already being scanned. Try again once it finishes.',
    loadFailed: 'Failed to load books',
    refreshFailed: 'Failed to update the book list',
    requestTimeout: 'Request timed out — the shelf may be slow or unavailable.',
    shelfNotReady: 'The shelf is still starting up and did not become ready.'
  },
  bookDetail: {
    documentTitle: 'Book',
    loading: 'Loading book details...',
    root: 'All books',
    layerPath: 'Book folder path',
    ratingLabel: 'Rated {rating} stars',
    emptyDetails: 'No additional details are available for this book.',
    sections: {
      publication: 'Publication',
      content: 'Content',
      notes: 'Notes'
    },
    fields: {
      format: 'Format',
      language: 'Language',
      publishedAt: 'Published',
      tags: 'Tags',
      lines: 'Lines',
      characters: 'Characters',
      comment: 'Book note',
      importNotes: 'Import note'
    },
    progress: {
      sectionLabel: 'Reading progress and actions',
      eyebrow: 'Reading progress',
      label: '{percent}% read',
      start: 'Not started',
      continue: 'Continue where you left off',
      reread: 'Finished'
    },
    actions: {
      startReading: 'Start reading',
      continueReading: 'Continue reading · {percent}%',
      reread: 'Read again',
      export: 'Export file',
      exporting: 'Exporting...',
      more: 'More',
      editMetadata: 'Edit book details',
      editSources: 'Manage sources',
      updateStats: 'Update content stats',
      updatingStats: 'Updating stats...',
      openFolder: 'Open folder',
      moveToTrash: 'Move to Trash',
      movingToTrash: 'Moving...',
      dismiss: 'Dismiss'
    },
    messages: {
      imported: 'Book imported successfully.',
      saved: 'Book details saved.'
    },
    errors: {
      restartReading: 'Failed to restart the book.',
      updateStats: 'Failed to update content stats',
      loadFailed: 'Failed to load detail',
      deleteFailed: 'Failed to delete book',
      downloadFailed: 'Failed to download book',
      openFolderFailed: 'Failed to open book folder'
    },
    delete: {
      description: 'The book will be moved to Trash. You can restore it later.'
    },
    cover: {
      options: 'Cover options',
      upload: 'Upload',
      remove: 'Remove',
      generate: 'Generate cover',
      uploading: 'Uploading cover...',
      removing: 'Removing cover...',
      updated: 'Cover updated.',
      removed: 'Cover removed.',
      dropHint: 'Drop the image to update the cover',
      unsupported: 'Only JPG, JPEG, PNG, WebP, and GIF are supported.',
      uploadFailed: 'Cover upload failed',
      uploadFailedWithReason: 'Cover upload failed: {reason}',
      removeFailed: 'Cover removal failed',
      removeFailedWithReason: 'Cover removal failed: {reason}',
      confirmTitle: 'Update book cover?',
      confirmQuestion: 'Use this image as the new book cover?',
      confirm: 'Update cover',
      generator: {
        title: 'Generate cover',
        close: 'Close cover generator',
        bookTitle: 'Title',
        author: 'Author',
        noAuthor: '(no author)',
        background: 'Background style',
        layout: 'Layout',
        cancel: 'Cancel',
        save: 'Save',
        saving: 'Saving...',
        noTitle: '(no title)',
        canvasUnavailable: 'Cover generation failed: canvas is not available.',
        exportFailed: 'Failed to export the cover image.',
        saveFailed: 'Failed to save the cover.',
        backgrounds: {
          plainLight: 'Plain light',
          plainDark: 'Plain dark',
          warmPaper: 'Warm paper',
          softGradient: 'Soft gradient',
          minimalSolid: 'Minimal solid color'
        },
        layouts: {
          centered: 'Centered title, author below',
          topBottom: 'Title near top, author near bottom',
          largeTitle: 'Large title centered',
          minimal: 'Minimal layout'
        }
      }
    }
  },
  bookCollection: {
    noLayer: 'No layer',
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
      mobileToolbarLabel: 'Selected books download bar',
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
    booksSuffix: ' books',
    firstPage: 'First page',
    lastPage: 'Last page'
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
      retry: 'Download failed, retry',
      downloadFailed: 'Failed to download book'
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
    errors: {
      loadFailed: 'Failed to load reader data',
      unknown: 'Unknown error'
    },
    actionsLabel: 'Reader actions',
    decreaseFontSize: 'Decrease font size',
    increaseFontSize: 'Increase font size',
    chooseFont: 'Choose reading font',
    fontDialog: {
      title: 'Reading font',
      description: 'Changes apply immediately and are saved on this device.',
      optionsLabel: 'Available reading fonts',
      close: 'Close font selection',
      done: 'Done',
      sample: 'A quiet place to read, between mountains and sea.',
      fonts: {
        system: {
          label: 'System serif',
          description: 'Keeps the current appearance; the exact font depends on the device.'
        },
        'noto-serif-tc': {
          label: 'Noto Serif TC',
          description: 'A consistent serif designed for comfortable long-form reading.'
        },
        'noto-sans-tc': {
          label: 'Noto Sans TC',
          description: 'A clear, consistent sans-serif reading font.'
        }
      }
    },
    showChapters: 'Show chapters',
    chapterDialog: {
      title: 'Chapters',
      closeLabel: 'Close chapter dialog'
    },
    imageUnavailable: 'Illustration unavailable',
    autosaveFailed: 'Reading progress could not be saved. PlainShelf will retry automatically.',
    mobile: {
      gestureHint: 'Tap the center for controls · Swipe left or right to change chapters',
      firstSection: 'You are at the first chapter',
      lastSection: 'You are at the last chapter'
    }
  },
  mobileConnect: {
    title: 'Connect to PlainShelf',
    description: 'Choose where your library lives so you can browse and read it on this device. The app is read-only: it never changes your library.',
    modeLabel: 'Library source',
    modeServer: 'PlainShelf server',
    modePCloud: 'pCloud',
    pcloud: {
      clientIdLabel: 'pCloud app key',
      clientIdPlaceholder: 'Your app key',
      clientIdHint: 'PlainShelf uses your own pCloud application. Create one in the pCloud developer console and paste its app key here.',
      clientIdRequired: 'Enter your pCloud app key first.',
      authorize: 'Authorize with pCloud',
      authorizing: 'Waiting for approval…',
      cancel: 'Cancel',
      authorized: 'Authorized ({host}).',
      authorizedAccount: 'Authorized account: {account} ({host}).',
      shelfRootLabel: 'Shelf folder',
      shelfRootPlaceholder: '/PlainShelf/default-shelf',
      shelfRootHint: 'The folder on pCloud that holds your shelf, the one containing books/.',
      shelfRootRequired: 'Enter the shelf folder first.',
      picker: {
        browse: 'Browse pCloud…',
        title: 'Choose the shelf folder',
        breadcrumbLabel: 'Folder path',
        rootLabel: 'pCloud',
        up: '↰ Up one level',
        loading: 'Loading…',
        empty: 'This folder has no sub-folders.',
        retry: 'Try again',
        currentPath: 'Selected: {path}',
        isShelf: 'This folder is a shelf: it contains books/.',
        notShelf: 'This folder has no books/, so it cannot be picked. Open the folder that contains it.',
        confirm: 'Use this folder',
        cancel: 'Cancel'
      },
      verify: 'Check shelf',
      verifying: 'Checking…',
      shelfFound: 'Found {count} books.',
      notAShelf: 'That folder has no books/ directory, so it is not a PlainShelf shelf.',
      verifyRequired: 'Check the shelf before saving.'
    },
    serverUrlLabel: 'Server URL',
    serverUrlPlaceholder: 'http://192.168.1.10:20000',
    tokenLabel: 'Access token (optional)',
    tokenPlaceholder: 'Needed to record reading history',
    tokenHint: 'Browsing and reading work without a token. Read history and reading time are recorded on this device either way. Add one only if the server has protect_read enabled.',
    loadShelves: 'Load library',
    loadingShelves: 'Connecting…',
    shelfLabel: 'Shelf',
    shelfPlaceholder: 'Select a shelf',
    save: 'Save and continue',
    saving: 'Saving…',
    serverUrlRequired: 'Enter a server URL first.',
    shelfRequired: 'Select a shelf to continue.',
    editTitle: 'Edit shelf',
    nameLabel: 'Name',
    namePlaceholder: 'Living room server',
    nameHint: 'What this shelf is called on this device. Optional.',
    cancel: 'Cancel',
    retargetHint: 'Changing where this shelf lives leaves books already downloaded from it stranded on this device.'
  },
  mobileShelves: {
    title: 'Shelves',
    description: 'Every library this device can open. One is in use at a time; switching restarts the app.',
    add: 'Add a shelf',
    edit: 'Edit',
    remove: 'Remove',
    active: 'In use',
    use: 'Use this shelf',
    removeTitle: 'Remove {name}?',
    removeDescription: 'Books downloaded from this shelf are deleted from the device. Reading progress and history are kept, so adding it back later picks up where you left off.',
    removeConfirm: 'Remove',
    removeCancel: 'Keep it'
  },
  libraryForms: {
    editBook: {
      title: 'Edit metadata',
      description: 'Update fields supported by the current API.',
      basicInfo: 'Basic info',
      titleLabel: 'Title',
      titlePlaceholder: 'Book title',
      authorsLabel: 'Authors (comma separated)',
      authorsPlaceholder: 'Author A, Author B',
      organization: 'Organization',
      publishedAt: 'Published At',
      languageLabel: 'Language',
      starRating: 'Star rating',
      starValueOne: '1 star',
      starValueMany: '{count} stars',
      clearRating: 'Clear',
      tags: 'Tags',
      tagsPlaceholder: 'Type a tag and press Enter',
      tagsHelp: 'Press Enter or comma to add tags. Click × to remove.',
      removeTag: 'Remove tag {tag}',
      comment: 'Comment',
      commentPlaceholder: 'Notes about this book',
      identifiers: 'Identifiers',
      identifierKeyPlaceholder: 'isbn',
      identifierValuePlaceholder: '9787020002207',
      identifierKeyLabel: 'Identifier key {index}',
      identifierValueLabel: 'Identifier value {index}',
      removeIdentifier: 'Remove identifier {name}',
      addIdentifier: 'Add identifier',
      save: 'Save metadata',
      saving: 'Saving...',
      loading: 'Loading book metadata...',
      loadFailed: 'Failed to load metadata',
      saveFailed: 'Failed to save metadata'
    },
    newBook: {
      title: 'New empty book',
      closeLabel: 'Close new empty book dialog',
      description: 'Create a new empty TXT book with title only.',
      titleLabel: 'Book Title',
      titlePlaceholder: 'Enter book title',
      create: 'Create',
      creating: 'Creating...',
      createFailed: 'Failed to create empty book.'
    },
    importBook: {
      title: 'Import Book',
      closeLabel: 'Close import dialog',
      description:
        'Upload a TXT, Markdown or EPUB file to create a new book entry, or drag-and-drop files here.',
      fileLabel: 'Book File (.txt, .md, .epub)',
      epubTitle: 'EPUB Conversion',
      epubDescription:
        'EPUB files are converted into a source. The original file is not kept; supported illustrations are retained by the Markdown preset.',
      convertTo: 'Convert to',
      presetMarkdown: 'Markdown (chapter headings)',
      presetPlain: 'Plain text',
      plainHint:
        'Plain text has no chapter navigation or Markdown illustrations. You can create a chapterized Markdown source afterwards without changing this TXT source.',
      includeDescription: 'Put the book description at the start of the text',
      selectedFiles: 'Selected Files',
      fileTitle: 'Title: {title}',
      fileStatus: 'Status:',
      submit: 'Import',
      submitting: 'Importing...',
      statuses: {
        pending: 'Pending',
        importing: 'Importing',
        success: 'Imported',
        failed: 'Failed'
      },
      errors: {
        noFiles: 'Please choose at least one TXT, Markdown or EPUB file.',
        unsupportedExtension: 'Book file must be .txt, .md or .epub.',
        allFailed: 'Import failed.',
        someFailed: '{count} file(s) failed.'
      },
      results: {
        one: 'Import successful.',
        many: 'Imported {count} files.',
        partial: 'Imported {count} of {total} files.'
      }
    }
  },
  sources: {
    list: {
      title: 'Sources',
      total: '{count} total',
      create: 'New',
      creating: 'Creating...',
      loading: 'Loading sources...',
      empty: 'No sources yet.',
      listLabel: 'Book sources',
      current: 'Current',
      delete: 'Delete',
      deleteLabel: 'Delete source {id}'
    },
    // A source without a stored format predates source-level format metadata.
    // The badge shows the format otherwise, uppercased, which needs no catalog
    // entry.
    format: {
      legacy: 'Legacy'
    },
    formatActions: {
      manualMarkdown: 'Manual TXT → MD',
      regexMarkdown: 'Regex → MD',
      lineCountMarkdown: 'Fixed lines → MD',
      plainText: 'Create plain-text source',
      plainTextHelp: 'Heading hierarchy and chapter navigation will be lost.'
    },
    editor: {
      loading: 'Loading source...',
      noSelection: 'Select a source to start editing.',
      dirty: 'Unsaved changes',
      clean: 'No pending changes',
      setCurrent: 'Set as current',
      settingCurrent: 'Setting...',
      contentLabel: 'Source content',
      find: {
        groupLabel: 'Find and replace',
        findLabel: 'Find',
        findPlaceholder: 'Search text',
        replaceLabel: 'Replace',
        replacePlaceholder: 'Replace with',
        scopeLabel: 'Scope',
        scopeSection: 'Current chapter',
        scopeSource: 'Whole source',
        caseSensitive: 'Match case',
        wholeWord: 'Whole word',
        regexp: 'Regular expression',
        previous: 'Prev',
        next: 'Next',
        replace: 'Replace',
        replaceAll: 'Replace all',
        // The catalog has no pluralization, so counted nouns that can be one
        // or many get a key per form. Replacing that with a "(s)" suffix would
        // have made the English worse than it was before it was translated.
        noMatches: 'No matches.',
        matchOrdinal: 'Match {ordinal} of {total}.',
        matchCount: '{total} matches.',
        replacedOne: 'Replaced 1 occurrence.',
        replacedMany: 'Replaced {count} occurrences.',
        replacedOneNoneRemain: 'Replaced 1 occurrence. No matches remain.',
        replacedOneThenMatch: 'Replaced 1 occurrence. Match {ordinal} of {total}.',
        replacedOneThenCount: 'Replaced 1 occurrence. {total} matches.',
        invalidRegexp: 'That regular expression is not valid.',
        // Spoken by the editor's own live region when a command moves or
        // replaces a match; the visible status line above stays separate.
        announce: {
          currentMatch: 'Current match',
          onLine: 'on line',
          replacedOnLine: 'replaced match on line $',
          replacedMatches: 'replaced $ matches'
        }
      }
    },
    conversion: {
      confirm: 'Create source',
      busy: 'Creating...',
      titles: {
        manualMd: 'Create chapterized Markdown source',
        regexMd: 'Convert title lines to chapters',
        lineCountMd: 'Split into fixed-size chapters',
        plainText: 'Create plain-text source'
      },
      descriptions: {
        manualMd:
          'Copy the current TXT source into a Markdown draft. You can add H2 chapters in the source editor after creation.',
        regexMd: 'Matching title lines will be rewritten as Markdown H2 headings.',
        lineCountMd: 'An H2 “Part N” heading will be inserted at every previewed boundary.',
        plainText:
          'Markdown markers will be removed. Heading hierarchy and chapter navigation will be lost.'
      },
      patternLabel: 'Chapter title regular expression',
      patternHelp: 'Capture group 1 becomes the H2 title. Without a capture group, the full match is used.',
      lineCountLabel: 'Lines per chapter',
      previewTitle: 'Preview',
      emptySource: '(empty source)',
      setCurrent: 'Set the new source as current',
      summaries: {
        manualMdOne: 'The Markdown draft contains 1 H2 chapter initially.',
        manualMdMany: 'The Markdown draft contains {count} H2 chapters initially.',
        regexMd: '{count} H2 chapter headings will be created.',
        lineCountMd: '{count} H2 chapter headings will be inserted.',
        plainText: 'A single unstructured TXT section will be created.'
      },
      errors: {
        emptyPattern: 'Enter a regular expression.',
        patternMatchedNothing: 'The regular expression matched no chapter title lines.',
        invalidLineCount: 'Lines per chapter must be a positive number.',
        previewFailed: 'Unable to preview this conversion.'
      }
    },
    page: {
      title: 'Edit Sources',
      back: 'Back',
      save: 'Save',
      saveDirty: 'Save*',
      saving: 'Saving...',
      loading: 'Loading sources...',
      panelsLabel: 'Source editor panels',
      paneSources: 'Sources',
      paneEditor: 'Editor',
      paneChapters: 'Chapters',
      discard: {
        title: 'Discard unsaved changes?',
        message: 'You have unsaved changes. Discard them?',
        confirm: 'Discard',
        cancel: 'Keep editing'
      },
      deleteSource: {
        title: 'Delete source?',
        confirm: 'Delete',
        question: 'Are you sure you want to delete source {id}? This action cannot be undone.',
        dirtyWarning: 'You have unsaved changes that will be lost.'
      },
      renameChapter: {
        title: 'Rename chapter',
        confirm: 'Rename',
        titleLabel: 'Chapter title'
      },
      mergeChapter: {
        title: 'Merge chapter?',
        confirm: 'Merge',
        question: 'Remove the H2 heading “{title}” and merge its text with the adjacent section?'
      },
      messages: {
        sourceSaved: 'Source saved.',
        currentUpdated: 'Current source updated.',
        derivedCreated: 'Derived source created.'
      },
      errors: {
        loadSources: 'Failed to load sources',
        loadContent: 'Failed to load source content',
        save: 'Failed to save source',
        setCurrent: 'Failed to set current source',
        create: 'Failed to create source',
        createDerived: 'Failed to create derived source',
        delete: 'Failed to delete source'
      }
    },
    chapters: {
      title: 'Chapters',
      headingCount: '{count} H2 headings',
      add: 'Add',
      allMarker: 'All',
      wholeSource: 'Whole source',
      rename: 'Rename',
      merge: 'Merge',
      empty: 'No H2 chapters yet.'
    }
  },
  notFound: {
    title: 'Page not found',
    description: 'There is nothing at this address. It may have been renamed, or the link may be out of date.',
    backToLibrary: 'Back to the library'
  }
} as const;

export default en;
