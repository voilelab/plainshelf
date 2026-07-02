# 00 — Harness 快速診斷（2026-07-02，由 Fable 5 撰寫）

> 本檔是一次性的診斷報告，後續所有制度檔（10/20/30/40）都引用這裡的問題編號。
> 屬歷史文件：**不要修改**（維護規則見 `40-maintenance.md`）。

診斷對象：voilelab/plainshelf 的 Claude Code cloud session 環境（每次 session 都是全新容器、repo 重新 clone、無跨 session 記憶）。

---

## 問題 1：零專案記憶 —— token 流失最大宗

**症狀**：本 session 開始時 repo 內沒有任何 CLAUDE.md。每個新 session 都要重新推導：專案結構、build/test 指令、資料模型、預設分支。以本次實測為例，光是搞清楚「go test 之前必須先 build frontend」就需要讀 README、justfile、`frontend/web.go` 三個檔案外加一次失敗的測試執行。

**成本**：每 session 數千至上萬 token 的重複勘查；更糟的是推導常出錯（見問題 3），錯誤結論不會留下來，下個 session 再錯一次。

**修法**（本次已落實）：
- 根目錄 `CLAUDE.md`：只放每個 session 都需要的內容 —— 指令對照表、陷阱清單、路由表。用**路徑引用**指向 `.claude/rules/` 各檔，不用 `@import` 自動載入（自動載入會把所有規則常駐塞進每個 context，重演 token 流失）。
- 踩坑教訓寫回 `.claude/rules/50-lessons.md`（格式見 `40-maintenance.md`），讓錯誤只犯一次。

---

## 問題 2：主對話下場做大量讀取 —— 失焦最大宗

**症狀**：主對話直接掃 repo、整檔 Read、`git log` 全開、逐檔找關鍵字。原始資料（檔案內容、log、工具 schema）灌進主 context 後，任務目標被稀釋，典型表現是：忘記使用者原本要什麼、重複做已做過的查詢、summary 觸發後遺失中間結論。

**本 repo 的具體地雷**：
- 根目錄 `image.png`（870 KB）—— Read 它一次就吃掉大量 context，且對任何任務都沒有幫助。
- github MCP 有 100+ 個 deferred tools —— 用 ToolSearch 下模糊關鍵字會一次載入多個大 schema。載入用 `select:工具名` 精準指定；github MCP 的 list/search 類呼叫帶 `perPage` 5–10 並在支援時設 `minimal_output: true`。
- `CHANGELOG.md`、`docs/` 全文閱讀 —— 多數任務只需要其中一小段。

**修法**（規則落在 `10-delegation.md`）：主對話是指揮官，不下場。凡是「位置未知的搜尋」「超過 3 個檔案的閱讀」「網頁研究」「批次改檔」一律派 subagent，主對話只收結論與 `檔案:行號`。

---

## 問題 3：未驗證的假設直接執行 —— 出錯最大宗

**症狀**：憑印象或憑文件執行指令，撞牆後在錯誤方向上重試。本環境實測到的例子：
- README 與 justfile 都說用 `just`，但 **cloud 容器裡沒有 `just` 也沒有 `zsh`**（justfile 第一行 `set shell := ["zsh", "-cu"]`）。照文件執行必失敗。
- 直接跑 `go test ./...` 會因 `frontend/web.go` 的 `//go:embed dist/*` 在 `frontend/dist/` 不存在時**編譯失敗**，錯誤訊息看起來像 Go 問題，其實是 frontend 沒 build。
- 跑 `playwright install` —— 容器已預裝 chromium 於 `/opt/pw-browsers`，重裝既慢又可能失敗。
- 派工時模型名憑印象亂寫（例如寫 `claude-3-5-sonnet`）—— Agent tool 的 `model` 參數只接受 `haiku` / `sonnet` / `opus` / `fable` 四個值。

**成本**：每個錯誤假設觸發 2–5 輪重試，且弱模型傾向把重試花在「讓錯誤指令跑起來」而不是「質疑指令本身」。

**修法**：
- `CLAUDE.md` 的指令對照表只收錄**在 cloud 容器實測通過**的指令，未驗證的明確標註。
- 陌生指令執行前先 `which <cmd>` 或 `<cmd> --version` 確認存在。
- 同一個錯誤第三次出現 = 假設錯了，換路而非重試（判準見 `20-judgment.md`）。

---

## 排名以外、但值得知道的次要問題

- **e2e 測試很重**（npm ci + playwright + 起 server），不適合當作日常驗證手段；日常用 `go test` + `vue-tsc`，e2e 留給碰 UI 行為的改動。
- **desktop/（Wails）在 headless 容器內無法實跑**，只能編譯與跑單元測試；涉及桌面視窗行為的改動要明說「容器內無法驗證」。
- **git 預設分支是 `dev`**（不是 main）；歷史上所有工作分支都是 `claude/*` 且經 PR 合回 dev。從錯誤的 base 開分支會做出無法合併的 diff。
