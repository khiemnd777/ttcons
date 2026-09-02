# [Closed] Active plans archive — 2026-09-02

Closed by user request on 2026-09-02.

## [Closed] Design material visibility and design package revisions — 2026-09-02

### Will be done

- Add a dedicated read-only `material.view` capability for the Design role so designers can inspect the material catalog without receiving warehouse, stock, receipt, issue, transfer, adjustment, procurement or supplier-management authority.
- Add append-only project design-package revisions with draft/released/superseded lifecycle, categorized multi-file evidence, immutable released history and explicit actor/timestamp auditability.
- Let Design create drafts, add files and release a revision; let PM explicitly hand off the current released revision to QS; keep QS read-only and flag a newer release as requiring PM re-handoff.
- Preserve existing BOQ/procurement records and historical workflow evidence. A new design release must never silently replace an earlier handoff or mutate downstream commercial records.
- Add the approved material-only Warehouse presentation and the approved design-revision workspace inside relevant Preparation work items, including loading, error, empty and responsive states.
- Update migrations, seed parity, backend/frontend contracts, focused regression tests and all mandatory central/backend/frontend/admin/material/workflow maps.
- Verify formatting, focused/backend tests, migration manifest checks and the frontend production build. Browser/UAT remains out of scope because it was not requested.

### Done

- Client feedback was converted into a code-evidenced review package and Mockup as Text; the user explicitly approved implementation with `OK triển khai theo review package`.
- Loaded the mandatory module-memory bootstrap plus backend, frontend, admin-governance, materials-warehouse, workflow-orchestration and impact-analysis guardrails.
- Confirmed the implementation baseline has no pre-existing working-tree changes and the next custom migration version is V134.
- Added V134 plus seed parity for the dedicated `material.view` permission. The Design role can now read the material master only; Warehouse stock, movement, document, supplier-management and procurement authorities are unchanged.
- Added the project design-release domain with one editable draft, immutable released/superseded history, categorized protected files, explicit Design release and an append-only PM-to-QS handoff. A later release leaves the earlier handoff stale and requires PM to hand off again.
- Tightened `source.design_documents_ready`: after the first design release exists, readiness requires the exact current released revision to be the latest PM handoff. The implementation does not mutate BOQ, procurement or historical workflow evidence; projects with no design release retain the legacy evidence gate.
- Added the read-only material catalog presentation for Design and the design-revision workspace in Preparation and relevant work-item detail dialogs, including loading, error, empty, responsive, draft, release and PM handoff states.
- Added focused backend lifecycle/RBAC coverage and frontend presentation/navigation contracts; updated the central, backend, frontend, admin-governance, materials-warehouse and workflow-orchestration maps with the new owners and invariants.
- Current-working-tree static verification passed: uncached `go test -count=1 ./...`, `go vet ./...`, 294/294 frontend tests, TypeScript no-emit checking, Vite production build and `git diff --check`.
- `CONSFLOW_TEST_DATABASE_URL` is absent, so the new PostgreSQL-backed lifecycle test is registered but skipped; V134 has not been applied or runtime-smoke-tested against a database in this session.
- Status: `IMPLEMENTED`; non-database source, tests and production build are `TECHNICALLY VERIFIED`. Database-backed integration remains `BLOCKED` by the unavailable test database, and the UI is `NOT UX ACCEPTED` because browser/UAT was not requested and did not run.

## [Closed] Backend/frontend overloaded-file code split — 2026-09-01

### Will be done

