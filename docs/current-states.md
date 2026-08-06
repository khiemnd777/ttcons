# Current States

## [Active] Responsive Shell Branding And Mobile Navigation

### Will be done

- Reuse the approved Thủ Thiêm Workflow brand lockup at the top of the Pipeline Builder and project-workspace desktop sidebars, with the same permission-aware `/` home behavior as the standard AppShell.
- Add an accessible mobile navigation drawer to the standard AppShell so the permission-filtered `Công việc`, `Workflow`, and `Quản trị` groups remain reachable below the desktop breakpoint.
- Preserve the existing mobile `Mở cây nghiệp vụ` / `Mở cây quy trình` project control and keep Pipeline Builder canvas responsiveness outside this scoped change.
- Add focused regression coverage, update central/frontend/root-cause mapping, and run non-browser frontend verification. Browser/UAT remains skipped unless explicitly requested in the same turn.

### Done

- Confirmed from the supplied 430×932 Edge screenshot and current source that the AppShell desktop sidebar is hidden below `lg` without any mobile menu trigger or replacement navigation.
- Confirmed Project Workspace already exposes a mobile tree toggle, while Pipeline Builder has a fixed desktop-first template sidebar; these are separate from the AppShell navigation defect.
- Produced the evidence-backed review package and Mockup as Text, then received explicit `OK triển khai` for the combined branding and mobile-navigation scope.
- Added shared `BrandHomeLink` behavior and reused it for standard AppShell, Pipeline Builder and project-workspace desktop brand rows plus compact AppShell/project mobile marks.
- Added an AppShell mobile hamburger and modal left drawer that reuses the desktop permission-filtered navigation, active-route states, user/logout context and powered-by footer; backdrop, close, Escape and navigation dismiss it, body scroll is locked and keyboard focus is contained/restored.
- Preserved the project mobile tree control, placed its return action alongside that control and aligned the desktop 320px sidebar brand row with a 64px project header. Pipeline Builder remains desktop-first beyond the approved brand addition.
- Added a focused failing-then-passing source-contract regression for mobile navigation and focus-sidebar branding; focused test and TypeScript no-emit check pass.
- Updated central/frontend/root-cause mapping for the shared brand-home owner, responsive AppShell navigation contract, focus-shell brand placement and mobile-menu triage path.
- Technically verified the current frontend source with all 42 tests, the production TypeScript/Vite build, the bundle budget (121.3 KiB gzip eager entry, 105 JavaScript chunks under the 250 KiB ceiling) and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.

## [Active] Client Workflow Revision 4 And Definition PDF

### Will be done

- Publish immutable official Rev4 template `40000000-0000-0000-0000-000000000023` from Rev3 without mutating published Rev3 or existing project runtime snapshots.
- Add Purchasing to the BOQ-without-design pricing step, separate QA/QC quality-dossier ownership from Supervisor field-report ownership, and make the approved document/photo evidence gates explicit for future Rev4 runs.
- Keep optional client-node cross-reference data on the template definition and export it with node/step business codes plus checklist/document/photo flags; exclude runtime/UAT reminders, statuses and test results from the template PDF.
- Fix PDF connector-label placement so branch labels stay visually attached to their own route, then add focused regression coverage and visually inspect the rendered Rev4 PDF.
- Update destructive seed replay, Golden validation and the central/backend/workflow/commercial/construction/quality mapping files, then run focused and full backend/database verification.

### Done

