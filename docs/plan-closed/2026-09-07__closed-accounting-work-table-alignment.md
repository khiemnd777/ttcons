# [Closed] Nghiệp vụ Kế toán — đồng bộ bảng với Bàn giao kế toán

- Closed on 07-09-2026 at the user's explicit request: `Đóng plan, commit, push`.
- Implementation approval: user confirmed `OK triển khai` after the review package and Mockup as Text.
- Primary purpose: review source dossiers and acknowledge/return them directly from Bàn làm việc.
- Status at closure: IMPLEMENTED and TECHNICALLY VERIFIED for the local frontend build and focused checks; NOT UX ACCEPTED. Closing the plan does not assert browser/UAT acceptance.
- Baseline: `main` at `0d13715`; working tree was clean before this task.
- Durable ownership and invariants were recorded in `module-keyword-map.md`, `frontend-ui-utility-map.md` and Accounting Finance `domain-map.md`. This closed plan is historical evidence only.
- Owners / first searches: `AccountingWorkLane.tsx`, shared `DataTable.tsx` and `AccountingHandoffPage.tsx` as references; `AccountingWorkLane|AccountingRowActions|HandoffStatusCell|buildAccountingWorkPath`.

## Delivered scope

- Replaced the custom grid with shared DataTable: actions first, source, project, dossier group, reference amount and handoff status.
- Desktop and mobile reuse local cell/action helpers; dates match the current status event and return reasons wrap in full.
- Aligned 40px filters, removed the duplicated project label, compacted guidance, and added filtered-empty reset and the filtered-total loaded count.
- Shared DataTable renders loading/error/empty states. Parent-owned server filters, 30-row cursor batches, Load More, source links and canonical acknowledge/return dialogs retain their existing contracts. Local DataTable search/sorting/pagination are not enabled for partial cursor data.
- Updated the central, frontend and Accounting Finance mapping files.
- No backend, database, migration or API files changed. The shared DataTable and action hook remain unchanged.

## Verification evidence — 07-09-2026

- Scope/environment: local working tree on macOS Darwin arm64, Node `v23.11.0`, npm `11.4.2`, Vite `6.4.3`; no Docker rebuild, browser session, live API action or UAT was run.
- Tested source: baseline `0d13715` plus this task's working-tree diff. `AccountingWorkLane.tsx` SHA-256 `913662ce4395280ad518a505b7723d7b81bef3c26cecfca5b4ccf684d7855054`, confirmed unchanged after build/tests. Subsequent plan closure changes documentation only.
- Build: `cd frontend && npm run build` exited 0 in the 14:05 ICT run, with completion observed at `14:05:27`. TypeScript completed and Vite built 1,925 modules; Vite's reported build phase was `4.25s`. This validates the local source, not the running Docker app.
- Focused checks: `cd frontend && node --experimental-strip-types --test tests/myWorkListPresentation.test.ts tests/myWorkQueue.test.ts tests/tableActionColumns.test.ts tests/manualActionColumns.test.ts` exited 0: 28 passed, 0 failed, 0 skipped. Coverage includes existing My Work presentation/action contracts, server cursor/query construction, queue ordering and action-column conventions. These checks do not execute browser interactions.
- Diff review: `git diff --check` reported no whitespace errors; implementation changes were restricted to one frontend owner and the three relevant maps.
- Known verification failures: none. Node emitted its informational experimental type-stripping warning.
- Database/migration state: not inspected or altered; not required for this presentation-only change.
- Skipped checks: backend tests, full frontend suite, Docker rebuild, browser responsive checks, live acknowledge/return flows and UX acceptance. User visual acceptance remains unverified at closure.
