# [Closed] Post Backend Automation Rollout Plan

Closed: 2026-07-06
Reason: User confirmed this workflow/backend automation thread should be closed because release is not ready until UI instability is handled in a separate refactor thread.

## Handoff For Next UI Refactor Thread

- Backend workflow automation, Vietnamese cleanup, V40 migration, and post-restart runtime UAT are implemented and verified.
- UI is explicitly not release-ready; next thread should start with a UI refactor review package and Mockup as Text before edits.
- Do not run seed or reset data unless explicitly approved.
- Preserve the existing draft template and the published official client template.
- Known backend bug still worth reviewing before release: `GET /projects/:id/pipeline-run` can return `steps: null` due runtime step ordering query evidence captured in UAT notes.
- Suggested skills for next thread: `consflow-frontend-workflow`, `consflow-workflow-orchestration-domain`, `consflow-impact-analysis`, `consflow-diagnose`.

## Closed Plan Notes

## [Closed] Post Backend Automation Rollout Plan

Status: post-restart UAT runtime completed

Will be done:
- Slice S0: repo/change ownership review. Separate workflow automation changes from older unrelated worktree changes before any commit, cleanup, or next implementation.
- Slice S1: Vietnamese language implementation from the approved matrix. Translate user-facing backend messages, blocker/activity text, edge condition labels, workflow form/template labels, frontend labels, status labels, report/export labels, and official template visible text while preserving API keys, route names, DB enum values, and internal action keys.
- Slice S2: real workflow UAT. Run the official client workflow through the expected business paths and capture pass/fail evidence per node without running seed or creating fake invoices.
- Slice S3: role and permission audit. Map each official workflow node/step to expected roles, required permissions, and backend enforcement points.
- Slice S4: domain automation trace audit. Verify the draft records created by automation stay idempotent and do not auto-approve, auto-pay, auto-receive, or auto-create fake invoices.
- Slice S5: staging/deploy checklist. Prepare migration/deploy runbook with backup, migrate, health check, official template verification, and rollback/stop points.
- Slice S6: UI monitoring proposal only if approved later. If monitoring is needed, prepare Mockup as Text first; do not implement UI without explicit approval.

