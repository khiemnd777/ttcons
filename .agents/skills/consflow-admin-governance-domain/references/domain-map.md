# Admin Governance Domain Map

## Owners

- Frontend modules: `frontend/src/modules/auth`, `frontend/src/modules/staff`, `frontend/src/modules/rbac`, `frontend/src/modules/core-data`, `frontend/src/modules/audit`, `frontend/src/modules/settings`
- Frontend auth/navigation: `frontend/src/lib/auth/AuthContext.tsx`, `frontend/src/app/router.tsx`, `frontend/src/layouts/AppShell.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `repository.go`, `types.go`
- Shared auth: `backend/internal/shared/middleware`, `backend/internal/shared/config`

## Routes

- Auth: `/auth/login`, `/auth/refresh`, `/auth/logout`, `/auth/me`
- Core data: `/partners`
- Staff/RBAC: `/staff`, `/roles`, `/permissions`, `/roles/:id/permissions`, `/users/:id/roles`
- Project membership governance: `/projects/:id/members`, `/projects/:id/members/:memberId`
- Audit/settings: `/audit-logs`, `/projects/:id/audit-logs`, `/settings`
- Reports governance: `/reports/options`, `/reports/staff` and `/reports/staff/export`

## Tables And Migrations

- `users`, `staff`, `roles`, `permissions`, `user_roles`, `role_permissions`, `project_members`
- `auth_sessions`
- `partners`
- `system_settings`, `audit_logs`
- Relevant migrations: `V1`, `V36`, `V37`, `V60`, `V68`, legacy accounting RBAC additions in `V73`, project-finance boundary/RBAC in `V83`, external-only client access in `V89`, actor/project explorer cache revisions/outbox retention in `V97`-`V100`, and revision-scoped workflow-form confirmations in `V101`

## Invariants

- Protected endpoints stay under auth middleware.
- The shared IP rate limiter runs before JWT/session work. `AuthClaimsRequired` must stop on missing/invalid JWT; only verified claims supply the actor key, so actor rate limiting runs before authoritative `AuthSessionRequired`/`AuthSessionActive` DB validation. Heavy-route concurrency runs after the session gate. Rejected `401`/`429`/`503` requests never reach business/audit handlers; backpressure responses use the shared envelope with `Retry-After`. Trusted proxy CIDRs must identify only the immediate Nginx/internal proxy network.
- Backend permission helpers are the source of truth for authorization.
- Audit read APIs are server-filtered and page-number paginated, return project labels plus permission-scoped filter options, use global `ProjectAccessFilter` or project `requireProjectPermission`, and expose metadata read-only. Audit persistence is centralized in `backend/internal/core/audit.go`: critical governance/workflow/finance and quality approval/terminal mutations use the same DB transaction and fail closed; classified non-critical operational events remain best-effort but emit structured failure logs. Metadata serialization and insert errors must never be silently discarded on a critical transition.
- Activity reply/reaction/edit/withdraw endpoints remain protected and reuse `ActivityVisibleToUser`; edit/withdraw additionally require exact author ownership, activity type `comment`, an unwithdrawn row and the 15-minute server window. UI capability flags never replace backend enforcement, managers receive no override, and no new global permission is introduced.
- Frontend navigation and row actions mirror permissions but do not replace backend enforcement.
- Work-item detail may expose the permission catalog name/description as presentation metadata, but authorization continues to use the raw snapshotted key through backend permission helpers. User-facing requirement text must not reveal raw keys; missing catalog metadata falls back to generic administrator guidance. V96 standardizes `planning.edit` as `Cập nhật kế hoạch` / `Cập nhật nội dung kế hoạch được phân công` without adding or removing any role grant.
- Submitted workflow-form amendment capability is server-owned and combines `CanActOnWorkItem` with the work item's snapshotted required permission; the UI `can_amend` flag only controls presentation. Amendment does not grant managers/directors a new override and does not require the historical work item to remain the current pipeline node. The atomic `workflow_form_amended` audit stores actor, mandatory reason, from/to revision and exact field-label-aware before/after changes; audit failure rolls back the content revision. Later signatures, seals and PDFs remain separately attributable to their form revision.
- My Work is one adaptive actor-scoped role workbench, not one page per role. List/detail `readiness`, capabilities and `available_actions` are actor-specific presentation hints only. `pending_approval` retains requester separation, required decision roles and project scope; `upcoming` work remains visible but current-time/current-node backend gates deny every mutation path. The authenticated SSE still exposes invalidation-only `work_queue_changed`. No new global permission, role filter, manager override, table or migration is introduced.
- Ordinary Reports navigation requires `report.view` and exposes only operational catalog entries. `Tài chính công trình` is separately gated by `project_finance.view`; its management totals/export require `project_finance.report_view`. Legacy `report.finance.view`/`accounting.*` are removed from every non-admin role by V83 and seed parity.
- Staff report is a project-allocation view: only active staff memberships in accessible non-cancelled projects are returned and distinct accessible projects are counted. Full-company staff is available only through the `staff.view` Staff module.
- `project.member.manage` is granted to Admin/Director/PM and always combined with project scope. Project member responses expose server capabilities; mutation synchronizes only project-scoped `user_roles`, never removes global roles, never exposes Admin/Director as assignable project roles, and writes project audit.
- Partner catalog updates preserve downstream snapshots for historical documents.
- Project setup presents the active `client` partner selector as `Chọn chủ đầu tư có sẵn` and its selected summary as `Chủ đầu tư đã chọn`; these user-facing labels do not change active/type filtering, `partner_id`, name snapshot behavior or catalog governance.
- Session refresh/revoke lifecycle remains DB-backed and revocable.
- `AuthSessionActive` is never cached: immediate session revoke remains authoritative even when Redis supplies distributed rate limiting and SSE Pub/Sub.
- The project explorer index cache is actor-scoped, not role-name scoped. Membership, staff-user linkage, user-role and role-permission mutations bump the affected actor revision in the same database transaction. Redis index IDs are still rechecked with current `ProjectAccessFilter` before any cached row is returned; cache keys or frontend visibility never authorize access.
- `roles.external_only` classifies business counterparts that cannot act in the App. V89 marks `client`, fails when migration would strand an external-only open work item/pending approval or a mixed-role user, removes App permissions, revokes sessions, inactivates linked users/staff/project memberships and preserves `user_roles` plus historical workflow/runtime labels. Login lookup, session creation/refresh/active checks, role edit, role-permission assignment, user-role assignment and project-member save all fail closed independently of frontend filtering.
- V68 adds the system roles `Thiết kế`, `Sale`, `Pháp lý` with the minimum permission sets needed by their official-workflow steps and grants existing `Đội thi công` pipeline/planning access. The migration does not auto-assign users or project memberships and does not broaden existing runtime role snapshots.
- Project-finance permissions are independent capability gates: view, material-cost create/review, cash-request create/approve/process, and management report view. Accounting owns capture/review/handoff processing, Director owns cash-request approval/reporting, and all actions still combine permission with project scope. Legacy ledger/tax permissions remain admin-only historical support.
- Manual posting, source posting, reversal, period close/reopen and migration-reconciliation resolution write actor/time/reason audit in the same transaction. Reconciliation resolution is one-way: a second resolution returns conflict and cannot overwrite the original conclusion.

## Cross-Domain Checks

- Pair with every affected source domain when adding permissions, roles, navigation entries, project scopes, or row actions.
- Pair with commercial, quality, and accounting domains when partner catalog fields or snapshot semantics change.
- Check audit logs and actor propagation when changing any privileged action, approval action, or session lifecycle behavior.

## Golden Live Accounts

- The canonical local/dev display identities are `Ngân Kế Toán` for `accounting@consflow.local` and `Hoàng Long` for `director@consflow.local`; `users.full_name` and the linked `staff.full_name` must remain aligned, while historical audit/activity actor-name snapshots are not rewritten by an account rename.
- Local/dev seed includes active App users for procurement, QA/QC, construction team, subcontractor, design, sales, legal and contract-department roles. `client@consflow.local` is an inactive negative-login fixture linked only to the external-only, permissionless `client` role; it is never a positive UAT actor.
- `GOLDEN-LIVE-001` has fourteen active App project members, including dedicated `Thiết kế`, `Sale` and `Pháp lý` actors. Client confirmation is evidenced through documents/forms/evidence and business fields, never by a client login or active membership.
- V72 grants `planning.view` and `planning.edit` to PM because the official workflow assigns PM to planning-gated steps. This does not bypass project scope or work-item role assignment: `CanActOnWorkItem` remains the first mutation gate.

## Rev3 Contract Governance

- V84 adds system role `contract_officer` displayed as `Bộ phận HĐ`, plus narrow dispatch/scan/QS-review/Purchasing-variation permissions. Contract workflow assignments include QS, Kế toán, Pháp lý, Mua hàng, Bộ phận HĐ, PM and Giám đốc; assignment does not replace permission and project-scope enforcement.
- V94 adds global `seal.manage` and `seal.use` capabilities. Admin receives both; Director and `contract_officer` receive only `seal.use`. Management can upload/list archived/archive protected seal assets through Settings; use still combines `seal.use` with current active work-item assignment and its required permission. Frontend gates are presentation only. Archiving is one-way metadata and never erases historical workflow seal applications or their audit trail; a seal is organizational confirmation and cannot substitute for a user's signature or contract `approved_by` App signature.
- Contract App signing, paper dispatch, returned-scan receive, activation and each variation stage record actor/time/comment in domain audit/event history. Frontend action visibility mirrors these gates but never authorizes them.
- V88 adds `contract.variation.propose` to Admin, PM and Supervisor only. It authorizes the project-scoped construction proposal endpoint, not contract list/detail or authoring: Supervisor must remain without `contract.view` and `contract.create`. The proposal transaction writes source work-item/action provenance, two variation events and one project audit before entering `qs_review`.
