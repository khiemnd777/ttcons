# [Closed] Project Workspace Account Presentation Parity

Status: closed on 2026-09-14 by user request.

## Objective

Restore the current-account avatar and profile navigation in the Project Workspace header at `/projects/:id`, matching the standard `/projects` account action.

## Completed Scope

- Reused the shared `UserAvatar` in `ProjectWorkspaceLayout.tsx`.
- Made the desktop account control accessible as `Mở hồ sơ cá nhân` and route it to `/profile`.
- Preserved the protected avatar request, initials fallback, role summary, existing narrow-screen visibility rule, notification control and logout flow.
- Kept backend APIs, auth/RBAC, database schema and migrations unchanged.
- Added a focused regression source contract in `frontend/tests/appShellNavigation.test.ts`.
- Updated the durable frontend avatar ownership mapping.

## Verification At Closure

- `node --experimental-strip-types --test tests/appShellNavigation.test.ts`: passed, 4/4 tests.
- `cd frontend && npm run build`: passed after the implementation changes.
- `git diff --check`: passed.
- Browser/UAT was not run because it was not explicitly authorized; UX status remains `NOT UX ACCEPTED`.
