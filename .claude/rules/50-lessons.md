# 50 — 踩坑教訓（每個 session 開工前掃一遍）

格式與新增規則見 `40-maintenance.md` 第 2 節。一條一行，新條目加在最上面。
已被 CLAUDE.md 陷阱清單涵蓋的不重複寫；同主題累積 ≥3 條時升格進 CLAUDE.md 或對應規則檔（見 `40-maintenance.md` 第 4 節）。

格式範例（這一條只是示範，不是真教訓）：
`- [YYYY-MM-DD] 症狀：觀察到什麼 → 根因：真正原因 → 規則：下次怎麼避免，一句可執行的話。`

---

- [2026-07-04] 症狀：reka-ui DropdownMenu 的選單容器樣式全部失效（透明背景），但 item 樣式正常 → 根因：popper 定位類元件（DropdownMenu/Popover/Tooltip 的 Content）外層有 `data-reka-popper-content-wrapper`，使用端的 scoped `data-v-*` 落在 wrapper、class 落在內層元素，scoped selector 對不上；Dialog 沒有 wrapper 所以沒此問題 → 規則：portal + popper 的 Reka 內容一律用**非 scoped** `<style>` 寫樣式；驗證法：起 server 用 playwright-core 腳本查 computed style（見 scratchpad/check-dropdown.js 模式）。
- [2026-07-04] 症狀：改了 frontend 重新 build 後，跑著的 server 仍吐舊頁面 → 根因：`go:embed dist/*` 在編譯期打包，且 `go run` 起的 server 程序名稱不是 plainshelf-srv，`pkill -f plainshelf-srv` 殺不掉 → 規則：用 `fuser -k 20000/tcp` 殺掉佔 port 的程序後重新 `go run`（會重新編譯、重吃新 dist）。
- [2026-07-03] 症狀：`npm --prefix e2e test` 8 個測試全在 browserType.launch 失敗（`Executable doesn't exist at /opt/pw-browsers/chromium_headless_shell-1223/...`）→ 根因：e2e 鎖定的 playwright 1.60 需要 chromium rev 1223，容器 `/opt/pw-browsers` 只預裝 rev 1194 → 規則：臨時在 `e2e/playwright.config.ts` 的 `use` 加 `launchOptions: { executablePath: '/opt/pw-browsers/chromium' }`，跑完 `git checkout e2e/playwright.config.ts` 還原；不要 `playwright install`（實測 8/8 過）。

（2026-07-02 建檔時的初始踩坑 —— go:embed 依賴 frontend/dist、容器無 just/zsh、playwright 已預裝、預設分支是 dev —— 已直接升格進 CLAUDE.md 陷阱清單，故此處不重複。）
