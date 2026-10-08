# [Closed] All active Consflow plans

Closed on 2026-10-08 (Asia/Ho_Chi_Minh) at the user's request: close all plans, commit, push.

Closure archives the six approved work scopes and their recorded evidence. Browser/UAT remains NOT_RUN and NOT UX ACCEPTED. No production deployment or database operation is part of closure. Database counts, migration counts and build fingerprints below are dated historical snapshots, not current runtime claims. Existing dates and test limitations are preserved.

Closure verification on 2026-10-08: current dirty working tree based on HEAD `0c111ac`, native local environment; `npm test` PASS 512/512 (2.959s), `npm run build` PASS TypeScript/Vite (Vite 4.34s), native `go test ./... -count=1`, `go vet ./...`, `go build ./...` and diff checks PASS. No database/Redis integration environment was configured for the closure test run; those tests were skipped, and earlier isolated integration results below remain historical evidence. No migration, seed, runtime restart, Browser/UAT or production action ran. Logs are in `/private/tmp/consflow-close-plans-20261008-*`.


## [Closed] Accounting reconciliation presentation regressions — 2026-10-06

Status: IMPLEMENTED and TECHNICALLY VERIFIED on the current local working tree based on HEAD `0c111ac`. Browser/UAT NOT_RUN, NOT UX ACCEPTED. User approved fixing the two P2 findings from the preceding read-only comparison review.

### Done

- Added render regressions and observed both failing on the previous source state.
- `AccountingOperationsDialog` now filters `Có chênh lệch` by nonzero signed difference, independently of review status or allocated amount. Fully matched dossiers stay excluded; unmatched, partial and excess dossiers stay visible without inferring payment status.
- MISA balance rows use `Số dư phải thu` / `Số dư phải trả`; actual transaction rows keep `Thu` / `Chi`. Existing Dialog, WorkspaceTabs and Money components are reused.
- Targeted accounting render tests PASS 15/15 after the fix. Full `cd frontend && npm test` PASS 512/512 (2.611s); `npm run build` PASS TypeScript and Vite (Vite 4.53s) after the final source edits. `git diff --check` PASS. Logs: `/private/tmp/consflow-accounting-p2-tests.log` and `/private/tmp/consflow-accounting-p2-build.log`.
- Central/frontend/triage/accounting domain mappings updated with the filter and label invariants. Existing unrelated working-tree edits were preserved.

### Remaining work and verification limits

- No remaining approved implementation or technical checks for these two P2 fixes.
- No API, schema, migration or data change. Browser verification was not authorized in this turn.

## [Closed] Local RBAC fixture cleanup — 2026-10-05

Status: IMPLEMENTED and TECHNICALLY VERIFIED on local database and current working tree. Browser/UAT NOT_RUN, NOT UX ACCEPTED. User approved cleanup review for local only; production was not accessed.

### Approved scope

- Delete exactly the 18 inventoried `batch2_viewer_<8hex>` role UUIDs after backup/reference checks; preserve 19 canonical roles and existing account/project/member/permission/Pipeline/Form records.
- Fix the faulty Project Operations test to remove both owned role fixtures and run against an isolated schema. Custom fixture roles must not be labelled canonical system roles.
- Preserve the current Startup Wizard/Project Members shared UI and API behavior; removing fixture data clears both choice lists. No name-based frontend blacklist, production change or migration is required.

### Done