- Audited the client notes against Rev3 PDF, source and local PostgreSQL data; confirmed Rev3 has 15 nodes, 16 connectors and 51 stable coded steps.
- Confirmed template `...022` is published and immutable, template `...023` is unused, and projects 020/021/022 already have historical runtime snapshots that must remain unchanged.
- Proved the BOQ/QA-QC/Supervisor role and evidence-flag gaps from V85 and the database, and proved the branch-edge data is correct while PDF label placement can drift away from the owning connector.
- Rendered the source PDF at high resolution and retracted the earlier false clipping finding: page 6 is complete, so no pagination/clipping change is in scope.
- Produced the cross-domain review package and approved Mockup as Text; received explicit `OK triển khai` for Rev4, report-only client mapping, runtime/UAT exclusion and the PDF label fix.
- Added V86/V87: published immutable Rev4 `...023`, preserved Rev3 and all existing runs, added report-only client references, corrected Purchasing/QS and QA/QC/Supervisor responsibility/evidence gates, and corrected the quality dossier from `daily_report` to `document_upload`.
- Extended template repository/clone/hash handling for report presentation metadata and extended the on-demand PDF with node/step business codes, evidence flags and Appendix A while excluding runtime/UAT content.
- Added a failing-then-passing connector-label regression; labels now try every own-route anchor before offsetting and use a leader line back to the owning connector when collision avoidance requires it.
- Updated Makefile replay, Golden validation and central/backend/workflow/commercial/construction/quality/root-cause maps. All 87 migrations plus destructive seed replay passed on an isolated database; the temporary database was removed.
- Applied V86/V87 to local PostgreSQL and verified Rev4 has 15 nodes, 16 edges, 51 steps and 14 client references while projects 020/021/022 retain their existing engine-2 run snapshots.
- Exported and visually inspected all 7 pages of `output/pdf/workflow-client-workflow-hoan-thien-cong-trinh-chinh-thuc-rev-4.pdf`; no clipping was observed, branch labels remain attached, Appendix A is complete and extracted text contains no runtime/UAT content.
- Full `go test ./...` including PostgreSQL-backed integration coverage passed on a separate disposable database; focused PDF/hash tests, migration manifest check and Golden fixture generator check also passed. Temporary test databases were removed. Browser/UAT was not run in this implementation turn.

## [Active] Adaptive Role Workbench

### Will be done

- Make `Việc của tôi` the authenticated default landing page and retain the existing executive Dashboard under the user-facing label `Điều hành`.
- Turn `/my-work` into one adaptive role-aware workbench instead of separate duplicated role pages, using the actor-scoped work-item queue as the only task source of truth.
- Default to work the actor can perform now, expose explicit `Đang làm` and read-only `Sắp tới` lanes, and keep future-node work non-actionable through backend capabilities and normal `WorkItemAction` gates.
- Show the complete `Dự án › Node › Bước` context on every work card and selected-work preview while preserving the shared work-item detail dialog and its one-primary-action hierarchy.
- Add focused backend/frontend regression coverage, update the central/detail/domain mapping files, and run non-browser verification. Browser/UAT remains skipped unless explicitly requested in the same turn.

### Done

- Audited the existing Dashboard, HomeRedirect, My Work frontend, `/me/work-items` handler/service/repository contract, work-item capability logic, RBAC/project scope, shared work-item detail dialog, realtime invalidation and focused tests.
- Produced the cross-domain impact matrix, evidence-backed review package and approved Mockup as Text.
- Received explicit approval for one adaptive role workbench, `/my-work` as the default landing page, `Điều hành` as the existing Dashboard label, complete project/node/step context and read-only future work.
- Made authenticated root open `/my-work`, placed `Việc của tôi` first in global work navigation and retained the unchanged report dashboard under `Điều hành`.
- Added actor-scoped `Làm ngay`, `Đang làm`, `Chờ tôi duyệt`, `Bị chặn` and `Sắp tới · chỉ xem` buckets; default state is `all + need_action`, with secondary queue/status/deadline/type/priority/sort controls under `Bộ lọc thêm`.
- Added server `readiness`, node/step order context and source-backed `in_progress`/`upcoming` summary counts. Planned-future and engine-v2 future-node work stays visible but is excluded from current actions/decisions.
- Added shared complete `Dự án › Node › Bước` presentation to work cards, selected preview and detail; responsive layout puts the selected next-work panel first on narrow screens.
- Made future detail read-only across action, activity, evidence review/upload and workflow form edit/submit/sign/PDF controls, with matching backend current-time/current-node guards on work actions and mutation services.
- Added backend readiness/future/context tests and frontend context/default-route tests; updated central, frontend, backend, workflow, governance and reporting maps.
- Technically verified current working tree with backend `go test ./...`, frontend `npm test`, `npm run build`, bundle-budget check and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.

## [Active] Client Workflow Revision 3 And Contract Execution

### Will be done

- No mandatory implementation or verification work remains. Keep this completed plan in active memory until the user explicitly asks to close it.

### Done

