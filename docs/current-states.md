# Current States

## Active Plan: Chuẩn hóa minh chứng hạng mục bàn giao thi công

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-24)

### Will be done

- Remove the misleading editable `Bắt buộc minh chứng` field from the exact `client_construction_handoff_plan.inspection_items` workflow-form presentation and from the Planning handoff activation panel.
- Replace the control with one fixed explanation that every handed-off item requires at least one photo or document when the construction result is recorded, including narrow-screen presentation.
- Normalize draft/submit answers and every new/revised Planning-to-Construction activation to `evidence_required=true`; keep the request/DB field for compatibility while preventing a client-supplied `false` from weakening the evidence contract.
- Preserve published Form definitions, existing form/runtime references, immutable activated handoff snapshots and historical construction records. Do not edit V66, add a migration or backfill history.
- Omit the obsolete nested field from new workflow-form PDF presentation, add focused backend/frontend regression coverage, update central/backend/frontend/root-cause and Planning/Workflow/Construction maps, then run current-tree backend/frontend validation. Browser/UAT remains skipped unless separately authorized.

### Done

- User approved the review package and Mockup as Text with `Ok triển khai`, including always-required evidence, removal of both editable controls and no historical rewrite.
- Loaded the mandatory central/backend/frontend/root-cause, Project Planning, Workflow Orchestration and Construction Operations maps plus Backend/Frontend/Impact instructions; confirmed the exact form, activation, generated work-item/check and result-completion owners.
- Confirmed the root cause: the published nested field accepts `false` and activation copies it to metadata, while the canonical Operations completion UI and backend already require at least one result evidence for every construction check.
- Added an exact-template presentation compatibility rule for `client_construction_handoff_plan.inspection_items`: the workflow form omits the editable `evidence_required` column, shows one fixed always-required explanation and leaves the published field definition unchanged. The Planning activation dialog removes its checkbox, shows the same policy and submits normalized item/criterion values.
- Added backend fail-closed normalization at both workflow-form answer save/submit and Planning activation. A client-supplied `false` now becomes `true` in the handoff item, generated work-item requirement and construction-check criteria; the PDF presentation also omits the obsolete choice. Existing request/DB fields, applied V66, activated snapshots and historical construction records remain untouched, so no migration or backfill was added.
- Added focused frontend/backend/PDF and database-backed handoff regression coverage, then updated the central/backend/frontend/root-cause and Project Planning/Workflow Orchestration/Construction Operations maps with the fixed evidence invariant.
- Current working-tree validation passed: focused frontend 7/7; full frontend 251/251; full backend `go test ./...`; focused backend default/PDF tests; local PostgreSQL-backed Planning handoff integration; frontend TypeScript/production build; 250 KiB gzip and 141-chunk bundle budget; Go formatting and `git diff --check`.

### Skipped / remaining acceptance

- Browser/UAT is not authorized in this turn, so the interface remains `NOT UX ACCEPTED` until separately verified in-browser.

## Active Plan: Mở hồ sơ trực tiếp từ Kế hoạch công việc

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-24)

### Will be done

- Make the actionable `Tiếp tục hồ sơ` / `Xem hồ sơ` entry in `/projects/:id/planning` open the existing shared `WorkItemDetailDialog` owned by `ProjectTaskContext`; opening the dialog must not submit a form, complete a work item or bypass any server gate.
- Carry the explicit open intent in transient React Router state, consume it once, and keep plain `?task=` deep links/refreshes presentation-only without auto-opening the dialog.
- Normalize the Planning return path by removing stale `task` and `returnTo` parameters before creating the backend-owned `execution_target` URL, while preserving unrelated query parameters and the hash.
- Preserve state-specific queue labels/actions, backend-owned execution targets, permissions, form/evidence/approval requirements and the existing top `Kiểm tra điều kiện & chốt bước` re-entry action.
- Add focused regression coverage, update central/frontend/root-cause/Project Planning/Workflow Orchestration maps, then run frontend tests, production build and whitespace checks. Browser/UAT remains skipped unless separately authorized.

### Done

- User approved the diagnosis, proposed behavior and Mockup as Text with an explicit `OK triển khai`.
- Loaded the mandatory central/frontend/root-cause, Project Planning and Workflow Orchestration maps plus the Frontend Workflow instructions; confirmed `PlanningPage.tsx`, `ProjectTaskContext.tsx`, `workItemNavigation.ts` and `workItemExecutionTarget.ts` as the owning frontend files.
- Confirmed the current failure mode: the lower CTA navigates to the already-active Planning `execution_target` with the same `task`, while the shared dialog remains closed because only the top context button changes `detailOpen`.
- Added a work-item-scoped router-state intent for embedded Planning execution targets. `ProjectTaskContext` consumes the matching intent once, opens its existing shared `WorkItemDetailDialog` and immediately replaces history state without the intent; plain `?task=` links and refreshes remain context-only.
- Added Planning workspace return normalization that removes stale `task` and `returnTo` parameters while retaining unrelated query parameters and the hash. Domain targets, state-specific action labels, the top re-entry button and every server-owned permission/form/evidence/approval/completion gate remain unchanged.
- Added focused navigation/intent/wiring regression coverage and updated the central/frontend/root-cause, Project Planning and Workflow Orchestration maps. No backend/API/schema/migration/seed contract changed, so no backend mapping or migration update was required.
- Current working-tree frontend validation passed: focused navigation and execution-target tests 6/6, full frontend suite 248/248, production build, 250 KiB gzip/141-chunk budget and `git diff --check`.

### Skipped / remaining acceptance

- Browser/UAT is not authorized in this turn, so the interface remains `NOT UX ACCEPTED` until separately verified in-browser.

## Active Plan: Tối ưu Trao đổi cần phản hồi trên Bàn làm việc

Status: IMPLEMENTED AND FOCUSED TECHNICALLY VERIFIED; FULL DB SUITE HAS UNRELATED FIXTURE FAILURES; UX NOT ACCEPTED (approved by user on 2026-08-24)

### Will be done

