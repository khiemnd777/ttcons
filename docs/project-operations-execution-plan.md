# Project Operations Execution Plan

Status: active execution plan as of 2026-06-12. Batches 1, 2, and 3 have been implemented in the current working tree.

This document turns `docs/current-states.md` into implementation batches. It is the handoff surface for long-context runs and subagent orchestration.

## Execution Rules

- Keep the human review gate. A batch may run automatically only after its review package, and UI mockup when applicable, has been explicitly confirmed.
- Batch 1 is implemented.
- Batch 2 is implemented.
- Batch 3 is implemented.
- Do not edit applied migrations. Add new `V<N>__description.sql` files only.
- Use backend contract work before frontend composition. UI should consume server-scoped operations/work-item payloads rather than filtering broad module data.
- Use Browser, Playwright, or browser-use only when explicitly requested in that turn.
- End each batch with test/build status, residual risks, and the next review package.

## Subagent Pattern

- Main agent owns integration, conflict resolution, and final validation.
- `consflow-backend-worker`: schema, API, service/repository, seed, backend tests.
- `consflow-frontend-worker`: route/module/shared UI work after backend response shapes are stable.
- `consflow-db-reviewer`: read-only review of migrations, constraints, indexes, seed consistency, and persistence semantics.
- `consflow-reviewer`: read-only review of RBAC, workflow invariants, contract drift, and missing tests.
- `consflow-explorer`: read-only path mapping when a future batch touches unclear ownership.

## Batch 1: Work Item Foundation

Status: implemented.

Confirmed scope:

- Add first-class `work_items`, `work_item_assignments`, `work_item_activities`, and `evidence_records`.
- Generate runtime work items from pipeline run steps.
- Make `StartPipeline` idempotent for active runs.
- Add server-scoped project/work-item read APIs.
- Add initial work-item action APIs for start, complete, block, and resolve blocker when feasible.
- Fix BOQ-to-procurement generation to use approved BOQ items only.
- Add focused backend tests.

Exit criteria:

- New migration added without modifying prior migrations.
- Backend tests cover generation/idempotency and approved-only procurement.
- New APIs are server-scoped and response envelopes match the existing client.
- DB reviewer and correctness reviewer findings are addressed or documented.

## Batch 2: Operations API Hardening

Status: implemented.

Goal: make `/projects/:id/operations` dashboard-ready and safe for PM/director/project-member views.

Expected work:

- Add project summary, health counts, workflow timeline, active work, blockers, approvals, latest evidence, activity summary, and audit summary.
- Enforce project membership, role, department, assignee, watcher, client visibility, and permissions in repository/service queries.
- Add response types in backend and frontend domain contracts.
- Add tests for PM, director, department member, assignee, and no-scope user views.

Review gate before implementation:

- Confirm exact operations response shape.
- Confirm role visibility matrix and no-permission behavior.

## Batch 3: Work Item Actions, Evidence, And Activity

Status: implemented.

Goal: make work execution production-grade.

Expected work:

- Complete transition coverage: request review, request approval, approve, reject, request changes, upload evidence, add scoped activity.
- Enforce required evidence before review request, approval request, approve, and completion.
- Generate activity/system events from transitions.
- Preserve actor, comment, payload, and timestamps in audit/activity.
- Keep binary file storage out of scope unless explicitly approved; start with metadata/path records.
- Added V6 migration for expanded activity types.
- Added DB-backed coverage for evidence-required workflow gates, approval decision scope, self-approval blocking, and generic approval/work-item sync.

Review gate before implementation:

- Confirm status transition matrix per work type.
- Confirm evidence metadata contract and visibility rules.

## Batch 4: RBAC And Legacy Module Scope

Goal: make legacy module endpoints match the Project Operations visibility model.

Expected work:

- Add permission/project-scope checks to approvals, BOQ, procurement, warehouse, operations, acceptance, rework, handover, accounting, reports, staff/RBAC as appropriate.
- Update seed role permissions for operational roles.
- Add backend denial tests for seeded demo users.
- Keep user-visible lists filtered server-side.

Review gate before implementation:

- Confirm module permission matrix.
- Confirm whether director/admin remain global and whether PM is project-scoped or global.

## Batch 5: Project Operations Dashboard UI

Goal: replace project detail as the main project control room.

Expected work:

- Use `GET /projects/:id/operations`.
- Build project header, health strip, workflow timeline, active work, command center, and scoped department surfaces.
- Keep module links secondary.
- Reuse shared work-item detail, action footer, activity, and evidence components.
- Add loading, empty, no-permission, blocked, overdue, waiting approval, completed, and API error states.

Review gate before implementation:

- Provide Mockup as Text for desktop and mobile.
- Confirm which department surfaces are included in the first UI pass.

## Batch 6: `/my-work` Inbox

Goal: add a personal cross-project inbox derived from the same `work_items`.

Expected work:

- Add `/my-work` route.
- Use `GET /me/work-items`.
- Render mine, department, pending approval, and watching queues.
- Add `Open project dashboard` as the primary context link.
- Keep authenticated default route role-aware only after product confirmation.

Review gate before implementation:

- Provide Mockup as Text for queue/detail/activity layout.
- Confirm default route behavior by role.

## Batch 7: Domain Transition Fixes

Goal: close production correctness gaps outside the work-item foundation.

Expected work:

- Tighten procurement, stock, acceptance/rework, handover, payables, receivables, and payment transitions.
- Remove generic status updates where actor/comment/audit semantics are required.
- Add domain tests for stock balance, acceptance failure history, handover blockers, and accounting approval/payment context.

Review gate before implementation:

- Confirm domain transition priorities and acceptable compatibility changes.

## Batch 8: Production QA And Handoff

Goal: lock regressions and hand off a production-ready operating model.

Expected work:

- Complete backend regression coverage for workflow, RBAC, evidence, activities, and domain transitions.
- Run backend tests and frontend build.
- Run Docker/migration checks when approved and environment is available.
- Update `docs/current-states.md` to reflect implemented state.
- Move closed implementation notes to `docs/plan-closed` when the active plan is complete.