- Read-only local diagnosis: 37 roles, including 18 Batch2 fixtures marked `system_role=true` with zero role grants or user assignments; no references in checked FK, membership or definition/runtime JSON surfaces. Local database has 151 applied migrations and currently contains real newly imported staff/project data.
- Identified missing cleanup in `TestProjectOperationsAccessPolicy` and all-true metadata in shared `ensureRole`; role-choice API surfaces amplify persistent fixture rows.
- Updated this test only to use an isolated Project Operations schema, custom-role metadata and checked cleanup of both fixtures. Extracted the existing Accounting isolation mechanism into a shared test-only helper, retaining its relation guard and wrapper behavior.
- Targeted integration command `go test ./internal/core -run '^(TestProjectOperationsFixtureIsolation|TestProjectOperationsAccessPolicy|TestAccountingOperationsBasisEvidenceAndImmutableSnapshot)$' -count=3 -v` PASS in 31.882s on a disposable PostgreSQL database. All three repetitions passed, including two nested policy runs per isolation regression and the Accounting wrapper consumer. The owned test database was removed.
- Created owner-only, Git-ignored backup `tmp/role-cleanup-20261005T090626Z/before.dump` (1,065,557 bytes); restored it with `pg_restore --exit-on-error` into a separate database, rehearsed the exact guarded deletion and removed the clone. Initial rehearsal reporting hit multiline JSON parsing after clone-only verification; no main deletion ran on that attempt. JSONB reporting corrected it; the second rehearsal and main transaction both passed.
- Main `consflow` transaction deleted exactly the 18 approved UUID/name pairs, after fresh all-public-table UUID/name/display reference checks. Locks and fail-closed guards prevented drift or unapproved cascading changes. All 139 other public table counts/content fingerprints plus the retained canonical-role fingerprint matched before/after (140 checks). Roles 37→19; users 10 and projects 1 unchanged; 151 applied migrations unchanged. No seed/reset/runtime migration ran.
- Authenticated Admin and Developer HTTP checks PASS for `/roles` and `/projects/7023f7e3-28f0-438e-84f9-3a08e652effb/members?include_inactive=true`: 19 canonical roles, 16 project access choices, 18 operational labels, no Batch2 fixture rows. Both accounts retained management access. Verification sessions were logged out and rejected with 401 afterward.
- Central/backend/frontend/triage/Admin/Planning mappings updated. Existing frontend `Dialog`, `CheckOption` and `appRoleChoices` remain the owners; no application/frontend code or API contract changed. Evidence: restricted `report.json` and `api-verification.json` in the same backup folder. Final database check: zero Batch2 roles and zero owned temporary databases.

### Remaining work and verification limits

- No remaining approved data cleanup or targeted technical verification. Tests compiled the core package against the current dirty working tree based on HEAD `0c111ac`; no application rebuild is required for these test-only source changes. Existing unrelated working-tree edits were preserved; no commit/push or plan closure requested.
- Browser not authorized; rendered choice lists/UX remain NOT UX ACCEPTED. The open dialog can retain previously cached options until refreshed. Do not run DB-backed test fixtures against the user database.

## [Closed] Internal accounting retirement and direct source reconciliation — 2026-10-05

Status: IMPLEMENTED and TECHNICALLY VERIFIED on current working tree, isolated PostgreSQL/Redis fixtures and Docker local. Browser/UAT NOT_RUN, NOT UX ACCEPTED. User approved “Chốt, triển khai” and “Duyệt, triển khai đối soát trực tiếp hồ sơ”. Production was not accessed.

Plan: [Approved retirement scope](2026-10-08__closed-accounting-legacy-retirement-plan.md). Mockup/flow: [Approved source reconciliation](../accounting-source-reconciliation-review.md).

### Approved scope

- Project management and operational reports; accounting only compares imported MISA receipts/payments with source dossiers.
- No construction gate by collected money. Remove obligations, installment proposals, advance recovery and retention-release actions; retain contractual source amounts/terms.
- Remove ledger/AP/AR/invoices/VAT/budgets, material-cost/cash-request workspace, money/tax reports and stale APIs/UI/catalog/seed. No default-book restoration or archive compatibility branch.
- Keep current source dossiers, protected evidence/shared viewers, operational approval/quality/stock invariants and Rev9 accounting-handoff close gate.
- Direct reconciliation uses evidence-confirmed immutable bases, basis_id, exact money, MISA-record cap, partial/excess basis coverage with signed difference, source revalidation, idempotency/concurrency/audit.

### Done

