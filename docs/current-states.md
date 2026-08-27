# Current States

## Release v2026-08-27.2

### Will be done

- Release the approved Warehouse corrections/material editing, global Warehouse navigation and Partner catalog tab changes from the current clean `main` branch.
- Run the full CI-equivalent validation on the final release revision: backend format, all migration checksum checks, serial backend tests on fresh PostgreSQL 16/Redis 7, release/operator tooling tests, frontend tests/build/bundle budget, production Caddy image validation and whitespace checks.
- Run the production configuration preflight before creating the immutable annotated tag `v2026-08-27.2`.
- Monitor the tag-triggered GitHub Actions workflow through validation, digest-pinned GHCR image builds, fail-closed VPS migration/bootstrap/deployment and public HTTPS verification.
- Run independent post-deploy gateway, readiness and frontend checks; keep browser/UAT explicitly out of scope.

### Done

- Release/deploy review package approved by the user.
- Confirmed the pre-release source payload baseline at `0681148769f6624319d7d5e07188c9023e030a85`: clean local `main`, exact match with remote `origin/main`, two commits ahead of `v2026-08-27.1`, and no GitHub Actions run for this SHA because ordinary `main` pushes do not trigger workflows.
- Confirmed the existing production baseline before release: `/gateway-health` returned HTTP 204, `/api/ready` returned HTTP 200 and the frontend root returned HTTP 200 with valid TLS.
- Status: `TARGET`; validation, tag creation and deployment are still pending.

## Global Warehouse navigation — 2026-08-27

### Will be done

- Add one permission-filtered global `Kho vật tư` navigation item at `/warehouse`, immediately below `Đối tác` in the standard App shell.
- Keep the existing project-scoped `Kho vật tư` business-tree entry and preserve the current global/project filter semantics of all four Warehouse tabs.
- Add focused navigation coverage for label, route, `warehouse.view` RBAC and ordering after `Đối tác`.
- Update the central module, frontend UI/utility and Materials Warehouse domain maps, then verify focused/full frontend tests and the production build without browser/UAT.

### Done

- Review package and Mockup as Text approved by the user.
- Added the permission-filtered global `Kho vật tư` item at `/warehouse`, immediately below `Đối tác` and before `Nhân sự` in the shared desktop/mobile App navigation.
- Preserved the existing project-scoped Warehouse business-tree entry and made no route, API, database, stock-ledger or Warehouse-tab behavior change.
- Added focused source-contract coverage for the route, label, `warehouse.view` permission and exact navigation order.
- Updated the central module, frontend UI/utility and Materials Warehouse domain maps with the global/project navigation and tab-scope contract.
- Focused App-shell navigation tests passed 2/2; the full frontend suite passed 287/287; the production TypeScript/Vite build passed on the resulting frontend source, with only the existing Vite chunk-size warning.
- Status: `TECHNICALLY VERIFIED`; `NOT UX ACCEPTED` because browser/UAT was not requested and did not run.

## Partner catalog tabs — 2026-08-27

### Will be done

- Keep one global `Đối tác` navigation item while presenting `Đầu vào` and `Đầu ra` as route-backed tabs inside the shared partner workspace.
- Restore `/core-data/input` and `/core-data/output` to their existing server-backed partner groups, with `/core-data` defaulting to Input.
- Preserve partner multi-type membership, global soft deletion, permissions, API/storage compatibility and downstream snapshots.
- Update focused frontend coverage and the central/frontend/admin mapping references, then verify frontend tests and production build without browser/UAT.

### Done

- Review package and Mockup as Text approved by the user.
- Kept one global `Đối tác` navigation item and added responsive route-backed `Đầu vào`/`Đầu ra` tabs inside `CoreDataPage`.
- Restored `/core-data/input` and `/core-data/output` to `group=input|output`; `/core-data` now defaults to Input.
- Removed the frontend-only `all` catalog mode while preserving multi-type membership, output exact-type filtering, global soft deletion, permissions and downstream snapshot behavior.
- Updated focused partner presentation coverage plus the central module, frontend UI/utility and Admin Governance maps.
- Focused navigation/partner tests passed 4/4; the full frontend suite passed 287/287; the production TypeScript/Vite build passed on the resulting frontend source.
- Status: `TECHNICALLY VERIFIED`; `NOT UX ACCEPTED` because browser/UAT was not requested and did not run.

## Release v2026-08-27.1

### Will be done

- Retain this completed release record until the user explicitly asks to close the plan; then move it to `docs/plan-closed/` under the normal close-plan workflow.

### Done

- Reproduced the previous release failure locally on a migration-only PostgreSQL 16 and Redis 7 environment.
- Confirmed 11 backend failures: eight missing domain execution contexts, one stale handoff lifecycle assertion, and two sample-data tests coupled to demo seed data.
- Confirmed the runtime Work Item lock query joined `pipeline_run_steps` without selecting `surface_kind` or `domain_action_key`.
- Implemented the persisted Run Step context selection, domain-aware integration fixture binding, handoff lifecycle assertion, and isolated Developer test actor.
- Passed all 11 targeted DB-backed regressions on a fresh migration-only PostgreSQL 16 and Redis 7 environment.
- Updated the module keyword, backend function, and root-cause triage maps with the corrected execution-context contract.
- Passed backend formatting, all 131 migration checksum checks, and `go test -p 1 ./...` on a fresh PostgreSQL 16 and Redis 7 environment.
- Passed release-tag validation, production operator tooling tests, all 287 frontend tests, frontend production build, the 250 KiB gzip bundle budget, production Caddy gateway image validation, and `make production-check`.
- Committed and pushed runtime revision `a19b2d6debe416c24e1e97a116c2058ae9fe4482`, then created release tag `v2026-08-27.1` from that exact revision.
- GitHub Actions run `33027115506` completed successfully: tag validation, reusable validation, immutable backend/admin image build, fail-closed VPS production deployment, and public HTTPS verification all passed.
- Independent post-deploy checks passed: `/gateway-health` returned HTTP 204, `/api/ready` returned HTTP 200 with database `ok`, and the frontend root returned HTTP 200.