- Work on branch `split-overloaded-files`, created from revision `4bcedc7`, and keep the refactor behavior-preserving: no API, schema, migration, route path, status, permission, SQL semantics or UI behavior changes.
- Split `handlers.go`, `types.go` and the frontend domain type registry first; keep Go receivers/signatures and a TypeScript compatibility barrel so existing consumers remain source-compatible.
- Split `routes.go` by bounded route groups without changing registration order, middleware nesting, concurrency classification or handler binding.
- Split `repository_domain.go` one bounded context at a time while preserving method bodies, transaction boundaries, SQL text, audit/activity/outbox writes and soft-delete filters.
- Split the largest React containers through pure helpers, hooks and focused components while preserving query keys, invalidation families, URL state, action readiness and rendered behavior.
- Split Workflow Definition Excel, Project Excel and report owners last because their hashing, idempotency, transaction and export-parity contracts are higher risk.
- Update the central, backend, frontend and affected domain maps for every moved owner; add no database migration.
- Verify baseline and final declaration/route parity, Go formatting/tests, frontend tests/build, Pipeline Builder checks, Docker restart/startup migrations and local health/readiness. Browser/UAT remains out of scope unless explicitly requested.

### Done

- Created PR #2 from `split-overloaded-files` into `main`. Its first GitHub `validate / Validate` run exposed three DB-backed integration failures in unchanged accounting/RBAC files that local non-gated tests had skipped: one PostgreSQL polymorphic parameter in the Accounting observer fixture and two stale Admin/Developer catalog assumptions about the Accounting-only `project.reconciliation.view` marker. Merge was blocked until the approved narrow corrections passed the full PR pipeline.
- Approved pre-merge remediation keeps the product contract unchanged: cast the fixture source code parameter to `text`, exclude the Accounting-only marker from Admin bootstrap completeness and fresh-production Admin/Developer completeness expectations, and assert that exactly the Accounting role receives the marker. No migration, seed, API, UI or runtime permission grant changes are introduced.
- Implemented the approved remediation and verified it against isolated temporary PostgreSQL database `consflow_pr2_validation_20260901`: all three previously failing focused tests passed, then the exact CI backend command `go test -p 1 ./...` passed across core/shared packages. `gofmt` check, `go vet` for the touched packages, all 133 migration checksum/order checks and `git diff --check` also passed. The temporary database was dropped after validation; the existing local application database was not modified.
- Committed the remediation as `364636c`, pushed it to PR #2 and obtained green GitHub `validate / Validate` plus GitGuardian checks. GitHub validated backend DB tests, release/operator tooling, frontend tests/build, the production Caddy image, bundle budget and whitespace before merging PR #2 into `main` as merge commit `fb92ce8`; local `main` was then aligned to `origin/main`. The feature branch remains available for history comparison.
- User approved the review package and the proposed safe split order.
- Created branch `split-overloaded-files` from clean revision `4bcedc7`; the initially proposed slash name could not be created inside the sandbox, so the approved no-`codex` fallback name is used.
- Corrected the accidental earlier push of `4bcedc7` to `main`: first anchored that commit at `origin/split-overloaded-files`, then moved `origin/main` back to `cbbdfb6` with an exact `--force-with-lease` and aligned local `main` to the same revision using a ref-only branch update. No checkout/reset ran; the existing unstaged split worktree was preserved byte-for-byte across the ref correction.
- Re-ran the pre-commit integrity review after restoring `main`: local/remote `main` remained at `cbbdfb6`, local/remote `split-overloaded-files` remained at `4bcedc7`, and `main` was the direct ancestor. Exact Go AST parity passed for all 484 service, 407 handler, 133 shared-type, 482 domain-repository, 55 Workflow Definition, 36 Project package and 32 report declarations; all 458 routes retained their original order/hash; the frontend retained all 215 public domain-type exports with the same checksum and no duplicate name. Current `go test -count=1 ./...`, `go vet ./...`, server build, 292/292 frontend tests, TypeScript/Vite production build, Pipeline Builder regression check, 250 KiB bundle budget and `git diff --check` all passed.
- Loaded the mandatory central/backend/frontend mapping and cross-domain impact guardrails; confirmed this is a physical ownership refactor with no intended product behavior or contract change.
- Split the core handler, route, shared-type and domain-repository monoliths into bounded files in the same `core` package. Route parity is 458 registrations in the original order; AST parity confirms 407 handler, 133 type and 482 domain-repository declarations are unchanged.
- Kept `routes.go`, `handlers.go`, `types.go` and `repository_domain.go` as compatibility/foundation entry points. No receiver, signature, SQL body, transaction, audit/activity/outbox write, route path, middleware nesting, status, permission or response contract was intentionally changed.
- Split the frontend domain registry behind the existing `lib/types/domain.ts` barrel and updated source-contract tests to load the bounded modules in the original declaration order.
- Extracted pure domain/presentation owners from Pipeline Builder, Workflow Form structured lists, Work Item detail, My Work, Contracts, Workflow Forms administration and Activity Thread while leaving queries, mutations, URL state and invalidation in their existing page/container owners.
- Split Workflow Definition and Project XLSX/JSON processing into contract, service, validation, repository and export files; split report SQL into Dashboard/options, cashflow, tax and profit owners. AST parity confirms 55 Workflow Definition, 36 Project package and 32 report declarations are unchanged.
- Updated the central/backend/frontend/root-cause maps and all affected business-domain maps with the new physical owners. No migration or seed change is required because persistence and business behavior are unchanged.
- Final completed-working-tree verification passed: route parity (458 registrations, original order/hash); AST parity for handlers (407), shared types (133), domain repository (482), Workflow Definition package (55), Project package (36) and reports (32); `gofmt -d`; uncached `go test -count=1 ./...`; `go vet ./...`; server build; 292/292 frontend tests; TypeScript/Vite production build (1,896 modules); and `git diff --check`.
- `make restart` completed the required `down -> build -> run` pipeline after rerunning outside the sandbox for Docker Buildx metadata access. Startup reported storage ready and migrations `applied=0 skipped=133`; PostgreSQL, Redis, Redis cache, backend and admin remained running with restart count 0. PostgreSQL/Redis health checks were healthy and backend `/api/health`, `/api/ready` plus admin root returned HTTP 200; no panic/fatal/error log was observed after startup.
- `CONSFLOW_TEST_DATABASE_URL` is absent, so isolated destructive/database-backed and Golden tests did not run; the core JSON test stream reported 155 skip events and zero fail events. The Docker smoke proves compatibility with the existing local database/migration state, not isolated-database acceptance.
- Status: `TECHNICALLY VERIFIED` for move parity, compile/tests/build, Docker image build, startup migration compatibility and local health/readiness. `NOT UX ACCEPTED` because browser/UAT was not requested and did not run.

