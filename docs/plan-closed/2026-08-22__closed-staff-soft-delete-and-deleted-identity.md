# [Closed] Soft-delete nhân sự và giữ danh tính tham chiếu

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (behavior and Mockup as Text approved and plan closed by user on 2026-08-22)

## Approved scope and invariants

- Allow an authorized actor to select one or more Staff rows and soft-delete them even when they remain referenced by projects, project memberships, work items, reports, workflow records, audit, finance or other business records.
- Preserve every business foreign key and immutable historical artifact. Never hard-delete `staff`/`users`, null a project/work assignment merely to satisfy deletion, rewrite a stored person name, or regenerate an immutable snapshot with a different attribution.
- Present a linked deleted identity as `<original name> (tài khoản đã xoá)` on dynamic FK-backed user-facing references. Live ownership surfaces additionally show `Cần phân công lại`; active assignee/member/recipient selectors exclude deleted identities.
- Keep immutable revision/PDF/signed-document snapshots exactly as issued. When an audit or history row retains a live actor ID, the UI may add a current deleted-account badge beside the stored snapshot name without changing the snapshot.
- Reserve the deleted login email. Reuse, anonymization and account restoration are separate future governance flows.
- Use a dedicated `staff.delete` capability, server-side authorization and fail-closed audit. The current generic Staff update path must no longer accept `status=deleted`.
- Deletion is atomic per selected batch. Ordinary references are impact warnings, not blockers. Self-deletion and deletion that would leave no active Admin remain blockers.

## Will be done

### 1. Freeze the deletion and identity-reference contracts

- Inventory every user-visible dynamic FK-backed `staff`/`users` reference and classify it as: active selector, live ownership, historical dynamic reference, immutable snapshot, or non-user-facing internal relation.
- Define one backward-compatible API convention for deleted identities: keep existing name fields and add a companion boolean such as `pm_account_deleted`, `assignee_account_deleted` or `author_account_deleted` rather than mutating names in persistence or API services.
- Define a shared frontend identity presentation component that renders the exact approved suffix and an optional `Cần phân công lại` state.
- Record the out-of-scope boundaries: hard delete, automatic reassignment, retroactive immutable PDF/revision changes, email reuse and account restoration.

### 2. Add migrations V114-V115 and production RBAC parity

- Add an ordered custom migration `V114__staff_soft_delete_governance.sql`; do not edit V113 or another applied migration.
- Keep impact-query indexes in the follow-up `V115__staff_deletion_impact_indexes.sql` so V114 remains byte-identical after application.
- Add `users.deleted_at`, `users.deleted_by`, `staff.deleted_at` and `staff.deleted_by`, with safe foreign keys/indexes and constraints that prevent a deleted user from remaining active.
- Backfill any pre-existing `staff.status='deleted'` rows deterministically; mark a linked user deleted only when it has no remaining non-deleted Staff profile.
- Add `staff.delete` to the permission catalog, grant it only to Admin and Developer, enforce the reserved grant boundary in the same layers used by V110-V113, and update seed/bootstrap catalog parity.
- Keep login email uniqueness unchanged so a deleted identity cannot be silently replaced by a new account.

### 3. Implement authoritative preview and execute APIs

- Register protected `POST /staff/deletions/preview` and `POST /staff/deletions/execute` routes under the existing auth/session middleware.
- Require `staff.delete` and `staff.account_security.manage`; validate UUIDs, deduplicate selections, cap a batch at 100 and never trust frontend impact calculations.
- Preview returns per-Staff identity, deletability, blockers and impact counts/details for active PM projects, project memberships, nonterminal work assignments and other indexed user-visible references. Ordinary references remain warnings.
- Execute re-locks selected Staff/User rows in deterministic order, re-evaluates blockers, binds the request to the authoritative preview and uses a UUID idempotency key. Any invalid/blocking row aborts the whole batch.
- For each valid target, set Staff deleted metadata; if no linked Staff profile remains active/non-deleted, set User inactive/deleted metadata, revoke all sessions, invalidate and clear recovery state, prevent first-login continuation, deactivate project memberships and remove current authorization assignments after capturing them in audit metadata.
- Preserve `projects.pm_staff_id`, work-item assignments and all creator/reviewer/approver/recipient/history foreign keys.
- Write fail-closed per-identity and batch-summary audits without password, recovery address, token or other credential material.
- Remove `deleted` from the allowed generic `PUT /staff/:id` status transitions so deletion cannot bypass the dedicated permission and audit flow.

### 4. Add the Staff-list selection and deletion workflow