Done:
- Created active Goal for the post backend-automation rollout plan.
- Read `consflow-repo-architect`, `AGENTS.md`, `README.md`, `docker-compose.yml`, module keyword map, and current plan state.
- Confirmed the next work is cross-cutting: workflow orchestration, frontend/admin UI text, backend messages, database/runtime UAT, RBAC, finance/accounting, quality/handover, and deploy operations.
- Confirmed review gates still apply: audit and planning can proceed, but code/UI changes need a review package, and UI changes need Mockup as Text.
- Drafted the initial slice plan and Vietnamese audit matrix for user review.
- User approved moving into S1 audit with `Approve`.
- Ran audit-only scans over frontend modules, shared status utilities, backend workflow/service/repository messages, migrations, and seed/demo visible data.
- Completed the initial S1 audit-only pass before implementation; that pass only recorded the review matrix.
- User clarified terminology: `GĐ` means `Giám đốc`; `workflow form` is `biểu mẫu quy trình`; `action item` should not be called `đầu việc họp`; `evidence` is translated by context.
- User approved S1 implementation with `OK triển khai S1 theo matrix`.
- Implemented S1 Vietnamese cleanup across backend validation/API messages, workflow activity/blocker text, edge condition labels/reasons, PDF export labels, shared status/activity labels, and work item/planning/meeting/rework/checklist UI surfaces.
- Added `backend/migrations/V40__vietnamese_official_workflow_text.sql` to update official client template/runtime visible strings while preserving the existing draft template and leaving applied V38 unchanged.
- Kept internal keys/enums/routes/table names unchanged where they are not directly rendered to users.
- Did not run seed and did not run browser/Playwright verification for S1.
- Ran `cd backend && make fmt` successfully.
- Ran `git diff --check` successfully before and after validation.
- Ran `cd backend && make migrate-check` successfully; 40 custom migrations are valid, including V40.
- Ran focused backend workflow tests successfully: `go test ./internal/core -run 'TestPipelineEdge|TestClientOfficialWorkflow|TestClientWorkflowAutomation'`.
- Ran `cd frontend && npm run build` successfully; only the existing Vite chunk-size warning appeared.
- Ran full backend tests successfully with `GOCACHE="$PWD/.gocache" make test`.
- Confirmed host `127.0.0.1:5432` points to a local Postgres process that is not the Docker compose DB, so host `make migrate` connected to the wrong database and failed with missing role `consflow`.
- Applied V40 through the Docker compose network with the backend migration binary; result: `applied=1 skipped=39`, migration 40 `vietnamese official workflow text`.
- Ran a final visible-text sweep and fixed remaining backend-rendered English strings for invoice upload timeline entries, work item creation activity, and workflow form instance creation event.
- Re-ran `cd backend && make fmt`, focused official workflow tests, `git diff --check`, and full backend tests successfully after the final sweep.
- Ran S2 real workflow UAT through API/DB only; did not run seed, did not run browser/Playwright, and did not create fake invoices or fake payments.
- Created clean S2 UAT projects `S2-UAT-DESIGN-20260706033656` and `S2-UAT-NO-DESIGN-20260706033656`; each generated 12 runtime nodes and 52 work items from the official published template.
- Verified branch decision form answers: design branch submitted `needs_design=yes` and routed to `Thiết kế`; no-design branch submitted `needs_design=no` and routed to `BOQ (Không thiết kế)`.
- Verified the mandatory `Đầu Vào: Tổ Đội / Thầu Phụ / Cung cấp vật tư` gate blocks before procurement approval and advances to `Đàm phán giá` after all project procurement requests are approved.
- Verified official template coexistence after UAT: `[Client] Quy trình hoàn thiện công trình chính thức` remains `published` with 12 nodes; preserved `[Template] Hoàn thiện công trình` remains `draft` with 13 nodes.
- Confirmed migrations V38, V39, and V40 are applied in the Docker compose database.
- Confirmed no fake finance records were created by automation in S2 UAT: both UAT projects have `input_invoice_count=0`, `output_invoice_count=0`, and `payment_count=0`.
- Confirmed finance blockers are Vietnamese: incoming acceptance blocks with `Cần tải lên và xác minh hóa đơn đầu vào thật`; outgoing acceptance blocks with `Cần tải lên và phát hành hóa đơn đầu ra thật`; handover/closure blocks with `Bàn giao/đóng tài chính còn điều kiện chưa hoàn tất`.
- Confirmed construction runtime safety gate blocks both UAT runs at `QA & QC / Thi công` with `Chưa thu đủ tối thiểu 30% giá trị hợp đồng để bắt đầu thi công`; no fake receipt/payment was created to bypass this gate.
- Captured S2 API defect: `GET /projects/:id/pipeline-run` returns `steps: null` because the backend `GetPipelineRun` step query orders by non-existent `rs.created_at`; `/projects/:id/work-items` and DB queries were used as reliable UAT evidence.
- Captured S2 domain gap: `generate_procurement_from_boq` is blocked with `Chưa có dòng BOQ để sinh yêu cầu mua` because completing the BOQ workflow steps does not create domain `boq_items`; S4 must decide whether BOQ workflow forms should create/link real BOQ rows before procurement automation can be end-to-end.
- Captured S2 idempotency/grain risk for S4: automation fires per completed work item, producing repeated events and multiple draft domain rows in acceptance/AR surfaces. Counts per UAT project: incoming acceptance records 8, payable 1, payable schedule 1, outgoing acceptance records 5, receivables 5, receivable schedules 5, handover 1.
- Completed S3 role/permission audit against the Docker compose database and backend permission code; did not edit code, run seed, or run browser verification.
- Confirmed the official template has 12 nodes and 52 steps; node-level permissions cover `planning.edit`, `boq.update`, `procurement.create`, `boq.approve`, `project.update`, `operation.report`, `acceptance.approve`, and `accounting.payment_approve`.
- Confirmed step-level permissions are domain-specific and copied into runtime work item `evidence_requirement_json.required_permission`; work item actions enforce these permissions with project scope.
- Confirmed all official-template permission keys exist in the `permissions` table and all official-template role labels map to existing system role names/display names.
- Confirmed runtime node actions use `RunNodePermissionGate`, and run-step submit uses both node gate and work-item permission/evidence gates.
- Captured S3 UX gap: planning UI posts runtime actions and relies on backend errors for authorization; it does not pre-compute project-scoped permission visibility for every node/step.
- Captured S3 role-permission risk: `Chủ đầu tư`, `Viewer`, and `Thủ kho` currently have no official-template permissions. This is safe only if those labels remain watcher/coordination roles and are not expected to complete protected workflow steps.
- Completed S4 domain automation trace audit against the clean S2 UAT projects; did not edit code, run seed, create fake invoices, or create fake payments.
- Confirmed `workflow_automation_events` has unique idempotency by source/action, but the source is currently each completed work item: `clientOfficialWorkflowTemplateID:work_item:<work_item_id>:<action_key>`.
- Confirmed automation safety remains intact for both S2 UAT projects: `input_invoices=0`, `output_invoices=0`, `payments=0`, `stock_movements=0`, and AP/AR records remain draft.
- Confirmed procurement gate behavior: each UAT project has one approved procurement request created manually for test, and `input_procurement_gate` points multiple completed Đầu Vào steps to that same request.
- Captured S4 grain defect: incoming acceptance automation creates 8 draft incoming acceptance records per project while sharing one draft payable and one payable payment schedule; this is per-step noise rather than a clean source-document grain.
- Captured S4 grain defect: outgoing acceptance automation creates 5 draft outgoing acceptance records, 5 draft receivables, and 5 receivable schedules per project. This is not appropriate for one final outgoing acceptance milestone.
- Captured S4 blocker trace: BOQ procurement generation is blocked because no real `boq_items` exist; incoming invoice, outgoing invoice, and handover/financial closure blockers are Vietnamese and attached through existing activity/blocker surfaces.
- Root cause confirmed in code: `RunClientWorkflowAutomationForWorkItem` is invoked when a work item completes, and `recordClientWorkflowAutomationOutcome` idempotency is keyed by work item rather than action-specific node/domain source.
- User approved S4 implementation after the review package with `Approve`.
- Implemented S4 backend-only automation grain fix: incoming acceptance scaffold now reuses one incoming acceptance per approved procurement request; outgoing acceptance scaffold now reuses one final automation acceptance per runtime node; incoming/outgoing/handover automation events now use `pipeline_run_node` source grain when available.
- Preserved BOQ procurement automation and input procurement gate work-item traces so BOQ blockers and mandatory Đầu Vào checks still record per triggering work item.
- Added regression coverage that completes multiple official workflow checklist work items in the same incoming/outgoing acceptance node and verifies only one domain scaffold/event is created while `run_count` records all triggers.
- Verified S4 on an isolated temporary Postgres Docker container, not the active compose database: `go test -v ./internal/core -run 'TestClientWorkflowAutomation'` passed.
- Verified package-wide backend tests: `GOCACHE="$PWD/.gocache" go test ./...` passed.
- Ran `cd backend && make fmt` successfully and `git diff --check` successfully.
- User approved S5 preflight/read-only verification with `làm đi`.
- Ran S5 non-mutating checks successfully: `make migrate-check`, backend `GOCACHE="$PWD/.gocache" go test ./...`, frontend `npm run build`, `git diff --check`, and `make docker-config`.
- Frontend build passed with the existing Vite chunk-size warning only.
- Verified Docker compose runtime without restart: `admin`, `backend`, `postgres`, and `redis` are up; backend health endpoint returned `status=ok`; frontend returned HTTP 200.
- Ran read-only PostgreSQL checks through Docker compose. Confirmed migrations V38, V39, and V40 are applied; official client template is `published` with 12 nodes and 52 steps; preserved `[Template] Hoàn thiện công trình` remains `draft` with 13 nodes and 53 steps.
- Confirmed S2 UAT safety still holds in compose DB: both S2 UAT projects have `input_invoices=0`, `output_invoices=0`, `payments=0`, and `stock_movements=0`.
- Confirmed historical S2 automation traces still show the pre-S4 grain counts (`incoming_events=8`, `outgoing_events=5`, `handover_events=2` per UAT project). This is expected because S5 did not restart/deploy the updated backend code and did not mutate historical event rows.
- Did not run `make restart`, did not run active DB migrate, did not run seed/reset, and did not modify deployment configuration.
- User ran `make restart` manually after S5 preflight.
- Verified post-restart runtime: `admin`, `backend`, `postgres`, and `redis` are up; backend health endpoint returned `status=ok`; frontend returned HTTP 200.
- Backend startup log after restart reported `database migrations applied=0 skipped=40`, confirming no new migration was applied during that restart.
- Re-confirmed V38, V39, and V40 are present in `schema_migrations`; official client template remains `published` with 12 nodes and 52 steps; preserved draft template remains `draft` with 13 nodes and 53 steps.
- Re-confirmed S2 UAT projects still have `input_invoices=0`, `output_invoices=0`, `payments=0`, and `stock_movements=0`.
- Confirmed existing automation event rows still use historical `work_item` source grain. This is expected for old rows because S4 changes runtime behavior for new events and does not backfill or mutate historical trace rows.
- Ran disposable post-restart UAT through the active backend API after user restart. Created project `S5-UAT-POST-RESTART-20260706045536` and approved one procurement request for the mandatory `Đầu Vào` gate.
- Completed target work items through API runtime: `Nghiệm thu Đầu Vào` 8/8, `Nghiệm thu Đầu Ra` 5/5, and `Kết thúc: Bàn giao + Đóng tài chính` 2/2.
- Verified new post-restart automation events use node-grain source: `incoming_acceptance_finance_scaffold` has `source_type=pipeline_run_node`, `event_rows=1`, `run_count=8`; `outgoing_acceptance_finance_scaffold` has `source_type=pipeline_run_node`, `event_rows=1`, `run_count=5`; `handover_financial_closure_check` has `source_type=pipeline_run_node`, `event_rows=1`, `run_count=2`.
- Verified domain scaffolds are idempotent for the UAT project: `incoming_acceptances=1`, `payables=1`, `payable_schedules=1`, `outgoing_acceptances=1`, `receivables=1`, `receivable_schedules=1`, `handovers=1`.
- Verified finance/warehouse guardrails for the UAT project: `input_invoices=0`, `output_invoices=0`, `payments=0`, and `stock_movements=0`.
- Verified blocker messages remain Vietnamese: `Cần tải lên và xác minh hóa đơn đầu vào thật`, `Cần tải lên và phát hành hóa đơn đầu ra thật`, and `Bàn giao/đóng tài chính còn điều kiện chưa hoàn tất`.

