# Current States

## Active Plan: Typed system settings, Developer governance, seal table, local login defaults

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; NOT UX ACCEPTED (approved by user on 2026-08-19)

### Will be done

- Keep bootstrap/deployment/root-secret fields in `.env` or Docker secrets.
- Move approved runtime/application policy fields into typed database-backed settings with validation, masking, audit and restart metadata.
- Wire `client_payment_min_percent` into the construction payment gate and normalize `surplus_handling_options` to the existing warehouse enum.
- Add system role `developer` with full permissions; keep Admin seal management but make system settings read-only; hide Settings from other roles.
- Redesign `/settings` as `Con dấu` and `Thiết lập hệ thống` tabs; render seals as an actions-first, thumbnail-second responsive table.
- Remove the demo-account selector while retaining local-development prefill `admin@consflow.local` / `123456`; production login fields stay blank.
- Add local `developer@consflow.local` / `123456`; production migration creates the role but no developer user.
- Preserve current custom migration runner, Docker service names and down -> build -> run Make workflow.
- Update affected module/detail/domain maps in the same implementation turn.
- Validate backend formatting/tests, frontend tests/build and local/production Compose configuration. Browser/UAT remains skipped unless explicitly requested.

### Done

- Review package, environment/database field split, RBAC matrix, Mockup as Text and validation plan approved.
- Mandatory module mapping and Admin Governance, Backend, Frontend, Database, Docker, Workflow, Materials and Accounting guardrails loaded.
- Cross-domain collision review completed: no ledger/status transition ownership conflict; runtime bootstrap and settings/audit/RBAC contracts require coordinated changes.
- Added V110 typed/versioned `system_settings`, Developer system role, reserved Settings RBAC cleanup and future-assignment guard. The custom migration runner and applied-migration checksum contract remain unchanged.
- Added two-stage backend bootstrap: validate core `.env`, migrate/connect, one-time import only missing database settings, apply database-authoritative runtime values, validate and reconnect with configured pool values.
- Added a 78-field typed database/UI registry, atomic optimistic bulk update, audit events, restart-pending metadata, API secret redaction and AES-GCM storage for `SMTP_PASSWORD` using the core `SETTINGS_ENCRYPTION_KEY`.
- Wired the hot `client_payment_min_percent` setting into the construction payment gate and `surplus_handling_options` into server validation plus the work-item selector without rewriting historical records.
- Enforced Developer full settings/seal management, Admin seal management plus read-only system settings, and no Settings permissions/navigation for other roles in migration, seed and role-permission replacement logic.
- Rebuilt Settings into `Con dấu` and `Thiết lập hệ thống`: actions-first/thumbnail-second seal table with mobile cards; typed categorized controls, validation, secret clear flow, restart state and read-only 26-field core `.env` inventory.
- Removed the demo-account selector. Development prefills `admin@consflow.local` / `123456`; production fields are blank. Local seed adds `developer@consflow.local` / `123456`; migration creates no production user.
- Updated `.env` examples, local/production Compose secret wiring, definitive field/RBAC documentation, README behavior and all affected module/detail/domain maps.
- Technical verification on the final working tree: `go test ./...` passed; frontend `207/207` tests passed; frontend production build passed; `git diff --check` passed; local and production Compose configs passed.
- Database/runtime verification: V110 applied through the custom runner; clean disposable database ran all 110 migrations plus seed with correct Admin/Developer grants; final backend image rebuilt and started with 110 migrations recorded, PostgreSQL/Redis services healthy and `/api/ready` returned 200/database `ok`.
- Final API verification: Admin receives 78 DB/UI settings, 26 core fields and `can_update=false`; Admin PUT returns 403; Developer receives `can_update=true`; sensitive plaintext count in the Settings response is zero.

### Skipped / remaining acceptance

- Browser/UAT was not run because the user did not explicitly request browser control. UI/UX status remains `NOT UX ACCEPTED`; frontend tests and build prove technical integrity only.