- Extend `DataTable` with an optional controlled selection mode without changing consumers that do not enable it. The Staff screen owns selection state; role-dialog selection remains independent.
- Show per-row and current-page select-all checkboxes. Selection is limited to the visible page and is cleared when search, filter, sorting or page changes so hidden rows cannot be deleted accidentally.
- Show the approved selection toolbar: selected count, clear action and permission-gated delete action.
- Add `StaffDeleteDialog` with authoritative preview loading, impact groups, blockers, confirmation checkbox, all-or-nothing explanation, execute progress, retry-safe error state and dismissible success/warning/error toasts.
- On small screens, place selection in each Staff card and show a sticky bottom selection bar; the confirmation dialog becomes full-width while retaining the same information hierarchy.
- On success, invalidate Staff, role-assignment, project/member, work-item, report/statistics and other affected cached queries identified by the inventory.

### 5. Standardize deleted-identity presentation

- Add a shared `IdentityReferenceLabel` component and a shared TypeScript deleted-reference contract. Render the original name plus the exact `(tài khoản đã xoá)` suffix; never append the suffix to persisted `full_name` or email.
- Add accessible text/tone for deleted identities and an optional operational warning/CTA. Only expose `Phân công lại` where an existing permission-authoritative mutation flow already exists; otherwise show guidance without inventing a frontend-only action.
- Keep active selectors and recipient/member/assignee option endpoints filtered to active Staff/User accounts.
- Ensure inactive-but-not-deleted accounts do not receive the deleted label.

### 6. Cover every approved downstream scope

- Projects: list, explorer, workspace header, detail and progress board expose deleted PM state while retaining `pm_staff_id`; current-project surfaces show `Cần phân công lại`.
- Project members: management history includes inactive deleted Staff with the deleted label; ordinary active member directory and member options continue to exclude them.
- Work Items and My Work: project work lists, cards and details retain deleted assignees/watchers/assignment rows, show the label and expose reassignment only through existing authorized flows. Deleted users receive no actor queue or mutation capability.
- Work Reports and operational reports: author/recipient/reviewer references backed by live IDs expose deleted state; recipient options remain active-only; immutable work-report revision snapshots and PDFs remain unchanged.
- Activity, approval and audit: linked dynamic actor references may display current deletion state beside stored attribution; actor names captured in audit/activity are never rewritten.
- Construction, quality, procurement, warehouse, project-finance and legacy accounting views: add the marker to user-visible creator/assignee/reviewer/approver fields discovered by the inventory without changing their domain status transitions or financial/stock calculations.
- Statistics and dynamic CSV/XLSX/PDF exports: preserve active-only staff allocation metrics, but format deleted identities consistently in exported dynamic reference columns. Immutable signed/revision artifacts remain untouched.
- Notifications, mentions, recovery and other recipient lookups: deleted/inactive identities must not be newly selected or receive delivery work, while historical mention/read records remain preserved.
- Project explorer/cache revisions and frontend query caches are invalidated when Staff/User deletion state changes so stale active labels are not served.

### 7. Add focused regression and migration coverage

- Migration tests cover fresh install, V113-to-V115 upgrade, backfill, constraints, indexes, permission catalog, reserved grants, seed parity and Admin bootstrap completeness.
- Backend integration tests cover permission denial, self/last-Admin blockers, referenced PM/member/work-item deletion, batch all-or-nothing, idempotent replay, multiple Staff profiles per User, session revoke, recovery invalidation, role removal, selector exclusion, preserved foreign keys, deleted-reference flags, audit atomicity and reserved email/import conflict behavior.
- Query-contract tests cover Projects, members, work items, reports, audit/history and representative construction/quality/procurement/warehouse/finance actor references; add an inventory assertion so a new user-visible person reference cannot silently omit deletion state.
- Frontend tests cover permission visibility, current-page selection/reset behavior, desktop/mobile states, preview warnings versus blockers, confirmation, retry/success, shared label accessibility, reassignment guidance and unchanged rendering for active/inactive-not-deleted identities.
- Export tests verify UI/query/export parity for dynamic references and byte/hash stability for immutable revision/PDF artifacts.

### 8. Maintain maps, documentation and release verification

- Update the central module map, backend function map, frontend UI/utility map and Admin Governance domain map; update Project Planning, Workflow Orchestration and Reporting Analytics domain maps for their affected reference contracts.
- Keep this active-plan entry current as work moves from `Will be done` to `Done`.
- Run backend formatting and focused tests, a fresh PostgreSQL migration through V115, full backend tests, frontend tests/build, agent artifact validation and `git diff --check` against the final source state.
- Verify production-like migration/seed/permission state before release. Browser/UAT remains explicitly separate; without an authorized browser run the final UI status is `NOT UX ACCEPTED`.

## Acceptance criteria

