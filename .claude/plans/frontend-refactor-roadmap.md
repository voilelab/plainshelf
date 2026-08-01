# Frontend 內部結構重整路線圖

> 狀態：**PR 1–7 已合併**（PR 7 範圍縮小，見該節）。撰寫於 2026-07-30。
> 唯一剩餘工作是 PR 7 的 section 元件拆分，前置條件是補 settings 的 e2e。
>
> 這是一份一次性的工作文件，不是規則檔：不受 `.claude/rules/40-maintenance.md` 的
> rules 責任表管轄，也不進 `mkdocs.yml`（不對外發布）。路線圖走完或被取代後請直接
> 刪除，Git 歷史是存檔。
>
> 文件末尾的「可翻轉的決定」列出撰寫當時未經使用者確認、由該 session 自行選了保守
> 預設的項目。開工前先確認那幾項。

## Context

`frontend/src` 目前是兩種組織風格共存的狀態：只有 `dashboard`、`reader`、`sources`
三個功能進了 `features/`，其餘（library/books、maintenance、trash、settings、
mobile/offline、layers）仍然攤平在 `pages/` + `components/` + `composables/` +
`utils/`。git 歷史顯示兩種風格是同時期被加進來的，沒有一次收斂。

這造成三類具體成本：

1. **沒有 path alias**（`vite.config.ts` 只有 `plugins` + `server`），feature 內部靠
   `../../../providers` 這種相對路徑往外接。任何檔案搬移都會連帶改寫一堆不相干檔案的
   import，使「搬檔」這件事的 diff 大到難審。
2. **三個列表頁重複貼上同一段邏輯**。`LibraryPage.vue`、`MaintenanceBooksPage.vue`、
   `ReadHistoryPage.vue` 逐字重複 action adapter、分頁切片、page-clamp watcher、
   query 寫入器；其中 `openBook` / `openEdit` 還遮蓋了 `useBookActions` 已經匯出的
   `openDetail` / `goEdit`。
3. **兩個超大 SFC**：`MainLayout.vue` 1341 行（側邊欄 rail/drawer/段落摺疊 + 完整
   layer CRUD + 書架選擇 + 語言選擇，12 個 busy/error ref 並列）、`SettingsPage.vue`
   1101 行（6 組互不相干的設定 + 一整套 shelf CRUD）。

**目標**：把 `frontend/src` 收斂成單一組織慣例、消掉可驗證的重複、拆開兩個超大 SFC，
**不改變任何使用者可見行為**。依 repo 慣例拆成一系列小 PR 進 `dev`，每個 PR 一件事。

## Scope

- 只動 `frontend/src`、`frontend/vite.config.ts`、`frontend/tsconfig.json`。
- 不動 `shelf/`、`server/`、`desktop/`、`frontend/web.go`。`desktop/main.go:24` 消費的是
  `frontend.WebFS`（編譯期嵌入的 `dist`），搬 `src` 檔案不影響 Go 模組。
- 不改 API 契約、不改 `local_token` 邊界、不改 on-disk 格式。

## 目標佈局

```
src/
  features/<name>/{pages,components,composables,utils}/   # 單一 feature 專屬
  components/ composables/ utils/                          # ≥2 個 feature 共用者才留在頂層
  api/ providers/ types/ i18n/ layouts/ styles.css         # 跨 feature 基礎設施，不動
```

判準：**只有一個 feature import 的東西搬進該 feature；兩個以上 import 的留在頂層。**

---

## PR 1 — 導入 `@/` path alias（enabler，純機械）

必須第一個做：沒有 alias 的話，後面每個搬檔 PR 都會順帶改寫一批不相干檔案的
`../../../` 鏈，diff 失控。

- `frontend/vite.config.ts`：加 `resolve.alias`，`'@' → fileURLToPath(new URL('./src', import.meta.url))`。
- `frontend/tsconfig.json`：加 `baseUrl: "."` 與 `paths: { "@/*": ["src/*"] }`。
- 把**跨目錄**的相對 import 改成 `@/`；同目錄的 `./x` 保持原樣。
- 專案沒有獨立的 vitest 設定檔，vitest 直接讀 `vite.config.ts`，所以同一份 alias 也覆蓋
  單元測試；`vue-tsc` 走 tsconfig `paths`。兩邊都要驗。

驗收：diff 內**只有 import 路徑**變動，沒有任何邏輯改動；`npm --prefix frontend test`
與 `npm --prefix frontend run build` 皆通過。

