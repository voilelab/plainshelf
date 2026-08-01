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
    railNavLabel: '側邊欄導覽',
    openMenu: '開啟選單',
    closeMenu: '關閉選單',
    sections: {
      layers: '資料夾',
      reading: '閱讀',
      maintenance: '維護',
      admin: '管理'
    },
    sectionToggleLabels: {
      layers: '切換側欄資料夾區塊',
      reading: '切換側欄閱讀紀錄區塊',
      maintenance: '切換側欄維護區塊',
      admin: '切換側欄管理區塊'
    },
    createLayer: {
      add: '新增資料夾',
      title: '新增資料夾',
      nameLabel: '資料夾名稱',
      namePlaceholder: '資料夾名稱',
      parentLabel: '位置',
      rootOption: '所有書籍（最上層）',
      closeLabel: '關閉新增資料夾對話框',
      invalidName: '資料夾名稱不得為空，也不能包含 /。',
      creating: '建立中...',
      create: '建立',
      loadingLayers: '載入資料夾中...'
    },
    deleteLayer: {
      title: '刪除資料夾',
      shortAction: '刪除',
      description: '若資料夾內仍有書籍或子資料夾，刪除會失敗。',
      failed: '刪除資料夾失敗',
      notEmpty: '此資料夾尚未清空，無法刪除。\n請先移出書籍並刪除子資料夾。'
    },
    renameLayer: {
      shortAction: '改名',
      title: '重新命名資料夾',
      nameLabel: '資料夾名稱',
      placeholder: '資料夾名稱',
      help: '目前名稱：{layerName}',
      confirm: '重新命名',
      renaming: '重新命名中...',
      closeLabel: '關閉重新命名資料夾對話框',
      invalid: '資料夾名稱不得為空，也不能包含 /。',
      failed: '重新命名資料夾失敗'
    },
    moveLayer: {
      failed: '移動資料夾失敗。請將資料夾拖曳到既有的目標資料夾上。'
    },
    openLayerFolder: {
      shortAction: '開啟資料夾',
      failed: '開啟資料夾失敗。'
    },
    layerErrors: {
      emptyPath: '資料夾路徑不得為空',
      createFailed: '建立資料夾失敗'
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
    dashboard: '儀表板',
    recentlyRead: '最近閱讀',
    trash: '垃圾桶',
    downloads: '已下載',
    adminLogs: '日誌',
    settings: '設定',
    readOnly: {
      banner: '唯讀模式已啟用。仍可瀏覽與閱讀，但寫入操作已停用。',
      writeDisabled: '伺服器目前為唯讀模式，寫入操作已停用。'
    }
  },
  dashboard: {
    title: '儀表板',
    refresh: '重新整理',
    loading: '載入儀表板中...',
    loadFailed: '載入儀表板資料失敗',
    stats: {
      totalBooks: '藏書總數',
      addedThisMonth: '本月新增',
      avgStar: '平均星等',
      totalChars: '總字數',
      starBar: '{star} 星：{count} 本',
      currentStreak: '目前連續閱讀',
      currentStreakValue: '{days} 天'
    },
    tags: {
      title: '標籤',
      empty: '尚無標籤'
    },
    randomBook: {
      title: '隨機一本',
      empty: '尚無書籍',
      shuffle: '換一本',
      viewDetail: '查看詳情',
      readNow: '開始閱讀'
    },
    heatmap: {
      title: '閱讀熱力圖',
      empty: '開始閱讀後，這裡會累積你的閱讀足跡。',
      legendLess: '較少',
      legendMore: '較多',
      cellLabel: '{date}：閱讀 {minutes} 分鐘'
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
    reader: {
      title: '閱讀器'
    },
    defaultSplitConfig: {
      label: '預設章節分割規則',
      description: '書籍沒有明確分割設定時自動套用。',
      typeLabel: '分割類型',
      typeNone: '無',
      typeLineCount: '固定行數',
      typeRegex: '正規表示式',
      lineCountLabel: '每節行數',
      lineCountPlaceholder: '例如 100',
      regexLabel: '正規表示式模式',
      regexPlaceholder: '例如 ^第[一二三四五六七八九十百]+章',
      regexHelp: '符合此模式的行會開始新的章節。',
      invalidRegex: '無效的正規表示式。',
      invalidLineCount: '行數必須是正整數。',
      save: '儲存',
      saving: '儲存中...'
    },
    readHistoryLimit: {
      label: '閱讀紀錄數量限制',
      description: '最多保留的最近閱讀書籍數量。設為 0 可停用紀錄保留。',
      invalid: '閱讀紀錄數量限制必須是非負整數。'
    },
    about: {
      title: '關於',
      description: 'PlainShelf 是本機優先的個人閱讀書庫，適合管理輕量閱讀內容，採用檔案系統優先的資料模型並提供網頁閱讀介面。',
      version: '版本',
      repository: 'Repository'
    },
    mobileConnect: {
      title: '連線',
      description: '變更這台裝置使用的 PlainShelf 伺服器、存取權杖或書架。',
      open: '編輯連線設定'
    },
    downloads: {
      title: '已下載書籍',
      description: '管理已下載到這台裝置、供離線閱讀的書籍。',
      open: '管理已下載書籍'
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
    },
    lowCharCount: {
      title: '字數較少',
      empty: '沒有低於此字數的書籍。',
      thresholdLabel: '字數上限',
      filterDescription: '字數在 {threshold} 字以下的書籍',
      unknownNote: '其中 {count} 本字數未知'
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
    titleLayer: '資料夾'
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
    },
    selection: {
      toolbarLabel: '已選書籍操作',
      selectedCount: '已選 {count} 本',
      selectBook: '選取「{title}」',
      selectAll: '選取本頁',
      move: '移動',
      trash: '移到垃圾桶',
      download: '下載到裝置',
      downloading: '下載中…',
      moveTitle: '移動已選書籍',
      moveTarget: '目的地',
      rootLayer: '所有書籍（最上層）',
      chooseLayer: '選擇目的地',
      confirmMove: '移動 {count} 本',
      moving: '移動中…',
      trashTitle: '將已選書籍移到垃圾桶',
      trashQuestion: '確定將已選的 {count} 本書移到垃圾桶嗎？',
      trashDescription: '之後仍可從垃圾桶還原這些書籍。',
      confirmTrash: '移到垃圾桶',
      processing: '處理書籍中…',
      progressLabel: '批次操作進度',
      complete: '已完成 {count} 本書。',
      partial: '成功 {succeeded} 本；失敗 {failed} 本。',
      failed: '批次操作失敗。',
      close: '關閉',
      startFailed: '無法啟動批次操作',
      pollFailed: '無法繼續取得批次操作進度。',
      downloadComplete: '已下載 {count} 本書。',
      downloadPartial: '成功下載 {succeeded} 本；失敗 {failed} 本。',
      failureCodes: {
        not_found: '找不到書籍',
        unsupported_schema: '書籍由較新的 PlainShelf 版本建立',
        move_failed: '無法移動書籍',
        trash_failed: '無法將書籍移到垃圾桶',
        download_failed: '無法下載書籍'
      }
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
      originalLayer: '原始資料夾',
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
    },
    emptyAll: {
      action: '清空垃圾桶',
      title: '清空垃圾桶',
      question: '確定要永久刪除垃圾桶中的 {count} 本書嗎？',
      questionUnknownCount: '確定要永久刪除垃圾桶中的所有內容嗎？中繼資料無法讀取的書籍不會列在上方，但同樣會被移除。',
      description: '此操作會永久刪除資料，且無法復原。',
      confirm: '清空垃圾桶',
      busy: '清空中...',
      close: '關閉',
      progressLabel: '清空垃圾桶進度',
      pending: '等待開始...',
      running: '刪除書籍中...',
      completed: '垃圾桶已清空。',
      partiallyCompleted: '有部分書籍未能刪除，詳情請查看日誌。',
      failed: '無法開始刪除，垃圾桶可能無法讀取或被鎖定。',
      startFailed: '啟動清空垃圾桶失敗',
      pollFailed: '無法取得進度，請重新整理頁面查看目前狀態。'
    }
  },
  downloads: {
    title: '已下載書籍',
    description: '已下載到這台裝置、供離線閱讀的書籍。',
    overview: {
      countLabel: '已下載書籍數',
      count: '已下載 {count} 本',
      totalSizeLabel: '總佔用空間',
      deviceStorageLabel: '裝置儲存空間',
      storageUsed: '已使用 {used} / {quota}',
      storageUnsupported: '此裝置不支援儲存空間估算。'
    },
    list: {
      loading: '載入已下載書籍中...',
      loadFailed: '載入已下載書籍失敗',
      empty: '尚未下載任何書籍。'
    },
    item: {
      remove: '移除下載'
    },
    detail: {
      download: '下載到裝置',
      downloading: '下載中...',
      remove: '移除下載',
      retry: '下載失敗，重試'
    },
    removeConfirm: {
      title: '移除下載',
      description: '此操作會移除這台裝置上的離線副本，之後仍可隨時重新下載。',
      confirm: '移除下載',
      busy: '移除中...',
      failed: '移除下載失敗'
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
    description: '輸入你的 PlainShelf 伺服器位址，即可在這台裝置瀏覽與閱讀書庫。本 App 為唯讀，不會變更你的書庫。',
    serverUrlLabel: '伺服器網址',
    serverUrlPlaceholder: 'http://192.168.1.10:20000',
    tokenLabel: '存取權杖（選填）',
    tokenPlaceholder: '記錄閱讀紀錄時需要',
    tokenHint: '瀏覽與閱讀不需權杖。若要讓這台裝置記錄閱讀紀錄與閱讀活動，或伺服器啟用了 protect_read，才需要填入。',
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
