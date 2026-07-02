# PlainShelf — Claude session 指南

PlainShelf：local-first 個人閱讀庫（TXT 為主），Go 後端 + Vue 前端 + Wails desktop。
filesystem-first：`books/` 下的 `.bookpkg` 資料夾是 source of truth，runtime state 可重建。
狀態 pre-alpha。Non-goals（不要提議）：EPUB/PDF/DRM、多使用者、雲同步、plugin。

## 指令對照表（cloud 容器實測；容器內沒有 `just` 和 `zsh`，不要照 README/justfile 用 just）

| 目的 | 指令 |
|---|---|
| Build frontend（跑任何 Go 指令前必做一次） | `npm --prefix frontend install && npm --prefix frontend run build` |
| Go 測試（主 module） | `go test ./...` |
| Go 測試（desktop module） | `cd desktop && go test ./...` |
| 前端型別檢查 + build | `npm --prefix frontend run build`（內含 `vue-tsc --noEmit`） |
| 前端 dev（mock 資料，不需後端） | `cd frontend && VITE_USE_MOCK_API=true npm run dev` |
| 跑 server | `mkdir -p workspace && cp cmd/plainshelf-srv/conf/config.yaml workspace/ && cd workspace && go run ../cmd/plainshelf-srv/main.go -conf config.yaml`（監聽 127.0.0.1:20000） |
| e2e（重，只在碰 UI 行為時跑） | `npm --prefix e2e ci && npm --prefix e2e test` — **不要** `playwright install`，chromium 已在 `/opt/pw-browsers` |

## 陷阱（每一條都是實測撞過的）

- `go test ./...` 在 `frontend/dist/` 不存在時會**編譯失敗**（`frontend/web.go` 有 `//go:embed dist/*`）。錯誤長得像 Go 問題，實際是 frontend 沒 build。
- 根目錄 `image.png` 有 870 KB —— 不要 Read 它。
- 預設分支是 **`dev`**（不是 main）。開分支：`git fetch origin dev && git checkout -b <name> origin/dev`。PR 目標 dev。
- desktop/（Wails）在 headless 容器內不能實跑 GUI，只能編譯與單元測試。
- server 的 mutating API 有 `local_token` 檢查（`server/security.go`）；改 API 時 `server/api_contract_test.go` 是行為契約，先讀再改。

## 規則路由（需要時再讀對應檔，不要一次全讀）

| 情境 | 讀這個 |
|---|---|
| 要派 subagent、選 model、驗證產出 | `.claude/rules/10-delegation.md` |
| 不確定該升級/該問人/算不算完成/方向對不對 | `.claude/rules/20-judgment.md` |
| 要寫派工 prompt（搜尋/實作/重構/研究/審查） | `.claude/rules/30-prompt-templates.md` |
| 要修改本檔或 rules/ 下任何檔 | `.claude/rules/40-maintenance.md`（先讀，有分級權限） |
| 開始任何任務前，快速掃過往教訓 | `.claude/rules/50-lessons.md` |
| 想了解這套規則的來由與極限 | `.claude/rules/00-diagnosis.md`、`.claude/rules/90-letter.md` |

## 每個 session 的最低要求

1. 動手前掃一遍 `50-lessons.md`（很短，直接讀）。
2. 位置未知的搜尋、>3 個檔案的閱讀、網頁研究、批次改檔 → 派 subagent（規則見 10-delegation.md），主對話不下場。
3. 宣告完成前：跑過對照表裡對應的測試指令，且產出檔案 read-back 過。測試沒跑或沒過就不能說「完成」。
4. 踩到新坑：修完後把教訓寫進 `50-lessons.md`（格式見 40-maintenance.md）。
