# [Closed] Bàn giao kế toán công trình — 2026-08-27

Closed by user request on 2026-08-27.

Status: `IMPLEMENTED`; `TECHNICALLY VERIFIED` for migrations, focused backend coverage, frontend tests/build and local Docker runtime; `NOT UX ACCEPTED` because browser/UAT was not requested.

## Deferred verification and test-isolation debt

- Run browser/UAT only when explicitly requested against the current build.
- Investigate the pre-existing shared-database integration-suite fixture collisions separately if full-suite DB cleanliness becomes a release criterion. The Accounting Handoff/project-finance focused DB suite is isolated and passing.

## Done

- Review package, impact matrix, comparison table, and Mockup as Text approved by the user.
- Mandatory module mapping and accounting/commercial/materials/quality/admin/reporting/workflow domain maps loaded.
- Existing V83 storage, project-finance routes, provisional-report query, procurement BOQ derivation, warehouse material pricing, partner catalog, and handover finance blockers traced to current source.
- Added and locally applied forward-only V131 with `accounting_handoffs`, three capability permissions, reviewed role grants and immutable official Workflow Rev9 (`...028`, 15 nodes, 21 steps). Only no-run Rev8 projects without legacy finance rows adopt Rev9.
- Added source-backed Procurement/main-client milestone candidates, immutable source snapshots and the audited `handed_off -> acknowledged|returned -> handed_off` lifecycle. Rev9 close readiness requires physical handover and every current candidate acknowledged.
- Replaced the default frontend with the one-queue `Hồ sơ bàn giao kế toán` workspace and explicit MISA boundary. The old five-tab workspace remains selected by backend mode for active Rev8/legacy-record projects.
- Unified partner navigation under `Đối tác`; removed payment terms from its primary form, removed editable Procurement total/budget status, and removed Warehouse standard price from its primary form/table while preserving storage/API compatibility.
- Updated route labels/guards, work-item target presentation, types/statuses, seed parity, focused tests and all affected central/detail/domain maps.
- Technical verification: migration check passed for 131 files; PostgreSQL records V131 checksum `3532c06d...bb578`; table/RBAC/Rev9 shape verified; `go test ./...` passed without DB; focused PostgreSQL tests for Project Finance/Accounting Handoff/work-item target passed; frontend 287/287 tests and production build passed.
- Rebuilt and recreated the local Docker backend from the current source. Startup logs confirm `database_migrations applied=0 skipped=131`, storage ready and port 8080 listening; authenticated application traffic resumed successfully. No `/health` route exists, so process/log/request evidence is used instead of a health-endpoint claim.
- Full backend suite with the shared local DB was attempted but is not a valid green signal: unrelated existing fixtures collide on fixed payment/advance codes and shared workflow state. The failures are recorded as out-of-scope environment/test-isolation debt, not overridden by the focused passing suite.
- Improved the true-empty Accounting Handoff state with source eligibility guidance and permission-aware links to the same project's Procurement and Contracts pages. Filter/search-empty remains neutral, and no manual accounting-dossier action was added.
- Added focused presentation coverage and updated the central, frontend, root-cause and Accounting Finance mappings for the new empty-state contract.
- Removed the exact leaked Audit fixture project `b126fac9-b794-4ca9-998b-f3cad7891d17` plus its Issue/Audit/outbox rows from local PostgreSQL. Project, Issue, Audit and project-version post-checks are all zero; catalog invalidation revision `4063` was published.