- Exact request ca963357-9e01-49b4-9ec9-f14a03797b0c failed at project INSERT, PostgreSQL23503 projects_accounting_book_fk; PRJ-78805 did not exist. V149 regression reproduces missing default book; V150 removes the full dependency.
- Retired code/routes/UI/reports/types/permissions/source links and two financial physical-handover criteria. CurrentRev9 operational/source gates remain; unknown actions fail closed.
- V150 drops31 tables with explicit RESTRICT, guards empty business/unexpected book/closed period data; withdraws10 obsolete templates while preserving definition contents. V151 removes proposals/events, renames bases and matching keys, adds same-project/cash-record caps and reason checks.
- Sample blueprintv5 supports book-independent draft/planning/completed; completed uses actual Rev9 source dossiers, real independent actor/audit/evidence and gate validation before commit. No fake cash/MISA/journal.
- Seed now master-only additive, no TRUNCATE/migration replay/directprojects;12 legacySQL fixtures removed,8PDF fixtures preserved.
- Source-only reconciliation detail cannot read stored handoff snapshot/actions without handoff.view; current source/files require procurement.view/contract.view. source_available=false makes current reviewed=false while history remains.
- Reused Dialog/FormDrawer/SimpleForm/Field/toast/CurrencyInput/EvidenceRecordViewerDialog and source adapters at current entrypoints. Updated central/detail/domain mappings.
- Current-source frontend510tests PASS and build PASS (Vite4.55s), including exact large-USD/history and negative-difference rendering; native backend make test, go vet ./... and go build ./... PASS. Migration full PostgreSQLsuite PASS59.562s;8sample DBtests PASS4.344s; current workflow/source/handoff/permission/startup/import tests passed in isolated fixtures.
- First full backend PostgreSQL/Redis run identified6 fixtures incompatible with retired tables/template statuses/checklist count and snapshot-time boundary; fixed preserving current business assertions. Second full run found shared-test-DB approver contamination; isolated the exact fixture and retained permission/count/name assertions. Final go test ./... -count=1 PASS on fresh retirement_acceptance PostgreSQL16/Redis7 DB: core188.242s, migrations85.192s, all packages passed. Earlier full runs were FAIL, not PASS. Evidence: /private/tmp/consflow-retirement-acceptance.log.
- Independent review found post-lock authorization waits and decimal JSON history risks; both fixed. Canonical RBAC/EvidenceDetail reads reuse held transaction via querySource/withReadTransaction, check active actor and exact permissions after eligibility locks, without extra pooled connections. Final13 project-lock and7 secondary-row-lock revocations plus pool1/recovery/largeUSD regressions PASS61.806s; independent secondary-lock/source-only verification PASS28.533s, no remaining reviewer blocker.
- Main preflight: Docker projectttcons, PostgreSQL16.13,149migrations,2users,0projects/runs. New owner-only ignored backup tmp/accounting-retirement-20261005/before.dump SHA256f6df5930a65b7c09e66ca90e89128e44695ccaf62ba6d9febb589816f49f4654.
- pg_restore --exit-on-error rehearsal succeeded; V150/V151 applied atomically on restored clone. All10 preserved identity/role/form/node/step/edge table counts/content fingerprints match main. Rev9 remains sole published template;79Forms/174versions/694fields retained.
- Stopped only backend/admin to seal final owner-only backup before-final.dump (1,298,731bytes), SHA2568bc63a706a9e67005386422bdfcf2fb79cfe48cd027f5bbd6f512c603d876e64. Restored this exact backup into a second dedicated clone, reapplied150/151 and verified preserved data and rollback INSERT; both owned clones were removed afterward. Backups retained, Git-ignored; no archive product introduced.
- Docker local make restart completed exact down → build → run on2026-10-05 around11:50 Asia/Ho_Chi_Minh. Linux Go1.23 server build3.3s; backend image sha256:c5012915685cdd6d89228838a5644be6bacbe67c3ece1ad090e3128aef79998e, /app/consflow SHA2561f28bd79bb5d0b9e74f675399f7b62c1ae659c588af5437086ce3f01e332da23. No relevant source changed after build.
- Custom startup runner reported applied2/skipped149; main now151migrations. V150 SHA25616ab00773de877874b60912c1f57750cae6b0a96811c71d5b81327ba6d90e8ab; V151 SHA25638ff8e9cf9a9cadff742785830ae5ab1ba54afa4bcbe5c82764f9d4dd81ed0f1. V1–V149 files unchanged.
- Main schema/catalog verification PASS; all10 preserved table counts/content fingerprints match backup.2users retained,0projects/runs, sole published Rev9;79Forms/174versions/694fields. Original PRJ-78805/name/client/budget/contract INSERT succeeds without a book, then ROLLBACK leaves0projects. This is database regression plus isolated startup/import service tests, not a browser click/UAT claim.
- Main /api/ready databaseOK and admin5173 HTTP200. Fresh configured local login/me confirms current reconciliation permissions and no retired grants. GETprojects/project-finance choices/templates and operational-only Dashboard contract PASS;6 selected retired endpoints return404. Verification login was logged out and its token returns401; no credential/token was logged. api-verification.json/restart.log/source-state.json are ignored local evidence.
- Final diffcheck PASS. Disposable PostgreSQL/Redis test containers stopped/removed; main admin/backend and three datastore services remain running. No seed/reset/real project creation/production operation ran on main.

