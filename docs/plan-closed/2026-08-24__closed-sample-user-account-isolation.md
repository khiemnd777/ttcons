# [Closed] Tài khoản mẫu cách ly cho Developer Sample Data

Closed on: 2026-08-24

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-24)

## Will be done

- Add an explicit sample-account flag and role identity through custom migration V123 without converting or mutating existing real users.
- Add Developer-authorized, atomic and idempotent sample-user provisioning for exactly the eligible roles required by the selected published Pipeline; derive portable `@dev.local` email addresses from role keys and keep the approved password bcrypt-only.
- Make sample-project generation use only ready sample accounts, grant their exact roles project-scoped, and enforce two-way sample/real isolation in project membership and role assignment paths.
- Update Developer Sample Data and Staff UI states, focused backend/frontend regression coverage, and the central/backend/frontend/Admin/Project/Workflow maps; run non-browser technical verification.

## Done

- User approved the review package, exact password `Dev240489.`, no forced password change, no global role grants, per-project role grants, eligible Pipeline roles only, and two-way isolation from real users and projects.
- Loaded and checked the mandatory central/backend/frontend maps plus Admin Governance, Project Planning, Workflow Orchestration, Backend, Frontend, Impact Analysis and Repo Architect instructions against current source.
- Confirmed current sample generation selects active globally assigned real staff by role and generic project-member options have no sample boundary; existing users currently have no sample-account discriminator.
- Added V123 `users.is_sample|sample_role_id`, one-sample-account-per-role uniqueness and database triggers that reject sample global roles plus sample/real project-role, membership and PM mixing. Existing users remain real and the migration creates no account.
- Added protected `POST /settings/sample-data/users`. It resolves only the eligible exact roles required by the selected current-environment published Pipeline, derives lowercase `<role.name>@dev.local`, atomically creates only missing active User/Staff rows with the approved bcrypt-only password, `must_change_password=false`, no recovery email/global role, and secret-free audits. Ambiguous/missing/restricted/unsafe roles, email conflicts and drifted/deleted sample identities fail closed; a fully provisioned Pipeline returns the all-ready no-duplicate result.
- Sample Project generation now loads only ready flagged accounts, adds only Pipeline-required members, grants their exact roles project-scoped, and uses a sample PM/actor for work, Forms, signatures, approvals and completed business/finance history. The real Developer is retained only for generation replay ownership and the explicit generation audit.
- Generic project member options/save and user-role replacement enforce the same two-way isolation; Staff list/presentation exposes the sample flag and badge and disables manual role management for sample accounts. Developer Sample Data UI adds role/email/status provisioning before profile/progress/accounting selection and keeps Project creation disabled until all sample roles are ready.
- Corrected the production Admin bootstrap completeness check to exclude existing Developer-only `sample_data.create`, matching V121 and seed policy. Updated central/backend/frontend and Admin Governance/Project Planning/Workflow Orchestration maps plus focused backend/frontend coverage.
- Current working-tree technical verification passed: migration manifest checked 123 files; V123 applied to the local development database; a separate clean PostgreSQL database applied all 123 migrations and passed full sequential DB-backed `go test -p 1 ./...`, then was removed. Focused DB tests prove bcrypt credential, no forced change/global role, idempotent no-duplicate provisioning, only sample members/actors, exact project roles, My Work visibility and two-way repository/database isolation. Ordinary backend tests and `go vet` passed. Frontend passed 240/240 tests, production build and the 250 KiB gzip/141-chunk budget.
- The long-lived development database full DB suite still exposes five unrelated stale-fixture collisions plus old Pipeline fixture state; none reproduced on the clean database. Rebuilt/restarted the full local Docker stack through `down -> build -> run` after the sandbox-only Docker Buildx denial was rerun with approved access. PostgreSQL/Redis are healthy, backend reports all 123 migrations present and listens successfully, and Admin `/settings` returns HTTP 200. This is a non-browser runtime smoke.

## Skipped / remaining acceptance

- Browser/UAT was not authorized in the implementation turn, so the interface remains `NOT UX ACCEPTED` until explicitly tested.