- Audited client notes against source, database data for projects 020/021/022 and current Browser behavior using Admin and Accounting.
- Produced and received explicit approval for the cross-domain review package, implementation phases, acceptance decisions and Mockup as Text.
- Confirmed the client workflow lineage: revision 1 is template `40000000-0000-0000-0000-000000000020`; published revision 2 is immutable template `40000000-0000-0000-0000-000000000021`; revision 3 must use a new template row in the same family.
- Confirmed `SignatureCaptureDialog` and immutable `workflow_form_signatures` already provide visual signature, actor/role/time, revision, payload hash, checksum and protected file access; contract integration will reuse this infrastructure rather than duplicate it.
- Added custom migrations V84/V85 for stable node/step business codes, project entry mode, protected project documents, immutable contract-signature links, paper dispatch/returned scan, staged outside-plan variations, contract RBAC and official engine-3 Rev3 template `40000000-0000-0000-0000-000000000022`.
- Implemented the reviewed Rev3 topology: client-BOQ projects skip survey/meeting/design, outbound contract gates all takeoff/procurement/deployment work, Node 7 negotiation is one step, design review is merged, contract/field/QA-QC roles are separated, and existing 020/021/022 runtimes remain unchanged.
- Reused the existing signature dialog and workflow signature persistence for App signing. Contract activation now requires the current version's App signature, paper dispatch and returned signed scan; project contract UI warns when input/output signed scans are missing.
- Implemented outside-plan variation priority and the auditable Proposal -> QS -> Purchasing -> PM/Director -> procurement stages, including construction-work-item provenance and role-targeted activity mentions.
- Updated Project intake, Pipeline Builder, Contracts and Operations UI plus shared types/status labels; production frontend build and the complete non-DB Go suite pass on the current source.
- Added `contract_officer` / `Bộ phận HĐ` seed parity and untouched `GOLDEN-REV3-BOQ-001` project 023. A full destructive seed replay plus Golden validator passes on isolated database `consflow_rev3_test3`: Rev3 has 15 nodes, 51 unique coded steps/work items and 49 draft forms, while project 022 remains on engine 2.
- Applied all 85 migrations locally, passed the full DB-backed core integration suite on a clean isolated database and passed the migration checksum/manifest check. Updated central/backend/frontend and all affected domain mapping files.
- Recovered V85 to the exact already-applied checksum `93358db...` after Docker's fail-closed startup check exposed an accidental post-apply edit; clean database test6 then applied all 85 migrations successfully. Accounting's isolated-schema test helper now retains `public` extension functions without weakening table isolation.
- Browser UAT on the final rebuilt Docker source passed: project 023 shows/locks `client_boq` intake and Rev3; Pipeline Builder shows 15 nodes plus immutable node/step business codes; Contracts shows missing input/output scan reminders and the four execution checkpoints; Operations opens the mandatory urgent/normal variation proposal; `Bộ phận HĐ` sees exactly five role-scoped contract work items; and the App-sign action opens the existing immutable `SignatureCaptureDialog` locked to `Người phê duyệt`.
- Browser UAT found and closed two defects before handoff: an existing project's intake selector remained editable despite backend locking, and a legacy active contract was presented as deployment-ready without Rev3 files. The selector is now disabled after runtime creation, and readiness counts only the current contract version while labeling legacy active contracts explicitly. No browser warning/error was observed; only normal Vite development/debug messages appeared.
- Final current-source verification passed: frontend production build, `go test ./...`, full DB-backed `go test ./internal/core/... -count=1` on fresh `consflow_rev3_test6` (23.187s), focused current-version readiness regression, 85-migration checksum check, full seed/Golden validation on isolated `consflow_rev3_test3`, Docker backend/admin rebuild, API health and `git diff --check`.

## [Active] Project Finance Management Refactor

### Will be done

- No mandatory implementation or verification work remains. Keep this completed plan in active memory until the user explicitly asks to close it.

### Done

- Confirmed the business boundary: MISA remains the professional accounting/tax/double-entry system; Consflow does not integrate with MISA and does not duplicate professional books, VAT declarations, accounting periods, or journal correction workflows.
- Completed the required local mapping bootstrap, cross-domain impact review, evidence review, API/UI architecture, Mockup as Text, and received explicit approval of implementation points 1–7.
- Selected separate `project_material_costs` and `project_cash_requests` management sources so legacy payments, invoices, journals, ledgers, and accounting-period data remain historical and isolated from the new management report.
- Implemented V83 schema constraints, project-finance RBAC, future-template wording and non-destructive legacy permission isolation; active/historical pipeline runs are not rewritten.
- Implemented source-validated/audit-atomic material-cost and internal cash-request lifecycles, permission-redacted management summary, same-source CSV export, and handover readiness based on confirmed costs plus processed requests.
- Replaced ordinary global/project Accounting routes and navigation with the five-tab `Tài chính công trình` workspace; removed professional finance panels from the normal Reports catalog and rerouted finance work items/handover blockers.
- Added responsive create/detail/confirmation UI, mandatory reasons for `Yêu cầu bổ sung`, explicit external-system handoff wording, demo data, shared types/statuses and regression coverage.
- Browser UAT exposed and closed two in-scope usability defects: the action-heavy header compressed its purpose text at 1280px and global project context was lost on reload. The finance header now stays stacked until `2xl`, the shared selector has an accessible label, and `/accounting` persists its selected project in the `project` query parameter.
- Latest-source verification passed: frontend tests 31/31, production build, bundle budget (entry 120.1 KiB gzip; project-finance chunk 7.6 KiB gzip), full backend suite on an isolated PostgreSQL database, custom migration/complete Golden seed validation, Docker rebuild/restart and `/api/health`.
- Browser UAT on the rebuilt source completed the real Golden Project flow across `Ngân Kế Toán` and `Hoàng Long`: create/submit/confirm material cost, report/provisional-profit update, create/submit/approve/handoff/process cash request, required correction reason, role-separated actions, CSV success feedback and a clean task queue. Desktop 1280px and mobile 390×844 had no document-level horizontal overflow; mobile tabs use contained horizontal scrolling, dialogs fit the viewport, selected project survives reload, and console warnings/errors were zero. Scenario-based heuristic UX rating: 8.8/10; this is not a human usability study.

