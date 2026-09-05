# [Closed] Dashboard activity, shared date formatting and bundle budget

Closed on 05-09-2026 at the user’s request: “Đóng plan, commit.”

This is a historical record. Closing the plans does not imply browser/UAT acceptance or production deployment. Durable feature and formatting contracts were already promoted into the central/frontend/backend/triage and affected domain maps; this archival action changes no mapping contract.

## [Closed] Shared date formatting and Dashboard activity columns — approved 2026-09-05

Status: IMPLEMENTED / TECHNICALLY VERIFIED in the local scopes below. NOT UX ACCEPTED. User approved the two-point review and desktop/mobile Mockup as Text with “OK triển khai.”

Purpose: consistent `dd-mm-yyyy`, `HH:mm`, `HH:mm dd-mm-yyyy` display and a readable project → activity → actor → time layout that uses the middle of the Dashboard feed.

Done:
- Loaded existing central/frontend/triage and Reporting maps; inspected shared formatting and local date formatters. Reproduced `3/9/2026`, `3/9/26` and `14:26:55 3/9/26` from current code; screenshot and row flex layout confirm the empty center.
- Preserved existing Dashboard/bundle working-tree changes on `8ff0bbb`.
- Added shared `dateText`, `timeText`, `dateTimeText` with padded full dates and minute precision. Explicit-offset timestamps/Date/epoch values use `Asia/Ho_Chi_Minh`; date-only values preserve the calendar day, offset-free datetime values preserve wall time and time-only values never acquire a date. Invalid dates/clocks return safe contextual placeholders.
- Consolidated local display formatters in Timeline, Work Item/signatures, activity/day grouping, Dashboard, Operations, Handover, Work Reports, Notifications, Audit/Event Logs, BOQ, Settings, Project Progress, Warehouse/Procurement, Accounting and QA/QC. Existing dateText consumers inherit the convention. An AST audit identified three raw date cells in Rework and QA/QC; these now use dateText. Activity reminder deadlines use the shared datetime formatter. Input/ISO serializers and business sorting/filtering values are unchanged.
- Dashboard rows/header share four columns from `lg`, with content in the widest center column. Smaller screens stack the same content. Full day headers, minute-only row times, full datetime in tooltip/detail, two-line previews and source detail navigation follow the approved mockup. Filtered empty state offers Reset; unfiltered short windows offer Expand time. Scope, API, pagination and read-only semantics remain intact.
- Updated central/frontend/triage and Reporting/Project Planning mapping. Remaining Intl.DateTimeFormat outside format.ts is only dashboardPeriod's machine calendar-parts calculation, not an absolute display formatter.
- On the final working tree, `npm test` passed all 333 tests (0 failed/skipped, 1.61s); log `/tmp/consflow-date-tests-final.log`. Eight focused date/feed tests passed separately under both `TZ=UTC` and `TZ=America/Los_Angeles`; logs `/tmp/consflow-date-utc.log` and `/tmp/consflow-date-los-angeles.log`.
- One initial test assumed the present-day UTC+7 offset for epoch zero; ICU returned a historical offset instead. Replaced the fixed-offset fixture with contemporary epoch milliseconds and separately verified that numeric zero is accepted. No final test failures remain. Tests cover padded dates, seconds/fractions, offsets, date-only/offset-free values, midnight, leap/invalid dates, placeholders and Vietnam day grouping shared by activity labels.
- Final `npm run build:budget` passed TypeScript, Vite 6.4.3 production build (4.27s) and both raw/gzip limits on local macOS / Node v23.11.0. Artifact timestamp: 2026-09-05 11:14:49 Asia/Ho_Chi_Minh; log `/tmp/consflow-date-build-final.log`. All 127 JavaScript files pass (entry + 126 chunks, including workers). Entry is 276,402 bytes; largest file is WorkItemDetailDialog at 296,566 bytes. No >500 kB warning.
- `git diff --check` passed. Verification targets `8ff0bbb` plus the preserved Dashboard/bundle and current date/layout working-tree changes. No runtime/config/test sources changed after this final build. SHA-256: `format.ts` = `8d6834917645e7367a10cf2da30098eaec7f99c4923001ba7905a8aa58d8199b`; `DashboardActivityPanel.tsx` = `318113de585ff09cb953ceeda2841bfd3d3f559a73bb3dc2a93fe68a29c5fdfc`; `dashboardActivity.ts` = `2585306450bee9798c542d38a243f26d25791f050e1f623d88c42ddb1d313071`.

