# [Closed] Pipeline node approval role alignment — 2026-09-10

Closed by user request on 2026-09-10. Closing this plan records the implemented and technically verified state; it does not change the `NOT UX ACCEPTED` limitation.

## Status

- TARGET: completed for the approved pipeline-node approval alignment scope.
- IMPLEMENTED: yes.
- TECHNICALLY VERIFIED: yes for the scoped regression, migration, backend build and running health check.
- UX ACCEPTED: no; browser/UAT verification was not requested and was not run.
- BLOCKED: no blocker for the approved fix. The repository-wide suite still has one unrelated pre-existing failing integration test recorded below.

## Done

- Reproduced the contradiction on the current local stack and traced the exact request shown in the screenshot.
- Confirmed the root cause: `CanDecideApprovalRequest` granted generic privileged overrides before the structured policy mutation path enforced the current-stage role.
- Confirmed the same mismatch affected My Work capabilities and approval-request notifications.
- Review package and text mockup approved by the user with `OK triển khai theo review package này`.
- Added `pipelineNodeApprovalDecisionEligibility` as the canonical pipeline-node decision rule used by pending-list capabilities, mutation precheck/recheck, and notification recipient resolution.
- Removed implicit Admin/Director/project-manager decision overrides for pipeline-node approvals while retaining cross-project observation and preserving generic non-pipeline approval behavior.
- Added one-stage `required_roles` compatibility handling for legacy pipeline-node requests and persisted the actor's real matching `role_label`.
- Rechecked request status, stage, requester/prior-action state, project scope and current role after the approval row lock before persisting the action.
- Added regression coverage proving Admin-outside-stage exclusion across inbox, mutation and notification, explicit matching-role admission, and legacy request behavior.
- Added and applied V143. It preserves notification history, resolves invalid action-required projections, and bumps recipient revisions. After the final cleanup pass, `Admin hệ thống` has zero unresolved `workflow.node_approval_requested` notifications.
- Added project creators to notification approval candidate discovery so every actor accepted by the project-scope branch can be evaluated by the same decision rule.
- Updated the central, backend, frontend, root-cause, workflow-orchestration and admin-governance maps.
- Built image `ttcons-backend:latest`, recreated the backend container, and confirmed `GET /api/health` returns HTTP 200.

## Impact Matrix

| Surface | Result | Compatibility boundary |
| --- | --- | --- |
| My Work / pending approvals | `decidable_only=true` contains only requests the actor can actually decide | Response shape unchanged |
| Approval mutation | Current-stage role is required for every pipeline-node action | Generic non-pipeline approvals keep existing privileged overrides |
| Ordered/all/quorum policies | Persists the actor's real matching stage role | No synthetic Admin/Director/PM role label |
| Notifications | Action-required audience equals current decision audience | Historical invalid projections are resolved, not deleted |
| Frontend | Existing server-driven capabilities remain authoritative | No frontend source change |
| Database | Forward-only repair migration, no schema change | Applied migration files remain immutable |

## Verification

- Source base: Git HEAD `cc21ab77c20ce1d32dadca33f6480d12371073cd` plus the implementation working tree before this archive/commit.
- Focused DB integration: seven approval/My Work/notification tests passed on the local PostgreSQL database with the old backend dispatcher stopped.
- Clean isolated DB: `go test -p 1 ./... -skip '^TestAutoAdvancedWorkflowFormKeepsConfirmationAndPDFCapabilities$' -count=1` passed every included package/test; the temporary database was then dropped.
- Known unrelated failure: an unskipped clean-suite run fails only `TestAutoAdvancedWorkflowFormKeepsConfirmationAndPDFCapabilities` because its fixture violates the existing `pipeline_run_steps_surface_kind_check`. The implementation does not touch that constraint or test scenario.
- Existing localhost DB limitation: an earlier broad run against the long-lived application DB also exposed unrelated duplicate fixture codes and stale seed/template expectations, so that database is not evidence of a clean full-suite result.
- Migration state: V143 is recorded in `schema_migrations`; backend startup reports `applied=0 skipped=143` after rebuild.
- Runtime: backend container is running on `127.0.0.1:8080`; `/api/health` returned `200` with status `ok`.
- Frontend source/build: unchanged and not rebuilt.
- Browser/UAT: skipped by repository policy because it was not explicitly requested; status remains `NOT UX ACCEPTED`.

## Remaining Work

- None in the approved implementation scope.
- Browser/UAT may be run only if separately authorized.