## [Active] Reliable CTGS Tooltip Regression Fix

### Will be done

- Obtain one direct Edge/manual pointer-hover acceptance signal against the rebuilt source. The in-app Browser session could not produce a CSS `:hover` state from its pointer-move control, so mouse hover remains `NOT UX ACCEPTED` even though the implemented handler and the shared rendered tooltip path are present.
- Close this plan only when the user says `Đóng plan`.

### Done

- User confirmed that hovering the compact CTGS number did not display the approved full-number tooltip.
- Root cause was proven: the served Vite source contained only an HTML `title` attribute, no rendered `role="tooltip"` element; the previous build/tests did not exercise hover behavior.
- User approved points 1–4 and explicitly authorized implementation plus Browser UAT.
- Replaced native `title` with a portalled application tooltip, kept the full immutable CTGS in the accessible label/raw data, and added mouse-hover, keyboard-focus, Escape, touch/pen and outside-pointer handling.
- Rebuilt and restarted the local Docker runtime after the final source change; PostgreSQL and Redis were healthy and backend/admin started successfully.
- Browser UAT passed desktop focus/full-value/viewport placement and Escape closure; at 390×844 it passed tap/click, full-value rendering, outside closure and no horizontal overflow. No browser console warnings/errors were recorded.
- The same Browser session could not synthesize a pointer hover: moving over the proven visible CTGS element left `element.matches(':hover') === false`. This criterion is recorded as skipped by tool limitation, not passed.
- User required click-to-copy for `BF-PAY-ffd22b…58faeb` and approved review points 1–4, including implementation and Browser UAT.
- Made every non-empty CTGS a copy control with an explicit copy icon and accessible label. Click, Enter and Space copy the full immutable CTGS value, while compact display text remains presentation-only; clipboard rejection produces an error toast without changing any journal entry.
- Added focused clipboard and keyboard-key regression coverage. The complete frontend suite passed 31/31 tests, the production build passed, and the bundle-budget gate passed on the final source state.
- Rebuilt and restarted the Docker stack after the final keyboard change; PostgreSQL and Redis became healthy and backend/admin started successfully.
- Browser UAT against the rebuilt `Golden Project – Hoàn thiện Văn phòng An Phát` Sổ Nhật ký chung passed desktop click, Enter, Space, exact full-value clipboard content, success toast, tooltip text and Escape closure. At 390×844 it passed click-to-copy, success toast, tooltip containment and zero document-level horizontal overflow. No browser console warnings/errors were recorded.

## [Active] Production Readiness And Maintainability Program

### Will be done

- Rerun C1 and C2 J0-J9 with the 15-role Golden Sample matrix in an isolated disposable environment when Browser verification is explicitly authorized in a future turn.
- Provision the selected production S3-compatible provider and migrate existing local objects with inventory, checksum and cross-replica validation before enabling multiple application replicas.
- Select the managed Kubernetes/edge vendor, production cluster/network CIDRs, managed PostgreSQL/Redis/S3 instances and external secret controller; Compose remains the controlled single-host topology until the reviewed Kubernetes overlay and cross-replica gates pass.
- Continue decomposing oversized backend core owners after the reporting slice, without changing public API, schema, RBAC or workflow behavior.
- Update the central/detail/domain maps whenever implementation changes owner files, CI/runtime contracts, audit behavior, storage, or route-loading behavior.

### Done

