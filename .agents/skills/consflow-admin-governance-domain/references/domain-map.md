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
- Audit/settings: `/audit-logs`, `/projects/:id/audit-logs`, `/settings`

## Tables And Migrations

- `users`, `staff`, `roles`, `permissions`, `user_roles`, `role_permissions`, `project_members`
- `auth_sessions`
- `partners`
- `system_settings`, `audit_logs`
- Relevant migrations: `V1`, `V36`, `V37`

## Invariants

- Protected endpoints stay under auth middleware.
- Backend permission helpers are the source of truth for authorization.
- Frontend navigation and row actions mirror permissions but do not replace backend enforcement.
- Partner catalog updates preserve downstream snapshots for historical documents.
- Session refresh/revoke lifecycle remains DB-backed and revocable.

## Cross-Domain Checks

- Pair with every affected source domain when adding permissions, roles, navigation entries, project scopes, or row actions.
- Pair with commercial, quality, and accounting domains when partner catalog fields or snapshot semantics change.
- Check audit logs and actor propagation when changing any privileged action, approval action, or session lifecycle behavior.