### Remaining work

- No remaining implementation or non-browser technical checks in approved scope. Browser/UAT remains NOT_RUN and NOT UX ACCEPTED. No commit/push or plan closure requested.

### Verification limits

- Source state: baseHEAD0c111ac54fe03111ac0941bf695fd5e1333186d0 plus uncommitted retirement and existing login/Developer edits; application source/test fingerprint7be6218df60aaacb1cfc1ea110bd4d302d2c8e629f1bc54c0606da36eb643259. No user edits reverted.
- Integration environment: disposable PostgreSQL16/Redis7 loopback55437/55438, isolated fixture schemas and all151migrations. Not browser/UAT or production.
- Main Docker migration/build/readiness and selected authenticated API checks ran; populated main-project workflow/matching UX remains untested because main has0projects. Local backup contains database state only; physical storage retained.

## [Closed] Local login defaults — 2026-10-05

Status: TECHNICALLY VERIFIED on local development server; Browser NOT_RUN and NOT UX ACCEPTED. User approved the review and Mockup as Text for local only.

### Done

- Replaced stale hardcoded development credentials in `frontend/src/modules/auth/LoginPage.tsx` with optional local environment values guarded by `import.meta.env.DEV`; production and missing-config values are empty.
- Added environment types in `frontend/src/vite-env.d.ts` and updated central/frontend/Admin Governance mapping. Existing `AuthCard`, `PasswordInput`, submit, validation and responsive composition remain the owners.
- Configured approved defaults in Git-ignored `frontend/.env.development.local` with mode 0600; neither password nor its literal is in tracked source/maps/docs. Existing frontend Docker ignore rules also exclude `.env.*` from build context, and the production Dockerfile does not copy environment files.
- Verification source: `0c111ac` plus current LoginPage/env-typing edits; concurrent Developer support source/maps/plan edits were preserved. Local admin container ran `npm run build` after the final frontend edits: TypeScript check and Vite 6.4.3 production build passed (Vite build 6.27 seconds). No relevant frontend source changed after this build.
- `node --experimental-strip-types --test tests/authRecoveryPresentation.test.ts tests/mobileInputZoom.test.ts`: 5 tests passed. These are existing source/presentation checks, not rendered-browser acceptance.
- Evaluated the actual LoginPage default initializer with development config, production mode even with populated config, and missing config: configured local defaults passed; production and missing-config cases were empty. `loadEnv` confirmed development-only values and no production values.
- Scanned all 136 production output files without printing credentials: neither local default email nor password appears in the build artifacts.
- Restarted only `admin` via `docker compose restart admin`. HTTP 200 from the running Vite LoginPage module confirmed current development env values and updated default references; configured defaults authenticated through the backend API and the verification session was logged out. No database/account/role configuration was changed by this feature.
- `git diff --check` passed; all local services remained running. Reused the full existing `AuthCard` and `PasswordInput` composition, manual login submit and inline validation. No layout or responsive markup was changed.

### Remaining work

- No remaining implementation or non-browser technical checks in the approved scope. Rendered desktop/mobile verification was not requested and was not run; no UX acceptance claim. Production was not modified or deployed.

## [Closed] Developer company-wide project support — 2026-10-05

Status: IMPLEMENTED and TECHNICALLY VERIFIED on the current working tree and isolated PostgreSQL/Redis fixtures. Docker local rebuilt and ready; Browser/UAT NOT_RUN, NOT UX ACCEPTED. Production was not accessed.

### Approved scope

- Active internal, non-sample accounts with the global system `developer` role may access every non-cancelled project, including sample projects, without project membership.
- Project mutations retain their explicit permission and business-state checks. Developer identity is separate from Admin/Director/PM identity and does not grant assignment, Work Item execution or approval overrides.
- Preserve existing UI components, routes and account assignments; no production operation or migration is planned.

### Done

