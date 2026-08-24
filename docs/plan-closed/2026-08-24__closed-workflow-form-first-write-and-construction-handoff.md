# [Closed] Đồng bộ Bắt đầu với Workflow Form và bàn giao thi công

Closed on: 2026-08-24

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED

## Will be done

- Treat the first successful draft save or submit of a current-node form-backed Work Item as the explicit start of that work: transition `ready|needs_changes -> in_progress`, set `started_at`, synchronize the run step and write exactly one `started` activity/audit atomically with the form write.
- Keep future/out-of-sequence work behind the existing explicit early-start confirmation, reason and snapshot-token contract. Do not auto-start on dialog open, read-only access, validation failure, permission failure or a failed form transaction.
- Make first-write actions transparent in the UI (`Bắt đầu & lưu nháp`, `Bắt đầu & chốt hồ sơ`), then remove the redundant generic `Bắt đầu` after a form is submitted.
- For `construction.activate_handoff`, present the actual next business action after form submission: open the existing Planning handoff review/activation dialog; after activation, keep the separate backend-gated `Kiểm tra điều kiện & chốt bước` action rather than auto-completing the Work Item.
- Preserve routes, request/response payloads, permissions, evidence/signature/approval/completion gates, published/runtime definitions and historical records. Add no migration and perform no backfill.
- Add focused backend/frontend/database-backed regression coverage, update central/backend/frontend/root-cause and Workflow/Planning/Construction maps, then run current-tree backend/frontend validation. Browser/UAT remains skipped unless separately authorized.

## Done

- User approved the review package, impact matrix and Mockup as Text with `OK triển khai theo review package này`, including shared first-write behavior and an explicit post-handoff completion step.
- Loaded the mandatory central/backend/frontend/root-cause, Workflow Orchestration and Project Planning maps plus Backend/Frontend/Impact instructions; confirmed `SaveWorkflowForm`, `WorkItemAction`, `ApplyWorkItemTransition`, `WorkflowFormPanel`, `WorkItemDetailDialog` and `ConstructionHandoffPanel` as the owners.
- Confirmed the root cause: a submitted Workflow Form and its Work Item have separate lifecycle mutations; the form actions disappear after submit while the still-`ready` Work Item exposes `start` as its first action.
- Added an atomic first-write bridge in `SaveWorkflowForm`: after existing validation, scope, permission and active-node checks, the transaction locks the linked Work Item, transitions `ready|needs_changes -> in_progress`, sets `started_at`, synchronizes the run step and records one `started` activity/audit tagged `workflow_form_first_write`. Later writes do not duplicate the start, and any form write failure rolls the Work Item change back. Successful starts publish the ordinary Work Item invalidation event.
- Made the combined mutation explicit on desktop/mobile/internal form actions with `Bắt đầu & lưu nháp`, `Bắt đầu & chốt hồ sơ` and the auto-advance equivalent. Submitted form presentation removes legacy `Bắt đầu`, including existing submitted/ready records, without changing API payloads or backfilling state.
- Added the `construction.activate_handoff` continuation: while the submitted form's source is blocked, the action rail shows `Mở bàn giao thi công` and uses a project-scoped one-time navigation intent to open the existing Planning activation dialog. Activation refreshes Work Item source readiness; after activation the retained backend `complete` action is presented as `Kiểm tra điều kiện & chốt bước`, so no Work Item is auto-completed.
- Added backend transaction/idempotency/rollback coverage plus frontend action/navigation/continuation coverage. Updated the central backend/frontend/root-cause mappings and Workflow Orchestration, Project Planning and Construction Operations domain invariants. No schema, migration, route, request/response contract, published definition or historical record changed.
- Current working-tree technical validation passed: full frontend 254/254 tests, TypeScript lint, production build, 250 KiB gzip/141-chunk budget, full backend `go test ./...`, focused PostgreSQL-backed first-write transaction regression, Go formatting and `git diff --check`.

## Skipped / remaining acceptance

- Browser/UAT was not authorized in this turn, so the interface remains `NOT UX ACCEPTED` until separately verified in-browser.