- Refactor `/me/activity-attention` theo hai pha: chọn tối đa 120 ứng viên theo đúng phạm vi/quyền/thứ tự hiện tại, sau đó mới enrich attachments, replies, reactions, mentions và nhãn đối tượng.
- Chạy truy vấn attention trong read-only transaction với `SET LOCAL jit = off`, dùng chung helper với `/me/work-items`; không thay đổi cấu hình PostgreSQL, biến môi trường hay deployment.
- Thêm `GET /me/activity-attention/summary?project_id=...` trả `{count, has_more}`, với `has_more=true` khi vượt ngưỡng chi tiết 120.
- Giữ nguyên contract mảng của endpoint chi tiết, RBAC, project/client visibility, queue semantics, thứ tự và payload; không thêm migration/index nếu performance gate chưa chứng minh cần thiết.
- Frontend luôn tải summary nhẹ cho badge, chỉ tải chi tiết khi mở tab `Trao đổi cần phản hồi`; badge hiển thị `…`, `—` hoặc `120+` theo trạng thái.
- Xóa repository `ListMyWorkItems` legacy không có call site; bổ sung contract/integration/performance coverage và cập nhật central/backend/frontend/root-cause/Workflow/Project maps.
- Chạy backend/frontend tests, frontend build, performance gate và `git diff --check`. Browser/UAT tiếp tục bị bỏ qua nếu không được ủy quyền riêng.

### Done

- User approved the review package and Mockup as Text, including the summary endpoint, lazy detail loading, removal of the dead legacy query and the no-migration gate.
- Loaded the mandatory central/backend/frontend/root-cause, Workflow Orchestration and Project Planning maps plus the approved Consflow backend/frontend/impact instructions; confirmed My Work / Activity Attention owner files and preserved invariants.
- Audit baseline confirmed the current attention query is fast at today's volume but crosses PostgreSQL's JIT cost threshold as activity grows; forcing that threshold locally measured about 6.68x overhead. Other audited modules remained below their performance gates, and the legacy `ListMyWorkItems` query had no call site.
- Extracted shared `beginReadTransactionWithJITDisabled` in the repository layer and moved both My Work page reads and personal-attention reads onto read-only transactions with transaction-local `SET LOCAL jit=off`. Connection/server defaults remain unchanged after commit or rollback.
- Refactored personal attention into materialized candidate selection followed by bounded enrichment. Queue predicates, actor/project/work-item/client scope and existing ordering are resolved before attachments, conversation, reactions, mentions and target labels; the existing detail route still returns the same array payload and accepts the same queue/project/limit contract.
- Added `GET /me/activity-attention/summary?project_id=...`, sharing the detail authorization/scope path and returning a capped `{count,has_more}` probe over at most 121 candidates. Counts up to 120 are exact; more rows produce `120+` presentation without loading details.
- Updated My Work so the lightweight summary stays active for the tab badge while enriched detail is enabled only in attention mode. Loading/error/capped badge states are `…`, `—` and `120+`; SSE and mutation invalidation refresh both query families, and manual refresh fetches detail only for the active attention tab. Existing panel filters/cards and their loaded-row counts remain unchanged.
- Removed the uncalled legacy `Repository.ListMyWorkItems`; no active call site, route or response contract depended on it. Added backend candidate-query, summary/cap, RBAC/project-scope and transaction tests plus focused frontend source-contract coverage.
- The isolated 1k/5k/10k performance gate passed. At 10,000 activities, summary p95 was 37.279 ms and detail p95 was 30.181 ms against 150/200 ms budgets, so no index, migration, cache or manual configuration was added.
- Current working-tree validation passed for the relevant database-backed integration tests, read-transaction commit/rollback test, performance gate, full backend suite with optional DB tests skipped, frontend 242/242 tests, production build, 250 KiB/141-chunk budget and `git diff --check`.
- The sequential full backend run against the long-lived development database was not green because of five pre-existing fixture/state failures: four duplicate accounting codes and one missing official workflow edge. A fresh-schema run cleared those failures but the repository's sample-data integration tests require seeded Developer/sample sources that migrations alone do not provide; the attempted disposable schema was removed. These failures did not reproduce in the focused attention/transaction/performance coverage and remain outside this change.
- Updated central/backend/frontend/root-cause, Workflow Orchestration and Project Planning maps with the summary route, candidate-first execution, shared JIT helper, lazy detail UI and no-migration performance result.
- Rebuilt and restarted the local Docker stack through the required `down -> build -> run` sequence after the sandbox-only Docker Buildx activity-file denial was rerun with approved Docker access. PostgreSQL and both Redis services are healthy, backend readiness returns HTTP 200, and the rebuilt Admin frontend serves `/my-work` with HTTP 200.

### Skipped / remaining acceptance

- Browser/UAT is not authorized in this turn and will remain skipped; frontend work remains `NOT UX ACCEPTED` until separately verified in-browser.
- Production deployment was not requested and was not performed. Once this source state is released through the normal production pipeline, no manual database, JIT, environment or feature configuration is required.

## Active Plan: Tăng tốc Bàn làm việc bằng JIT cục bộ

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-24)

### Will be done

- Run the four `/me/work-items` page queries in one read-only transaction and apply `SET LOCAL jit = off` only to that transaction.
- Preserve the existing route, handler/service contract, actor/project scope, capability calculation, filters, totals, summary, facets, pagination and response payload.
- Add focused PostgreSQL-backed coverage proving the transaction is read-only, JIT is off inside it and the connection-level setting is restored automatically afterward.
- Update the backend/root-cause/workflow mapping with the durable query-execution contract.
- Run focused and full backend validation plus a current local timing comparison. No frontend, schema, migration, seed or manual environment configuration is in scope.

### Done

