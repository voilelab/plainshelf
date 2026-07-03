# 50 — 踩坑教訓（每個 session 開工前掃一遍）

格式與新增規則見 `40-maintenance.md` 第 2 節。一條一行，新條目加在最上面。
已被 CLAUDE.md 陷阱清單涵蓋的不重複寫；同主題累積 ≥3 條時升格進 CLAUDE.md 或對應規則檔（見 `40-maintenance.md` 第 4 節）。

格式範例（這一條只是示範，不是真教訓）：
`- [YYYY-MM-DD] 症狀：觀察到什麼 → 根因：真正原因 → 規則：下次怎麼避免，一句可執行的話。`

---

- [2026-07-03] 症狀：`npm --prefix e2e test` 8 個測試全在 browserType.launch 失敗（`Executable doesn't exist at /opt/pw-browsers/chromium_headless_shell-1223/...`）→ 根因：e2e 鎖定的 playwright 1.60 需要 chromium rev 1223，容器 `/opt/pw-browsers` 只預裝 rev 1194 → 規則：臨時在 `e2e/playwright.config.ts` 的 `use` 加 `launchOptions: { executablePath: '/opt/pw-browsers/chromium' }`，跑完 `git checkout e2e/playwright.config.ts` 還原；不要 `playwright install`（實測 8/8 過）。

（2026-07-02 建檔時的初始踩坑 —— go:embed 依賴 frontend/dist、容器無 just/zsh、playwright 已預裝、預設分支是 dev —— 已直接升格進 CLAUDE.md 陷阱清單，故此處不重複。）
