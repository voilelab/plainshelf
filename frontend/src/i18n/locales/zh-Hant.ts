const zhHant = {
  app: {
    name: 'PlainShelf',
    mockModeBadge: '模擬 API 模式',
    desktopHistoryNavigation: '桌面版歷史導覽',
    previousPage: '上一頁',
    nextPage: '下一頁',
    renderError: {
      title: '這個頁面停止回應',
      description: '繪製這個頁面時發生錯誤。重新載入通常就能恢復。',
      reload: '重新載入'
    }
  },
  toast: {
    label: '通知',
    dismiss: '關閉通知'
  },
  language: {
    label: '語言',
    en: 'English',
    zhHant: '繁體中文',
    // 書籍本身的語言，與上面兩個 key 不同：那兩個指的是介面語言，
    // 且無論介面切到哪一種都維持各自的母語寫法。
    book: {
      unspecified: '未指定',
      zhHant: '中文（繁體）',
      zhHans: '中文（簡體）',
      ja: '日文',
      ko: '韓文',
      en: '英文',
      custom: '自訂...',
      customPlaceholder: '例如 zh-TW, zh-HK, fr, de',
      help: '建議使用 en、ja、ko、zh-Hant、zh-Hans；也可填 zh-TW 這類 BCP 47 language tag。',
      invalidTag: '語言格式不正確，請使用 en、ja、zh-Hant、zh-TW 這類格式。'
    }
  },
  common: {
    retry: '重試',
    cancel: '取消',
    confirm: '確認',
    working: '處理中…',
    closeDialog: '關閉確認對話框',
    prev: '上一頁',
    next: '下一頁',
    page: '第 {page} / {total} 頁',
    inLayer: '（在 {layer}）',
    taskStartFailed: '啟動工作失敗',
    taskPollFailed: '讀取工作進度失敗'
  },
  layout: {
    expandSidebar: '展開側欄',
    collapseSidebar: '收合側欄',
    railNavLabel: '側邊欄導覽',
    layersNavLabel: '資料夾',
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
      createFailed: '建立資料夾失敗',
      loadFailed: '載入資料夾失敗',
      shelfNotReady: '書架仍在啟動中，尚未就緒。'
    },
    moveBookErrors: {
      notFound: '找不到書籍。',
      failed: '移動書籍失敗。'
    },
    shelf: {
      label: '書架',
      loading: '載入書架中...',
      empty: '尚未設定書架',
      unavailableTitle: '尚未選擇書架',
      unavailableDescription: '請先設定至少一個書架，才能瀏覽與閱讀書籍。',
      manage: '管理書架'
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
    shelfInitializing: '書架載入中，請稍候...',
    shelfNotReady: '書架仍在啟動中，尚未就緒。',
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
    serverModeLoadFailed: '載入伺服器模式失敗',
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
    import: {
      title: '匯入'
    },
    epubImport: {
      title: 'EPUB 轉換',
      description:
        'EPUB 匯入時會轉換成 source，不保留原始檔案；Markdown 可保留支援的圖片。這裡設定預設值，匯入時可針對單次批次覆寫。',
      presetLabel: '轉換成',
      presetHelp:
        'Markdown 以 H2 保存章節並保留支援的圖片；純文字沒有章節導覽，也不支援 Markdown 圖片。',
      presetMarkdown: 'Markdown（章節標題）',
      presetPlain: '純文字',
      includeDescriptionLabel: '正文開頭放入簡介',
      includeDescriptionHelp: '把書籍簡介也寫在正文開頭。無論是否勾選，簡介都會存進書籍中繼資料。',
      save: '儲存',
      saving: '儲存中...'
    },
    readHistoryLimit: {
      label: '閱讀紀錄數量限制',
      description:
        '這台裝置最多保留的最近閱讀書籍數量。閱讀紀錄只保存在本機，不會送到伺服器。設為 0 可停用紀錄保留。',
      invalid: '閱讀紀錄數量限制必須是非負整數。'
    },
    about: {
      title: '關於',
      description: 'PlainShelf 是本機優先的個人閱讀書庫，適合管理輕量閱讀內容，採用檔案系統優先的資料模型並提供網頁閱讀介面。',
      version: '版本',
      repository: '程式碼倉庫',
      thirdPartyFonts: '第三方字型授權',
      fontAttribution: 'Google Inc. · SIL Open Font License 1.1 · 套件版本 5.3.0',
      source: '來源',
      license: '完整授權條款',
      licenseTitle: '{font} 授權條款',
      licenseLoading: '正在載入授權條款…',
      licenseLoadFailed: '無法載入隨附的授權條款。',
      licenseClose: '關閉'
    },
    mobileConnect: {
      title: '書架',
      description: '這台裝置可以開啟的所有書庫，來源可以是 PlainShelf 伺服器或 pCloud。',
      open: '管理書架'
    },
    downloads: {
      title: '已下載書籍',
      description: '管理已下載到這台裝置、供離線閱讀的書籍。',
      open: '管理已下載書籍'
    },
    bookCache: {
      title: '行動裝置書籍快取',
      description:
        '每個書架都會在自己的 app 資料夾裡保存一份書目清單，讓透過雲端儲存讀取同一個書架的手機不必自行掃描。這份清單會自動更新；若希望手機立刻看到剛才的變更，可以在這裡手動更新。',
      export: '立即更新',
      exporting: '更新中...',
      success: '書籍快取已更新。',
      failed: '更新書籍快取失敗。'
    },
    shelves: {
      title: '書架',
      loadFailed: '載入書架失敗',
      empty: '尚未設定書架。',
      serverManaged: '書架由伺服器設定管理。',
      name: '名稱',
      idColumn: 'ID',
      remove: '刪除',
      removing: '移除中...',
      removeFailed: '移除書架失敗',
      removeShelfTitle: '刪除書架',
      removeConfirmDescription: '此操作只會從 PlainShelf 中移除書架，不會刪除目錄。',
      addShelf: '新增書架',
      addShelfTitle: '建立書架',
      addShelfCloseLabel: '關閉建立書架對話框',
      addShelfNamePlaceholder: '書架名稱',
      addShelfDirectoryPlaceholder: '目錄路徑',
      addShelfScanIntervalPlaceholder: '掃描間隔（選填，例如 10m）',
      addShelfScanIntervalHelp: '留空會使用預設的 1 分鐘掃描間隔。',
      addShelfBrowse: '瀏覽…',
      addShelfSubmit: '新增書架',
      addShelfAdding: '新增中...',
      addShelfFailed: '新增書架失敗',
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
    duplicates: {
      description: '用來檢視內容完全相同的書籍。',
      scanning: '掃描重複內容群組中...',
      empty: '沒有發現重複內容。',
      emptyHint: '你的書庫很乾淨。',
      loadFailed: '載入重複內容失敗',
      groupTitle: '重複群組 #{index}',
      groupSummary: '{count} 本書的內容完全相同',
      open: '開啟',
      delete: '刪除',
      deleting: '刪除中...',
      deleteLabel: '刪除「{title}」',
      untitledBook: '這本書',
      deleteFailed: '刪除書籍失敗'
    },
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
    charCount: {
      label: '字數',
      minLabel: '字數下限',
      maxLabel: '字數上限',
      minPlaceholder: '下限',
      maxPlaceholder: '上限',
      clear: '清除字數範圍',
      unknownNote: '其中 {count} 本字數未知',
      refreshStats: {
        action: '更新 {count} 本的內容統計',
        busy: '更新中… {percent}%',
        done: '已更新 {count} 本',
        partial: '已更新 {succeeded} 本，{failed} 本失敗',
        failed: '無法更新內容統計。'
      }
    },
    import: '匯入 ▾',
    importFromFiles: '從檔案匯入',
    newEmptyBook: '建立空白書籍',
    empty: {
      noBooksFound: '找不到「{query}」相關書籍{layerSuffix}。',
      noBooksInLayer: '{layer} 目前沒有書籍。',
      noBooksYet: '目前尚無書籍。',
      noBooksInCharCountRange: '沒有落在此字數範圍的書籍。'
    },
    titleSearch: '搜尋',
    titleLayer: '資料夾',
    refreshShelf: '更新書單',
    refreshingShelf: '更新中…',
    lastSynced: '上次更新 {time}',
    neverSynced: '尚未更新',
    scanFound: '找到 {books} 本書、{layers} 個資料夾',
    scanInProgress: '這個書架正在掃描中，請等這次掃描結束後再試。',
    loadFailed: '載入書籍失敗',
    refreshFailed: '更新書單失敗',
    requestTimeout: '請求逾時——書架可能較慢或無法連線。',
    shelfNotReady: '書架仍在啟動中，尚未就緒。'
  },
  bookDetail: {
    documentTitle: '書籍',
    loading: '載入書籍詳情中...',
    root: '所有書籍',
    layerPath: '書籍所在資料夾',
    ratingLabel: '評分 {rating} 顆星',
    emptyDetails: '這本書目前沒有其他詳細資料。',
    sections: {
      publication: '出版資訊',
      content: '內容資訊',
      notes: '備註',
      chapters: '章節'
    },
    chapters: {
      showAll: '顯示全部 {count} 章',
      showLess: '收合章節'
    },
    fields: {
      format: '格式',
      language: '語言',
      publishedAt: '出版日期',
      tags: '標籤',
      lines: '行數',
      characters: '字數',
      comment: '書籍備註',
      importNotes: '匯入備註'
    },
    progress: {
      sectionLabel: '閱讀進度與操作',
      eyebrow: '閱讀進度',
      label: '已閱讀 {percent}%',
      start: '尚未開始',
      continue: '繼續上次的進度',
      reread: '已讀完'
    },
    actions: {
      startReading: '開始閱讀',
      continueReading: '繼續閱讀 · {percent}%',
      reread: '重新閱讀',
      export: '匯出檔案',
      exporting: '匯出中...',
      more: '更多',
      editMetadata: '編輯書籍資料',
      editSources: '管理來源',
      updateStats: '更新內容統計',
      updatingStats: '更新統計中...',
      openFolder: '開啟資料夾',
      moveTo: '移動到…',
      moveToTrash: '移至垃圾桶',
      movingToTrash: '移動中...',
      dismiss: '關閉'
    },
    messages: {
      imported: '書籍匯入成功。',
      saved: '書籍資料已儲存。'
    },
    errors: {
      restartReading: '無法重新開始閱讀。',
      updateStats: '更新內容統計失敗',
      loadFailed: '載入詳情失敗',
      deleteFailed: '刪除書籍失敗',
      downloadFailed: '下載書籍失敗',
      openFolderFailed: '開啟書籍資料夾失敗',
      moveFailed: '移動書籍失敗'
    },
    move: {
      title: '移動書籍'
    },
    delete: {
      description: '書籍將移至垃圾桶，之後仍可復原。'
    },
    cover: {
      options: '封面選項',
      upload: '上傳',
      remove: '移除',
      generate: '產生封面',
      uploading: '正在上傳封面...',
      removing: '正在移除封面...',
      updated: '封面已更新。',
      removed: '封面已移除。',
      dropHint: '放開圖片即可更新封面',
      unsupported: '僅支援 JPG、JPEG、PNG、WebP 與 GIF。',
      uploadFailed: '封面上傳失敗',
      uploadFailedWithReason: '封面上傳失敗：{reason}',
      removeFailed: '封面移除失敗',
      removeFailedWithReason: '封面移除失敗：{reason}',
      confirmTitle: '更新書籍封面？',
      confirmQuestion: '要使用這張圖片更新書籍封面嗎？',
      confirm: '更新封面',
      generator: {
        title: '產生封面',
        close: '關閉產生封面對話框',
        bookTitle: '書名',
        author: '作者',
        noAuthor: '（沒有作者）',
        background: '背景風格',
        layout: '版面',
        cancel: '取消',
        save: '儲存',
        saving: '儲存中...',
        noTitle: '（沒有書名）',
        canvasUnavailable: '無法產生封面：不支援畫布。',
        exportFailed: '無法匯出封面圖片。',
        saveFailed: '封面儲存失敗。',
        backgrounds: {
          plainLight: '簡潔亮色',
          plainDark: '簡潔深色',
          warmPaper: '溫暖紙張',
          softGradient: '柔和漸層',
          minimalSolid: '極簡純色'
        },
        layouts: {
          centered: '書名置中，作者置於下方',
          topBottom: '書名靠上，作者靠下',
          largeTitle: '大型置中書名',
          minimal: '極簡版面'
        }
      }
    }
  },
  bookCollection: {
    noLayer: '未分類',
    noSummary: '沒有簡介',
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
      mobileToolbarLabel: '已選書籍下載列',
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
      confirmMoveOne: '移動 1 本',
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
    booksSuffix: ' 本',
    firstPage: '第一頁',
    lastPage: '最後一頁'
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
      retry: '下載失敗，重試',
      downloadFailed: '下載書籍失敗'
    },
    removeConfirm: {
      title: '移除下載',
      description: '此操作會移除這台裝置上的離線副本，之後仍可隨時重新下載。',
      confirm: '移除下載',
      busy: '移除中...',
      failed: '移除下載失敗'
    }
  },
  readerApp: {
    noBook: '目前沒有開啟任何書籍。',
    openBook: '開啟書籍…',
    openFailed: '無法以書籍格式開啟這個資料夾。'
  },
  reader: {
    backToDetail: '返回詳情',
    title: '閱讀器',
    progress: '進度：{percent}%',
    loadingContent: '內容載入中...',
    errors: {
      loadFailed: '載入閱讀器資料失敗',
      unknown: '未知錯誤'
    },
    actionsLabel: '閱讀器操作',
    decreaseFontSize: '縮小字體',
    increaseFontSize: '放大字體',
    chooseFont: '選擇閱讀字型',
    fontDialog: {
      title: '閱讀字型',
      description: '選擇後會立即套用，並保存在這台裝置上。',
      optionsLabel: '可用閱讀字型',
      close: '關閉字型選擇',
      done: '完成',
      sample: '山海之間，靜靜讀一本好書。',
      fonts: {
        system: {
          label: '系統明體',
          description: '維持目前外觀，實際字型依裝置而異。'
        },
        'noto-serif-tc': {
          label: 'Noto Serif TC 明體',
          description: '跨平台一致、適合長篇正文閱讀的明體。'
        },
        'noto-sans-tc': {
          label: 'Noto Sans TC 黑體',
          description: '筆畫清楚、跨平台一致的無襯線字型。'
        }
      }
    },
    showChapters: '顯示章節',
    chapterDialog: {
      title: '章節',
      closeLabel: '關閉章節對話框'
    },
    imageUnavailable: '插圖無法載入',
    autosaveFailed: '閱讀進度無法儲存，PlainShelf 將自動重試。',
    mobile: {
      gestureHint: '點按中央顯示工具列 · 左右滑動切換章節',
      firstSection: '已是第一章',
      lastSection: '已是最後一章'
    }
  },
  mobileConnect: {
    title: '連線到 PlainShelf',
    description: '選擇書庫的來源，即可在這台裝置瀏覽與閱讀。本 App 為唯讀，不會變更你的書庫。',
    modeLabel: '書庫來源',
    modeServer: 'PlainShelf 伺服器',
    modePCloud: 'pCloud',
    pcloud: {
      clientIdLabel: 'pCloud 應用程式金鑰',
      clientIdPlaceholder: '你的 app key',
      clientIdHint: 'PlainShelf 使用你自己的 pCloud 應用程式。請到 pCloud 開發者後台建立一個，並把 app key 貼在這裡。',
      clientIdRequired: '請先輸入 pCloud 應用程式金鑰。',
      authorize: '以 pCloud 授權',
      authorizing: '等待授權中…',
      cancel: '取消',
      authorized: '已授權（{host}）。',
      authorizedAccount: '已授權帳號：{account}（{host}）。',
      shelfRootLabel: '書架資料夾',
      shelfRootPlaceholder: '/PlainShelf/default-shelf',
      shelfRootHint: 'pCloud 上存放書架的資料夾，也就是含有 books/ 的那一層。',
      shelfRootRequired: '請先輸入書架資料夾。',
      picker: {
        browse: '瀏覽 pCloud…',
        title: '選擇書架資料夾',
        breadcrumbLabel: '資料夾路徑',
        rootLabel: 'pCloud',
        up: '↰ 上一層',
        loading: '載入中…',
        empty: '這個資料夾沒有子資料夾。',
        retry: '重試',
        currentPath: '已選擇：{path}',
        isShelf: '這個資料夾是書架：含有 books/。',
        notShelf: '這個資料夾沒有 books/，無法選取。請打開含有它的那一層。',
        confirm: '選擇這個資料夾',
        cancel: '取消'
      },
      verify: '檢查書架',
      verifying: '檢查中…',
      shelfFound: '找到 {count} 本書。',
      notAShelf: '該資料夾沒有 books/ 目錄，不是 PlainShelf 書架。',
      verifyRequired: '請先檢查書架再儲存。'
    },
    serverUrlLabel: '伺服器網址',
    serverUrlPlaceholder: 'http://192.168.1.10:20000',
    tokenLabel: '存取權杖（選填）',
    tokenPlaceholder: '記錄閱讀紀錄時需要',
    tokenHint: '瀏覽與閱讀不需權杖。閱讀紀錄與閱讀時間一律記在這台裝置上。只有伺服器啟用 protect_read 時才需要填入。',
    loadShelves: '載入書庫',
    loadingShelves: '連線中…',
    shelfLabel: '書架',
    shelfPlaceholder: '選擇書架',
    save: '儲存並繼續',
    saving: '儲存中…',
    serverUrlRequired: '請先輸入伺服器網址。',
    shelfRequired: '請選擇一個書架。',
    editTitle: '編輯書架',
    nameLabel: '名稱',
    namePlaceholder: '客廳的伺服器',
    nameHint: '這個書架在這台裝置上顯示的名稱，可留空。',
    cancel: '取消',
    retargetHint: '更改書架的位置後，先前從它下載的書會留在裝置上但無法再開啟。'
  },
  mobileShelves: {
    title: '書架列表',
    description: '這台裝置可以開啟的所有書庫。一次使用其中一個，切換時會重新啟動 App。',
    add: '新增書架',
    edit: '編輯',
    remove: '移除',
    active: '使用中',
    use: '改用這個書架',
    removeTitle: '要移除「{name}」嗎？',
    removeDescription: '從這個書架下載的書會從裝置上刪除。閱讀進度與紀錄會保留，日後重新加入時可以接續。',
    removeConfirm: '移除',
    removeCancel: '先留著'
  },
  libraryForms: {
    editBook: {
      title: '編輯中繼資料',
      description: '可更新目前 API 支援的欄位。',
      basicInfo: '基本資訊',
      titleLabel: '書名',
      titlePlaceholder: '書名',
      authorsLabel: '作者（以逗號分隔）',
      authorsPlaceholder: '作者 A, 作者 B',
      organization: '整理',
      publishedAt: '出版日期',
      languageLabel: '語言',
      starRating: '星等',
      starValueOne: '1 星',
      starValueMany: '{count} 星',
      clearRating: '清除',
      tags: '標籤',
      tagsPlaceholder: '輸入標籤後按 Enter',
      tagsHelp: '按 Enter 或逗號新增標籤，點 × 可移除。',
      removeTag: '移除標籤「{tag}」',
      comment: '備註',
      commentPlaceholder: '關於這本書的筆記',
      commentHelp: '詳情頁會渲染 Markdown 與基本 HTML；儲存的是你輸入的原文。',
      commentPreviewShow: '顯示預覽',
      commentPreviewHide: '隱藏預覽',
      commentPreviewLabel: '備註預覽',
      commentPreviewEmpty: '目前沒有可預覽的內容。',
      identifiers: '識別碼',
      identifierKeyPlaceholder: 'isbn',
      identifierValuePlaceholder: '9787020002207',
      identifierKeyLabel: '第 {index} 個識別碼名稱',
      identifierValueLabel: '第 {index} 個識別碼內容',
      removeIdentifier: '移除識別碼「{name}」',
      addIdentifier: '新增識別碼',
      save: '儲存中繼資料',
      saving: '儲存中...',
      loading: '載入書籍中繼資料中...',
      loadFailed: '載入中繼資料失敗',
      saveFailed: '儲存中繼資料失敗'
    },
    newBook: {
      title: '新增空白書籍',
      closeLabel: '關閉新增空白書籍對話框',
      description: '只用書名建立一本空白的 TXT 書籍。',
      titleLabel: '書名',
      titlePlaceholder: '請輸入書名',
      create: '建立',
      creating: '建立中...',
      createFailed: '建立空白書籍失敗。'
    },
    importBook: {
      title: '匯入書籍',
      closeLabel: '關閉匯入對話框',
      description: '上傳 TXT、Markdown 或 EPUB 檔案來建立新書，也可以把檔案拖曳到這裡。',
      fileLabel: '書籍檔案（.txt、.md、.epub）',
      epubTitle: 'EPUB 轉換',
      epubDescription:
        'EPUB 匯入時會轉換成 source，不保留原始檔案；Markdown 預設可保留支援的圖片。',
      convertTo: '轉換成',
      presetMarkdown: 'Markdown（章節標題）',
      presetPlain: '純文字',
      plainHint:
        '純文字沒有章節導覽，也不支援 Markdown 圖片。日後仍可另外建立分章的 Markdown 來源，不會動到這份 TXT。',
      includeDescription: '正文開頭放入書籍簡介',
      selectedFiles: '已選檔案',
      fileTitle: '書名：{title}',
      fileStatus: '狀態：',
      submit: '匯入',
      submitting: '匯入中...',
      statuses: {
        pending: '等待中',
        importing: '匯入中',
        success: '已匯入',
        failed: '失敗'
      },
      errors: {
        noFiles: '請至少選擇一個 TXT、Markdown 或 EPUB 檔案。',
        unsupportedExtension: '書籍檔案必須是 .txt、.md 或 .epub。',
        allFailed: '匯入失敗。',
        someFailed: '有 {count} 個檔案失敗。'
      },
      results: {
        one: '匯入成功。',
        many: '已匯入 {count} 個檔案。',
        partial: '已匯入 {count} 個檔案，共 {total} 個。'
      }
    }
  },
  sources: {
    list: {
      title: '來源',
      total: '共 {count} 個',
      create: '新增',
      creating: '建立中...',
      loading: '載入來源中...',
      empty: '尚無來源。',
      listLabel: '書籍來源',
      current: '使用中',
      delete: '刪除',
      deleteLabel: '刪除來源 {id}'
    },
    // 沒有記錄格式的來源屬於 source format 出現之前的舊資料。
    // 其他情況徽章直接顯示大寫的格式名，不需要字典條目。
    format: {
      legacy: '舊格式'
    },
    formatActions: {
      manualMarkdown: '手動 TXT → MD',
      regexMarkdown: '正規表示式 → MD',
      lineCountMarkdown: '固定行數 → MD',
      plainText: '建立純文字來源',
      plainTextHelp: '標題階層與章節導覽會遺失。'
    },
    editor: {
      loading: '載入來源中...',
      noSelection: '請先選擇一個來源再開始編輯。',
      dirty: '有未儲存的變更',
      clean: '沒有待儲存的變更',
      setCurrent: '設為使用中',
      settingCurrent: '設定中...',
      contentLabel: '來源內容',
      find: {
        groupLabel: '尋找與取代',
        findLabel: '尋找',
        findPlaceholder: '搜尋文字',
        replaceLabel: '取代',
        replacePlaceholder: '取代為',
        scopeLabel: '範圍',
        scopeSection: '目前章節',
        scopeSource: '整份來源',
        caseSensitive: '大小寫相符',
        wholeWord: '全字比對',
        regexp: '正規表示式',
        previous: '上一個',
        next: '下一個',
        replace: '取代',
        replaceAll: '全部取代',
        noMatches: '沒有符合的結果。',
        matchOrdinal: '第 {ordinal} 個，共 {total} 個。',
        matchCount: '共 {total} 個符合。',
        // 中文沒有單複數變化，兩個形式的譯文相同；分成兩個 key 是為了英文。
        replacedOne: '已取代 1 處。',
        replacedMany: '已取代 {count} 處。',
        replacedOneNoneRemain: '已取代 1 處，沒有其他符合的結果。',
        replacedOneThenMatch: '已取代 1 處。第 {ordinal} 個，共 {total} 個。',
        replacedOneThenCount: '已取代 1 處。共 {total} 個符合。',
        invalidRegexp: '這個正規表示式不合法。',
        // 由編輯器自己的 live region 唸出，與上方看得見的狀態列各自獨立。
        announce: {
          currentMatch: '目前的符合項目',
          onLine: '位於行號',
          replacedOnLine: '已取代第 $ 行的符合項目',
          replacedMatches: '已取代 $ 處'
        }
      }
    },
    conversion: {
      confirm: '建立來源',
      busy: '建立中...',
      titles: {
        manualMd: '建立分章的 Markdown 來源',
        regexMd: '把標題行轉成章節',
        lineCountMd: '依固定長度分割章節',
        plainText: '建立純文字來源'
      },
      descriptions: {
        manualMd: '把目前的 TXT 來源複製成 Markdown 草稿。建立之後可以在來源編輯器裡自行加上 H2 章節。',
        regexMd: '符合的標題行會改寫成 Markdown 的 H2 標題。',
        lineCountMd: '每個預覽到的分界處都會插入一個 H2「Part N」標題。',
        plainText: 'Markdown 標記會被移除，標題階層與章節導覽會遺失。'
      },
      patternLabel: '章節標題的正規表示式',
      patternHelp: '第 1 個擷取群組會成為 H2 標題；沒有擷取群組時，會使用整個符合的內容。',
      lineCountLabel: '每章行數',
      previewTitle: '預覽',
      emptySource: '（來源是空的）',
      setCurrent: '把新來源設為使用中',
      summaries: {
        manualMdOne: 'Markdown 草稿一開始會有 1 個 H2 章節。',
        manualMdMany: 'Markdown 草稿一開始會有 {count} 個 H2 章節。',
        regexMd: '會建立 {count} 個 H2 章節標題。',
        lineCountMd: '會插入 {count} 個 H2 章節標題。',
        plainText: '會建立一個沒有章節結構的 TXT 段落。'
      },
      errors: {
        emptyPattern: '請輸入正規表示式。',
        patternMatchedNothing: '這個正規表示式沒有比對到任何章節標題行。',
        invalidLineCount: '每章行數必須是正整數。',
        previewFailed: '無法預覽這個轉換。'
      }
    },
    page: {
      title: '編輯來源',
      back: '返回',
      save: '儲存',
      saveDirty: '儲存*',
      saving: '儲存中...',
      loading: '載入來源中...',
      panelsLabel: '來源編輯器面板',
      paneSources: '來源',
      paneEditor: '編輯器',
      paneChapters: '章節',
      discard: {
        title: '要捨棄未儲存的變更嗎？',
        message: '你有尚未儲存的變更，要捨棄嗎？',
        confirm: '捨棄',
        cancel: '繼續編輯'
      },
      deleteSource: {
        title: '要刪除來源嗎？',
        confirm: '刪除',
        question: '確定要刪除來源「{id}」嗎？這個動作無法復原。',
        dirtyWarning: '你有尚未儲存的變更，將會遺失。'
      },
      renameChapter: {
        title: '重新命名章節',
        confirm: '重新命名',
        titleLabel: '章節標題'
      },
      mergeChapter: {
        title: '要合併章節嗎？',
        confirm: '合併',
        question: '移除 H2 標題「{title}」，並把它的內文與相鄰段落合併？'
      },
      messages: {
        sourceSaved: '來源已儲存。',
        currentUpdated: '已更新使用中的來源。',
        derivedCreated: '已建立衍生來源。'
      },
      errors: {
        loadSources: '載入來源失敗',
        loadContent: '載入來源內容失敗',
        save: '儲存來源失敗',
        setCurrent: '設定使用中來源失敗',
        create: '建立來源失敗',
        createDerived: '建立衍生來源失敗',
        delete: '刪除來源失敗'
      }
    },
    chapters: {
      title: '章節',
      headingCount: '{count} 個 H2 標題',
      add: '新增',
      allMarker: '全部',
      wholeSource: '整份來源',
      rename: '重新命名',
      merge: '合併',
      empty: '尚無 H2 章節。'
    }
  },
  notFound: {
    title: '找不到頁面',
    description: '這個網址沒有對應的內容。可能已經改名，或是連結過期了。',
    backToLibrary: '回到書庫'
  }
} as const;

export default zhHant;
