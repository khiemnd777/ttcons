# [Closed] Project Progress Board Bounded Loading

## Will be done

- After one successful deployment cycle and a separate consumer audit, prepare a follow-up review before removing or capping the retained legacy full-snapshot endpoint.
- Run browser/UAT only when explicitly requested; until then the changed UI remains `NOT UX ACCEPTED` despite technical verification.

## Done

- Diagnosed the current full-snapshot/local-filter/full-render path and confirmed there is no per-stage cursor, board response cache, or list virtualization; React Query supplies only a 30-second in-memory stale window and Redis is used for SSE rather than board data.
- Measured the current local dataset at 58 active projects, about 37 KB of selected-field JSON, with 41 records concentrated in `not_started`; the representative aggregate query was fast on this small dataset and therefore did not rule out scale risk.
- Produced the evidence-backed review package, impact analysis, 10-phase rollout/rollback plan, acceptance targets, and responsive Mockup as Text. The user approved the plan and explicitly authorized implementation through one persistent goal.
- Created the persistent implementation goal and reloaded the mandatory central, backend, frontend, root-cause, and project-planning mappings plus the backend/frontend/impact/diagnosis workflow skills.
- Added rolling-compatible `GET /projects/progress-board/summary` and `GET /projects/progress-board/stages/:stageKey` routes while retaining the old full endpoint and unchanged invalidation-only stream. Both new routes keep `project.view`, `ProjectAccessFilter`, non-cancelled scope and the standard response envelope.
- Added a shared SQL stage classifier, server search/PM UUID/project-status filters, default 25/max 50 lookahead paging and an opaque descending `created_at,id` cursor bound to the stage plus normalized filter hash. Independent issue/approval/blocked/overdue aggregates run only for page rows.
- Switched `/progress` from one full snapshot/local filtering to actor-scoped summary and per-stage infinite queries. Search debounces 300 ms; mobile loads only its selected stage; desktop lazily enables visible/near 286px columns; cursor pages deduplicate by project ID and expose `Tải thêm 25`.
- Added `@tanstack/react-virtual` and dynamic-height card virtualization with explicit deferred/loading/stale/error/retry/empty/end states. The project-progress lazy chunk is 12.5 KiB gzip and remains under the 250 KiB per-chunk budget.
- Kept SSE invalidation debounced at 500 ms. Active first pages auto-refresh; once any stage is deeper than one page the UI preserves scroll and shows `Có dữ liệu mới`, whose refresh resets every stage cursor. Disconnected summary and first pages poll every 60 seconds.
- Added unit, HTTP/DB integration and opt-in skew performance coverage. On isolated PostgreSQL, rolling legacy/summary/stage routes, visibility, cancelled exclusion, filter-bound cursor and page de-duplication pass. At 1k/5k/10k projects with 80% in one stage, 10k summary p95 was 14.123709 ms/364 bytes and stage page p95 was 14.745458 ms/14,739 bytes, so no index migration or Redis response cache was activated.
- Updated central/backend/frontend/root-cause/project-planning mappings for the new owners, routes, contract, frontend loading lifecycle and performance triage path. No migration map changed because no schema/index/revision/cache migration was added.
- Technical verification on the current implementation includes 104/104 frontend tests, production build and bundle budget; full non-DB `go test ./...`; focused DB/API integration; the opt-in 10k performance gate; and the full DB-backed core suite on a fresh disposable database. The first full DB run against the long-lived dev database failed only on pre-existing fixture/Golden collisions; the fresh database passed in 28.403 seconds and was then removed.
- `npm audit --omit=dev` remains non-clean for the pre-existing locked `react-router-dom`/`react-router` 7.17.0 advisories; the progress dependency change did not modify those lock entries. No out-of-scope router upgrade was attempted.