- User approved the diagnosis and review package after local evidence showed `/api/me/work-items` at 3.28-3.46 seconds and isolated `summary`/`facets` JIT compilation at about 2.65 seconds.
- A temporary read-only diagnostic proved service/repository timing changed from 2.93-3.21 seconds with JIT enabled to 38-42 ms with JIT disabled for the same actor and 2,395 work items; the temporary diagnostic source was removed.
- Loaded the mandatory central/backend/root-cause/Workflow Orchestration maps plus Backend Workflow and Diagnose instructions. Confirmed the owner is `backend/internal/core/my_work_repository.go`; no frontend/API/schema contract change is required.
- Implemented one read-only transaction for the four `/me/work-items` page reads and applied `SET LOCAL jit = off` only inside that transaction. The route, payload, actor/project scope, filters, capability calculation, totals, summary, facets and pagination remain unchanged; commit/rollback automatically restore the connection setting.
- Added PostgreSQL-backed coverage for commit and rollback paths, proving `transaction_read_only = on`, JIT is off only inside the transaction and the original connection setting is restored. The existing page-contract integration coverage also passed.
- Current working-tree verification passed: the focused database tests, the full backend suite using a workspace-local Go module cache and `git diff --check`. Post-change repository/service timing measured 96.9-114.7 ms for `need_action` and 44.7-46.4 ms for `all`, compared with the prior 2.93-3.21 seconds diagnostic baseline.
- Updated the central, backend, root-cause and Workflow Orchestration maps with the durable transaction-local JIT contract. No frontend, API contract, schema, migration, seed or environment configuration changed.
- Rebuilt and restarted the full local Docker stack through the required `down -> build -> run` sequence. PostgreSQL/Redis are healthy, all services are running and the rebuilt backend readiness endpoint returns HTTP 200. Production has not been deployed; once this source version is released through the normal production pipeline, no manual JIT/environment/database configuration is required.

### Skipped / remaining acceptance

- Browser/UAT was not authorized in this turn and remains skipped; this backend-only performance change is `NOT UX ACCEPTED` until separately verified in-browser.
- Production deployment was not requested and was not performed.

## Active Plan: Import/Export định nghĩa Pipeline và Form/Fields bằng Excel

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-23)

### Will be done

- Add versioned multi-sheet Workflow Definition workbooks for `form`, `pipeline`, and dependency-complete `bundle` scopes. Relationships use portable package keys, permission keys and role names; source UUIDs remain provenance only.
- Add protected template, preview, execute, and export endpoints with authoritative server validation, HMAC preview binding, reference fingerprinting, same-actor idempotency and one atomic draft-only import transaction.
- Add `workflow_definition.import` and `workflow_definition.export`, a secret-free import batch table, portable Form definition hashes, and preserve published/runtime immutability. Imported definitions never publish, start runs, create work items or create form instances.
- Block execution on an existing draft conflict or unresolved required Form/permission/role; allow empty-system Form/bundle bootstrap and keep export independent from global Project-import readiness.
- Harden Pipeline clone engine-version preservation, Form schema/validation preservation and Pipeline/Form UI permission parity.
- Add responsive shared Import/Export dialogs to Pipeline Builder and Workflow Forms, parse `.xlsx` locally in a Worker, expose blockers/mappings/warnings, and require explicit draft-only confirmation.
- Update central/backend/frontend and Workflow/Admin/Project maps; run focused/full backend tests, frontend tests/build and whitespace checks. Browser/UAT remains skipped unless explicitly authorized.

### Done

- User approved the review package, Excel transport with canonical JSON semantics, full Form dependency export, draft-only/no-overwrite/no-auto-publish rules and the proposed RBAC boundary.
- Loaded the mandatory module-memory, Repo Architect, Impact Analysis, Backend, Database, Frontend, Workflow Orchestration, Admin Governance and Project Planning instructions; mapped the current Pipeline/Form/Project Excel owner routes, services, repositories, tables and UI surfaces.
- Confirmed the current source has Project Excel import/export and on-demand Pipeline PDF export, but no Pipeline/Form data workbook routes. Confirmed clone forces engine v2, Form UI can erase omitted schema/validation JSON, and current Pipeline/Form action gates drift from backend permissions.
- Added protected Workflow Definition template/preview/execute/export routes and a versioned 12-sheet workbook for `form`, `pipeline` and dependency-complete `bundle`. Portable package keys own internal relations; source IDs/hashes are provenance. Export can include the exact draft or published Form/Fields referenced by the selected Pipeline.
- Added authoritative server preview for package/sheet/graph/edge/surface/Form Field/role/permission references, external-only role rejection, existing draft conflicts, target lineage and exact published external Form refs. Execute revalidates a server-keyed HMAC/reference fingerprint and commits Forms before Pipelines in one same-actor UUID-idempotent transaction, with fail-closed per-entity/batch audit and no overwrite/publish/run/work-item/Form-instance side effects.
- Added V118 `workflow_definition.import|export`, Form version portable `definition_hash`, secret-free `workflow_definition_imports` replay state and database assignment allowlists. Added V119 to prevent Project/Workflow Excel grants surviving a role rename outside Admin/Developer import or Admin/Developer/PM export allowlists; repository role-permission replacement and seed grants match the same boundary.
- Hardened Form partial updates so omitted schema/validation/Fields are preserved, Form create/update/publish hash the current portable definition, and Pipeline clone preserves the source engine version. Pipeline/Form create/edit/publish/clone/import/export actions now mirror their backend permissions; published/non-editable Form fields remain read-only.
- Added shared responsive Workflow Definition Import/Export dialogs and a browser Worker that accepts `.xlsx` only, enforces the exact 12-sheet/version/header/formula/10 MB/row-count contract, sends structured rows, shows entity actions/blockers/warnings and requires explicit draft-only/no-runtime confirmation.
- Updated central/backend/frontend and Workflow Orchestration/Admin Governance/Project Planning maps. Project Excel readiness remains published-only, so imported drafts intentionally do not unblock project import/template until separately reviewed and published.
- Current working-tree technical verification passed: V118 and V119 applied to the local development database; a separate clean PostgreSQL database applied all 119 migrations, passed full sequential DB-backed `go test -p 1 ./...`, then accepted the full seed and produced exactly Admin/Developer import plus Admin/Developer/PM export grants before the temporary database was removed. Focused DB coverage proves the 12-sheet package, portable hashes, edge field ownership, bundle import, draft-only/no-runtime persistence, exact draft dependency export, conflict handling, Form partial-update preservation, clone engine preservation, idempotency, audit, repository/DB RBAC and role-rename fail-closed behavior. The ordinary full backend suite also passed. Frontend passed 234/234 tests, the current production build and the 250 KiB gzip/139-chunk budget.
- Rebuilt and restarted the local Docker stack through the required `down -> build -> run` sequence after a sandbox-only Docker Buildx activity-file denial was rerun with approved Docker access. PostgreSQL/Redis are healthy, the rebuilt backend reports database/service `ready`, and the rebuilt Admin frontend serves `/projects` with HTTP 200. This is a non-browser runtime smoke, not UX acceptance.

