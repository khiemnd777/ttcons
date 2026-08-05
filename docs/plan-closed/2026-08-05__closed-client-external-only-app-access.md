# [Closed] Client External-only App Access

Status: closed

Closed at: 2026-08-05

## Will be done

- Classify the system `client` role as external-only, fail closed for App authentication/session use, and prevent new staff or project-member assignments to that role.
- Preserve historical published templates, runtime snapshots, work items and business-facing client semantics while rejecting external-only required roles in future template publication.
- Remove positive client App access from seed/UAT documentation, keep an inactive negative-login fixture, and update the Roles, Staff and Pipeline Builder surfaces to expose the approved boundary.
- Add migration/backend/frontend regression coverage, update all affected mapping files, and run non-browser technical verification. Browser/UAT remains skipped unless explicitly requested in the same turn.

## Done

- Loaded the mandatory central/backend/frontend/admin/workflow/project-planning maps and checked the proposed boundary against current auth, RBAC, project-member, pipeline-publish, seed and Golden UAT code.
- Queried the local PostgreSQL state read-only: the client account is sole-role with no active session; no open work item or pending approval is client-only, and historical client-required work also includes an internal PM or QA/QC role.
- Produced the cross-domain review package and approved Mockup as Text; received explicit `OK triển khai` for the external-only role, fail-closed migration checks, historical snapshot preservation and non-browser implementation scope.
- Rechecked the clean `main` worktree and complete current migration manifest; V86/V87 belong to Rev4 and V88 belongs to the construction runtime contract, so this plan added V89 without modifying applied migrations.
- Added V89 with `roles.external_only`, fail-closed checks for mixed-role users and external-only open work/pending approvals, permission removal, session revocation and soft inactivation of linked user/staff/project-member App access while retaining `user_roles` and historical workflow/runtime snapshots.
- Closed backend bypasses independently of the UI: auth lookup/session create-refresh-active checks reject external-only users; external-only role identity is read-only; permission, user-role and project-member assignments reject external roles; pipeline publish rejects external role labels in node, step and approval-policy responsibilities. Existing published templates and runtime copies remain unchanged.
- Updated the Roles, Staff and Pipeline Builder UI to label external scope, keep the Client role view-only, filter it from assignment choices and remove `Chủ đầu tư` from new pipeline actor choices while preserving stored historical labels for display.
- Updated destructive seed parity, Golden project membership/validation, README, UAT/runbook wording and all affected central/backend/frontend/admin/workflow/project maps. The repo seed now declares fifteen active App accounts plus inactive negative-login fixture `client@consflow.local`; the existing local database was not destructively reseeded.
- Applied V89 to the local Docker database and verified: one external-only Client role, zero Client permissions, active users, active staff, active project memberships or live sessions, while one historical `user_roles` link remains. Rebuilt/restarted the stack from the latest working tree; PostgreSQL/Redis are healthy, backend/admin are up, startup skipped all 89 already-applied migrations, `/api/health` returned `200` and Client login returned `401`.
- Latest-source technical verification passed on 2026-08-05: all 89 migration manifests checked; V1-V89 applied to clean disposable PostgreSQL; DB-backed `internal/core` and `internal/shared/migrations` passed serially; non-DB `go test ./...` passed; complete Golden seed/fixture replay and validator passed on a disposable database; fixture generator check passed for 8 PDFs; frontend tests passed 40/40; production build passed; bundle budget passed with 105 chunks and 120.4 KiB gzip entry against 250 KiB. `git diff --check` passed.
- Browser/UAT verification was not authorized and was skipped. Status remains `NOT UX ACCEPTED`; the evidence above establishes technical verification only.
