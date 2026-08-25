# [Closed] Đồng bộ vòng đời Work Item và Run Step khi bàn giao Thi công

Closed on: 2026-08-25

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-24)

## Will be done

- Add a fail-closed transactional postcondition for terminal completion consistency between a linked Work Item and Pipeline Run Step on every production completion path.
- Keep Planning-to-Construction handoff activation separate from workflow Node advancement; prove the full transition using the real service lifecycle rather than direct SQL state simulation.
- Correct the Developer Sample Data blueprint so a completed Work Item never leaves its linked Run Step submitted, and bump the blueprint version.
- Add a custom migration that repairs only the exact invalid generated-data pattern on projects with `is_sample=true`; do not backfill real projects and do not auto-advance any Node.
- Add focused regression and integration coverage, update the central/backend/root-cause and domain maps, then run database-backed backend verification and migration checks. No UI change is included; browser/UAT remains skipped.

## Done

- User approved this exact implementation scope with `OK triển khai theo phạm vi mới`.
- Confirmed the reported project is generated sample data and its planning execution-package Work Item is completed while the linked Run Step remains submitted.
- Confirmed the production service paths already attempt to update both records transactionally, but they lack a shared fail-closed postcondition and the sample blueprint explicitly generated the mismatched statuses.
- Confirmed the handoff activation contract does not advance the current Node; normal Node advancement remains a separate explicit workflow action.
- Added the shared transactional terminal-pair postcondition to Work Item transition/decision completion and legacy Run Step submit. A missing referenced Run Step or one-sided `completed` state now fails closed before audit/commit.
- Corrected Developer Sample Data blueprint v4 so simulated completed Run Steps and Work Items are both `completed`, and added a project-wide terminal-pair validator before the generation transaction commits.
- Added and applied V124. Its SQL targets only pre-v4 `is_sample=true` rows with Work Item `completed` and Run Step `submitted`; it does not update real projects, Node status or `pipeline_runs.current_node_id`.
- Replaced the handoff integration test's direct SQL Node simulation with the real `ActivateConstructionHandoff -> AdvanceRunNode -> ProjectConstructionWorkspace` service lifecycle and proved generated construction work becomes ready/report-eligible only after explicit advance.
- Added focused completion-atomicity and sample-generator regressions. The initial generator regression failed with `run_step=submitted work_item=completed`; it passes after the source fix.
- Updated the central/backend/root-cause, Workflow Orchestration, Construction Operations and Admin Governance maps. No frontend map changed because no UI/API response contract was modified.
- On working-tree state at HEAD `56f6629` plus these changes: full non-DB backend suite passed; focused PostgreSQL-backed lifecycle/sample tests passed; migration check validated all 124 checksums; `git diff --check` passed; backend Docker image built, backend was recreated and `/api/health` returned `ok`.
- Local V124 data verification shows zero terminal mismatches for sample projects. Project `8494484c-4d94-47a0-88ab-29c443506137` now has `planning.execution_package` Run Step/Work Item `completed/completed`; its run intentionally remains at `construction_planning` until the normal explicit advance action is used.

## Skipped / remaining acceptance

- The full PostgreSQL-backed `internal/core` suite still has five unrelated pre-existing failures: four accounting fixture code collisions and one legacy workflow-template edge expectation. All four focused tests for this change pass independently.
- Browser/UAT was not authorized in the implementation turn and remains `NOT UX ACCEPTED`.
