# [Closed] Sticky headers for operational tables — approved 2026-09-08

Status when closed: TECHNICALLY VERIFIED; NOT UX ACCEPTED because browser/UAT verification was not requested.

Purpose: keep every current desktop table's column labels visible while its rows scroll, without moving the header before scroll or allowing data rows to show through it.

## Pending acceptance

- Browser/UAT remains pending explicit authorization. Validate an at-rest and vertically-scrolled table at `/statistics/projects`, `/staff` and `/projects`, including a dialog table and a horizontal scroll case.

## Done

- User approved the initial sticky-header review with “OK triển khai”, then approved the local-scrollport correction with “OK triển khai sửa header” after the global page-offset rule made headers drop at rest.
- Replaced the global `thead` sticky rule with `consflow-table-viewport` on all 27 current semantic HTML tables. Their `th` cells stick at `top: 0` inside the local two-axis table viewport; existing dialogs preserve their compact heights.
- Preserved the app-header-aware `consflow-sticky-grid-header` behavior for the verified non-table grids in Project Explorer, Project Detail and Evidence Gallery.
- User approved the opaque-header correction with “OK sửa nền header” after sticky header labels blended with data rows. Sticky header cells now use `--consflow-table-header-background`: slate-50 by default with slate-100/white overrides matching each existing `thead` variant.
- Added static regression coverage for the local viewport inventory, sticky positioning and opaque header-color tokens; updated the frontend mappings. No API, database, permission, workflow, audit or reporting contract changed.

## Verification — 2026-09-08

- Environment: local macOS working tree.
- `cd frontend && node --experimental-strip-types --test tests/stickyTableHeaders.test.ts tests/projectExplorerResponsive.test.ts tests/tableActionColumns.test.ts`: passed, 9 tests.
- `cd frontend && npm run build`: passed TypeScript and Vite `6.4.3`; 1,934 modules transformed, Vite stage 4.87s.
- `git diff --check`: passed with no whitespace errors.
- Skipped: browser/DOM UAT and direct live interaction verification; therefore NOT UX ACCEPTED. The full frontend test suite was not rerun. A prior whole-suite attempt identified an existing esbuild CSS-import issue in `activityTopicRendering.test.ts` through `Dialog.tsx -> dialog.css`.