## PR 2 — 刪除無人 import 的模組

grep 全 `src` 找不到任何 import：

- `src/composables/useBooks.ts`（31 行，`useBookStore.ts` 的死複本，兩者都呼叫
  `listBooks(1, Number.MAX_SAFE_INTEGER)`）
- `src/components/BookCard.vue`（188 行，已被 `BookCardView.vue` 取代）
- `src/pages/ImportBookPage.vue`（35 行，`/import` 路由改成 redirect 到
  `/books?import=1`，見 `src/router.ts:81-91`）
- `src/utils/csv.ts`（6 行）

`src/providers/indexedDbMobileBookCache.ts`（298 行）**保留**：`src/providers/index.ts:16-18`
的註解說明了當初刻意選 Filesystem 版而非 IndexedDB，讀起來像刻意保留的替代實作。此 PR
只在檔頭加一行註解標明它目前未接線。（見末尾「可翻轉的決定」。）

驗收：build + test 通過；PR 描述附上 grep 證據。

## PR 3 — 抽出三個列表頁共用的邏輯（本輪最高價值）

已驗證逐字重複（`diff` 比對零差異）：

| 重複內容 | 位置 |
|---|---|
| 25 行 action adapter（`findBook` / `onOpenBookFolder` / `onDownloadBook` / `onRequestDeleteBook`） | `LibraryPage.vue:222-246`、`MaintenanceBooksPage.vue:237-261`、`ReadHistoryPage.vue:169-193` |
| `totalPages` + `visibleBooks` 切片 | `LibraryPage.vue:294-299`、`MaintenanceBooksPage.vue:167-172`、`ReadHistoryPage.vue:76-80` |
| page-clamp watcher（`{immediate:true}` + `router.replace`） | `MaintenanceBooksPage.vue:262-277`、`ReadHistoryPage.vue:194-209`（`LibraryPage.vue:539-572` 是加上 layer/search/sort 的超集） |
| `buildQuery` 路由 query 寫入器 | `MaintenanceBooksPage.vue:148-164`、`ReadHistoryPage.vue:84-89` |
| `onPageChange` / `onPageSizeChange` | `MaintenanceBooksPage.vue:174-191`、`ReadHistoryPage.vue:125-142` |
| `useBookActions({onDeleted})` 的同一組 10 欄位解構 | `MaintenanceBooksPage.vue:220-235`、`ReadHistoryPage.vue:152-167` |

做法：

1. 新增 `src/composables/useBookCollectionRoute.ts` —— 擁有 `page`（由 route 讀）、
   `totalPages`、`visibleBooks`、`onPageChange`、`onPageSizeChange`、page-clamp watcher。
   `buildQuery` 以參數形式注入（callback 或 extra-query ref），讓 Maintenance 的
   `maxChars` 與 Library 的 layer/search/sort 額外 query 鍵能沿用。
   重用既有的 `useBookPagination.ts` 的 `pageSize` / `setPageSize` / `toPage` /
   `toSingleQueryValue`，不要另造。
2. 新增 `src/composables/useBookCollectionActions.ts` —— 吃 `books` ref + `readOnly`，
   回傳那 25 行 adapter 的等價物，內部包 `useBookActions`。
3. 刪掉 `MaintenanceBooksPage.vue` / `ReadHistoryPage.vue` 裡本地的 `openBook` /
   `openEdit`，改用 `useBookActions` 已有的 `openDetail`（`useBookActions.ts:37-39`）與
   `goEdit`（`:41-43`）。
4. 重複 4 次的字面字串 `"The book will be moved to Trash. You can restore it later."`
   （`LibraryPage.vue:6`、`ReadHistoryPage.vue:6`、`BookDetailPage.vue:6`、
   `MaintenanceBooksPage.vue:6`）收成一個共用常數。**不要**改成 `DeleteModal` 的
   prop 預設值——它有 9 個消費者，改 prop 契約要逐一核對每個 call site
   （見 `.claude/rules/50-lessons.md` 的 shared component contracts）。把它翻譯進
   i18n 是另一個 PR 的事。

順序：先 `ReadHistoryPage` → `MaintenanceBooksPage`（兩者幾乎同形），最後才動
`LibraryPage`（它是超集，風險最高）。

驗收：新 composable 的純邏輯部分（clamp、切片、query 組裝）加單元測試——`package.json`
沒有 `@vue/test-utils`，所以**只能測純函式，不能測 render**；元件行為交給既有 e2e
`library-search.spec.ts`、`read-history.spec.ts`、`maintenance-low-char-count.spec.ts`。