- An authorized Admin/Developer can select and soft-delete one or many Staff rows even when they remain PMs, members, assignees or historical actors.
- Deleted accounts cannot log in, refresh sessions, recover passwords, appear in active assignment/recipient selectors or exercise retained authorization.
- All business foreign keys and immutable artifacts remain intact; no hard delete, forced NULL or rewritten stored name occurs.
- Every inventoried dynamic user-facing reference shows `<original name> (tài khoản đã xoá)` and every live orphaned responsibility shows `Cần phân công lại`.
- Self-deletion, last-active-Admin deletion, stale preview and mixed invalid batches fail closed with no partial mutation.
- Staff list, dynamic reports and exports use consistent labels; active staff metrics and immutable report/PDF snapshots retain their approved semantics.
- Migration, backend, frontend, permission, audit, export and cache regression suites pass on the exact final working-tree revision.

## Done

- User approved soft-delete with preserved business references, deleted-account presentation, live reassignment warnings and immutable-snapshot boundaries.
- Loaded the mandatory module-memory, Repo Architect, Impact Analysis, Admin Governance, Backend and Frontend instructions and mapped the work to Staff / RBAC / Account Security with downstream Projects, Work Items, Work Reports, Audit and Reporting scopes.
- Confirmed current source preserves FK targets under soft delete but presents deletion inconsistently: project PM joins retain the old name, project-member history filters deleted Staff out, active selectors correctly exclude them, and existing flattened name contracts carry no deleted-account marker.
- Added custom migration V114 with User/Staff deletion metadata and constraints, deterministic backfill/revocation, secret-free idempotent deletion batches, reserved `staff.delete` catalog/grants/trigger and seed/bootstrap parity. V115 owns the nine impact-query indexes without changing V114's applied checksum. Login email uniqueness remains unchanged.
- Added protected preview/execute APIs requiring `staff.delete` plus `staff.account_security.manage`. Preview validates/deduplicates at most 100 Staff UUIDs, returns authoritative warnings/blockers and binds them with an HMAC; execute revalidates, locks deterministically, blocks self/last-active-Admin, commits a batch atomically and supports same-actor/same-preview UUID idempotent replay.
- Soft-delete now preserves Staff/User and all business foreign keys, deactivates active project memberships, and only when the final live Staff profile is deleted marks the User inactive/deleted, revokes sessions, invalidates recovery tokens/state, clears first-password continuation and removes role assignments. Per-identity and batch audits fail closed and contain no credential/recovery secrets.
- Added controlled current-page selection to the shared DataTable and the permission-gated Staff deletion workflow for desktop/mobile, including preview loading, grouped impact warnings, blockers, explicit confirmation, all-or-nothing execution, retry-safe state and broad affected-query invalidation.
- Added the shared deleted-identity formatter/component with exact `(tài khoản đã xoá)` output and optional `Cần phân công lại`. Companion API flags and presentation now cover Projects/Explorer/Progress, project member history, Work Items/My Work, Work Reports and dynamic export, Activity/Approval/Audit, evidence/forms, Planning/handoff, Operations/QA, Acceptance, BOQ/Contracts/procurement reconciliation, Warehouse, Project Finance/VAT and Settings. Active selectors remain active-only; immutable revision/form/PDF/signature snapshots are not rewritten.
- Added focused migration/RBAC, middleware, backend integration and frontend presentation coverage, including an explicit V115 assertion for all nine impact-query indexes, referenced PM/member/work item preservation, construction/recurring live ownership flags, session/token/role revocation, reserved email, idempotent replay, shared-User multi-profile behavior and blocked-batch atomicity.
- Updated the central, backend, frontend and affected Admin/Project/Workflow/Reporting/Construction/Quality/Commercial/Warehouse/Finance domain maps.
- Exact current backend/migration state is technically verified: full DB-backed `go test -p 1 ./...` passed against a fresh disposable PostgreSQL database with migrations 1-115; V114/V115 checksums and all nine V115 indexes were confirmed before the disposable database was removed. The development database accepted restored V114, applied V115 exactly once, then reported `applied=0 skipped=115` on rerun. Frontend source did not change during this repair, so its prior 224/224 tests, production build and bundle-budget result remain applicable. Agent-artifact validation and `git diff --check` passed.

## Skipped / remaining authorization

- Browser/UAT is not authorized and has not run, so the UI remains `NOT UX ACCEPTED`.
- Production migration/deployment has not run. Migrations were verified on the repaired development database and a separate fresh disposable PostgreSQL database only.
- The local checksum incident was caused by running a DB-backed integration test against the shared development `DATABASE_URL`, which applied interim V114 before later index edits. V114 is now byte-identical to its recorded checksum `b5457bc5...`; the later SQL is isolated in V115 without rewriting migration history.