- Traced role catalog, project list/detail/update gates, operations/work-item visibility and Explorer cache scope. Identified the missing Developer scope classification and the cached-index rollout risk.
- Implemented `developerProjectSupportUserSQL` and separate `IsDeveloper` classification; aligned list/detail/update, operations snapshots, full Work Item/timeline/evidence/activity observation and permission-gated embedded Audit. Existing action/responsibility/approval overrides are unchanged; mixed Developer/Deputy observation retains sample scope.
- Explorer uses the same support predicate in its revision snapshot; index keys now include `BypassAccess`, preventing reuse of older restricted indexes without flushing shared row caches.
- Added `developer_project_access_test.go`: PostgreSQL-backed real/sample/cancelled project scope, actual list/detail/update handlers and persisted metadata, explicit permission removal, scoped role boundaries, unchanged Work Item execution/responsibility/decisions, audit permissions, mixed-role sample reads; Redis-backed old-key rollout, canonical system identity, warm-cache deactivation/deletion/revocation/restoration and cancellation.
- Updated central/backend/frontend/triage mapping and Admin Governance, Project Planning and Workflow domain maps. Frontend source and API routes/request shapes are unchanged; `visibility.scope=developer` fits the existing string-compatible operations response contract. Existing `ProjectsPage`, `ProjectWorkspaceLayout`, `FormDrawer` and `SimpleForm` remain the UI owners.
- Source verified: base HEAD `0c111ac` plus this uncommitted working-tree change. Existing database-cleanup plan edits were preserved. Native checks used Go 1.25.0 on macOS arm64 with writable `/private/tmp/consflow-developer-go-cache`.
- Current-source focused PostgreSQL/Redis run passed all four top-level tests: `go test ./internal/core -run '^(TestDeveloper.*|TestProjectExplorerRedisCacheInvalidatesByDatabaseRevision|TestDeputyExplorerCacheRevokesAndRestoresCompanyScope)$' -count=1 -v` with dedicated loopback-only PostgreSQL 16/Redis 7 containers. Isolated schemas applied all 149 existing migrations; no new migration was added. Eight existing DB-backed project scope/options/evidence, Deputy, Accounting and My Work responsibility regression tests also passed on a separate test database.
- `GOCACHE=/private/tmp/consflow-developer-go-cache go test ./...`, `go vet ./...` and `git diff --check` passed. The full native suite had no DB/Redis environment, so its integration tests were skipped; only the focused runs above provide DB/Redis evidence. Earlier fixture failures used invalid source/deletion/audit values and were corrected to obey existing constraints; final focused runs passed.
- Docker local sequence completed on 2026-10-05 at approximately 09:32 Asia/Ho_Chi_Minh: `make restart` completed down but sandbox blocked buildx cache writes; authorized `make build` then `make run` completed the remaining build/run sequence. Backend built with the existing Linux Go 1.23 image; no source changed afterward. Running backend image `sha256:3ece3071810bf6f5f9b42c2fe93b803d92e32093af79f324db10b78dd9884276`; `/app/consflow` SHA-256 `d6a7fd9bff268784e62fe4441f88941686cafbe0d003821994731556dcaa1eb9`.
- `GET http://127.0.0.1:8080/api/ready` returned HTTP 200, database OK. Running local PostgreSQL 16.13 retained 149 migration rows through V149, 2 users, 0 projects and active global Admin/Developer assignments. No seed or account assignment mutation ran on the main database. Temporary test containers were stopped/removed after verification.

### Remaining work and verification limits

- Browser/user UAT was not requested and did not run: NOT UX ACCEPTED. Main Docker local currently has no projects, so actual project visibility/edit scenarios are proven by isolated fixtures, not by populated main-database UAT.
- The focused handler fixture injects a test actor; it does not claim a fresh real-account login/browser acceptance. Existing authenticated route placement/session behavior is unchanged. Production rollout and plan closure were not requested.

## [Closed] Database cleanup and replacement support accounts — 2026-10-05

Status: TECHNICALLY VERIFIED on Docker local; authorized database operation finished. Browser verification was NOT_RUN; NOT UX ACCEPTED. Production was not accessed.

### Approved scope

- Remove business/runtime data and old accounts; preserve all Pipeline definitions and Workflow Form templates, versions and fields.
- Preserve schema/migration history, RBAC catalog and system configuration needed to operate the retained definitions.
- Replace the previous account retention list with exactly two new accounts: `admin@thuthiemcons.com` with canonical global `admin`, and `khiemnd777@gmail.com` with canonical global `developer`.
- Passwords were supplied by the user in chat. Do not put plaintext passwords, password hashes, tokens or recovery secrets in this plan, scripts, reports or Git.
- Retained definitions may clear nullable creator/publisher references to deleted accounts; preserve definition content, IDs, versions, statuses, hashes and links. Never attribute old authorship to a new account.
- Physical storage objects remain outside the database cleanup scope.