## PR 4 — 把 mock backend 從 `api/` 拆出

`src/api/books.ts` 731 行裡有兩套實作：真 HTTP wrapper（`:437-731`）與 in-file mock
backend（`mockBooks` fixture `:146-340`、`mockListBooks`/`mockGetBook`/… `:341-436`）。
`src/api/layers.ts`（299 行）同病，mock 狀態在 `:45-117`。

- 新增 `src/api/mocks/books.ts`、`src/api/mocks/layers.ts`，把 fixture 與 mock handler
  搬過去。
- `api/client.ts` 的 `isMock()` gate（`:86`）與 production 防呆（`:47-55, 94`）**原封不動**。
- 純搬移，不改任何 mock 行為或資料。

驗收：`VITE_USE_MOCK_API=true npm --prefix frontend run dev` 仍能列出書單；build + test 通過。

## PR 5a–5f — 逐 cluster 搬進 `features/`（每個 cluster 一個 PR）

每個 PR 都是**純 rename + import 更新 + `router.ts` 的 lazy import 路徑**，零邏輯改動。

**歸屬判準**（本輪一致適用，各 cluster 的清單都是照它掃 import 得出的）：模組只有一個
消費者 → 跟著那個消費者進它的 feature；有兩個以上消費者、或消費者橫跨 cluster → **留頂層**。
把共用模組塞進某個 feature 會製造 feature 之間的反向依賴，比攤平更難拆。

由邊界最乾淨的開始：

- **5a `features/maintenance/`** — `MaintenanceBooksPage.vue` + 4 個 13 行 wrapper
  (`MissingAuthorPage` / `MissingCoverPage` / `MissingLanguagePage` / `LowCharCountPage`)
  + `DuplicateContentPage.vue` + `DuplicateGroupCard.vue` + `DuplicateBookRow.vue`
  + `useCharCountBooks.ts`。
  注意 `src/utils/maintenance.ts` 有兩個消費者：`MaintenanceBooksPage.vue:67-72` 與
  `src/layouts/MainLayout.vue:485`（側邊欄 nav registry）。依判準它是共用的 →
  留在 `src/utils/`，或把 nav registry 與 filter registry 分開，只搬 filter 那半。
- **5b `features/trash/`** — `TrashPage.vue`（自成一體，唯一消費者）。
- **5c `features/settings/`** — `SettingsPage.vue`、`AdminLogsPage.vue`、
  `utils/externalLinks.ts`（唯一消費者 `SettingsPage.vue:366`）。
  `useShelvesStore.ts` **留頂層**：消費者有三處（`SettingsPage.vue`、`MainLayout.vue`、
  `MobileConnectPage.vue`）。`useServerMode.ts` 也**留頂層**，而且它根本不是 settings
  的模組 —— 消費者是 `MainLayout.vue` 與 `BookDetailPage` / `LibraryPage` /
  `MaintenanceBooksPage` / `ReadHistoryPage`，`SettingsPage.vue` 沒有 import 它。
- **5d `features/mobile/`** — `MobileConnectPage.vue`、`DownloadsPage.vue`、
  `utils/bytes.ts`（唯一消費者 `DownloadsPage.vue:98`）。
  `useOfflineDownload.ts` 的唯一消費者是 `BookDetailPage.vue`，不是本 cluster 的任何一頁；
  依「唯一消費者跟著走」它該進 5e library，但檔名讀起來屬於 mobile —— 搬之前先確認
  （見末尾可翻轉的決定 5）。
  `src/providers/mobile*` 留在 `providers/`（runtime 基礎設施，不是 feature）。
