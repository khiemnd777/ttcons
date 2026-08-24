# [Closed] Làm sạch danh sách công việc trên Bàn làm việc

Closed on: 2026-08-24

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED

## Will be done

- Replace bordered multi-badge work cards with compact operational rows: one primary title, one `Dự án › Node › Bước` context line, one metadata line and quiet exception-only accents.
- Separate row selection from the direct-open control, use a subtle selected rail/background instead of nested border/ring treatment, and retain the backend-owned `execution_target` action.
- Move work description and next-action guidance into a simplified desktop inspector; remove its repeated project/node/step fact table while preserving the primary CTA and project link.
- Deduplicate repeated role names in the My Work page header without changing authentication or role assignment data.
- Preserve filters, source-backed ordering/pagination, status/readiness, upcoming read-only behavior, blocker visibility, identity deletion warning, RBAC and API contracts.
- Add focused presentation coverage, update central/frontend/Workflow maps, run the full frontend suite, production build, bundle budget and `git diff --check`. Browser/UAT remains skipped unless explicitly authorized.

## Done

- User approved the review package and Mockup as Text for compact rows, a simplified inspector and header-role deduplication.
- Loaded the mandatory central/frontend/Workflow maps plus Frontend Workflow and Impact Analysis instructions. Confirmed `MyWorkPage.tsx` is the list/inspector owner and `WorkItemCard.tsx` has only one runtime caller, while existing tests still protect deleted-assignee and workflow-form status presentation.
- Replaced the noisy card stack with one compact divided list. Each semantic row now has one status/title hierarchy, one workflow path, one operational metadata line, exception-only blocker/upcoming emphasis and a sibling direct-open action; selection uses a quiet blue rail/background without nested interactive controls.
- Simplified the desktop-only `Việc tiếp theo` inspector: it retains status, path, execution workspace, description, next-action guidance, canonical CTA and project link while removing the repeated project/node/step fact table. Header role labels are deduplicated for presentation only.
- Added three focused source-contract tests for semantic row structure, inspector/header behavior, compact skeletons and stale-data retention. Existing deleted-assignee and workflow-form presentation contracts remain green.
- Updated the central keyword map, frontend UI utility map and Workflow Orchestration domain map with the durable compact-list ownership and invariants. No backend route, API payload, database schema, migration, seed, permission or deployment configuration changed for this UI package.
- Current working-tree frontend validation passed: focused 10/10 tests, full 245/245 tests, TypeScript plus production Vite build, 250 KiB gzip bundle budget with a 124.9 KiB entry and 141 chunks, and `git diff --check`.
- Rebuilt and restarted the local Docker stack through the required `down -> build -> run` sequence. The first sandboxed build stopped after Docker Buildx could not write its activity metadata; the approved rerun succeeded. PostgreSQL and both Redis services are healthy, all application containers are up, backend `/api/ready` returns HTTP 200 and frontend `/my-work` returns HTTP 200.

## Skipped / remaining acceptance

- Browser/UAT was not authorized in this turn and remains skipped; the redesigned interface is `NOT UX ACCEPTED` until separately verified in-browser.
- Production deployment was not requested and was not performed. The package requires no manual configuration when released through the normal production pipeline.
