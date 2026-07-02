# 50 — 踩坑教訓（每個 session 開工前掃一遍）

格式與新增規則見 `40-maintenance.md` 第 2 節。一條一行，新條目加在最上面。

- [2026-07-02] 症狀：`go test ./...` 編譯失敗，錯誤指向 `frontend/web.go` 的 embed → 根因：`frontend/dist/` 不存在，`//go:embed dist/*` 要求它必須存在 → 規則：任何 Go 指令前先確認 dist 存在，沒有就先 `npm --prefix frontend install && npm --prefix frontend run build`。
- [2026-07-02] 症狀：照 README 跑 `just test-go` 找不到指令 → 根因：cloud 容器沒裝 `just` 和 `zsh`（justfile 指定 zsh shell） → 規則：容器內不用 just，改用 CLAUDE.md 指令對照表的等效指令；陌生指令先 `which` 確認存在。
- [2026-07-02] 症狀：（預防性）e2e 設定想跑 `playwright install` → 根因：容器已預裝 chromium 於 `/opt/pw-browsers`，環境變數 `PLAYWRIGHT_BROWSERS_PATH` 已指向它 → 規則：永不執行 `playwright install`；若專案 pin 的 playwright 版本不符，用 `executablePath: '/opt/pw-browsers/chromium'`。
- [2026-07-02] 症狀：（預防性）從 `main` 開分支會失敗或 base 錯誤 → 根因：本 repo 預設分支是 `dev`，沒有 main → 規則：一律 `git fetch origin dev && git checkout -b <name> origin/dev`，PR 目標 dev。