Vietnamese audit matrix for review:

| Area | Authoritative scan targets | Translate rule | Review decision needed |
|---|---|---|---|
| Backend validation/API messages | `backend/internal/core/*.go`, `backend/internal/shared/response` | User-facing `validationError`, `forbiddenError`, response messages, audit descriptions, and blocker text should be Vietnamese. | Decide tone: short operational wording or explanatory wording. |
| Workflow blocker/activity | `workflow_automation_events`, `work_item_activities`, `repository_domain.go`, `activity_stream.go` | Text rendered to users must be Vietnamese; metadata keys/action keys stay internal English. | Decide whether the product term `blocker` remains or becomes `điểm chặn`. |
| Edge condition catalog | `/api/pipeline-edge-conditions`, `pipeline_edge_conditions.go`, edge condition reason helpers | Label/description/reason strings are Vietnamese; condition type keys stay unchanged. | Approve canonical names for edge conditions. |
| Workflow forms/templates | `workflow_form_*` migrations/seed, form library UI, form instance panels | Visible form name, field label, help text, and validation copy should be Vietnamese. | Decide whether visible `Generic submission` should be renamed. |
| Frontend UI labels/buttons/toasts | `frontend/src/modules`, `frontend/src/components`, `frontend/src/lib/utils/status.ts` | Buttons, empty states, loading/error states, chips, and toast text should be Vietnamese. | Approve canonical translations for `work item`, `workflow`, `template`, `pipeline`. |
| Status and enum display | shared status utilities, enum chips, frontend domain mappings | DB/API enum keys can stay English; all rendered labels should map to Vietnamese. | Approve status vocabulary for draft/ready/blocked/approved/etc. |
| Reports and exports | reports pages, backend report handlers, CSV export headers | Report labels and export headers visible to users should be Vietnamese. | Decide if any accounting/tax export needs bilingual headers. |
| Seed/demo visible data | `backend/seed/seed.sql`, migrations that insert templates/forms | Visible demo/template/workflow text should be Vietnamese unless intentionally technical. | Decide whether dev-only demo data can remain partially English. |
| Internal keys/enums/logs | JSON keys, action keys, DB enum values, route names, technical logs | Keep unchanged when not directly rendered to users. | Confirm boundary between internal keys and UI-visible text. |
| Accepted business abbreviations | BOQ, PM, QS, QA/QC, GĐ, VAT, AP, AR | Keep familiar industry abbreviations where client already uses them. | Approve allowed abbreviation list. |