## [Closed] Backend core service code split — 2026-09-01

### Will be done

- Split `backend/internal/core/service.go` by bounded context while keeping every declaration in `package core` and preserving the existing `Service` type, method signatures, route/handler contracts, repository call order, transaction boundaries, status values and error behavior.
- Keep this action move-only: do not introduce subpackages, new service abstractions, API/schema/migration/frontend changes or unrelated refactors.
- Move low-coupling administration/project/catalog/audit groups first, domain ledgers second, and workflow/work-item/form/activity/storage orchestration last, with formatting and test checkpoints between phases.
- Update the central module map, backend function map and affected domain maps so service owner paths remain durable and accurate.
- Verify declaration parity, backend formatting/build/tests and DB-backed integration/Golden validation when an isolated PostgreSQL test environment is available.

### Done

- Review package approved by the user with explicit confirmation to implement.
- Baseline source revision is `cbbdfb6` with a clean working tree; `backend/internal/core/service.go` contains 12,060 lines and 469 function declarations.
- Baseline `go test ./...` passed on Go 1.25.0 for the current working tree.
- Split the original 484 top-level declarations into 28 bounded owner files in the existing `core` package. `service.go` is now 180 lines and retains only `Service`, `NewService`, shared limits, and runtime/upload/storage/auth/recovery configuration.
- Kept cross-package helpers `cleanOptionalText`, `anySlice`, and `statusIn` in `service_helpers.go`; no new service type, subpackage, route, handler contract, repository call, schema, migration, frontend behavior, or runtime initialization hook was introduced.
- Compared the split against `HEAD:backend/internal/core/service.go`: all 484 declarations were found exactly once and their declaration text was unchanged. The moved package variables are literal lookup maps and there are no moved `init` functions, so file-order initialization introduces no new dependency.
- Updated the central module map, backend function map, root-cause triage map, and all eight affected business-domain maps with the new service ownership paths.
- Current uncached `go test -count=1 ./...` passed on Go 1.25.0; `go vet ./...`, `go build -o /private/tmp/consflow-server ./cmd/server`, and `git diff --check` also passed. The core suite reports the same 155 skipped tests as baseline and zero failed test events.
- A later approved `make restart` completed the required `down -> build -> run` pipeline against this working tree. After about 40 seconds, `admin`, `backend`, PostgreSQL, Redis and Redis cache remained running with restart count 0; PostgreSQL/Redis services were healthy, backend `/api/health` and `/api/ready` returned HTTP 200, and admin returned HTTP 200. Backend startup reported storage ready and migrations `applied=0 skipped=133` with no startup panic/fatal/error observed.
- Status: `TECHNICALLY VERIFIED` for declaration parity, formatting, compile, non-gated backend tests, Docker image build, startup migration compatibility and local HTTP health/readiness. Destructive DB-backed integration/Golden suites were not run against the existing local data volume, and browser/UAT was not requested.