- **5e `features/library/`** — 只收消費者全在 library 內的模組：`LibraryPage.vue`、
  `BookDetailPage.vue`、`EditBookPage.vue`、`BookDetail.vue`、`BookCover.vue`、
  `GenerateCoverModal.vue`（唯一消費者 `BookCover.vue`）、`EditBook.vue`、
  `ImportBookModal.vue`、`NewEmptyBookModal.vue`、`useBooksRouteQuery.ts`（唯一消費者
  `LibraryPage.vue`）。`ImportBookModal` 原有兩個消費者，但 `ImportBookPage.vue` 在 PR 2
  已刪，到這一步只剩 `LibraryPage.vue`。

  **以下留頂層**：它們有跨 cluster 消費者，搬進 `features/library/` 會讓 maintenance、
  read-history、dashboard 反向依賴 library 內部，違反本輪自訂的「兩個以上消費者就留頂層」
  判準。

  | 留頂層的模組 | 實際消費者 |
  |---|---|
  | `BookCollectionPage.vue` | `LibraryPage`、`MaintenanceBooksPage`、`ReadHistoryPage` |
  | `BookListView` / `BookCardView` / `BookTitleView` / `Pagination` | 唯一消費者是 `BookCollectionPage.vue`，跟著它一起留 |
  | `BookCoverImg.vue` | 上述四個 view 元件 + `features/dashboard/components/RandomBook.vue` |
  | `useCoverSrc.ts` | 唯一消費者 `BookCoverImg.vue`，跟著它留 |
  | `useBookStore.ts` | 6 處，含 `MainLayout`、`TrashPage`、`LayerTree`、`ImportBookModal` |
  | `useBookActions.ts` / `useBookPagination.ts` | library + maintenance + read-history 三邊共用 |
  | `useServerMode.ts` | 見 5c |

  read-history 不另立 cluster：`ReadHistoryPage.vue` 的組成幾乎全是上表的共用模組，
  獨立出來只會多一層 import 轉折，留在 `pages/`。
  務必在 PR 3 之後做，否則會跟去重的 diff 打結。
- **5f 統一 feature 內部命名** — `features/reader/views/` → `features/reader/pages/`，
  對齊 `dashboard`/`sources`。`src/layouts/ReaderLayout.vue` **留在 `layouts/`**：它同時
  是 reader 與 sources 的外殼（`src/router.ts:144-167`）。

驗收（每個 PR）：`git diff -M` 應把變動辨識為 rename；`npm --prefix frontend run build`
（含 `vue-tsc --noEmit`）通過；`npm --prefix frontend test` 通過。

## PR 6 — MainLayout 邏輯抽出（template / CSS 不動）

`src/layouts/MainLayout.vue` 1341 行 = template 447 + script 515 + CSS 374。只動 script：

- 抽 `useSidebarLayout()` — rail/expand/寬度持久化（`:505-517, 661-699`）、
  窄視窗 drawer（`:531-545`）、段落摺疊（`:509, 568-573, 697-699`）。
  重用既有的 `src/utils/sidebarMode.ts`（已有 `sidebarMode.test.ts`）。
- 抽 `useLayerCrud()` — create（`:701-760`）、rename（`:789-845`）、move（`:847-871`）、
  delete（`:888-946`）、book move-to-layer（`:762-787`），並把 `:558-568` 那 12 個並列的
  busy/error ref 收成 per-operation 的狀態物件。
- **維持** layer CRUD 直接呼叫 `src/api/layers.ts`（`MainLayout.vue:478`）的現狀。把 layer
  CRUD 提進 `BookshelfProvider` 會改變 Wails / mobile runtime 的行為，那是功能變更不是
  重整，需要另外決定（見下）。

驗收：e2e `sidebar-rail.spec.ts`、`sidebar-foldable.spec.ts`、`layer-tree.spec.ts` 是這個
PR 的回歸網，必跑。

## PR 7 — SettingsPage 抽邏輯（已完成，範圍縮小）

原訂拆成 `features/settings/components/` 下的五個 section 元件、頁面退化成組合外殼。
**實際只做了 script 抽取**：`useShelfManagement`（書架 CRUD）與 `utils/settingsDraft.ts`
（`parseReadHistoryLimit`、`buildDefaultSplitConfig` 兩個純函式，附單元測試），
`SettingsPage.vue` 1101 → 925 行，template 與 370 行 scoped CSS 的 diff 為零。

縮小的理由：`/settings` **完全沒有 e2e 覆蓋**，搬 template/CSS 沒有任何自動化回歸網。
只抽 script 則 render 結果不可能改變，`vue-tsc` 與既有測試就足以擔保。

**剩餘工作**（需獨立 PR，依序）：先補 settings 的 e2e，再拆 section 元件。六個分頁在
template 上邊界清楚、CSS 選擇器有 `.shelf-*` 這類前綴，拆分本身划算，缺的是驗證手段。
桌面目錄瀏覽器在此環境無法自動驗證，屆時 PR 描述要明說。

---

## 明確排除（已發現但不在本輪）

這些是調查中確認存在的問題，但屬於「可見行為變更」或「非結構」，各自需要獨立 PR 與決定：

