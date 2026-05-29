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
    prev: '上一頁',
    next: '下一頁',
    page: '第 {page} / {total} 頁',
    inLayer: '（在 {layer}）'
  },
  layout: {
    expandSidebar: '展開側欄',
    collapseSidebar: '收合側欄',
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
      description: '若圖層內仍有書籍或子圖層，刪除會失敗。',
      failed: '刪除圖層失敗',
      notEmpty: '此圖層尚未清空，無法刪除。\n請先移出書籍並刪除子圖層。'
    },
    layerErrors: {
      emptyPath: '圖層路徑不得為空',
      createFailed: '建立圖層失敗'
    },
    moveBookErrors: {
      notFound: '找不到書籍。',
      failed: '移動書籍失敗。'
    },
    recentlyRead: '最近閱讀',
    trash: '垃圾桶',
    adminLogs: '日誌'
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
    booksCount: '{count} 本書',
    viewMode: {
      list: '列表',
      card: '卡片',
      title: '標題'
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
  }
} as const;

export default zhHant;