## [Closed] Global Docker CLI PATH for zsh — 2026-08-31

### Will be done

- Add a user-global zsh startup file so `/usr/local/bin/docker` is available from login, non-login, interactive and non-interactive zsh sessions in every repository.
- Keep the existing `.zprofile` and `.zshrc` unchanged and avoid changing Docker Desktop, Compose or Consflow source/configuration.
- Verify Docker resolution from a minimal inherited PATH, then verify Docker Client/Server and the Consflow Make dry-run.

### Done

- Review package approved by the user.
- Confirmed the ChatGPT/Codex parent process was launched by macOS with `PATH=/usr/bin:/bin:/usr/sbin:/sbin`, while Docker CLI is installed at `/usr/local/bin/docker`.
- Confirmed sourcing `.zprofile` repairs only the current shell because the affected integrated terminal starts a non-login zsh session.
- Created `/Users/khiemnguyen/.zshenv` with a zsh-native unique PATH entry for `/usr/local/bin`; existing `.zprofile` and `.zshrc` timestamps remain unchanged.
- Verified syntax plus clean-environment non-interactive, interactive non-login and login zsh sessions; every mode resolved Docker to `/usr/local/bin/docker` from an initial system-only PATH.
- Verified Docker Client/Server `29.7.2`, `docker compose --env-file .env config --quiet`, and the Consflow `make -n down` expansion from the same clean GUI-style PATH.
- Status: `TECHNICALLY VERIFIED`; no container restart, migration, backend/frontend build or browser/UAT was needed for this user-level shell configuration fix.

## [Closed] Accounting company-wide reconciliation visibility — 2026-08-31

### Will be done

- Add a dedicated company-wide accounting reconciliation observation capability without weakening existing mutation permissions or project/work-item action gates.
- Let Accounting inspect every non-cancelled project, its runtime workflow, source records and Dashboard financial aggregates across the company while keeping Staff, Roles, Settings, Audit/Event Logs and technical Workflow Definition authoring out of the Accounting navigation.
- Add a focused Accounting lane to `Bàn làm việc` for actionable Accounting Handoff dossiers; do not mix company-wide read-only workflow records into the personal action queue.
- Preserve every existing Accounting permission and Accounting Handoff transition; this change is additive only.
- Add focused backend/frontend/migration regression coverage, update role-based UAT and mandatory module/domain maps, then run backend formatting/tests and the frontend production build without browser/UAT.

### Done