### Done

- Source/schema inspection at `0c111ac`; Docker local database `consflow`, PostgreSQL 16.13, 149 migrations through V149, 173 public tables.
- Read-only local snapshot: 2,151 users, 888 projects, 13 Pipeline templates, 79 Form templates, 174 Form versions, 694 Form fields, 173 runs, 3,473 Form instances, 4,248 work items.
- Confirmed nullable creator/publisher foreign keys and runtime immutable-signature/PDF triggers before execution. Constraints and triggers remained enabled throughout the operation.
- User approved cleanup plan and replacement identities, then explicitly confirmed Docker local only. Production is outside scope.
- Read-only preflight inspected all 173 public tables and fingerprinted the seven retained Pipeline/Form definition tables; neither replacement email exists locally.

- Execution on 2026-10-05 began at 09:11 Asia/Ho_Chi_Minh. Target was guarded by Compose project/service labels, database identity, an explicit 173-table manifest, and a no-other-connections check after stopping local admin/backend.
- Full custom-format `pg_dump` backup: `tmp/database-cleanup-20261005T021113Z/before.dump` (5,812,652 bytes, owner-only directory/files, ignored by Git). SHA-256: `7e34981c568f705985cb0113cec27902507dc6f7de0024103ea72d2f4286778a`.
- `pg_restore --exit-on-error` into `consflow_cleanup_verify_20261005` succeeded. Full row counts/content fingerprints matched for all 173 tables. The entire reset was rehearsed there before touching the target database; the verification database was removed afterward.
- Preserved 17 definition/configuration/history tables and reset 155 explicitly named tables using `TRUNCATE ... RESTART IDENTITY RESTRICT`. Cleared retained definition creator/publisher references before deleting all old users. Transaction-level assertions checked zero business data, unchanged retained fingerprints, and exactly two new accounts/staff/global role assignments before commit.
- Created new active non-sample identities with bcrypt cost 10 and no forced first-login password change: Admin `f5250196-91bb-4df6-9666-b318a7036b42`, Developer `d7f8888b-385b-4589-b008-a06c04dcbd47`. Canonical existing role grants were preserved: Admin 151 permissions, Developer 154 permissions; no role was elevated or renamed.
- Final data: 2 users, 2 staff, 2 global role assignments; 0 projects, runs, work items, Form instances, authentication sessions and recovery tokens. All old business/runtime/import/history data targeted by the manifest is empty. New cache revision/outbox metadata is infrastructure state for the two new identities.
- Preserved definitions: 13 Pipelines, 162 nodes, 507 steps, 152 edges; 79 Form templates, 174 versions, 694 fields. Definition/content fingerprints match the backup, excluding only the explicitly approved nullable creator/publisher references. Schema migration history remains 149 entries through V149; roles, permissions and settings are unchanged except nullable settings actor references to deleted users.
- Invalidated the dedicated local redis-cache; primary Redis was retained. `docker compose start backend admin` restarted the existing containers; readiness returned HTTP 200 with database OK. No application build or migration was needed or run.
- API checks passed for both new accounts: `POST /api/auth/login`, `GET /api/auth/me`, `GET /api/pipeline-templates`, `GET /api/workflow-form-templates`, `GET /api/projects`, `POST /api/auth/logout`; revoked access tokens were rejected with HTTP 401. Old local admin/developer email logins were rejected with HTTP 401. Deleted both revoked verification sessions afterward, leaving zero sessions.
- Detailed operator report: `tmp/database-cleanup-20261005T021113Z/report.json`. Physical storage objects are retained; no browser, production, seed, source-code change or migration execution occurred.

### Remaining work

- No remaining work in the approved local data-reset scope. Production changes and browser/user-based acceptance require separate requests. Plan closure has not been requested.

### Boundaries

- No application code or migration changes are required for this operator data operation; source/mapping semantics remain unchanged.
- Existing local data is evidence only for Docker local; it does not prove production state or matching production credentials.
- Database backup does not back up physical storage. Retain original storage objects for recovery.