Planned validation for slices:
- S1 audit: `rg` scans, route/page targeted reads, optional DB read-only scans for visible template/form/activity values.
- S2 UAT: API and/or app walkthrough only with existing official template, no seed, no fake invoices.
- S3 role/permission: static template/step matrix plus backend permission helper checks.
- S4 domain trace: read-only DB verification and focused backend tests if code later changes.
- S5 deploy: migration dry check, Docker health/log checks, template coexistence query, and rollback notes.

Slice execution matrix:

| Slice | Entry condition | Work products | Exit evidence |
|---|---|---|---|
| S0 Repo/change ownership | Current dirty worktree is known. | File ownership table: workflow automation, UI retouch, unrelated/user changes, generated/runtime artifacts. | `git status --short` plus ownership notes; no revert of user changes. |
| S1 Vietnamese audit | User approves audit areas and terminology rules. | Audit matrix with current text, proposed text, file/path/source, user-visible level, risk, and owner domain. | User-approved matrix before edits; no code change during audit-only pass. |
| S2 Real workflow UAT | Backend is healthy and official template is present. | UAT run sheet for both branches and required gates. | Pass/fail evidence per node; DB/API evidence for created draft records and blockers. |
| S3 Role/permission audit | Official workflow nodes/steps are frozen for this review pass. | Matrix of node, step, role list, required permission, backend enforcement, UI visibility. | Reviewed role/permission mismatches with proposed fixes. |
| S4 Domain automation trace | UAT project exists and has activity events. | Trace report: source work item -> automation event -> draft domain record -> blocker/audit. | Idempotency and no-fake-invoice/no-auto-payment evidence. |
| S5 Staging/deploy checklist | Code changes have passed review and local checks. | Runbook for backup, migrate, smoke test, template verification, stop/rollback points. | Staging signoff checklist completed. |
| S6 UI monitoring proposal | Backend automation and UAT are stable; user asks for monitoring UI. | Mockup as Text and review package only. | Explicit user approval before any UI implementation. |

UAT workflow run sheet draft:

| UAT path | Nodes to exercise | Must verify | Forbidden side effects |
|---|---|---|---|
| Branch Có thiết kế | Khảo sát -> Họp báo cáo khảo sát -> Thiết kế -> BOQ Có thiết kế -> Đầu Vào -> Đàm phán -> Hợp đồng -> Thi công -> Nghiệm thu Đầu Vào -> Nghiệm thu Đầu Ra -> Kết thúc | Form-answer routing selects the design branch; Đầu Vào gate blocks until procurement is approved; acceptance/accounting blockers appear in Vietnamese. | No seed, no fake invoice, no auto-pay, no auto-receive, no auto-approve final records. |
| Branch Không thiết kế | Khảo sát -> Họp báo cáo khảo sát -> BOQ Không thiết kế -> Đầu Vào -> Đàm phán -> Hợp đồng -> Thi công -> Nghiệm thu Đầu Vào -> Nghiệm thu Đầu Ra -> Kết thúc | Form-answer routing selects the no-design branch; BOQ/procurement automation is idempotent; downstream blockers are clear. | No mutation of preserved draft template, no historical project run mutation. |
| Missing procurement gate | Complete/check incoming acceptance work item before approved procurement exists. | Blocker `Chưa có yêu cầu mua/đầu vào đã duyệt để mở nghiệm thu đầu vào`. | No incoming acceptance/payable schedule created before the gate is satisfied. |
| Real invoice gate | Complete incoming/outgoing acceptance after draft AP/AR schedule exists. | Blockers `Cần tải lên và xác minh hóa đơn đầu vào thật` and `Cần tải lên và phát hành hóa đơn đầu ra thật`. | No input/output invoice row created by automation. |
| Handover/closure gate | Complete Kết thúc work item while handover/accounting conditions remain incomplete. | Draft handover exists and blocker lists unfinished conditions in Vietnamese. | No project financial closure without readiness conditions. |

Role/permission audit matrix draft:

| Workflow area | Roles expected from client flow | Permissions to verify | Backend/UI evidence to inspect |
|---|---|---|---|
| Khảo sát | PM, GĐ, QS, Sale, Đội kỹ thuật, Giám sát | `planning.edit`, work item evidence permissions | Template steps, work item assignments, `requireWorkItemPermission`, project member permissions. |
| Họp báo cáo khảo sát | QS, Thiết kế, Giám sát | planning/work item action permissions | Branch-decision form step and form submission permissions. |
| Thiết kế | PM, Thiết kế, Sale/Chủ đầu tư | `planning.edit`, document upload permissions | Workflow form/document requirements and frontend action readiness. |
| BOQ Có/Không thiết kế | QS, PM, GĐ, Sale | `boq.create`, `boq.update`, `boq.approve`, `procurement.create` | BOQ handlers/services, approval routes, procurement generation guards. |
| Đầu Vào | QS, Mua hàng, PM, GĐ, Giám sát | `procurement.create`, `procurement.approve`, relevant warehouse/material permissions | Procurement status transition, incoming acceptance gate, work item role assignments. |
| Đàm phán/Hợp đồng | Sale, QS, PM, GĐ, Kế toán, Pháp lý | procurement/accounting/project permissions | Approval action audit, contract/handover source records if visible. |
| Thi công / QA & QC | QS, PM, Thiết kế, QA | operation/report/evidence permissions | Work item action permissions, operation panels, activity/evidence gates. |
| Nghiệm thu Đầu Vào | QA/QC, QS, GĐ, Kế toán | `acceptance.approve`, `accounting.payable_view`, invoice permissions | Incoming acceptance, payable schedule, input invoice blocker. |
| Nghiệm thu Đầu Ra | QA/QC, QS, PM, GĐ, Kế toán | `acceptance.approve`, `accounting.receivable_view`, output invoice permissions | Acceptance, receivable schedule, output invoice blocker. |
| Kết thúc | Kế toán, GĐ, PM | `handover.create`, `handover.approve`, accounting payment permissions | Handover readiness check, financial blockers, payment/invoice absence. |

