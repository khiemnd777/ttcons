# [Closed] Global Project Scope For Reports Catalog

Closed on 2026-08-06 by explicit user request.

## Final status

- No mandatory implementation or non-browser verification work remains for this redesign.
- Browser/UAT was not authorized in the final implementation turn; the redesigned layout remained `NOT UX ACCEPTED` at closure pending user review of the local build.

## Done

- Loaded the mandatory central/frontend/reporting maps and checked the current Reports workspace source, shared controls, routing and existing regression tests.
- Confirmed that navigation already preserves URL `project_id`, but the draft/applied state and filter UI were owned by `OperationalReportPanel`, which visually misrepresented the scope as report-local.
- Produced the impact matrix, review package and approved Mockup as Text; received explicit `OK triển khai` for the global project scope and active-report CSV behavior.
- Moved the `Công trình` scope into one workspace-level section above both the report catalog and active content; `ReportsPage` now owns draft/applied URL state, Apply/Reset and the active-report CSV action.
- Made `OperationalReportPanel` accept only the applied project ID and own only its source-backed query plus responsive table/card states. List and CSV URLs now derive from the same applied `project_id`, including catalog navigation and the base-route redirect.
- Centralized active-report basis and CSV file metadata in `reportRegistry`, kept the legacy combined `ReportFilterActions` API intact for hidden finance panels, and added explicit loading/error/fallback states for project options.
- Added focused global-scope/CSV metadata regression coverage and updated the central, frontend and reporting-domain maps for the new ownership contract.
- User Browser acceptance on 2026-08-06 confirmed the global placement and cross-catalog behavior worked well, then exposed that the native select eagerly presented the entire actor-scoped project list and could not disambiguate duplicate names.
- Confirmed from source that `ReportsPage` eagerly called `/reports/options`, `ReportProjectField` rendered every returned row as a native option, and `ReportProjectOptions` had no search/limit/cursor. Produced and received explicit approval for the compact async project-picker review package, including code/name search, 20-row paging, direct-URL selection resolution and Reports-only scope.
- Removed the `Phạm vi báo cáo` heading and explanatory sentence while retaining one compact workspace-level `Công trình` filter above the catalog and active report.
- Added `ReportProjectPicker`: 300ms server search by code/name, 20-row opaque-cursor pages, explicit `Tải thêm`, duplicate-name disambiguation by code, keyboard/listbox semantics, loading/empty/error states and a fail-closed inaccessible-URL state. The existing draft/Apply/Reset/applied URL and list/CSV parity remain unchanged.
- Changed `/reports/options` to the bounded `{projects:{items,has_more,next_cursor,selected}}` contract with strict search/cursor/limit/selected-ID validation, deterministic keyset paging and the existing non-cancelled actor `ProjectAccessFilter` on both list and selected lookup. Added V90 PostgreSQL trigram search and keyset-order indexes; no report metric, CSV or source query changed.
- Added focused normalization/cursor, actor-scope/search/paging/direct-selection and frontend source-contract coverage; updated central/frontend/backend/root-cause/reporting maps.
- Technically verified the source with the full DB-backed Go suite on disposable PostgreSQL database `consflow_report_options_test_20260806_codex`, all 90 migrations including V90, migration manifest check, 46/46 frontend tests, production build, 250 KiB bundle budget and diff hygiene. An initial overlapping DB-suite invocation produced fixture/lock conflicts; a single clean rerun passed.
- Rebuilt and restarted the local Docker stack through the required `down -> build -> run` pipeline. `admin` and `backend` were up on 5173/8080, PostgreSQL and Redis were healthy, backend health and the frontend origin returned HTTP 200, and local PostgreSQL recorded V90 plus both report-project search/order indexes.
- Received explicit approval for the visual redesign: integrate the filter into the Reports page header, use a two-column wide-desktop layout with responsive stacking, keep Apply primary, make Reset quiet and preserve every filter/API/CSV invariant.
- Removed the standalone filter card and integrated the picker/actions into the purpose-first page header. Wide desktop uses a bounded right column, narrower screens stack the filter under the title, the selected trigger separates project code from name, and Reset is now a ghost action; async loading/search, draft/applied URL state, report queries and CSV behavior are unchanged.
- Updated focused responsive source-contract coverage and the central/frontend/reporting maps for the new header-integrated ownership and action hierarchy.
- Technically verified the final frontend working tree with 46/46 tests, the production TypeScript/Vite build, the bundle budget (121.4 KiB gzip entry; 106 JavaScript chunks under 250 KiB) and `git diff --check`. Backend/API/database checks were not rerun for the final visual pass because it changed presentation only.