- Client review package and Mockup as Text approved.
- Confirmed company-wide visibility scope, business-menu boundary, Dashboard/source-data requirement, Accounting workbench priority and additive-only permission policy.
- Added V133 and seed parity for the Accounting company reconciliation observer plus the missing business read grants.
- Split all-company observation from mutation authority in project/work-item access; retained company scope only for explicit Accounting-owned actions.
- Added the company-scoped Accounting Handoff queue and navigation summary count for `Bàn làm việc`.
- Added `Nghiệp vụ Kế toán` with project/search/status filters, source links and direct acknowledge/return using the shared dialog.
- Added the Accounting observer banner and excluded Pipeline/Form configuration, Staff, Role/Permission, Audit/Event Logs and Settings from navigation/direct routes; global/project Audit APIs also reject the observer marker without deleting the historical role grant.
- Dashboard finance visibility now recognizes the reconciliation marker while retaining the canonical approved-payment source.
- Added backend/frontend regression contracts and updated module/domain maps plus role-based UAT coverage.
- Final current-working-tree checks passed: `go test ./...`, 292/292 frontend tests, TypeScript no-emit check, Vite production build, all 133 migration manifests/checksums, eight Golden PDF fixtures and `git diff --check`.
- PostgreSQL-backed `TestAccountingReconciliationObserverSeesCompanyScopeButKeepsActionBoundaries` compiles and is registered, but the explicit run was skipped because `CONSFLOW_TEST_DATABASE_URL` is absent; Docker is not running and local PostgreSQL is not reachable. V133 has therefore not been applied or runtime-smoke-tested in this session.
- Status: `IMPLEMENTED`; source/build/static migration gates are `TECHNICALLY VERIFIED`. Database-backed integration remains `BLOCKED` by the unavailable test database, and the UI is `NOT UX ACCEPTED` because browser/UAT was not requested and did not run.

## [Closed] Release v2026-08-27.2

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
- Added and pushed the release-tracking commit, producing final release revision `e19ed813f15ae66061de181184c037ba5cdf888d`; local `main` and remote `origin/main` matched that revision at tag creation, and the annotated release tag remains pinned to it.
- Local CI-equivalent validation passed on the final release revision: backend format, all 132 migration checks, serial `go test -p 1 ./...` on fresh PostgreSQL 16/Redis 7, release-tag tests, production operator tooling, 289/289 frontend tests, frontend production build, 250 KiB gzip bundle budget, production Caddy image validation and whitespace checks.
- The first local `npm ci` attempt was blocked by root-owned files in the global npm cache; the same locked dependency install and all frontend gates passed with a dedicated temporary cache. npm reported four non-gating dependency audit findings: one moderate and three high.
- `make production-check` passed before release; the temporary PostgreSQL/Redis validation containers were stopped and automatically removed afterward.
- Created and pushed annotated tag `v2026-08-27.2` from final revision `e19ed813f15ae66061de181184c037ba5cdf888d`.
- GitHub Actions run `33046528516` completed successfully: tag/ancestry validation, the reusable validation job, digest-pinned backend/admin GHCR image builds, the fail-closed VPS deployment and public HTTPS verification all passed. GitHub emitted non-blocking Node 20 deprecation annotations for third-party actions running under Node 24.
- Production migration reported `applied=1 skipped=131` and applied V132 `warehouse corrections`; Admin bootstrap returned `already_exists`, the new backend was healthy, the new Admin Caddy container started and the release script recorded `deployed release v2026-08-27.2`.
- Independent post-deploy checks passed: HTTP redirected to HTTPS with 308; `/gateway-health` returned 204; `/api/ready` returned 200 with database `ok`; `/`, `/warehouse` and `/core-data/input` returned 200 over valid TLS.
- Status: `TECHNICALLY VERIFIED`; `NOT UX ACCEPTED` because browser/UAT was explicitly out of scope.

## [Closed] Global Warehouse navigation — 2026-08-27

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

## [Closed] Partner catalog tabs — 2026-08-27

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

## [Closed] Release v2026-08-27.1

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