Staging/deploy checklist draft:

| Stage | Action | Stop condition |
|---|---|---|
| Preflight | Confirm branch/diff ownership and no unrelated revert. | Unknown dirty files touching same modules without owner decision. |
| Backup | Snapshot database or export before migrations. | No usable backup for target environment. |
| Migration check | Run custom migration check, never Flyway. | Checksum mismatch or unexpected pending migration. |
| Deploy backend | Build/restart backend and watch logs for `applied`/`skipped`. | Backend unhealthy or migration failed. |
| Smoke API | `/api/health`, auth login, template list, official template detail. | Health/auth/template detail fails. |
| Template verification | Official template published, preserved draft still draft, node/step/edge counts correct. | Draft changed unexpectedly or official template missing. |
| UAT smoke | Start project pipeline and run minimal gate checks. | Routing/gate/blocker mismatch. |
| Finance safety | Verify no fake input/output invoices and no real payments created by automation. | Any automation-created invoice/payment without user action. |
| Rollback/stop | Stop further rollout and use backup/manual fix plan. | Data integrity, migration, or auth issue appears. |

UI monitoring gate draft:
- Current state: backend exists; dedicated monitoring UI does not exist.
- First allowed action: review whether existing activity/blocker surfaces are enough.
- If insufficient: create Mockup as Text for a workflow automation monitor showing project, node, action, status, target record, blocker reason, and retry/history.
- Do not implement UI until the user explicitly approves that mockup and review package.

S1 detailed Vietnamese audit matrix:

