# [Closed] System settings JSON backup and restore

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; NOT UX ACCEPTED (approved by user on 2026-08-20, closed by user on 2026-08-20)

## Will be done

- Export a versioned JSON backup of all non-sensitive database/UI system settings.
- Exclude core `.env` fields, seals, identities/business data and every secret value/ciphertext; expose only secret configured-state metadata.
- Let Admin and Developer download the backup through `settings.view`; reserve preview/restore for Developer through `settings.update`.
- Validate JSON format/version, 256-KiB size, duplicate keys, registry/type/range/cross-field compatibility and reject any core or sensitive payload key.
- Preview changed/unchanged/unsupported counts, warnings, old/new values and restart impact without writing database state.
- Restore changed known values atomically with optimistic versions, file-hash consistency, per-setting audit and one summary audit.
- Add responsive Settings UI actions, upload/preview/error/conflict/success states and preserve current secret values during restore.
- Update focused tests, settings documentation and affected module/backend/frontend/Admin Governance maps.
- Validate backend format/tests/vet, frontend tests/build, Compose configuration and authenticated API behavior. Browser/UAT remains skipped unless explicitly requested.

## Done

- Review package, security scope, RBAC behavior, compatibility rules, API outline, Mockup as Text and validation plan approved.
- Mandatory module/backend/frontend/Admin Governance/Database/impact maps and current owner code inspected.
- Added a versioned `consflow-system-settings` JSON export with registry fingerprint, environment/timestamp metadata, 77 non-sensitive database/UI settings and configured-state-only metadata for the excluded `SMTP_PASSWORD` secret.
- Added authenticated `GET /settings/backup`, no-write `POST /settings/backup/preview` and atomic `POST /settings/backup/restore`; Admin/Developer can export through `settings.view`, while only Developer can preview/restore through `settings.update`.
- Added a strict 256-KiB parser and registry analysis: unsupported non-core keys warn/skip, missing settings preserve current values, and duplicate/core/secret/known-invalid payloads reject the whole restore before writes.
- Restore requires the exact previewed file hash and current per-setting versions, revalidates the combined configuration, updates changed known values in one transaction and records per-setting plus summary audits without secret values.
- Added the responsive Settings backup card and shared restore dialog with file/drop validation, compatibility metrics, warnings, before/after values, no-change/error/conflict/success states and immediate/restart impact.
- Added focused backend and frontend regression coverage and updated `docs/system-settings.md` plus the module, backend, frontend and Admin Governance maps. No migration was required because backup/restore reuses the V110 typed settings registry/table.
- Final source verification on the post-format working tree: `go test ./...` passed, `go vet ./...` passed, frontend `209/209` tests passed, production frontend build passed and `git diff --check` passed.
- Local and production Compose configs passed. Backend/admin images were rebuilt from the verified working tree; the backend started with PostgreSQL/Redis healthy and all 110 migrations recorded, and `/api/ready` returned 200.
- Authenticated API round-trip passed: Admin export 200 with 77 settings/one excluded secret, Admin preview/restore 403, Developer preview/restore 200, payment setting restored `30 -> 31 -> 30`, secret ciphertext/configured state unchanged and two restore summary audits added.
- Conflict guards passed against the running API: mismatched preview hash and stale expected versions each returned 409, and the final database value remained 30.

## Skipped / remaining acceptance

- Browser/UAT was not run because the user did not explicitly request browser control. UI/UX status remains `NOT UX ACCEPTED`; frontend tests, build and API verification prove technical integrity only.
