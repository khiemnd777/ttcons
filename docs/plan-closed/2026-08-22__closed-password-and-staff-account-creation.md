# [Closed] Thống nhất mật khẩu và tạo tài khoản cùng nhân sự

Status: TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved and closed by user on 2026-08-22)

## Will be done

- Apply one canonical new-password policy across staff account creation, self change, recovery reset and Admin reset: at least 8 Unicode code points, at most 72 UTF-8 bytes, not whitespace-only; existing-account replacement must differ from the current password.
- Keep login/current-password verification as credential checks rather than new-password-strength checks, while rendering all password controls through one shared eye-toggle component.
- Replace Staff create's generic staff-only insert with an audit-atomic linked `users` + `staff` creation that requires `staff.create` and `staff.account_security.manage`, includes one password field (no confirmation) plus an optional immediately verified recovery email, and never returns or audits password material.
- Keep confirmation fields on self change, recovery reset and Admin reset; generate the Staff initial password client-side with Web Crypto through an icon-only rotating suggestion control.
- Synchronize linked user full name, login email and active/inactive status from Admin Staff edits; revoke sessions when login identity or activation changes. Keep role assignment as the existing separate shield action.
- Add focused backend/frontend coverage, update the central/frontend/backend/Admin Governance maps and run non-browser technical validation.

## Done

- User approved the review package and Mockup as Text, with two changes: Staff create has no password confirmation and includes a recovery email field.
- Confirmed the existing backend policy, frontend Unicode-count mismatch, generic Staff-only create path, account-security permission/audit flow and linked identity drift risk from current source.
- Loaded the mandatory central, frontend, backend, root-cause and Admin Governance maps plus Frontend, Backend and Impact Analysis instructions.
- Added one canonical backend/frontend new-password contract (8 Unicode code points, 72 UTF-8 bytes, non-whitespace), exact confirmation outside initial Staff creation, current-credential replacement rejection and a shared eye-toggle password input. Login/current-password checks remain presence-only credential verification.
- Replaced `POST /staff` with a fail-closed transaction that requires both create and account-security permissions, creates linked User + Staff rows with an optional immediately verified recovery email, writes two secret-safe audits and assigns no role. Admin Staff edits now require confirmation for password replacement, synchronize linked identity/status, reject direct relinking/external-only activation and revoke affected sessions/tokens.
- Reworked the Staff create UI into the approved shared dialog: one initial-password field, icon-only Web Crypto suggestion, eye toggle, optional recovery email and no confirmation field. Profile, Login, Recovery Reset and Staff edit use the shared password component/policy where applicable.
- Added focused backend/frontend regression coverage and updated the central, backend, frontend, root-cause and Admin Governance maps.
- Technical verification on the current working tree passed: frontend `npm test` (216/216), frontend `npm run build` (Vite 6.4.3, 1,854 modules), and backend `go test -p 1 ./...` against a fresh PostgreSQL database with all custom migrations. The temporary database was removed after the run.
- User approved reducing only the generated Staff initial-password suggestion from 16 to exactly 8 characters while preserving Web Crypto generation, all four character groups and the canonical password acceptance policy.
- Changed the shared suggested-password default to exactly 8 characters, added an explicit regression assertion for that length, updated the central/frontend maps and revalidated the current frontend source with `npm test` (216/216) plus `npm run build` (Vite 6.4.3, 1,854 modules).
- User approved making Staff `department` and `position` optional in both create and edit, storing blank values as SQL `NULL`, rendering missing values as `—` in the Staff table and accepting that department-only work assignment does not match until a department is supplied.
- Made `Phòng ban` and `Chức vụ` optional in the shared Staff create/edit field set, removed fabricated defaults, added routing/authorization helper text and rendered missing table values as `—`. Backend create/update now normalize blank values to SQL `NULL`; omitted update keys still preserve current values.
- Revalidated the current working tree after the optional-field amendment: frontend `npm test` (217/217), frontend `npm run build` (Vite 6.4.3, 1,854 modules), backend `go test -p 1 ./...` against the fresh `consflow_staff_optional_test` PostgreSQL database, and `git diff --check`. The temporary database was removed after verification.

## Skipped / remaining acceptance

- Browser/UAT was not authorized during implementation and remained skipped. Frontend technical checks do not establish UX acceptance.