| ID | Surface | Evidence | Current text / behavior confirmed | Proposed Vietnamese rule | Priority | Decision status |
|---|---|---|---|---|---|---|
| S1-01 | Shared enum/status/action display | `frontend/src/lib/utils/status.ts` lines 39, 90, 103, 107, 126-128, 136, 139-144, 153, 307, 316-317, 355-380 | Mixed visible labels: `Chờ review`, `Review thiết kế`, `Template pipeline`, `Work item`, `Evidence`, `Workflow form`, `blocked`, `blocker`, `review_requested`, `Activity blocker công trình`. | Keep enum/API keys unchanged. Render all labels in Vietnamese. | High | Needs review |
| S1-02 | Evidence gallery UI | `frontend/src/modules/work-items/EvidenceGallery.tsx` lines 24-30, 35, 66, 74, 81, 120, 128, 130, 344-350, 361, 370, 411-420, 428, 434 | Visible labels and copy include `All`, `Photo`, `Document`, `Submitted`, `Accepted`, `Rejected`, `Evidence`, `Visibility`, `Metadata`, `Storage`, `review`, `Protected files`, `Accept`, `Reject`, `Record`. | Translate filters, table headers, dialog labels, buttons, and toasts. Use contextual `evidence` vocabulary: workflow/QA = `minh chứng`, accounting = `chứng từ`, generic file = `tệp đính kèm`. Keep file MIME/storage technical values where needed. | High | User corrected |
| S1-03 | Workflow form / work item support panel | `frontend/src/modules/work-items/WorkflowFormPanel.tsx` lines 253, 322-353, 392, 400-425, 457-460, 478, 494-509 | Visible terms include `Submitted`, `Updated`, `Done`, `Partial`, `Blocked`, `Urgent`, `Remote`, `submit`, `Form`, `artifact`, `done`, `active`, `signed`, `file`, `System`, `Daily report`, `Reporter`, `Scope`, `Progress`, `Blocker / Risk`. | Translate runtime labels and helper copy. Keep `PDF` only if accepted as an abbreviation. | High | Needs review |
| S1-04 | Planning runtime UI | `frontend/src/modules/planning/PlanningPage.tsx` lines 377, 404, 412, 416-419, 471-481, 500, 517-519, 543-576 | Visible terms include `Pipeline`, `instance`, `Evidence`, `Node`, `Step`, `Next`, `Work item`, `Pipeline runtime`, `approval action`, `Role`, `Approval`, `Fail path`, `submit form`. | Translate page/runtime labels. Keep internal route/entity names unchanged. | High | Needs review |
| S1-05 | Meeting minutes UI | `frontend/src/modules/meeting-minutes/MeetingMinutesPage.tsx` lines 163, 171-177, 227-238, 305, 313-338 | Visible terms include `action item`, `submit`, `decision rows`, `Action items`, `work item`, `Meeting minutes`, `Generated`, `Status`, `Action`, `Location`, `Submitted by`, `Artifacts`, `evidence`. | Translate meeting UI labels. Canonical `action item` should be `đầu việc`; when context is meeting-specific, section copy can be `Đầu việc từ biên bản họp`. | Medium | User corrected |
| S1-06 | Rework / replanning UI | `frontend/src/modules/rework/ReworkPage.tsx` lines 45-55, 90-103, 106-125, 139-155 | Visible terms include `Draft`, `Approved`, `In progress`, `Done`, `Submitted`, `Rework`, `Replanning`, `timeline`, `status`, `board`, `requests`, `Deadline`, `Action`, `Complete`, `Approve`, `Impact scope`. | Translate status/action labels. Decide whether `Rework` and `Replanning` remain product terms. | Medium | Needs decision |
| S1-07 | Backend edge condition validation | `backend/internal/core/pipeline_edge_conditions.go` lines 54-143 | API validation messages are English: `from_node_id is required`, `label must be 80 characters or fewer`, `priority must be 0 or greater`, and related edge-condition copy. | Translate API messages surfaced to UI/toasts. Keep field names in structured details if clients rely on them. | Medium | Needs review |
| S1-08 | Backend workflow/form/service errors | `backend/internal/core/service.go` lines 525, 550-618, 707-775, 3349-3361, 3513, 3556-3594, 4595-4610; `backend/internal/core/repository_domain.go` lines 4076, 4212, 4259, 4534, 4724, 4861, 4868, 5699 | Mixed user-facing messages include `workflow form`, `map form published`, `Field key`, `Label field`, `Field type`, `submit`, `export PDF`, `work item`, `evidence`, `review`, `blocker`, `Action`. | Translate service/repository messages that can reach API responses, toasts, activities, or audit screens. Preserve JSON keys and internal action identifiers. | High | Needs review |
| S1-09 | Report/accounting warnings | `backend/internal/core/repository_domain.go` lines 13254-13303 | Report warnings include English accounting terms: `direct`, `indirect cost`, `posted`, `Net profit`, `VAT`. | Translate report warnings and export-visible labels. Keep `VAT` only if approved as an accepted abbreviation. | Medium | Needs decision |
| S1-10 | Official client template visible data | `backend/migrations/V38__client_workflow_template.sql` lines 167, 169, 180, 222; `backend/seed/seed.sql` lines 316, 319, 367-383, 396, 438 | Official/template strings include `Review & Check`, `check`, `Trảo đổi`, `Upload`, `review`, and `Đầu vào đã check`. | Fix client-visible template strings through a new migration, not by editing applied V38. Update seed only if reset/reseed fidelity is required later. | High | Needs review |
| S1-11 | Demo/seed visible data | `backend/seed/seed.sql` lines 57, 63, 190, 192, 204-213, 600, 603-631, 1098-1112 | Demo strings include `Publish pipeline`, `Reject nghiệm thu`, `Daily report`, `Closure checker`, `Upload`, `Pass/Fail`, `Financial Closure`, `[Demo] Full workflow form coverage`, `Seeded`, `Submitted`. | Demo/user-visible seed data should be Vietnamese unless explicitly marked developer-only. | Medium | Needs review |
| S1-12 | Accepted abbreviation boundary | Cross-cutting frontend/backend/template data | Current product uses BOQ, PM, QS, QA/QC, GĐ, VAT, AP, AR, PDF, Checklist, Rework, Replanning, Action item. | Approve which terms stay as abbreviations/product terms and which must be translated. | Decision | Needs decision |

S1 terminology decisions to review:
1. `work item`: proposed `đầu việc`.
2. `workflow form`: user approved `biểu mẫu quy trình`.
3. `evidence`: user approved context-specific wording: workflow/QA = `minh chứng`, accounting/finance documents = `chứng từ`, generic file attachment = `tệp đính kèm`.
4. `blocker`: proposed `điểm chặn`.
5. `review`: proposed `rà soát` for general review, `duyệt` only for approval actions.
6. `submit`: proposed `gửi` for user action, `đã gửi` for status.
7. `action item`: user questioned `đầu việc họp`; use canonical `đầu việc`, and use `Đầu việc từ biên bản họp` only when the UI needs meeting context.
8. `rework`: proposed `sửa lỗi` when generic, keep `Rework` only if product term is intentionally bilingual.
9. `replanning`: proposed `lập kế hoạch lại`.
10. `PDF`, `BOQ`, `PM`, `QS`, `QA/QC`, `GĐ`, `VAT`: proposed keep as accepted abbreviations; `GĐ` means `Giám đốc` unless client role taxonomy specifies a more exact director role.

S1 contextual `evidence` vocabulary:

| Context | Use | Avoid |
|---|---|---|
| Workflow/work item general | `minh chứng` | `evidence`, `bằng chứng` |
| Upload action in workflow | `Tải minh chứng` | `Upload evidence` |
| Single workflow file | `Tệp minh chứng` | `Evidence file` |
| Evidence list/gallery | `Thư viện minh chứng` | `Evidence gallery` |
| QA/QC and nghiệm thu | `Minh chứng nghiệm thu`, `minh chứng QA/QC` | `bằng chứng nghiệm thu` |
| Site photo/video | `Ảnh minh chứng`, `video minh chứng` | `photo evidence` |
| Accounting/finance invoice/payment docs | `chứng từ`, `chứng từ kế toán`, `chứng từ thanh toán` | `minh chứng kế toán` when the item is a formal document |
| Contract/payment attachment without verification meaning | `tệp đính kèm`, `tài liệu đính kèm` | `minh chứng` |
| Audit/history note | `minh chứng đã được rà soát` or `chứng từ đã được kiểm tra`, depending on domain | `evidence đã được review` |

## [Closed] Client Workflow Official Template Automation

Status: post-restart runtime UAT passed; UI monitoring deferred by user