- Assessed the repository at 78/100 and confirmed the seven readiness gaps against current code, tests, migrations, CI, production Compose, and runbooks.
- Prepared and received user approval for the cross-cutting implementation roadmap and delegated execution.
- Added the aggregate `npm test` command for all committed frontend tests and made it a required CI step before the frontend production build; all 23 tests pass locally.
- Added a PostgreSQL CI service and required DB-backed core/migration tests so those checks cannot silently skip; fixed the PostgreSQL `DISTINCT`/ordering defect exposed by that gate.
- Centralized audit writes: critical governance/workflow/finance/quality transitions are transactional and fail closed, while explicitly non-critical events remain observable best-effort with structured failure logs.
- Split the giant backend integration suite into domain-owned files with shared fixtures, and moved the reporting handler/service/repository/type slice into focused owner files without changing routes, API, schema or RBAC.
- Added the 12-role J0-J9 Golden Sample UAT matrix, isolated-DB/reset contract, evidence requirements and explicit `BLOCKED-COVERAGE` state for missing dedicated demo actors.
- Added `_FILE` production secrets, a pinned-session advisory-locked one-shot migration job, incompatible-rollback detection, safe drain/migrate/start runbooks and secret-safe backup guidance.
- Added internal Prometheus HTTP/Go/process/DB metrics with bounded labels and kept `/metrics` unavailable through public Nginx.
- Added a provider-neutral protected-file storage registry plus local provider and migrated evidence, workflow signatures/PDFs and input/output invoice files behind authorization-before-open streaming. Independent review closed post-commit cleanup data loss, symlink-swap TOCTOU and registry concurrency risks with regression/race coverage.
- Added the portable S3-compatible provider, fail-closed startup write/read/delete preflight, dual-provider legacy reads, V70/V71 provider and storage-key integrity migrations, and a dry-run-by-default local-to-S3 operator command with checksum verification, conditional metadata updates and no source deletion.
- Added a fail-closed portable Kubernetes/Kustomize application baseline with two backend/admin replicas, private Services and ingress NetworkPolicies, one-at-a-time two-node-safe rollout, PDBs, cross-node anti-affinity/zone spread, probes/resources/security contexts, enforced rate limiting, separate runtime DML versus migration DDL/owner credentials, release-unique migration Job, managed PostgreSQL/Redis/S3 and secret-manager contracts, startup S3 preflight, forward-only object-cutover guidance, optional gated HPA/Ingress examples and deterministic offline validation. No cluster was deployed.
- Made evidence gallery/preview and handover source presentation provider-aware: internal `local|s3` objects use protected endpoints, while external, none and unknown providers remain excluded.
- Converted authenticated page modules to route-level lazy loading below auth/permission guards while keeping Login, guards and AppShell eager; added standard/focus-shell loading and recoverable error states plus a CI gzip budget of 250 KiB for the entry and every JavaScript chunk. The production entry is now 119.9 KiB gzip and the largest lazy chunk is 35.8 KiB gzip.
- Corrected the production runbook: Redis Pub/Sub already provides shared SSE fanout, while shared object storage, deployment/runtime choices and cross-replica validation remain required before scaling.
- Updated central, backend, root-cause and affected domain maps for the new owners and operational contracts.
- Passed the full backend suite against an isolated PostgreSQL database, storage/core race checks, `go vet ./...`, all 72 migration manifest checks, 23 frontend tests, the production frontend build and bundle budget, Kubernetes render validation, agent mapping validation and diff hygiene checks; browser verification was not run by policy.
- Ran the authorized Golden Sample Browser UAT in isolated DB `consflow_uat_20260722_115155`: all 12 demo actors authenticated and saw Golden Live, completed fixtures remained unchanged, but C1 failed at J1 because PM lacks the required `planning.edit` permission for draft/evidence mutations; C2 and J2-J9 remain blocked by that shared prerequisite and missing dedicated `Thiết kế`/`Sale`/`Pháp lý` actors. Evidence is stored outside the repo at `/private/tmp/consflow-uat-evidence/20260722_115155`.
- Resolved the confirmed Golden Sample UAT blockers: V72 and seed parity grant PM `planning.view`/`planning.edit` while preserving project/work-item scope; dedicated `Thiết kế`, `Sale` and `Pháp lý` users/staff/roles raise Golden Live membership to 15; validators, runbooks, UAT matrix and maps now cover all actors and rate-limit-safe evidence capture. Migration/seed validation and the full backend suite pass on disposable PostgreSQL databases; Browser rerun remains explicitly pending.