### Skipped / remaining acceptance

- Browser/UAT is not authorized in this turn, so the interface remains `NOT UX ACCEPTED`.

## Active Plan: Import/Export Công trình bằng Excel

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-22)

### Will be done

- Add `project.import` and `project.export` governance plus a secret-free, same-actor idempotent project-import batch record through custom migration V117 and seed parity.
- Add protected readiness, reference-backed template, authoritative preview/execute and project-scoped export endpoints. Import is create-only, max 100 rows, atomic, does not start Pipeline and does not create runtime/Form/member/business records.
- Generate one multi-sheet XLSX contract containing instructions, project rows, export metadata, published-ready Pipeline references, active published Form versions/Fields and active client partners.
- Block template download/import when published Pipeline or valid Form/Field reference data is missing; keep export available and mark incomplete historical references in the workbook.
- Add responsive Import/Export dialogs on `/projects`, browser-local `.xlsx` parsing in a Worker, formula/header/version/size/row checks, preview/error/success states and permission-aligned actions.
- Add focused backend/frontend/migration coverage, update central/backend/frontend and Project/Workflow/Reporting/Admin maps, then run non-browser technical verification. Browser/UAT remains skipped unless explicitly authorized.

### Done

- User approved the review package and Mockup as Text, including create-only/no-auto-start behavior, the seven-sheet workbook, read-only `contract_amount`, Admin/Developer import, scoped Admin/Developer/PM export and export-with-warning behavior when references are incomplete.
- Loaded the mandatory module-memory, Repo Architect, Impact Analysis, Backend, Frontend, Project Planning, Workflow Orchestration, Reporting Analytics and Admin Governance instructions and mapped owner files/routes/tables.
- Confirmed the current source has project CRUD/explorer, published Pipeline/Form governance, browser-local Staff import and single-sheet XLSX generation, but no project Excel routes, batch table, permissions or UI.
- Added custom migration V117 and seed parity for `project.import` (Admin/Developer), `project.export` (Admin/Developer/PM), database/repository assignment guards and secret-free same-actor `project_excel_imports` replay state.
- Added protected readiness/template/preview/execute/export routes before dynamic project routes. Preview uses exact versioned nine-column structured rows, current Pipeline/Form/Field/client references and a server-keyed hash; execute revalidates and atomically creates draft project masters plus fail-closed audits without starting Pipeline or creating runtime/work/Form/member/domain records.
- Added a reusable multi-sheet Open XML generator and the seven-sheet project contract: Huong_dan, Cong_trinh, Thong_tin_xuat, Ref_Pipeline, Ref_Bieu_mau, Ref_Fields and Ref_Chu_dau_tu. Export keeps current explorer search/status/sort or max-100 selection, reuses project access scope, excludes cancelled rows and exposes missing/runtime-mismatched historical references without blocking export. VND import/export values remain exact integer strings; contract amount is read-only snapshot metadata.
- Added permission-aware Import/Export actions and responsive shared dialogs on `/projects`. Import readiness blocks template/file/preview with direct Pipeline/Form update links, the browser Worker enforces `.xlsx`, version, exact header, formula, size and row contracts, server preview owns validity, and execute requires an explicit create-only/no-auto-start confirmation. Export supports current filters or selected rows and describes the complete reference workbook.
- Updated the central/backend/frontend and Project Planning/Workflow Orchestration/Reporting Analytics/Admin Governance maps; added backend unit/PostgreSQL integration/migration coverage and frontend source-contract coverage.
- Current working-tree technical verification passed: V117 applied to the local development database; migration manifest checked all 117 files; a clean temporary PostgreSQL database passed full `go test -p 1 ./...`; focused tests prove readiness, exact workbook package, preview/execute, atomic persistence, same-actor idempotency, no runtime/work/Form/member side effects, scoped export, export-concurrency classification and database RBAC; seed SQL ran on a separate clean database and produced exactly Admin/Developer import plus Admin/Developer/PM export grants; frontend passed 231/231 tests, production build and bundle budget; `git diff --check` passed. All temporary databases were removed.
- A first full backend run against the long-lived development database exposed five unrelated stale-fixture collisions. None reproduced on either clean database; the clean full suite is the authoritative verification environment for this change.
- Rebuilt and restarted the full local Docker stack through the required `down -> build -> run` sequence. PostgreSQL/Redis services are healthy, the rebuilt backend reports database/service `ready`, and the rebuilt Admin frontend serves `/projects` with HTTP 200. This is a non-browser runtime smoke, not UX acceptance.

### Skipped / remaining acceptance

- No browser/UAT is authorized in this turn, so the interface remains `NOT UX ACCEPTED` even if source checks, tests and builds pass.

## Active Plan: Import tài khoản nhân sự production

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-22)

### Will be done

- Add Developer-only `staff.import` governance, an atomic import-batch record, and `users.must_change_password` through custom migration V113 plus seed parity.
- Add authoritative preview and execute APIs for structured XLSX/CSV rows: at most 100 active staff/accounts, one allowed global role per row, canonical password/profile validation, all-or-nothing persistence, idempotency and fail-closed audit without storing raw files or secret fields.
- Require imported users to change their initial password after login before any other protected application action; keep the database session lifecycle authoritative and revoke sessions after the change.
- Add the Staff import dialog, browser-local XLSX/CSV parsing, current-role template download, preview/error/success states and permission-aligned action visibility.
- Add focused backend/frontend/migration coverage, update central/backend/frontend/Admin Governance maps, and run non-browser technical verification. Browser/UAT remains skipped unless explicitly authorized.

### Done

