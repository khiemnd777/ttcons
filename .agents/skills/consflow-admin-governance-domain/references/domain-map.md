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
- Relevant migrations: `V1`, `V36`, `V37`, `V60`, `V68`, accounting RBAC additions in `V73`

## Invariants

- Protected endpoints stay under auth middleware.
- The shared IP rate limiter runs before JWT/session work. `AuthClaimsRequired` must stop on missing/invalid JWT; only verified claims supply the actor key, so actor rate limiting runs before authoritative `AuthSessionRequired`/`AuthSessionActive` DB validation. Heavy-route concurrency runs after the session gate. Rejected `401`/`429`/`503` requests never reach business/audit handlers; backpressure responses use the shared envelope with `Retry-After`. Trusted proxy CIDRs must identify only the immediate Nginx/internal proxy network.
- Backend permission helpers are the source of truth for authorization.
- Audit read APIs are server-filtered and page-number paginated, return project labels plus permission-scoped filter options, use global `ProjectAccessFilter` or project `requireProjectPermission`, and expose metadata read-only. Audit persistence is centralized in `backend/internal/core/audit.go`: critical governance/workflow/finance and quality approval/terminal mutations use the same DB transaction and fail closed; classified non-critical operational events remain best-effort but emit structured failure logs. Metadata serialization and insert errors must never be silently discarded on a critical transition.
- Activity reply/reaction/edit/withdraw endpoints remain protected and reuse `ActivityVisibleToUser`; edit/withdraw additionally require exact author ownership, activity type `comment`, an unwithdrawn row and the 15-minute server window. UI capability flags never replace backend enforcement, managers receive no override, and no new global permission is introduced.
- Frontend navigation and row actions mirror permissions but do not replace backend enforcement.
- My Work list/detail capabilities and `available_actions` are actor-specific presentation hints only. `pending_approval` must retain requester separation, required decision roles and project scope; department includes role scope; the authenticated SSE exposes invalidation-only `work_queue_changed` rather than unverified work-item metadata. No new global permission or manager override is introduced.
- Reports navigation is visible for any of `report.view`, `report.finance.view`, or `accounting.view`, but its catalog filters each report independently. Finance-only actors do not trigger operational report APIs, operational-only actors do not trigger finance APIs, and backend handlers remain authoritative.
- Staff report is a project-allocation view: only active staff memberships in accessible non-cancelled projects are returned and distinct accessible projects are counted. Full-company staff is available only through the `staff.view` Staff module.
- `project.member.manage` is granted to Admin/Director/PM and always combined with project scope. Project member responses expose server capabilities; mutation synchronizes only project-scoped `user_roles`, never removes global roles, never exposes Admin/Director as assignable project roles, and writes project audit.
- Partner catalog updates preserve downstream snapshots for historical documents.
- Session refresh/revoke lifecycle remains DB-backed and revocable.
- `AuthSessionActive` is never cached: immediate session revoke remains authoritative even when Redis supplies distributed rate limiting and SSE Pub/Sub.
- V68 adds the system roles `Thiết kế`, `Sale`, `Pháp lý` with the minimum permission sets needed by their official-workflow steps and grants existing `Đội thi công` pipeline/planning access. The migration does not auto-assign users or project memberships and does not broaden existing runtime role snapshots.
- Accounting ledger permissions are independent capability gates: `accounting.journal_view`, `accounting.journal_post`, `accounting.journal_reverse`, `accounting.period_close`, `accounting.period_reopen`, `accounting.tax_view`, and `accounting.tax_manage`. Seed parity grants the accountant all seven; Director receives read/reversal/period/tax visibility but not manual posting or tax master-data management. Every record action still combines the permission with project scope.
- Manual posting, source posting, reversal, period close/reopen and migration-reconciliation resolution write actor/time/reason audit in the same transaction. Reconciliation resolution is one-way: a second resolution returns conflict and cannot overwrite the original conclusion.

## Cross-Domain Checks

- Pair with every affected source domain when adding permissions, roles, navigation entries, project scopes, or row actions.
- Pair with commercial, quality, and accounting domains when partner catalog fields or snapshot semantics change.
- Check audit logs and actor propagation when changing any privileged action, approval action, or session lifecycle behavior.

## Golden Live Accounts

- The canonical local/dev display identities are `Ngân Kế Toán` for `accounting@consflow.local` and `Hoàng Long` for `director@consflow.local`; `users.full_name` and the linked `staff.full_name` must remain aligned, while historical audit/activity actor-name snapshots are not rewritten by an account rename.
- Local/dev seed includes dedicated users for procurement, QA/QC, construction team, subcontractor, client, design, sales and legal roles (`procurement@consflow.local`, `qaqc@consflow.local`, `construction@consflow.local`, `subcontractor@consflow.local`, `client@consflow.local`, `design@consflow.local`, `sales@consflow.local`, `legal@consflow.local`), all documented in `backend/seed/GOLDEN_LIVE_RUNBOOK.md`.
- `GOLDEN-LIVE-001` has fifteen active project members, including dedicated `Thiết kế`, `Sale` and `Pháp lý` demo actors, so role-scoped work, requester/approver separation and external client/subcontractor participation can be exercised without substituting Admin.
- V72 grants `planning.view` and `planning.edit` to PM because the official workflow assigns PM to planning-gated steps. This does not bypass project scope or work-item role assignment: `CanActOnWorkItem` remains the first mutation gate.