- **i18n 漂移**：`pages/ components/ features/ layouts/` 下有 24 個 `.vue` 沒有 import
  `useI18n`，包含整個 `features/sources/` 與 `DuplicateContentPage.vue`（全部硬編碼）。
- **Maintenance / ReadHistory 少傳 `shelf-initializing` / `shelf-unreachable`**：
  `BookCollectionPage.vue:3-7` 有這兩個狀態，但只有 `LibraryPage` 傳；
  `useCharCountBooks.ts:50-56` 算出的 503 retry 狀態到不了 UI。修它會改變可見行為。
- **`view-mode-storage-key` 只有 ReadHistoryPage 傳**（`:26`），Library 與 Maintenance 共用
  同一個全域 view-mode key。
- **Layer CRUD 不在 `BookshelfProvider`**：`bookshelfProvider.ts:39-91` 沒有 layer CRUD，
  所以建立/改名/搬移/刪除 layer 在 Wails 與 mobile runtime 上走不通。
- **`useDashboardData.ts:2-6` 的 `TODO(next phase)`**：`char_count` 沒設計進 provider 介面，
  該檔同時用 `api/books` 與 `getBookshelfProvider()` 兩條路。而 `useCharCountBooks.ts:10-17`
  記載了刻意把 char-count 排除在共用 store 外的理由——這是介面設計決策，不是重整。
- **`listBooks` 有 4 份實作**（`useBookStore.ts:25-60`、`useCharCountBooks.ts:33-67`、
  `useDashboardData.ts`、死掉的 `useBooks.ts`），其中兩份各有近乎相同的 503 retry 迴圈。
  統一它需要先解決上面那個介面決策，故不排入本輪。
- 未導入 eslint / prettier / `@vue/test-utils`。

## Verification

每個 PR 的最小檢查：

```bash
npm --prefix frontend ci            # 受限環境用 --ignore-scripts（sharp 抓不到 libvips）
npm --prefix frontend test          # vitest run
npm --prefix frontend run build     # vue-tsc --noEmit && vite build
go test ./...                       # dist 是編譯期嵌入的，build frontend 後才跑
```

碰 UI 行為的 PR（3、6、7）額外跑相關 e2e。此容器沒有 `just` / `zsh`，直接照 `justfile`
裡的底層指令跑；**不要**執行 `playwright install`，容器已預裝 chromium 於
`/opt/pw-browsers`，用一份臨時 config 設 `launchOptions.executablePath:
'/opt/pw-browsers/chromium'`（見 `.claude/rules/50-lessons.md`）。

`desktop/` 在 headless 容器內無法實跑，只能 `cd desktop && go test ./...`；本路線圖不動
desktop Go 碼，但 PR 描述要如實標註未驗證項。

## 交付方式

- 每個 PR 從最新的 `dev` 開分支（`dev` 是預設分支，不是 main），一個 PR 一件事。
- commit message 與程式碼註解維持英文，對話用繁體中文。
- `CHANGELOG.md` 用 `update-changelog` skill，不手寫；純內部重整原則上不需要 release
  notes，PR 5e / 6 / 7 若你認為值得記，再用 skill 補。

## 可翻轉的決定（我用了保守預設，你可以直接改）

1. **目標形狀**：我預設「全面收斂到 `features/`」（PR 5a–5f）。若你只想搬自包含的
   cluster，砍掉 5e（library）即可，其餘不受影響。
2. **`indexedDbMobileBookCache.ts`**：我預設**保留**（`providers/index.ts:16-18` 的註解
   讀起來像刻意留的替代實作）。要一起刪的話併進 PR 2，git 歷史可取回。
3. **大 SFC 拆到多深**：我預設 PR 6 只抽邏輯、不動 template/CSS（風險低很多），PR 7 才
   真的拆元件。若你想 MainLayout 也拆側邊欄子元件，那要再加一個 PR，且 374 行 scoped
   CSS 的搬移需要逐一比對 computed style。
4. **重複的 trash 描述字串**：我預設收成共用常數、不動 `DeleteModal` 的 prop 契約
   （9 個消費者）。若你想順手做成 i18n key，那應該歸到被排除的 i18n PR 系列。
5. **`useOfflineDownload.ts` 歸哪一邊**：唯一消費者是 `BookDetailPage.vue`，照歸屬判準
   該進 `features/library/`，但它的職責與檔名都偏 mobile/offline。我沒有替它選邊 ——
   進 library（照判準）或留頂層（照語意）都成立，開工到 5d／5e 時請指定一個。
