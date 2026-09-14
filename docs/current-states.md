# Current States

## My Work Attention List Presentation

Status: TECHNICALLY VERIFIED; NOT UX ACCEPTED.

Will be done:

- No remaining implementation work in the approved scope.
- Browser/UAT may be run later only with explicit authorization; until then the UI remains `NOT UX ACCEPTED`.

Done:

- Loaded the mandatory module, frontend, Project Planning, and Workflow Orchestration maps.
- Confirmed the current owner is `frontend/src/modules/my-work/MyWorkPage.tsx` and the existing desktop presentation is `xl:grid-cols-2` with per-item cards.
- Captured user approval for the one-column divided-list mockup.
- Replaced the attention card grid with one semantic divided list across breakpoints.
- Added list-shaped loading skeletons and an explicit retry action while preserving existing empty, unread, reply, attachment, reaction, mark-read, and navigation behavior.
- Added the focused presentation contract and updated the central/frontend mapping files.
- After the final responsive adjustment, `node --experimental-strip-types --test tests/myWorkAttentionLoading.test.ts tests/myWorkListPresentation.test.ts` passed 19/19 tests on the current source on 2026-09-10.
- On that same final source, `npm run build` completed successfully on 2026-09-10 (`tsc --noEmit` plus Vite production build, 1,947 modules transformed in 4.67s).
- No backend, API, database, migration, permission, or activity-state contract changed. Browser and user acceptance checks were skipped.

## Notification Archive Burst Stability

Status: TECHNICALLY VERIFIED FOR THE NOTIFICATION SCOPE; REPOSITORY-WIDE FRONTEND TEST SUITE HAS ONE UNRELATED EXISTING FAILURE; NOT UX ACCEPTED.

Will be done:

- No remaining implementation work in the approved notification scope.
- Browser/UAT may be run later only with explicit authorization; until then the UI remains `NOT UX ACCEPTED`.
- The unrelated `project evidence list separates file, stage and work item columns` contract failure remains outside this notification plan.

Done:

- Loaded the mandatory central/frontend/backend/root-cause/Admin Governance mappings and the diagnosis/frontend workflow skills.
- Confirmed from the supplied UI and local backend logs that fourteen successful archive mutations between 12:03:13 and 12:03:18 caused twenty-seven successful list refetches before a list refetch received HTTP 429.
- Confirmed the duplicate invalidation owners are the local notification mutation handlers and the app-level `notification_changed` SSE handler.
- Confirmed the current full-screen `isError` branches hide still-cached rows after a background refetch failure.
- Received explicit user approval to implement the reviewed frontend-only repair without changing backend rate-limit, API, schema, permission, or notification source behavior.
- Added `notificationCache.ts` as the single owner for successful read/archive/read-all actor-cache transitions, stale-data presentation state, and trailing notification-event coalescing.
- Updated the full notification page and bell panel to retain cached rows on background refresh failure, expose inline retry, and reserve the blocking error for an initial no-data failure.
- Updated the actor realtime provider to coalesce a notification burst for 750ms, fetch the authoritative summary revision once, and refresh notification rows only when the actor revision differs.
- Added focused cache/presentation regressions and updated the central, frontend, root-cause, and Admin Governance maps. The backend map required no update because no backend route, handler, service, repository, API contract, rate limit or data behavior changed.
- On the current notification source, `node --experimental-strip-types --test tests/notificationCache.test.ts tests/notificationPresentation.test.ts` passed 6/6 tests on 2026-09-10.
- On the current source, `npm run build` completed successfully on 2026-09-10 (`tsc --noEmit` plus Vite production build, 1,948 modules transformed in 3.96s).
- The repository-wide `npm test` run passed 462/463 tests; its sole failure is `tests/projectEvidenceColumns.test.ts`, whose source and target `EvidenceGallery.tsx` have no working-tree diff and are unrelated to notifications.
- `git diff --check` passed. Browser/UAT was skipped; no backend, database, migration, RBAC, recipient, source-action or resolution behavior changed.