Will be done:
- No implementation or required non-browser verification remains for the approved scope. Browser/UAT remains skipped unless explicitly requested; actual desktop/mobile appearance and UX are not accepted based on build/tests alone.
- No production deployment was requested or run. Backend/database checks were not rerun because this change touches only frontend presentation; no database/migration state was modified.

Owners: `frontend/src/lib/utils/format.ts`, local display formatters in components/modules, `modules/dashboard/{dashboardActivity.ts,DashboardActivityPanel.tsx,DashboardSummaryPanels.tsx}`, frontend tests and mapping.
Remaining limitation: actual browser layout and constrained-screen behavior remain unverified. Date/time and build checks above passed; UI remains NOT UX ACCEPTED until browser/UAT.

## [Closed] Frontend bundle 500 kB budget — approved 2026-09-05

Status: IMPLEMENTED / TECHNICALLY VERIFIED on the local source and production-build artifacts below. NOT UX ACCEPTED. User approved the three-item review with “OK triển khai.” No visual-interface, API or database changes.

Done:
- Loaded central/frontend/triage and Admin Governance maps; traced Vite config, lazy routes, bundle gate and Staff Excel export. Existing Dashboard working-tree changes on `8ff0bbb` are preserved.
- Reproduced the Vite warning in a read-only build. Existing emitted entry is 557,980 bytes and the full SheetJS chunk is 500,059 bytes. The old gate passes because it checks only 250 KiB gzip, with at least 20 chunks.
- Confirmed Staff export is the only full runtime SheetJS import; parsers already use the same-version mini build. Basic mini XLSX Unicode roundtrip works in Node.
- Grouped React/React DOM/Router/TanStack Query dependencies in `vendor-runtime`; preserved the source route/permission boundaries.
- Staff XLSX export now dynamically loads mini's default export. Existing workbook construction, role filtering, CSV and parser behavior are retained.
- Added the 500,000-byte raw ceiling alongside the existing 250 KiB gzip ceiling, including workers. Regression tests cover highly compressible oversized files, the exact boundary, UTF-8 byte size, existing gzip violations and the production Staff workbook builder/real mini encoder with an independent full-reader roundtrip.
- Updated central/frontend/triage/Admin Governance maps, including the actual CI owner `.github/workflows/validate.yml`.
- Before rebuilding, the new budget correctly rejected both old oversized artifacts. Then all 10 focused tests passed via `node --experimental-strip-types --test tests/bundleBudget.test.ts tests/staffImportTemplate.test.ts tests/staffImportPresentation.test.ts tests/routeLazyLoading.test.ts`.
- `npm run build -- --manifest` passed TypeScript and Vite 6.4.3 production bundling in 4.09s on local macOS / Node v23.11.0, with no >500 kB warning. Artifact timestamp: 2026-09-05 10:15:18 Asia/Ho_Chi_Minh. `npm run check:bundle-budget` passed for all 128 JS files (entry + 127 chunks), including four workers; output saved at `/tmp/consflow-bundle-budget-after.log`.
- Final sizes: entry 275,271 raw / 73,020 gzip bytes; runtime vendor 281,346 raw / 90,300 gzip bytes; XLSX mini 258,519 raw bytes (previous full XLSX 500,059). Largest file is `WorkItemDetailDialog` at 296,929 bytes. Gzip figures use the budget's level 9.
- Manifest assertions verified all static/dynamic targets exist, runtime vendor has no back-imports, the entry retains 39 dynamic imports and the initial static closure contains only entry + vendor. Initial total is 556,617 raw / 163,320 gzip bytes versus 557,980 / 163,564 before; splitting does not establish a material initial-download reduction.
- Imported the actual emitted XLSX chunk in Node using the namespace projection generated in StaffPage; default writer availability and a compressed Vietnamese XLSX roundtrip passed. The ad hoc inspector initially assumed a source-path manifest key and a direct default export; those inspector assumptions were corrected to the emitted key/projection, with no application source change required.
- `git diff --check` passed at that verification point. Tested source was `8ff0bbb` plus the preserved Dashboard and bundle working-tree changes; the newer date/layout plan above records the current frontend build. SHA-256: `vite.config.ts` = `8b1f408f9207dcdb4c5e7799b456c0567b78861469695bb49c7f0ed07f3b8cee`; `staffImport.ts` = `fedaf7105d512d14117a0e883512ea5507b2874ef07ba8e108b5e219dab6b591`; budget script = `3b14d873c36763c6f0e30baeaa071cd719d68eb0dee7c03f457a7abba06f7abc`.

