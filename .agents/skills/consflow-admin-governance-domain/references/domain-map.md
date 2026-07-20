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
- Relevant migrations: `V1`, `V36`, `V37`, `V60`, `V68`

## Invariants

- Protected endpoints stay under auth middleware.
- Backend permission helpers are the source of truth for authorization.
- Audit read APIs are server-filtered and page-number paginated, return project labels plus permission-scoped filter options, use global `ProjectAccessFilter` or project `requireProjectPermission`, and expose metadata read-only without changing `AddAudit` or source-domain audit semantics.
- Activity reply/reaction/edit/withdraw endpoints remain protected and reuse `ActivityVisibleToUser`; edit/withdraw additionally require exact author ownership, activity type `comment`, an unwithdrawn row and the 15-minute server window. UI capability flags never replace backend enforcement, managers receive no override, and no new global permission is introduced.
- Frontend navigation and row actions mirror permissions but do not replace backend enforcement.
- My Work list/detail capabilities and `available_actions` are actor-specific presentation hints only. `pending_approval` must retain requester separation, required decision roles and project scope; department includes role scope; the authenticated SSE exposes invalidation-only `work_queue_changed` rather than unverified work-item metadata. No new global permission or manager override is introduced.
- Reports navigation is visible for any of `report.view`, `report.finance.view`, or `accounting.view`, but its catalog filters each report independently. Finance-only actors do not trigger operational report APIs, operational-only actors do not trigger finance APIs, and backend handlers remain authoritative.
- Staff report is a project-allocation view: only active staff memberships in accessible non-cancelled projects are returned and distinct accessible projects are counted. Full-company staff is available only through the `staff.view` Staff module.
- `project.member.manage` is granted to Admin/Director/PM and always combined with project scope. Project member responses expose server capabilities; mutation synchronizes only project-scoped `user_roles`, never removes global roles, never exposes Admin/Director as assignable project roles, and writes project audit.
- Partner catalog updates preserve downstream snapshots for historical documents.
- Session refresh/revoke lifecycle remains DB-backed and revocable.
- V68 adds the system roles `Thiết kế`, `Sale`, `Pháp lý` with the minimum permission sets needed by their official-workflow steps and grants existing `Đội thi công` pipeline/planning access. The migration does not auto-assign users or project memberships and does not broaden existing runtime role snapshots.

## Cross-Domain Checks

- Pair with every affected source domain when adding permissions, roles, navigation entries, project scopes, or row actions.
- Pair with commercial, quality, and accounting domains when partner catalog fields or snapshot semantics change.
- Check audit logs and actor propagation when changing any privileged action, approval action, or session lifecycle behavior.

## Golden Live Accounts

- Local/dev seed includes dedicated users for procurement, QA/QC, construction team, subcontractor and client roles (`procurement@`, `qaqc@`, `construction@`, `subcontractor@`, `client@consflow.local`), all documented in `backend/seed/GOLDEN_LIVE_RUNBOOK.md`.
- `GOLDEN-LIVE-001` has twelve active project members so role-scoped work, requester/approver separation and external client/subcontractor participation can be exercised without weakening backend RBAC.
