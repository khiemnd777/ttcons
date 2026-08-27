# [Closed] Warehouse corrections and material editing — 2026-08-27

Closed at the user's request on 2026-08-27.

## Will Be Done

- Add audited material editing while locking material code/unit after operational usage.
- Add per-warehouse stock adjustment documents instead of directly overwriting derived stock positions.
- Add posted stock-document editing with metadata-only updates or an atomic correction/replacement chain for stock-affecting changes.
- Add audited project-site usage adjustments while keeping issued and surplus quantities derived from stock documents and usage history.
- Keep incoming-acceptance receipt caps, procurement/accounting handoff evidence, reports, handover blockers, RBAC and stock/site non-negative guards consistent.
- Add the required custom SQL migration, backend/frontend contracts and focused regression coverage; update all affected module/function/UI/root-cause/domain maps.
- Run backend formatting and focused/full tests plus frontend focused/full tests and production build; browser/UAT remains skipped unless explicitly requested.

## Done

- Review package and Mockup as Text for all four Warehouse tabs approved by the user.
- Confirmed from source that the previous workspace was list/create-only: material edit, stock adjustment, stock-document correction and global site-balance adjustment APIs/UI did not exist.
- Confirmed that warehouse positions and site balances are derived state, and posted documents are atomic ledger sources; the implementation uses traceable correction records rather than direct quantity overwrite.
- Added V132 correction-chain and site-adjustment persistence, plus protected material update, stock adjustment, stock-document update and site-usage adjustment routes.
- Material metadata is editable; code/unit lock after operational use, and all new stock movement units are validated against the material master.
- Stock positions use append-only adjustment documents; posted document ledger changes use atomic reversal/replacement links, while date/note-only edits stay on the original document with audit.
- Project-site usage adjustment changes only `used_quantity`, enforces `0 <= used <= issued`, recalculates surplus/status and appends adjustment/audit history.
- Added permission-aware row actions and shared dialogs across Tồn theo kho, Phiếu kho, Tại công trình & dư and Danh mục vật tư, including responsive presentations, inline warnings and toasts.
- Updated central module, backend function, frontend UI, root-cause and Materials Warehouse domain maps; added focused backend DB integration and frontend source-contract regressions.
- Three new DB-backed Warehouse regressions passed on the local PostgreSQL instance. The focused frontend set passed 6/6, the full frontend suite passed 289/289 and the final production build passed with only the existing Vite chunk-size warning.
- The first full backend run against the long-lived application database exposed five unrelated fixture/data-drift failures (duplicate finance codes and one stale workflow-template edge). A second full run on a fresh temporary PostgreSQL database passed every package; the temporary database was dropped after the run.
- Rebuilt and restarted the local Docker stack from the final source. Backend startup reported 132 migrations present (V132 already applied), PostgreSQL/Redis containers were healthy, `/api/ready` returned HTTP 200 and `/warehouse` returned HTTP 200.
- Status at closure: `TECHNICALLY VERIFIED`; `NOT UX ACCEPTED` because browser/UAT was not requested and did not run.