Will be done:
- No implementation or required non-browser verification remains for the approved scope. Browser/UAT and actual browser download remain skipped unless explicitly requested; Node tests substitute only the platform download and do not prove browser acceptance. Production deployment was not requested or run.
- Backend/database checks were not rerun for this frontend-only change; database/migration state was not modified.

Owners: `frontend/vite.config.ts`, `frontend/src/modules/staff/staffImport.ts`, `frontend/scripts/check-bundle-budget.mjs`, focused frontend tests, central/frontend/triage/Admin Governance maps.
Remaining limitation: browser runtime/download and UX acceptance are unverified. Static graph and Node workbook/runtime checks passed; no unresolved failure in the required local checks.

## [Closed] Dashboard project activity — approved 2026-09-05

Status: IMPLEMENTED / TECHNICALLY VERIFIED within the scopes below. NOT UX ACCEPTED. User approved the review package and text mockup with “OK triển khai theo đề xuất này.”

Purpose: replace the Dashboard project portfolio with a read-only, newest-first cross-project activity feed. Preserve management snapshot, cashflow and report links.

Done:
- Loaded module, backend/frontend and Reporting/Project Planning maps; inspected Dashboard, activity persistence, scoped visibility and existing dialogs.
- Confirmed clean working tree before implementation; reviewed existing event source and cursor patterns.
- Added actor/source-scoped `/reports/dashboard/activities`, typed projection, Vietnam-calendar filters and stable snapshot keyset paging. Shared bulk Operations scope with project discussions; no migrations or producers.
- Replaced the portfolio with responsive activity rows, filters, read-only focus-managed dialog and guarded source navigation. Cashflow remains independent; old portfolio API remains compatible.
- Updated central/backend/frontend and Reporting/Project Planning maps.
- On working tree based on `8ff0bbb`, frontend `npm run build` passed (Vite 4.92s; existing >500 kB chunk warning), with 10 focused Dashboard tests passing. This was the Dashboard verification point; the subsequent bundle plan above records the newer frontend build and removal of that warning.
- Backend `make test` passed; DB-dependent tests in that broad run skip without `CONSFLOW_TEST_DATABASE_URL`. Separately ran eight focused integration/unit tests against isolated `consflow_dashboard_test_20260905`, migrated through V136: all passed, including activity scope/paging, Project Updates and existing Dashboard finance/portfolio behavior.
- The 100-project/10,000-event fixture returned 20 rows in 29.50ms; EXPLAIN ANALYZE took 30.55ms and returned 21 rows including lookahead. These figures describe the isolated synthetic fixture only.
- Initial host Go cache/Docker build metadata locations were sandbox-restricted; used workspace Go cache and `/tmp/consflow-dashboard-buildx`. An added alternate-scope test initially violated the schema; corrected its fixture to a permitted `boq_item` target, then all eight focused tests passed.
- Final local verification on 2026-09-05 around 09:30 Asia/Ho_Chi_Minh used `8ff0bbb` plus this working-tree change set. `git diff --check` passed. Docker backend image `sha256:7850204c005a61e88984a3b7a29249aafbd2981f2aac744cf75112bec60b1659` was built from that source (server compile 3.4s) and only backend was recreated at 09:29:52; admin continues using the bind-mounted frontend.
- Live `/api/health` and `/api/ready` returned 200; admin port 5173 returned 200; the unauthenticated activity API returned 401. Startup logged `applied=0 skipped=136`; main database remains V136 with 136 records. Authenticated API behavior was verified through isolated DB-backed HTTP tests, not a live user session.
- Removed the owned temporary test database after validation. No migration files, seed data or existing project/workflow/notification records were changed by this feature.

Will be done:
- Browser/UAT requires an explicit request and remains skipped; no UX acceptance is claimed. Production deployment was not part of this task.

Owners: `frontend/src/modules/dashboard`, reporting route/handler/service/types and a bounded Dashboard activity repository; shared activity scope/presentation helpers only where needed.
Risks: only already-recorded system activity can be shown; cross-project query cost and scope parity require verification. UI remains NOT UX ACCEPTED until browser/UAT.
