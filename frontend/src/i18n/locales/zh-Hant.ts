const zhHant = {
  app: {
    name: 'PlainShelf',
    mockModeBadge: '模擬 API 模式',
    desktopHistoryNavigation: '桌面版歷史導覽',
    previousPage: '上一頁',
    nextPage: '下一頁'
  },
  language: {
    label: '語言',
    en: 'English',
    zhHant: '繁體中文'
  },
  common: {
    retry: '重試',
    cancel: '取消',
    prev: '上一頁',
    next: '下一頁',
    page: '第 {page} / {total} 頁',
    inLayer: '（在 {layer}）'
  },
  layout: {
    expandSidebar: '展開側欄',
    collapseSidebar: '收合側欄',
    openMenu: '開啟選單',
    closeMenu: '關閉選單',
    sections: {
      layers: '圖層',
      reading: '閱讀',
      maintenance: '維護',
      admin: '管理'
    },
    createLayer: {
      add: '新增圖層',
      cancel: '取消',
      placeholder: '例如 programming/rust',
      creating: '建立中...',
      create: '建立',
      created: '圖層已建立',
      enter: '進入',
      loadingLayers: '載入圖層中...'
    },
    deleteLayer: {
      title: '刪除圖層',
      shortAction: '刪除',
      description: '若圖層內仍有書籍或子圖層，刪除會失敗。',
      failed: '刪除圖層失敗',
      notEmpty: '此圖層尚未清空，無法刪除。\n請先移出書籍並刪除子圖層。'
    },
    renameLayer: {
      shortAction: '改名',
      title: '重新命名圖層',
      nameLabel: '圖層名稱',
      placeholder: '圖層名稱',
      help: '目前名稱：{layerName}',
      confirm: '重新命名',
      renaming: '重新命名中...',
      closeLabel: '關閉重新命名圖層對話框',
      invalid: '圖層名稱不得為空，也不能包含 /。',
      failed: '重新命名圖層失敗'
    },
    moveLayer: {
      failed: '移動圖層失敗。請將圖層拖曳到既有的目標圖層上。'
    },
    openLayerFolder: {
      shortAction: '開啟資料夾',
      failed: '開啟圖層資料夾失敗。'
    },
    layerErrors: {
      emptyPath: '圖層路徑不得為空',
      createFailed: '建立圖層失敗'
    },
    moveBookErrors: {
      notFound: '找不到書籍。',
      failed: '移動書籍失敗。'
    },
    shelf: {
      label: '書架',
      loading: '載入書架中...',
      failed: '載入書架失敗',
      empty: '尚未設定書架',
      unavailableTitle: '尚未選擇書架',
      unavailableDescription: '請先設定至少一個書架，才能瀏覽與閱讀書籍。',
      addShelf: '新增書架',
      addShelfTitle: '建立書架',
      addShelfCloseLabel: '關閉建立書架對話框',
      addShelfCancel: '取消',
      addShelfNamePlaceholder: '書架名稱',
      addShelfDirectoryPlaceholder: '目錄',
      addShelfBrowse: '瀏覽…',
      addShelfSubmit: '新增',
      addShelfAdding: '新增中...',
      addShelfFailed: '新增書架失敗'
    },
    recentlyRead: '最近閱讀',
    trash: '垃圾桶',
    adminLogs: '日誌',
    settings: '設定',
    readOnly: {
      banner: '唯讀模式已啟用。仍可瀏覽與閱讀，但寫入操作已停用。',
      writeDisabled: '伺服器目前為唯讀模式，寫入操作已停用。'
    }
  },
  settings: {
    title: '設定',
    description: '管理應用程式選項。',
    loadFailed: '載入設定失敗',
    saveFailed: '儲存設定失敗',
    cover: {
      title: '封面'
    },
    coverToJpg: {
      label: '將上傳封面轉為 JPG',
      description: '啟用後，封面圖片上傳時會轉換為 JPEG。'
    },
    readHistory: {
      title: '閱讀紀錄'
    },
    readHistoryLimit: {
      label: '閱讀紀錄數量限制',
      description: '最多保留的最近閱讀書籍數量。設為 0 可停用紀錄保留。',
      invalid: '閱讀紀錄數量限制必須是非負整數。'
    },
    about: {
      title: '關於',
      version: '版本'
    },
    mobileConnect: {
      title: '連線',
      description: '變更這台裝置使用的 PlainShelf 伺服器、存取權杖或書架。',
      open: '編輯連線設定'
    },
    shelves: {
      title: '書架',
      loadFailed: '載入書架失敗',
      empty: '尚未設定書架。',
      serverManaged: '書架由伺服器設定管理。',
      name: '名稱',
      directory: '目錄',
      remove: '刪除',
      removing: '移除中...',
      removeFailed: '移除書架失敗',
      removeShelfTitle: '刪除書架',
      removeConfirmDescription: '此操作只會從 PlainShelf 中移除書架，不會刪除目錄。',
      addShelf: '新增書架',
      addShelfTitle: '建立書架',
      addShelfCloseLabel: '關閉建立書架對話框',
      addShelfCancel: '取消',
      addShelfNamePlaceholder: '書架名稱',
      addShelfDirectoryPlaceholder: '目錄路徑',
      addShelfScanIntervalPlaceholder: '掃描間隔（選填，例如 10m）',
      addShelfScanIntervalHelp: '留空會使用預設的 1 分鐘掃描間隔。',
      addShelfBrowse: '瀏覽…',
      addShelfSubmit: '新增書架',
      addShelfAdding: '新增中...',
      addShelfFailed: '新增書架失敗',
      removeConfirm: '移除書架「{name}」？此操作只會從 PlainShelf 中移除，不會刪除目錄。',
      removeConfirmInline: '確定移除？',
      removeConfirmYes: '刪除書架',
      modify: '修改',
      modifyShelfTitle: '修改書架',
      modifyShelfCloseLabel: '關閉修改書架對話框',
      modifyShelfSubmit: '儲存',
      modifyShelfSaving: '儲存中...',
      modifyShelfFailed: '修改書架失敗',
      modifyShelfIDLabel: 'ID',
      modifyShelfPathLabel: '路徑',
      modifyShelfNamePlaceholder: '書架名稱',
      modifyShelfScanIntervalPlaceholder: '掃描間隔（選填，例如 10m）',
      modifyShelfScanIntervalHelp: '留空會使用預設的 1 分鐘掃描間隔。'
    }
  },
  adminLogs: {
    title: '日誌',
    description: '選擇日誌名稱與日期以查看日誌檔內容。',
    name: '名稱',
    date: '日期',
    source: '來源',
    filename: '檔名',
    empty: '目前沒有可用的日誌檔。',
    emptyContent: '所選日誌檔沒有內容。',
    missingForDate: '{date} 沒有可用的日誌檔。',
    loadingList: '載入日誌檔中...',
    loadingContent: '載入日誌內容中...',
    loadFailed: '載入日誌檔失敗',
    loadContentFailed: '載入日誌內容失敗'
  },
  maintenance: {
    duplicateContent: '重複內容',
    missingAuthor: {
      title: '缺少作者',
      empty: '沒有缺少作者的書籍'
    },
    missingCover: {
      title: '缺少封面',
      empty: '沒有缺少封面的書籍'
    },
    missingLanguage: {
      title: '缺少語言',
      empty: '沒有缺少語言的書籍。'
    }
  },
  library: {
    allBooks: '所有書籍',
    searchPlaceholder: '搜尋書籍...',
    clearSearch: '清除搜尋',
    search: '搜尋',
    sort: '排序',
    sortBy: {
      updated: '更新時間',
      created: '建立時間',
      title: '標題'
    },
    order: {
      asc: '升冪',
      desc: '降冪'
    },
    import: '匯入 ▾',
    importFromFiles: '從檔案匯入',
    newEmptyBook: '建立空白書籍',
    empty: {
      noBooksFound: '找不到「{query}」相關書籍{layerSuffix}。',
      noBooksInLayer: '{layer} 目前沒有書籍。',
      noBooksYet: '目前尚無書籍。'
    },
    titleSearch: '搜尋',
    titleLayer: '圖層'
  },
  bookCollection: {
    loadingBooks: '載入書籍中...',
    shelfInitializing: '書架載入中，請稍候...',
    shelfUnreachable: '書架回應逾時，可能無法連線（例如 SMB 掛載已中斷）。',
    booksCount: '{count} 本書',
    viewMode: {
      list: '列表',
      card: '卡片',
      title: '標題'
    },
    contextMenu: {
      read: '閱讀',
      openDetail: '開啟詳情',
      openBookFolder: '開啟書籍資料夾',
      download: '下載',
      edit: '編輯',
      delete: '刪除'
    }
  },
  pagination: {
    perPage: '每頁',
    booksSuffix: ' 本'
  },
  deleteModal: {
    closeLabel: '關閉刪除確認視窗',
    title: '確認刪除',
    description: '刪除後無法復原。',
    confirm: '刪除',
    cancel: '取消',
    busy: '刪除中...',
    question: '確定刪除「{itemName}」？'
  },
  readHistory: {
    title: '最近閱讀',
    empty: '目前沒有閱讀紀錄。開啟一本書後會顯示在這裡。',
    clear: '清除紀錄',
    clearing: '清除中...',
    loadFailed: '載入閱讀紀錄失敗',
    clearFailed: '清除閱讀紀錄失敗'
  },
  trash: {
    title: '垃圾桶',
    loading: '載入已刪除書籍中...',
    empty: '垃圾桶目前是空的。',
    loadFailed: '載入垃圾桶失敗',
    restoreFailed: '還原書籍失敗',
    permanentDeleteFailed: '永久刪除書籍失敗',
    columns: {
      title: '標題',
      authors: '作者',
      originalLayer: '原始圖層',
      originalPath: '原始路徑',
      deletedAt: '刪除時間',
      bookId: '書籍 ID',
      actions: '操作'
    },
    actions: {
      restore: '還原',
      permanentDelete: '永久刪除'
    },
    permanentDelete: {
      title: '永久刪除',
      description: '此操作會永久刪除資料，且無法復原。',
      confirm: '永久刪除',
      busy: '永久刪除中...'
    }
  },
  reader: {
    backToDetail: '返回詳情',
    title: '閱讀器',
    progress: '進度：{percent}%',
    loadingContent: '內容載入中...',
    actionsLabel: '閱讀器操作',
    decreaseFontSize: '縮小字體',
    increaseFontSize: '放大字體',
    showChapters: '顯示章節',
    splitSettings: '切分設定',
    saveBookmark: '儲存書籤',
    savingBookmark: '儲存書籤中'
  },
  mobileConnect: {
    title: '連線到 PlainShelf',
    description: '輸入你的 PlainShelf 伺服器位址，即可在這台裝置瀏覽與閱讀書庫。',
    serverUrlLabel: '伺服器網址',
    serverUrlPlaceholder: 'http://192.168.1.10:20000',
    tokenLabel: '存取權杖（選填）',
    tokenPlaceholder: '僅編輯時需要',
    tokenHint: '閱讀不需權杖；要修改內容才需填入。',
    loadShelves: '載入書庫',
    loadingShelves: '連線中…',
    shelfLabel: '書架',
    shelfPlaceholder: '選擇書架',
    save: '儲存並繼續',
    saving: '儲存中…',
    serverUrlRequired: '請先輸入伺服器網址。',
    shelfRequired: '請選擇一個書架。'
  }
} as const;

export default zhHant;