- User approved the review package and Mockup as Text, including client-side file parsing, server-side authoritative preview/execute, atomic activation, restricted-role handling and the recommended first-login password-change gate.
- Loaded the mandatory module-memory, Repo Architect, Impact Analysis, Admin Governance, Backend, Frontend and Database instructions and mapped the change to Staff / RBAC / account security.
- Confirmed current source has linked Staff-account creation, canonical password validation, DB-backed sessions and role assignment, but no import route/UI, no `staff.import` permission and no first-login password-change state.
- Added custom migration V113 with Developer-only `staff.import`, a database assignment trigger, `users.must_change_password` and secret-free/idempotent `staff_account_imports`; updated seed parity and Admin bootstrap completeness for the new Developer-reserved capability.
- Added protected preview/execute endpoints requiring all four approved permissions. Backend validation normalizes the exact eight-column contract, reports row errors without echoing passwords, rejects duplicate/existing accounts and restricted/external roles, binds valid previews to a server-keyed HMAC, then atomically creates active User/Staff/global-role/audit state with UUID idempotency.
- Added a DB-authoritative initial-password guard. Imported users may read `/auth/me` and change their own password, while every other protected action is denied until self-change or recovery reset clears the flag; password changes continue to revoke all sessions.
- Added the Staff Import action/dialog, browser-local Web Worker parsing for `.xlsx`/`.csv`, formula/header/size/row limits, dynamic Excel/CSV templates, current allowed-role sheet, masked/no-password preview, desktop table/mobile cards, explicit confirmation and success/error states.
- Updated central/backend/frontend/Admin Governance maps and added backend integration/middleware plus frontend source-contract coverage.
- Current working-tree verification passed: fresh PostgreSQL applied all migrations including V113; import integration tests covered preview masking, atomic persistence, activation, role assignment, same-actor-only idempotent replay, reserved RBAC and initial-password clearing; full backend `go test -p 1 ./...` passed; seed SQL ran successfully with `developer:1` and no other `staff.import` grant; frontend production build, all 221 tests, bundle budget, agent-artifact validation and `git diff --check` passed. The disposable test database was removed.

### Skipped / remaining acceptance

- No browser/UAT is authorized in this turn, so the interface will remain `NOT UX ACCEPTED` even if source checks, tests and builds pass.

## Active Plan: Ưu tiên Dashboard trong điều hướng Công việc

Status: TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-21)

### Will be done

- Place permission-filtered `Dashboard` before `Bàn làm việc` in the shared desktop/mobile AppShell navigation.
- Preserve `/my-work` as the authenticated landing page and preserve every existing route, permission, icon and active-state behavior.
- Add focused source-contract coverage for the order, update the central/frontend/Reporting Analytics maps and run non-browser frontend validation.

### Done

- User approved the review package and Mockup as Text.
- Confirmed `frontend/src/layouts/AppShell.tsx` is the shared desktop/mobile navigation owner and `frontend/src/app/router.tsx` independently keeps `/my-work` as the authenticated landing page.
- Loaded the mandatory central/frontend/Reporting Analytics maps and the Frontend UI guardrails.
- Moved the permission-filtered `Dashboard` entry before `Bàn làm việc` in the shared `navGroups` configuration without changing routes, permissions, icons or authenticated landing behavior.
- Added a focused source-contract assertion that both entries exist and `Dashboard` is registered first.
- Updated the central module map, frontend UI map and Reporting Analytics domain map to match the new navigation order.
- Current working-tree frontend state passes the focused AppShell test (2/2), the full frontend suite (212/212) and `npm run build`.

### Skipped / remaining acceptance

- Browser/UAT is not authorized in this turn and will remain skipped. Frontend technical checks will not establish UX acceptance.

## Active Plan: Tách danh mục Đầu vào và Đầu ra

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-21)

### Will be done

- Replace the single `Đối tác` navigation entry with independent `Đầu vào` and `Đầu ra` entries, backed by `/core-data/input` and `/core-data/output`; preserve `/core-data` as a redirect to `Đầu vào`.
- Reuse the partner catalog page with route-owned group configuration: `Đầu vào` contains partners with `client`; `Đầu ra` contains partners with any existing non-client type. A multi-type partner may appear in both groups.
- Add an additive server-side `/partners?group=input|output` filter while preserving existing status, exact-type, permission, active-row and downstream snapshot behavior.
- Keep the shared multi-select partner form, default new `Đầu vào` records to `client` and new `Đầu ra` records to `supplier`; keep the output-only type filter.
- Add focused backend/frontend regression coverage, update the central/frontend/backend/Admin Governance maps, and run non-browser technical validation.

### Done

- User approved the review package, route plan, multi-type membership semantics, output type filter and implementation scope.
- Mandatory central/frontend/backend/Admin Governance maps and Frontend, Backend, Impact Analysis and Git workflow instructions were loaded.
- Baseline `main` worktree is clean before implementation. Current code has one `/core-data` route/navigation item and the backend supports only one exact partner `type` filter.
- Replaced the single partner navigation entry with permission-equivalent `Đầu vào` and `Đầu ra` entries, added their keyed routes and kept `/core-data` as an authenticated redirect to input.
- Reused `CoreDataPage` through group-owned titles, descriptions, metrics, empty states and create defaults. Input fixes the list group to `client`; output exposes only the existing non-client type choices while the create/edit form remains multi-select.
- Added the validated additive backend `PartnerFilter.Group`, handler query parsing and PostgreSQL JSONB group conditions. Existing consumers with no group remain unchanged; multi-type partner membership is intentionally non-exclusive.
- Added PostgreSQL-backed coverage for active/inactive input, output, exact output type, multi-type membership and invalid group/type combinations; updated focused frontend source-contract coverage.
- Updated the central, frontend, backend and Admin Governance maps for the route, UI and API behavior.
- Current combined working-tree state passes the focused PostgreSQL partner-group test, backend `make test`, all 212 frontend tests, frontend production build and `git diff --check`. The dedicated temporary PostgreSQL test database was removed after the run.

### Skipped / remaining acceptance

- Browser/UAT is not authorized in this turn and will remain skipped. Frontend technical checks will not establish UX acceptance.
- No schema, migration or seed data changed, so migration/seed execution was not required.

## Active Plan: Canonical production bootstrap and operator commands

Status: PRODUCTION DEPLOYED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-20)

### Will be done

