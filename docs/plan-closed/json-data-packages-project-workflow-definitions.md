# [Closed] JSON data packages cho Pipeline, Form/Fields và Công trình

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-23)

## Will Be Done

- Add JSON v1 as a second transport beside existing XLSX for Project and Workflow Definition template/import/export while keeping one canonical backend preview/execute engine.
- Keep existing routes and permissions, default downloads to XLSX, and select JSON through `format=json`; preserve all current project scope, readiness, draft-only, no-overwrite, no-publish and no-runtime side-effect invariants.
- Add additive V120 `source_format` metadata to both legacy import-batch tables without renaming tables or changing V117-V119 checksums.
- Add browser-local JSON parsing, format selection and approved responsive states to the existing Project and Workflow Definition dialogs.
- Add focused migration/backend/frontend regression coverage, update central/detail/domain maps, and run non-browser technical verification. Browser/UAT remains skipped unless explicitly authorized.

## Done

- User approved the JSON v1 review package, read-only reference snapshots, compatibility contract, additive V120 plan and Mockup as Text.
- Reloaded mandatory module-memory, repository, impact, backend, frontend, database and Workflow/Project/Admin domain instructions; rechecked existing routes, canonical request/package types, batch tables and dialog owners in the current dirty working tree.
- Confirmed the implementation remains safe as an adapter/generator extension: no new permission, route family, workflow transition, runtime mutation or second import engine is required.
- Added JSON v1 beside the existing XLSX transport on the same Project and Workflow Definition template/import/export routes. Existing callers still default to XLSX; `format=json` downloads `consflow.project-package` or `consflow.workflow-definition-package`, and both browser Workers adapt their selected file into the existing canonical preview/execute requests.
- Project JSON carries editable nine-field `projects`, read-only `project_snapshots`, readiness and complete Pipeline/Form/Field/client references. Workflow Definition JSON carries nested Pipeline node/step/edge topology, Form template/version/Fields and permission/role/edge-condition/published-Form/Pipeline-family references. Portable package keys own internal relations; source IDs/hashes remain provenance.
- Kept all approved fail-closed invariants: Project template/import remains blocked without ready published Pipeline plus valid published Form/Fields; Workflow Definition import creates new drafts only, Forms before Pipelines in one transaction; neither import publishes, starts a run, creates work items/Form instances/members/business records, overwrites definitions or restores runtime.
- Added shared strict JSON validation in browser and backend: version/extension alignment, exact editable schema keys, duplicate-key and dangerous-key rejection, one-document/depth limits, unknown top-level/nested request rejection, and HMAC binding of `source_format` plus `package_version`. VND export values remain exact integer strings instead of inheriting PostgreSQL `.00` scale.
- Added additive V120 `source_format` with fail-closed `xlsx|json` checks and legacy default `xlsx` on both import batch tables. New and replayed results expose the persisted source format; audit event names and metadata are transport-neutral. No existing migration checksum was changed.
- Added format selectors and dynamic file accept/download/copy states to the approved responsive Project and Workflow Definition dialogs. Project page actions now read `Nhập dữ liệu`/`Xuất dữ liệu`; permissions, readiness links, blockers, confirmation and toast behavior are unchanged.
- Updated central/backend/frontend maps and Project Planning, Workflow Orchestration, Admin Governance and Reporting Analytics domain maps for the JSON contracts, V120 provenance and shared validators.
- Current working-tree technical verification passed on 2026-08-23: local development DB applied V120 (`applied=1 skipped=119`) and exposes both `source_format` columns with default `xlsx`; a separate clean PostgreSQL database applied all 120 migrations, accepted the full seed, passed focused DB-backed Project/Workflow JSON import-export plus RBAC tests, and was then removed. Focused JSON/security/VND tests passed; full backend `go test ./...` passed; frontend passed 237/237 tests, production TypeScript/Vite build and bundle budget; `git diff --check` passed.
- Closed by user request on 2026-08-23.

## Skipped / Remaining Acceptance

- Browser/UAT was not authorized, so the interface remains `NOT UX ACCEPTED`.
- The local Docker services were not rebuilt/restarted after these source changes; verification used source-level tests/build plus direct local and clean-database migration/test runs.