Will be done:
- Phase 1: monitor/review the newly applied template in the running app and address any client-flow data corrections without mutating the preserved draft template.
- Phase 2: implement backend-only workflow automation events and idempotent hooks for the official client template.
- Phase 2: use existing work item activity/blocker surfaces only; do not add new UI.
- Phase 2: create/link draft domain scaffolds for BOQ procurement, input acceptance/AP, output acceptance/AR, and handover/closure checks without auto-approving, auto-passing acceptance, or auto-paying/receiving money.
- Phase 2: add a new migration after V38; do not edit applied V38.
- Phase 2: do not run seed.
- Phase 3: implement backend-only finance completion scaffolding with no new UI and no seed.
- Phase 3: create idempotent draft payment schedules for AP/AR created by the official workflow automation.
- Phase 3: do not create fake input/output invoice records; use existing activity/blocker surfaces with Vietnamese user-facing messages for real invoice requirements.
- Later phases: UI monitoring/dashboard is deferred because the user explicitly requested backend-only work for Phase 2 and Phase 3.

Done:
- User approved Phase 1 implementation and migration run with `OK triển khai Phase 1 và sau đó chạy migrate nếu có`.
- Read workflow orchestration, backend workflow, database workflow, impact analysis, and relevant domain maps.
- Confirmed the current engine clones template nodes/steps into runtime work items and workflow form instances.
- Confirmed current edge routing has no form-answer condition and chooses the first matching edge by priority.
- Confirmed work item completion already enforces required evidence/workflow form gates.
- Added migration `backend/migrations/V38__client_workflow_template.sql` for the new published official client template, preserving the existing draft template.
- Added the `client_design_branch_decision` workflow form and `workflow_form_answer_equals` edge routing for Có thiết kế / Không thiết kế.
- Modeled `Đầu Vào: Tổ Đội / Thầu Phụ / Cung cấp vật tư` as the mandatory shared gate after both BOQ branches.
- Updated `backend/seed/seed.sql` so reseeding keeps the official client template and branch-decision form after template table truncation.
- Added backend tests for official template shape, branch routing intent, and workflow form answer matching.
- Ran `cd backend && make migrate-check` successfully.
- Ran `cd backend && make test` successfully.
- Rebuilt/restarted the Docker backend with `docker compose -f docker-compose.yml up -d --build backend`; backend auto-migrated V38 with `applied=1 skipped=37`.
- Verified DB state: `schema_migrations` has version 38 `client workflow template`; official template is published with 12 nodes and 52 steps; form-answer branch edges count is 2; Đầu Vào has `mandatory_gate=true`.
- User approved Phase 2 with `OK triển khai Phase 2 backend trước, không làm UI mới, chỉ dùng activity/blocker hiện có. Không chạy seed.`
- Added migration `backend/migrations/V39__workflow_automation_events.sql` for idempotent workflow automation events and updated the official Đầu Vào -> Đàm phán edge to `procurement_approved`.
- Added backend automation hooks when work items/run steps complete for the official client workflow template only.
- Implemented Phase 2 backend automation actions for BOQ -> procurement generation, mandatory input procurement gate, incoming acceptance/AP draft scaffolding, outgoing acceptance/AR draft scaffolding, and handover/financial closure blocker checks.
- Automation records outcomes into `workflow_automation_events`, work item activity, and audit log; blocker outcomes use the existing blocker activity surface.
- Added backend integration tests for blocked BOQ automation idempotency, procurement generation idempotency, official template edge condition, and outgoing acceptance/receivable draft scaffolding.
- Ran `cd backend && make migrate-check` successfully after V39.
- Ran focused workflow automation tests with `cd backend && GOCACHE="$PWD/.gocache" go test ./internal/core -run 'TestClientWorkflowAutomation|TestClientOfficialWorkflow'` successfully.
- Ran full backend tests with `cd backend && GOCACHE="$PWD/.gocache" make test` successfully.
- Rebuilt/restarted the Docker backend with `docker compose -f docker-compose.yml up -d --build backend`; backend auto-migrated V39 with `applied=1 skipped=38`.
- Verified DB state: `schema_migrations` has versions 38 and 39; `workflow_automation_events` exists; official Đầu Vào -> Đàm phán edge is `procurement_approved`.
- Verified API health with `curl -sS http://localhost:8080/api/health`.
- Did not run `make seed`.
- User approved Phase 3 with `OK triển khai Phase 3 chỉ phía máy chủ, không làm giao diện mới, không chạy seed, không tạo hóa đơn giả; tạo lịch thanh toán/thu tiền dạng nháp và ghi blocker tiếng Việt cho hóa đơn thật.`
- Implemented Phase 3 backend-only finance automation: incoming acceptance/AP now creates an idempotent draft payable payment schedule, and outgoing acceptance/AR now creates an idempotent draft receivable payment schedule.
- Kept invoice handling as blockers only: no fake input/output invoice records are created, and the blocker messages for required real invoices are Vietnamese.
- Updated official workflow automation messages that can surface to users to Vietnamese for procurement approval and handover/financial-closure blockers.
- Added backend integration coverage for incoming payment schedule + real invoice blocker and outgoing collection schedule + real invoice blocker idempotency.
- Ran `cd backend && make fmt` successfully.
- Ran focused workflow automation tests with `cd backend && GOCACHE="$PWD/.gocache" go test ./internal/core -run 'TestClientWorkflowAutomation|TestClientOfficialWorkflow'` successfully after Phase 3.
- Ran `cd backend && make migrate-check` successfully after Phase 3.
- Ran `git diff --check` successfully.
- Ran full backend tests with `cd backend && GOCACHE="$PWD/.gocache" make test` successfully after Phase 3.
- Did not run `make seed` for Phase 3.
- Continued execution after the Goal UI had been marked blocked; verified the active Docker stack and DB directly without running seed.
- Verified live DB has 39 applied migrations, including V38 `client workflow template` and V39 `workflow automation events`.
- Verified the official client template and the preserved draft template coexist in DB: official template `40000000-0000-0000-0000-000000000020` is published and draft template `75151291-ae2e-449e-83f7-57999a43baf0` remains draft.
- Created live backend verification project `AUTO-P4-20260704` through the API and started the official workflow template through the API.
- Verified the official runtime clones 12 nodes, 52 steps, 52 work items, and 51 workflow form instances.
- Completed two checklist-only incoming acceptance work items through the API with checklist payload only; no fake invoice or seed data was created.
- Verified automation recorded blocker events and activity using Vietnamese text for missing approved input procurement: `Chưa có yêu cầu mua/đầu vào đã duyệt để mở nghiệm thu đầu vào`.
- Cleaned the first verification blocker row that was created before backend rebuild so the verification project no longer displays the old English word `procurement` in blocker messages.
- Verified the verification project has 0 input invoices and 0 output invoices.
- Created, submitted, and approved live procurement request `PR-AUTO-P4-001` through the API for verification project `AUTO-P4-20260704`.
- Completed another checklist-only `Nghiệm thu Đầu Vào` work item through the API after procurement approval; verified automation created 1 incoming acceptance, 1 draft payable, draft payable schedule `LICH-THANH-TOAN-001`, blocker `Cần tải lên và xác minh hóa đơn đầu vào thật`, and still 0 input invoices.
- Completed one checklist-only `Nghiệm thu Đầu Ra` work item through the API; verified automation created 1 outgoing acceptance, 1 draft receivable, draft receivable schedule `LICH-THU-TIEN-001`, blocker `Cần tải lên và phát hành hóa đơn đầu ra thật`, and still 0 output invoices.
- Completed checklist-only `Kết thúc: Bàn giao + Đóng tài chính` work items through the API; verified automation creates/reuses a draft handover record and records blocker `Bàn giao/đóng tài chính còn điều kiện chưa hoàn tất` without creating payments or invoice records.
- After user ran `make restart`, ran disposable runtime UAT project `S5-UAT-POST-RESTART-20260706045536` through the active backend API.
- Verified post-restart node-grain idempotency in active compose runtime: incoming event `event_rows=1/run_count=8`, outgoing event `event_rows=1/run_count=5`, handover event `event_rows=1/run_count=2`, all with `source_type=pipeline_run_node`.
- Verified post-restart scaffold counts: 1 incoming acceptance, 1 payable, 1 payable schedule, 1 outgoing acceptance, 1 receivable, 1 receivable schedule, and 1 handover.
- Verified post-restart safety counts remain zero for input invoices, output invoices, payments, and stock movements.
- Replaced the remaining user-facing handover blocker text `Kho công trình chưa closure` with Vietnamese text `Kho công trình chưa đóng`, rebuilt backend, and verified the new blocker metadata is Vietnamese.
- Ran final `git diff --check` successfully after live verification note updates.
- Re-verified DB state after live checks: 39 migrations applied, official template remains published, preserved draft template remains draft, verification project has 1 payable schedule, 1 receivable schedule, 0 input invoices, and 0 output invoices.
- Rebuilt/restarted the backend container after message hardening; backend auto-migrate reported `applied=0 skipped=39`.
- Ran `cd backend && GOCACHE="$PWD/.gocache" make test` successfully after the final Vietnamese blocker fix.
- Verified backend health with `curl -sS http://localhost:8080/api/health` after the final rebuild.
- Hardened user-facing workflow edge condition labels/reasons toward Vietnamese while preserving internal enum keys.
- Verified `/api/health` and `/api/pipeline-edge-conditions` after rebuild.
- Ran `cd backend && make fmt` successfully after Phase 4/5 hardening.
- Ran focused workflow tests with `cd backend && GOCACHE="$PWD/.gocache" go test ./internal/core -run 'TestClientWorkflowAutomation|TestClientOfficialWorkflow|TestPipelineEdge'` successfully.
- Ran `cd backend && make migrate-check` successfully after Phase 4/5 hardening.
- Ran `git diff --check` successfully after Phase 4/5 hardening.
- Ran full backend tests with `cd backend && GOCACHE="$PWD/.gocache" make test` successfully after Phase 4/5 hardening.
- Ran S2 real workflow UAT on projects `S2-UAT-DESIGN-20260706033656` and `S2-UAT-NO-DESIGN-20260706033656` through API/DB only, with no seed, no browser, no fake invoices, and no fake payments.
- Verified both UAT projects clone the official template into 12 runtime nodes and 52 work items.
- Verified branch routing: `needs_design=yes` routes to `Thiết kế`; `needs_design=no` routes to `BOQ (Không thiết kế)`.
- Verified mandatory `Đầu Vào` gate blocks before procurement approval and advances after all project procurement requests are approved.
- Verified live finance safety: each UAT project has 0 input invoices, 0 output invoices, and 0 payments after automation.
- Verified expected finance blockers are Vietnamese for real input invoice, real output invoice, and handover/financial closure readiness.
- Verified construction remains blocked at `QA & QC / Thi công` without fake receipt/payment because the project has not collected at least 30% of contract value.
- Captured UAT follow-up defects/gaps: `GET /projects/:id/pipeline-run` returns `steps: null` due `rs.created_at` in the step query; BOQ procurement automation cannot run from checklist-only BOQ steps without real `boq_items`; acceptance/AR automation currently triggers per work item and needs S4 idempotency/grain review.