- Replace the one-off randomly named local secret bundle with an ignored, mode-0600 `deploy/config/production.env` created from a committed secret-safe schema.
- Add idempotent commands to bootstrap/generate secrets, render and validate the VPS payload, provision the VPS over pinned key-only SSH, configure only deploy-related GitHub secrets, and create an exact `vYYYY-MM-DD.N` release tag.
- Keep PostgreSQL/Redis/JWT/settings/S3/Admin runtime credentials off GitHub; provision them directly to `/opt/consflow/shared` while GitHub receives only VPS host/port/user/private-key/verified-known-hosts.
- Preserve GHCR digest-pinned releases, main-ancestry validation, no Actions trigger on push to `main`, Caddy Automatic HTTPS, one-shot migration and fail-closed Admin bootstrap.
- Migrate the already generated temporary secret material into the canonical ignored config without printing secrets, then validate permissions, placeholder gates, Compose rendering and shell contracts.
- Update the production runbook, README, central runtime map and root-cause triage map for the new operator workflow.

### Done

- Compared Noah's canonical `project.env`, GitHub-secret helper, VPS renderer/provisioner and tag helper with Consflow's current release design.
- User approved adopting Noah's compact operator experience while retaining Consflow's key-only pinned SSH, runtime-secret isolation, strict tag format and immutable GHCR digest deployment.
- Added ignored mode-`0600` canonical operator config and SSH material under `deploy/config`, with a committed secret-safe schema and shared parser that never evaluates config values as shell code.
- Added idempotent bootstrap, fail-closed validation, runtime payload rendering, explicit-scope GitHub secret upload, pinned key-only VPS provisioning and strict tag-release helpers behind concise Make targets.
- GitHub receives only five raw deploy-transport secrets; PostgreSQL, Redis, JWT, settings, Admin and S3 runtime credentials remain local/VPS-only and are never Base64-encoded into GitHub Secrets.
- Migrated the prior random temporary bundle byte-for-byte into the canonical ignored paths, verified file modes and ignore rules, then removed the exact old temporary directory.
- Added deterministic local production-tooling coverage to reusable validation. It exercises bootstrap idempotence, placeholder rejection, SSH key/fingerprint consistency, payload rendering and permissions, GitHub/VPS dry runs, Production Compose rendering and isolated annotated tag creation/push.
- Updated the production runbook, README, Makefile, CI validation step, central runtime map and root-cause triage map for the canonical operator workflow.
- Current approved source state passes Bash syntax, release-tag tests, production-tooling end-to-end tests, actionlint, generated-secret leak/ignored-path/mode checks, `git diff --check`, all five reconciled focused tests on separate clean databases, full `go test -p 1 ./...` on a sixth clean database, 212/212 frontend tests, frontend production build and bundle budget. `shellcheck` was unavailable and therefore skipped.
- The real canonical config has all provider-controlled values resolved, pins the user-confirmed ED25519 host fingerprint and passes `bash deploy/scripts/validate-production-config.sh --all` without printing secrets.
- Committed the implementation on `codex/production-deploy`, pushed it and opened draft PR [#1](https://github.com/khiemnd777/ttcons/pull/1). Follow-up CI and release reconciliation is committed through `c6030aa`; GitHub Actions run `32354371208` passed all backend, frontend, Caddy-image, bundle-budget, whitespace, release-tag and production-tooling checks.
- Remote CI at `529777e` failed on four stale invariant fixtures already failing on baseline `main` plus one accounting fixture newly exposed by V112. The approved reconciliation updates only test fixtures/expectations: Rev8 workspace counts, exact select-other validation, reserved seal-management actors, isolated V26 replay and reuse of V112's Admin role. All five now pass independently and in the full backend suite; no production gate was weakened or skipped.
- Replaced the long-lived VPS GHCR token with the release job's short-lived `GITHUB_TOKEN`. Provisioning no longer accepts or stores registry credentials; deployment sends the job token over SSH stdin into an ephemeral remote `DOCKER_CONFIG` and deletes it at shell exit. Static production tooling and actionlint cover this contract.
- Vietnix S3 credentials authenticate successfully to `tt-dnh-consflow-prod` with region `vn-hcm-1` and virtual-host addressing. The empty bucket initially granted anonymous listing; its ACL was changed to private, authenticated temporary put/get/delete passed with zero objects left behind, and anonymous access now returns HTTP `403`.
- Created GitHub Environment `production`, uploaded exactly five raw deploy-transport secrets (`VPS_HOST`, `VPS_PORT`, `VPS_USER`, `VPS_SSH_PRIVATE_KEY`, `VPS_SSH_KNOWN_HOSTS`) and limited deployments to tag pattern `v*`; the exact workflow validator still enforces `vYYYY-MM-DD.N` and main ancestry.
- GitHub rejected required-reviewer protection for this private repository with HTTP `422` because the current billing plan does not support it. No manual approval gate is active; a valid release tag will deploy automatically after validation unless the repository plan is upgraded and a reviewer is configured.
- Installed the dedicated public key through the rotated root password without logging or persisting the password, immediately cleared the clipboard, and verified pinned key-only SSH. The VPS is CentOS Stream 9; `tar` and Docker CE `29.7.2` plus Compose `v5.5.0` were installed from Docker's official RPM repository after matching GPG fingerprint `060A 61C5 1B55 8A7F 742B 77AA C52F EB6B 621E 9F35`.
- Provisioned `/opt/consflow/shared` through the canonical operator command. Docker is enabled/active, the VPS had approximately 55 GB free disk and 7.2 GiB available RAM at provision time, firewalld was inactive and SELinux was disabled.
- Merged PR [#1](https://github.com/khiemnd777/ttcons/pull/1) into `main` at `703ae825ae251df9a083ee6415cf3f13f33d7ee4`; push/merge to `main` triggered no deployment. Annotated tag `v2026-08-20.1` then triggered GitHub Actions run `32358994866`, whose tag validation, full validation, immutable backend/admin image build and VPS deploy all passed.
- Independent production smoke passed: HTTP `308` to HTTPS, TLS verification result `0`, root `200`, gateway health `204`, `/api/ready` reported service/database ready, and the production database recorded `112` migrations with numeric max version `112`. Let's Encrypt certificate SAN is `tt-dnh.com`, valid from 2026-08-20 through 2026-11-18.
- Verified the bootstrapped account can log in with role `Admin` and log out without printing the password or token. The VPS first-use bootstrap password source was then truncated to zero bytes, `BOOTSTRAP_ADMIN_PASSWORD_FILE` was set to `/dev/null`, and an image-local bootstrap rerun returned `status=already_exists` without reading/resetting the password.

### Skipped / remaining acceptance

- Required-reviewer protection is not active because the current GitHub billing plan rejected it; valid release tags deploy automatically after all workflow gates pass.
- No production rollback drill or disaster-recovery restore was executed. Database rollback remains explicitly manual/fail-closed.
- Browser/UAT was not run because the delivery change has no new UI behavior; production UI is therefore `NOT UX ACCEPTED` despite successful HTTPS/API/login smoke.
- The CentOS SSH server negotiated no post-quantum key exchange and the host has no swap configured. These are host-hardening/operations follow-ups, not blockers for the verified release.

## Active Plan: Tag-gated VPS production release, Caddy Auto HTTPS, S3 and Admin bootstrap

Status: PRODUCTION DEPLOYED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-20)

### Will be done

- Make Production Compose consume immutable backend/admin image references produced from an exact `vYYYY-MM-DD.N` release tag; a push to `main` must not trigger GitHub Actions.
- Run PR/manual validation separately, validate that every release tag is calendar-valid and points to a commit reachable from `main`, then build/push GHCR images and enter the GitHub `production` Environment before VPS deployment. The current repository plan cannot enforce required reviewers, so the deploy is automatic after validation until that protection becomes available.
- Change single-host Production Compose from managed-edge HTTP `:8080` to direct Caddy automatic HTTPS for `tt-dnh.com` on public TCP `80/443`, while preserving the reusable `:8080` application listener for Kubernetes and persisting Caddy certificate state.
- Pin the Vietnix production bucket contract to private bucket `tt-dnh-consflow-prod`, HTTPS endpoint `https://s3.vn-hcm-1.vietnix.cloud`, provider-confirmed region and startup read/write/delete preflight.
- Add a production-safe V112 RBAC catalog migration because fresh production migrations currently do not create the base roles/permissions held in demo seed data.
- Add an idempotent, fail-closed, non-HTTP Admin bootstrap command that reads its first-use password from a Docker secret, creates linked user/staff/global Admin assignment plus audit atomically, and never resets an existing Admin password.
- Add a single-host VPS deploy script and runbook covering host/bootstrap secrets, migration, Admin bootstrap, application rollout, DNS, HTTPS smoke checks and rollback boundaries.
- Update central/backend/root-cause/Admin governance maps for the new migration, command and production delivery contracts.
- Validate migration integrity and fresh-database RBAC, bootstrap behavior, backend tests/format, release-tag tests, workflow syntax, frontend build/Caddy validation, Production Compose shape and whitespace. Browser/UAT remains skipped because no UI behavior changes.

### Done

- User approved V112, fail-closed Admin bootstrap, direct Caddy Auto HTTPS, tag-only release with production approval, GHCR immutable images and Vietnix bucket name `tt-dnh-consflow-prod`.
- Mandatory central/backend/root-cause/Admin maps and Repo Architect, Impact Analysis, Docker Ops, Backend, Admin Governance and Git workflow instructions loaded.
- Baseline revision `27fb2be` on clean `main` confirmed. At that baseline, CI still ran on pushes to `main`; Production Compose built mutable local images and published Caddy HTTP `:8080`; Caddy certificate directories were tmpfs; fresh migrations depended on demo seed for the base Admin/RBAC catalog.
- Impact review found no frontend/API contract change. Required collision checks are migration checksums, reserved Admin/Developer permissions, external-only Client behavior, atomic audit, S3 startup preflight, Caddy/Kubernetes listener compatibility and production secret isolation.
- Added additive/idempotent V112 to create the production-safe 18-role/base-permission catalog without demo users or business seed data, preserve custom roles/assignments, reapply later quality/construction grants and enforce Client/Settings/Event Logs/professional-accounting boundaries.
- Added `consflow-bootstrap-admin` as a non-HTTP, serializable, advisory-locked one-shot command. It requires V112 and complete Admin grants, reads only a first-use password file, bcrypt-creates linked user/staff/global Admin plus audit atomically, fails closed on conflicting identity/Admin state and no-ops for the same healthy Admin without rereading or resetting the password.
- Production Compose now consumes required backend/admin image references, persists Caddy certificate/config named volumes, publishes only Caddy TCP 80/443, gives the unpublished backend a dedicated outbound network for S3/SMTP while data services stay internal-only, mounts AWS shared credentials as a Docker secret and fixes the S3 bucket to `tt-dnh-consflow-prod` with provider-confirmed Region pending.
- Caddy now selects `APP_DOMAIN=tt-dnh.com` for Automatic HTTPS while keeping default HTTP `:8080` for Kubernetes and a forced-HTTP loopback-only `:8081` Docker health listener. The image runs UID/GID 101, removes the inherited low-port file capability; Production Compose drops all capabilities and enables only the container-scoped unprivileged-low-port sysctl.
- Split reusable validation from CI. Pull requests/manual dispatch validate; push/merge to `main` has no workflow trigger. Only exact calendar-valid `vYYYY-MM-DD.N` tags reachable from `origin/main` build backend/admin GHCR images and pass digest references to the `production` Environment deploy job.
- Added fail-closed VPS deploy script: validate tag/digests/Compose/secrets, pull exact images, start dependencies, drain, run one-shot migration and Admin bootstrap, wait backend/Caddy, then record the successful release/current symlink; database rollback is never automatic.
- Added the authoritative Vietnamese VPS runbook including copy-ready DNS request, Vietnix bucket/key request, host directory ownership, secret layout, ephemeral GHCR authentication, GitHub Environment, release command, Admin first-use cleanup, HTTPS smoke checks and rollback boundary. Updated production/rollback docs, README and all matched mapping references.
- The approved test/invariant reconciliation and ephemeral GHCR authentication change pass local focused/full backend, frontend, workflow, release-tag, production-tooling and bundle checks on the current source state. GitHub Actions run `32354371208` passed the same PR validation surface on pushed revision `c6030aa`.
- Created GitHub Environment `production`, uploaded exactly the five deploy-transport secrets and set the custom deployment tag policy to `v*`. Required-reviewer configuration remains unavailable on the current private-repository billing plan; HTTP `422` was returned and no approval gate was applied.
- Final isolated Caddy smoke used the current built image as its default UID/GID 101 with read-only root, all Linux capabilities dropped and container-scoped low-port sysctl: local Automatic HTTPS returned 200 and loopback health returned 204. The public domain/ACME path was intentionally not exercised before DNS cutover.
- Provisioned the CentOS Stream 9 VPS over pinned key-only SSH, installed verified official Docker CE/Compose RPMs, and placed runtime secrets only under `/opt/consflow/shared`; the rotated root password was never placed in GitHub or a local file and the clipboard was cleared immediately after public-key installation.
- Merged the validated tree into `main` at `703ae825ae251df9a083ee6415cf3f13f33d7ee4`, then created annotated tag `v2026-08-20.1`. GitHub Actions run `32358994866` passed tag/main-ancestry validation, the full validation suite, digest-pinned GHCR builds, fail-closed VPS deployment and public HTTPS verification.
- Production `current` points to `/opt/consflow/releases/v2026-08-20.1`; PostgreSQL, both Redis services, backend and Caddy/admin became healthy. All `112` migrations are recorded through V112, Admin bootstrap completed, subsequent login/role/logout smoke passed, and the post-bootstrap password source was disabled with a verified `already_exists` no-op rerun.
- Independent public checks confirmed `http://tt-dnh.com` redirects with `308`, `https://tt-dnh.com/` returns `200`, `/gateway-health` returns `204`, `/api/ready` reports database `ok`, and the Let's Encrypt certificate covers `tt-dnh.com` through 2026-11-18.

### Skipped / remaining acceptance

- Required-reviewer protection remains unavailable on the current GitHub billing plan, so the production Environment does not provide a manual approval pause.
- No rollback drill or database restore was run; rollback remains an operator-controlled acceptance item.
- Browser/UAT is out of scope because there is no UI behavior change, so the production UI remains `NOT UX ACCEPTED`.
- Host-hardening follow-ups remain: evaluate an OpenSSH version/configuration with post-quantum key exchange and decide whether this 8 GiB single host should have swap.

## Active Plan: Migrate production origin gateway from Nginx to Caddy

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; PRODUCTION DEPLOYMENT NOT VERIFIED (approved by user on 2026-08-20)

### Will be done

- Replace the production `admin` image's Nginx gateway with pinned standard Caddy while preserving service name, port `8080`, SPA/static behavior, private backend routing and managed-edge TLS termination.
- Preserve `/gateway-health`, exact public `/metrics` deny, trusted-client-IP normalization, request IDs, security/cache headers, route-specific body limits, SSE streaming and controlled upstream-unavailable responses.
- Keep development on Vite; switch Production Compose backend rate limiting to `enforce` and retain Docker log rotation without adding non-standard Caddy modules.
- Simplify the Kubernetes admin pod by removing the Nginx-specific render init container while preserving non-root, read-only, probe, resource, affinity, NetworkPolicy and stable `backend:8080` contracts.
- Update focused tests, CI gateway validation, production/Kubernetes runbooks and the mandatory module/backend maps.
- Validate Caddy adaptation, production image build and smoke behavior, Production Compose shape, Kubernetes render assertions, focused backend tests and whitespace. Browser/UAT remains skipped because no UI behavior changes.

### Done

- Review package and three migration decisions approved: standard Caddy without third-party rate-limit modules; TLS remains at the managed edge; production/Kubernetes migrate while development remains on Vite and gateway access logs become JSON.
- Mandatory module-memory and Docker Ops instructions loaded; owner code, Compose, Kubernetes, observability test and operations documentation inspected.
- Baseline revision `2a178e7` was clean. Existing Kubernetes render assertions passed and focused observability tests passed with a sandbox-local Go cache.
- Replaced both Nginx configs and the production image stage with pinned `caddy:2.11.3-alpine`; image build now validates the reviewed `Caddyfile` and CI builds that production image.
- Preserved port `8080`, managed-edge TLS, strict trusted-proxy client-IP normalization, generated request IDs, route-specific body/timeouts, immediate SSE flushing, exact public `/metrics` deny, immutable assets, no-store SPA fallback and controlled JSON `503` responses.
- At that plan's verification revision, Production Compose ran Caddy as UID/GID `101:101` with a read-only root filesystem and owner-only bounded tmpfs for `/config`, `/data` and `/tmp`; the later active tag-gated VPS plan supersedes `/config` and `/data` with persistent named volumes for Automatic HTTPS certificate state. Backend rate limiting remains `enforce`; development Compose/Vite is unchanged.
- Kubernetes admin pods run the same image Caddyfile directly against stable `backend:8080`; the Nginx render init container and `SERVER_NAME` contract were removed while UID/GID/fsGroup, probes, resources, affinity, PDB, NetworkPolicy and bounded ephemeral storage remain asserted.
- Updated the focused observability contract test, Kubernetes deterministic render checks, production/Kubernetes operations documentation and all matched central/backend/root-cause maps.
- Final current-tree checks passed on 2026-08-20: Caddy adaptation; production image build; production and development Compose configuration; Kubernetes render/11-object deterministic contract; focused `go test ./internal/shared/observability`; and `git diff --check`.
- Final isolated Docker smoke used the built image as UID/GID 101 with a read-only root: `/gateway-health` returned `204`, exact `/metrics` returned `404`, SPA deep-link returned `200` with `no-store`, trusted forwarded IP/proto/request UUID reached a consuming mock backend, reviewed SSE emitted immediately, an oversized login body returned `413`, an unavailable upstream returned the JSON `503` envelope with `Retry-After: 5`, and access output was JSON without storage/runtime errors.

### Skipped / remaining acceptance

- No production host, managed edge/WAF policy or Kubernetes cluster was changed or exercised. Firewall allowlists, edge rate/connection quotas, real proxy CIDRs, rollout/drain behavior and cross-replica SSE remain production acceptance gates.
- Browser/UAT was not run because this migration changes no UI behavior. The production frontend compilation is technically verified, but no new UX acceptance claim is made.
- No database schema or migration changed, and no migration command was run for this gateway-only work.
- The production frontend dependency install reported the repository's existing npm audit warning (1 moderate and 3 high vulnerabilities); this migration did not attempt dependency remediation.

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
