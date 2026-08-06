# [Closed] Reports Mobile Responsive Cards

## Will be done

- Keep the existing desktop operational-report tables while rendering compact record cards below the `md` breakpoint for project, procurement, warehouse and staff reports.
- Preserve the same client-side search, sortable fields, pagination, applied project filter, permission-aware source links and CSV query scope across mobile and desktop presentation.
- Remove duplicated mobile report headings, keep one compact purpose-first report context and make `Áp dụng` the primary mobile filter action while Reset and CSV remain secondary.
- Add focused responsive regression coverage, update central/frontend/reporting mapping, and run non-browser frontend verification. Browser/UAT remains skipped because it was not requested for this implementation turn.

## Done

- Confirmed from the supplied 430×932 Edge screenshot and current source that the shared table is compressed into five or more columns on mobile because it has no mobile row renderer or column minimum-width contract.
- Confirmed that report selection, applied URL-backed project filtering, search, sorting, pagination, permission-aware source links and CSV export are already functional and must remain unchanged.
- Produced the evidence-backed review package and Mockup as Text, then received explicit `OK triển khai` for mobile cards below 768px, compact report context and unchanged report/export semantics.
- Added the backward-compatible `DataTable.renderMobileCard` path: report cards and compact sorting render below `md`, the same visible row slice remains a dense table from `md` upward, and loading/search-empty/pagination states stay shared.
- Added purpose-specific project, procurement, warehouse and staff cards with permission-aware source links; the unreachable legacy accounting branch also receives a safe responsive card without restoring it to the ordinary report catalog.
- Compacted the mobile report heading, removed the duplicated active-report panel heading below `lg`, made Apply the primary mobile filter action and exposed CSV export progress while retaining desktop layout and query behavior.
- Added focused responsive source-contract coverage and updated the central, frontend-detail and reporting-domain maps for the new presentation contract. The focused 2-test suite and TypeScript no-emit check pass.
- Technically verified the latest frontend working tree with all 45 tests, the production TypeScript/Vite build, the bundle budget (121.3 KiB gzip entry; 105 JavaScript chunks under the 250 KiB ceiling) and `git diff --check`. No backend, database or migration change is involved. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
