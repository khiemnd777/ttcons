# [Closed] Chọn nhiều dòng để xoá Đầu vào, Đầu ra và Công trình

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-22; closed by user on 2026-08-22)

## Will Be Done

- Reuse the Staff deletion interaction for current-page selection, responsive selection actions, authoritative preview and explicit confirmation on the Input, Output and Projects screens.
- Soft-deactivate selected partner records globally with `active = false`; a multi-type partner consequently leaves both Input and Output while historical references and snapshots remain intact.
- Soft-cancel selected projects with `status = 'cancelled'` while preserving workflow, finance, warehouse, acceptance and audit history; enforce `project.delete` for every selected project.
- Add atomic, idempotent preview/execute APIs, custom migration V116 for batch replay state, per-record audit events and compatibility through the existing single-project delete route.
- Add focused backend/frontend coverage, update central/backend/frontend/Admin Governance/Project Planning maps, and run non-browser technical verification. Browser/UAT remains skipped unless explicitly authorized.

## Done

- User approved the review package, global partner soft-deactivation, project soft-cancellation, current-page selection and preview/execute with atomic idempotency.
- Loaded the mandatory module-memory, Repo Architect, Impact Analysis, Admin Governance, Project Planning, Backend, Frontend and Database workflows.
- Confirmed Input and Output share `partners`, had no deletion API/UI, and use `active`; confirmed Projects had no selection UI while its existing single-delete API only updated `status` to `cancelled`.
- Added reusable selection checkbox/action-bar/preview-confirmation components. Input and Output expose current-page selection only to `catalog.manage` users for active rows; Projects expose it only to `project.delete` users, including responsive desktop/mobile presentation and selection reset when the visible result set changes.
- Added partner/project preview and execute APIs with a 100-record ceiling, server-keyed preview integrity, current-row revision binding, explicit confirmation, atomic execution and actor-scoped UUID idempotency. Partner deletion globally sets `active = false`; project deletion sets `status = 'cancelled'`; dependent workflow, finance, warehouse, acceptance and historical references are preserved.
- Added migration V116 for durable batch replay state, per-record and batch audit events, per-project authority checks, stale-preview rejection and audited compatibility for the existing single-project delete route.
- Added PostgreSQL-backed integration coverage for impact summaries, cross-group partner removal, preserved references, project authority, stale revisions, audit trails, replay safety and legacy project deletion; added frontend source-contract coverage for all three screens and the shared Staff selection presentation.
- Updated the central, frontend, backend, Admin Governance and Project Planning maps for the new routes, UI ownership, deletion semantics and audit/idempotency behavior.
- Current working-tree verification passed on 2026-08-22: a fresh isolated PostgreSQL database applied all 116 migrations including V116; focused bulk-deletion integration tests and full backend `go test -p 1 ./...` passed; `make migrate-check` passed; all 227 frontend tests, production build and bundle-budget check passed; agent-artifact validation and `git diff --check` passed. One earlier rerun against an already-used test database hit unrelated accounting fixture unique-key collisions, so it was discarded and repeated successfully against the clean database. Both disposable test databases were removed.

## Remaining Acceptance At Closure

- Browser/UAT was not authorized, so the interface remains `NOT UX ACCEPTED` regardless of source checks, tests or builds.
- The running local Docker application and main local database were not rebuilt or migrated; verification used source builds and disposable PostgreSQL databases only.

## Durable References

- Feature routing and ownership are maintained in `.agents/skills/consflow-module-memory/references/`.
- Partner governance invariants are maintained in `.agents/skills/consflow-admin-governance-domain/references/domain-map.md`.
- Project lifecycle invariants are maintained in `.agents/skills/consflow-project-planning-domain/references/domain-map.md`.
