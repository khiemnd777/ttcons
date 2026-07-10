# Consflow Module Keyword Map

Use this as the mandatory central routing cache. It is not a replacement for reading code. If a mapping conflicts with code, trust code and update this file when the correction is durable.

## Durable Memory Sources

- Durable repo rules: `AGENTS.md`.
- Durable routing and domain memory: this file plus `.agents/skills/consflow-*-domain/references/domain-map.md`.
- Durable backend function/helper map: `.agents/skills/consflow-module-memory/references/backend-function-map.md`.
- Durable frontend UI/utility/helper map: `.agents/skills/consflow-module-memory/references/frontend-ui-utility-map.md`.
- Durable diagnosis search map: `.agents/skills/consflow-module-memory/references/root-cause-triage-map.md`.
- Temporary handoff memory: `docs/current-states.md` only while a plan is active.
- Historical records: `docs/plan-closed/*`. Do not require closed plans for future routing, feature, bugfix, or impact analysis; if a closed plan contains durable knowledge, promote the useful part into this map or a domain map.
- Final verification source: current code, migrations, and tests. If durable memory conflicts with code, trust code and update memory when the correction is durable.

## Mandatory Bootstrap Sequence

1. Load this central map before broad scans, diagnosis, review, implementation planning, or code edits.
2. Match the prompt to a feature, domain, route, table, function group, frontend surface, shared utility, or root-cause symptom.
3. Load the targeted detail map:
   - Backend/API/schema/function work: `backend-function-map.md`.
   - Frontend/UI/helper/API usage work: `frontend-ui-utility-map.md`.
   - Bug/regression/performance/root-cause work: `root-cause-triage-map.md`.
   - Business-domain invariants: the matching `consflow-*-domain/references/domain-map.md`.
4. Start with owner files from the map, then run targeted `rg` searches in those paths.
5. State the matched feature/domain, mapping files loaded, owner files, and first searches before presenting a review package, diagnosis, implementation plan, or review findings.
6. If a subagent is spawned, pass the matched maps and owner files to that subagent and require the same bootstrap.

## Mapping Maintenance Gate

Any code change that creates, updates, deletes, renames, moves, or changes behavior for a mapped surface must update mapping in the same work turn.

Update this central map, a detail map, or a domain map when touching:

- feature/module ownership, routes, handlers, services, repositories, request/response types
- frontend routes, pages, dialogs, panels, components, hooks, helpers, API calls, shared types
- SQL tables, migrations, seed data, status/enum values, permissions, reports, CSV exports
- workflow gates, approval states, audit/activity behavior, stock/finance/acceptance invariants
- root-cause triage keywords, search recipes, or agent/subagent workflow

If no mapping update is required after a code change, state the reason in the final response or review notes.

## Stable Entry Points

- Repo rules: `AGENTS.md`.
- Backend route registration: `backend/internal/core/routes.go`.
- Backend handler/service/repository path: `backend/internal/core/handlers.go`, `backend/internal/core/service.go`, `backend/internal/core/repository.go`, `backend/internal/core/repository_domain.go`.
- Backend auth, response, DB, migrations: `backend/internal/shared`.
- SQL migrations: `backend/migrations/V<N>__description.sql`; add a new migration, never edit an applied migration.
- Frontend route registration: `frontend/src/app/router.tsx`.
- Frontend feature screens: `frontend/src/modules/<feature>`.
- Frontend API/auth/types/utils: `frontend/src/lib/api/client.ts`, `frontend/src/lib/auth/AuthContext.tsx`, `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils`.
- Shared frontend UI: `frontend/src/components`, `frontend/src/modules/_shared`.
- Docker/local stack: `docker-compose.yml`, `Makefile`, frontend service/container name `admin`.

## Domain Skill Routing

Use this table before the feature table when a prompt has business-domain words. Pair the domain skill with backend/frontend workflow skills as needed.

| Keywords and aliases | Primary domain skill | Notes |
|---|---|---|
| pipeline builder, workflow template, pipeline run, work item, approval, workflow form, form version, publish form, signature, PDF export, required_role_ids | `consflow-workflow-orchestration-domain` | Owns template-vs-runtime, work item gates, workflow form governance, signatures, PDF artifacts, and activity/evidence gates. |
| project, cong trinh, project member, planning, ke hoach, daily report ke hoach, meeting minutes, bien ban hop, planning checklist, project activity | `consflow-project-planning-domain` | Owns project setup and planning report workspaces backed by workflow form runtime. |
| thi cong, operations, site report, construction daily report, issue, evidence, photo, checklist thau phu, checklist chu dau tu, crew, actual quantity | `consflow-construction-operations-domain` | Owns field execution and evidence-heavy work item surfaces. Pair with materials/quality when quantities or acceptance are affected. |
| BOQ, du toan, estimate, unit price, don gia, supplier quotation, bao gia nha cung cap, quote comparison, so sanh bao gia, contract, hop dong, hợp đồng, contract version, milestone, variation, phu luc, phụ lục, retention, procurement, mua hang, purchase request, subcontractor reconciliation, owner/client reconciliation, variance | `consflow-commercial-procurement-domain` + `consflow-accounting-finance-domain` | Owns commercial quantity/price/quotation/contract/procurement flow. Contract milestones and retention cross into AP/AR. |
| material, vat tu, warehouse, kho, stock, ton kho, issue, receive, transfer, return, site material balance, material snapshot, before after, surplus, custody | `consflow-materials-warehouse-domain` | Owns stock movement traceability, site balances, material snapshots, and surplus handling. |
| quality, QA/QC, ITP, inspection test plan, RFI, inspection request, NCR, non-conformance, recurring inspection, acceptance, nghiem thu, incoming acceptance, outgoing acceptance, pass, conditional pass, fail, punch list, rework, replanning, handover, ban giao | `consflow-quality-acceptance-handover-domain` | Owns ITP revisions, inspection requests, NCR/corrective/reinspection gates, recurring quality work, acceptance history, rework/replanning, and handover readiness. |
| accounting, ke toan, budget, payable, receivable, cong no, payment, receipt, advance, settlement, invoice, hoa don, AP, AR, cashflow, tax, VAT, profit, P&L | `consflow-accounting-finance-domain` | Owns financial state, AP/AR reconciliation, invoice PDFs, payment schedules, advances, cashflow, tax, and profit reports. |
| admin, quan tri, auth, refresh token, session, staff, user, role, permission, RBAC, phan quyen, partner, catalog, core data, settings, audit | `consflow-admin-governance-domain` | Owns identity, permissions, project scopes, partner catalog/master data, settings, and audit. |
| dashboard, reports, bao cao, KPI, analytics, export, CSV, warehouse report, procurement report, accounting report, staff report, tax report, profit report | `consflow-reporting-analytics-domain` | Owns source-backed report/query/export parity. Pair with the source-domain skill for metric semantics. |

## Shared Utilities And UI

| Keywords | Owner files | Notes |
|---|---|---|
| api client, fetch, token, response envelope, request | `frontend/src/lib/api/client.ts` | Use these helpers; do not duplicate auth/fetch logic in modules. Backend returns `{ success, data, message }`. |
| auth context, current user, permissions in UI | `frontend/src/lib/auth/AuthContext.tsx`, `frontend/src/lib/types/domain.ts` | Pair with backend middleware and repository permission helpers. |
| status label, enum chip, badge, formatting | `frontend/src/lib/utils/status.ts`, `frontend/src/lib/utils/format.ts`, `frontend/src/components/StatusBadge.tsx`, `frontend/src/components/EnumChip.tsx`, `frontend/src/components/CurrencyText.tsx` | Update shared mappings when adding statuses or enum values. |
| toast, success, warning, error, dismiss x | `frontend/src/components/ToastProvider.tsx` | Save/edit flows must show dismissible toasts. |
| dialog, modal, create/edit/view detail | `frontend/src/components/Dialog.tsx`, `frontend/src/components/RecordDetailDialog.tsx`, `frontend/src/components/FormDrawer.tsx`, `frontend/src/modules/_shared/SimpleForm.tsx` | Prefer shared dialog/modal flows for create, view, and edit. |
| table, row actions, empty/loading states | `frontend/src/components/DataTable.tsx`, `frontend/src/components/RowActions.tsx`, `frontend/src/components/EmptyState.tsx`, `frontend/src/components/LoadingSpinner.tsx`, `frontend/src/components/PageHeader.tsx`, `frontend/src/components/StatCard.tsx` | Reuse before adding primitives. |
| project selector | `frontend/src/modules/_shared/ProjectSelector.tsx` | Used by scoped/global pages that need project context. |
| frontend API methods, refresh token, blob download, SSE stream | `frontend/src/lib/api/client.ts` | Map API contract changes here and in `frontend-ui-utility-map.md`; do not bypass `get`, `post`, `put`, `del`, `postForm`, `downloadBlob`, or `streamSSE`. |
| frontend domain types | `frontend/src/lib/types/domain.ts` | Update when backend response shapes change. Pair with route, handler, service, repository, and status label changes. |
| pipeline diagram geometry/routing/hit testing | `frontend/src/modules/pipeline-builder/components/diagram/*` | Read `frontend-ui-utility-map.md` before changing connector behavior, drag/drop, adorner, routing, persistence, or canvas interactions. |
| work item execution presentation/navigation | `frontend/src/modules/work-items/workItemPresentation.ts`, `frontend/src/modules/work-items/workItemNavigation.ts`, `frontend/src/modules/work-items/WorkItemPrimarySurface.tsx`, `frontend/src/modules/projects/ProjectWorkspaceLayout.tsx` | Shared by my-work, project work item detail, project operations, planning workspaces, activity surfaces. |

## Feature Routing

| Prompt keywords and aliases | Feature owner | Frontend owner | Backend/API owner | Data owner | Domain checks |
|---|---|---|---|---|---|
| login, auth, me, refresh, logout, dang nhap, tai khoan | Auth | `frontend/src/modules/auth/LoginPage.tsx`, `frontend/src/lib/auth/AuthContext.tsx` | `/auth/login`, `/auth/me`, `/auth/refresh`, `/auth/logout`; handlers `Login`, `Me`; service `Login`, `Me`; repo `FindUserByEmail`, `UserRoles`, `UserPermissions` | `users`, `staff`, `roles`, `permissions`, `user_roles`, `role_permissions` in `V1__init.sql` | Preserve JWT, actor ID/name, protected endpoints under `AuthRequired`. |
| dashboard, home, tong quan, KPI | Dashboard | `frontend/src/modules/dashboard/DashboardPage.tsx` | `/reports/dashboard`; handler/service `Dashboard`; repo `DashboardReport` | Aggregates across projects, approvals, warehouse, procurement, accounting | Keep scoped visibility and non-deleted list semantics. |
| project, projects, cong trinh, du an, project member, client | Projects | `frontend/src/modules/projects/ProjectsPage.tsx`, `frontend/src/modules/projects/ProjectDetailPage.tsx` | `/projects`, `/projects/:id`, `/projects/:id/operations`; handlers `ListProjects`, `CreateProject`, `ProjectOperations`; repo `ListProjects`, `GetProjectDetail`, `ProjectOperations` | `projects`, `project_members`, `audit_logs` | Project-visible lists must respect access filters and hide deleted/cancelled rows. |
| pipeline builder, template, workflow template, node, step, checklist, edge, connector, condition, fail path, publish, clone | Pipeline Builder | `frontend/src/modules/pipeline-builder/PipelineBuilderPage.tsx`, `frontend/src/modules/pipeline-builder/components`, `frontend/src/modules/pipeline-builder/components/diagram` | `/pipeline-templates`, `/pipeline-nodes`, `/pipeline-steps`, `/pipeline-edges`, `/pipeline-edge-conditions`; handlers `ListTemplates`, `PublishTemplate`, `CloneTemplate`, node/step/edge handlers; service `PublishTemplate`, `CloneTemplate`; repo `GetTemplateDetail`, `CreateTemplateWithNodes`, `CloneTemplate` | `pipeline_templates`, `pipeline_template_nodes`, `pipeline_template_edges`, `pipeline_template_steps`; `V1`, `V3`, `V4` | Publishing and cloning templates must not mutate historical project runs. Connector routing has specialized diagram code. |
| diagram, canvas, drag node, adorner, routing, edge persistence, connector layout | Pipeline Builder Diagram | `frontend/src/modules/pipeline-builder/components/diagram/*` | Same pipeline template edge/node endpoints | Same pipeline template tables | UI changes require Mockup as Text. Do not use browser verification unless explicitly requested. |
| planning, pipeline run, runtime pipeline, start pipeline, advance node, block node, submit step | Planning / Pipeline Run | `frontend/src/modules/planning/PlanningPage.tsx`, project detail pipeline actions | `/projects/:id/start-pipeline`, `/projects/:id/pipeline-run`, `/pipeline-run-steps/:id/submit`, `/pipeline-run-nodes/:id/start`, `/advance`, `/block`, `/request-approval`; service `StartPipeline`, `PipelineRun`, `SubmitRunStep`, `AdvanceRunNode`, `BlockRunNode`; repo `StartPipeline`, `GetPipelineRun`, `SubmitRunStep`, `AdvanceRunNode`, `BlockRunNode` | `pipeline_runs`, `pipeline_run_nodes`, `pipeline_run_steps`, `pipeline_run_edges`; `V1`, `V2`, `V3`, `V4` | Runtime copies are project-specific. Respect node permission gates and material/receivable blockers. |
| my work, work item, task, assignee, blocker, evidence, activity, comment, mention, unread, attention, review, watcher | Work Items / Activity | `frontend/src/modules/my-work/MyWorkPage.tsx`, `frontend/src/modules/work-items/*`, project detail work sections | `/me/work-items`, `/work-items/:id`, `/work-items/:id/start|complete|block|resolve-blocker|request-review|request-approval|approve|reject|request-change|evidence|activity`, `/projects/:id/work-items`, `/projects/:id/activity`, `/activities/:id/read`, `/me/activity-attention`; `activity_stream.go`; service `WorkItemAction`, `AddWorkItemEvidence`, `AddWorkItemActivity`, `MyActivityAttention`; repo work item/activity helpers | `work_items`, `work_item_assignments`, `work_item_activities`, `evidence_records`, `activity_mentions`, `activity_read_states`; `V5`, `V6`, `V7`, `V8`, `V9` | Preserve actor/audit, visibility, mentions/unread, evidence requirements, and work item permission gates. |
| approval, approve, reject, request change, phe duyet, duyet | Approvals | `frontend/src/components/ApprovalPanel.tsx`, planning/work-item/project detail surfaces | `/approvals`, `/approvals/pending`, `/approvals/:id/approve|reject|request-change`, node/work-item approval endpoints; service `ApprovalAction`, `RequestNodeApproval`, `WorkItemAction`; repo `ApprovalAction`, `RequestNodeApproval`, `RequestWorkItemApproval` | `approval_rules`, `approval_requests`, `approval_actions`, work item approval references | Approval actions must preserve actor, status, comments, and timestamps. |
| BOQ, boq, du toan, estimate, cost item, supplier quotation, bao gia nha cung cap, quote comparison, so sanh bao gia, select quote, generate procurement, sinh procurement | BOQ / Estimate / Supplier quotation | `frontend/src/modules/boq/BOQPage.tsx`, `frontend/src/modules/boq/SupplierQuotationWorkspace.tsx` | BOQ routes plus `/projects/:id/supplier-quotations`, `/projects/:id/supplier-quote-comparison`, `/supplier-quotations/:id/submit|qualify`, `/supplier-quotation-items/:id/select`; service/repo quotation save/action/comparison/selection and `GenerateProcurementFromBOQ` | `boq_items`, `boq_unit_price_events`, `supplier_quotations`, `supplier_quotation_items`, `supplier_quote_selections`, `procurement_requests`, `procurement_request_items`, `audit_logs` | Submitted quote items are immutable. Quote selection requires approved BOQ + norm lock + qualified quote, records the decision, and locks final price. Manual final lock requires a waiver reason. |
| procurement, mua hang, yeu cau mua, purchase request, order, receive, partial receive, PO | Procurement | `frontend/src/modules/procurement/ProcurementPage.tsx` | `/procurement-requests`, `/procurement-requests/:id`, `/submit|approve|order|receive`; handlers `ListProcurement`, `CreateProcurement`, `UpdateProcurement`, `ProcurementAction`; service `CreateProcurement`, `UpdateProcurement`, `ProcurementAction`; repo detail/list/action | `procurement_requests`, `procurement_request_items`, `supplier_quotation_items`, `incoming_acceptance_records`, `incoming_acceptance_items` | Generation groups requests per selected supplier. Ordering requires supplier/PO/expected delivery. Receive derives partial/full status only from submitted incoming acceptance; it does not create stock movement. |
| contract, hop dong, hợp đồng, contract version, milestone hop dong, variation, phu luc, phụ lục, retention | Contract lifecycle | `frontend/src/modules/contracts/ContractsPage.tsx`, contract primary surface in `WorkItemPrimarySurface.tsx` | `/projects/:id/contracts`, `/contracts/:id`, contract version/milestone/variation/retention action routes; handlers/services/repositories named `Contract*` | `contracts`, `contract_versions`, `contract_milestones`, `contract_variations`, `contract_retentions`, `contract_retention_releases`, `payables`, `receivables`, `projects.contract_amount` compatibility snapshot | Signed versions are immutable. Variation creates a new draft revision. Milestones open idempotent AP/AR only after active/eligible gates. Retention uses an append-only release ledger. |
| warehouse, kho, vat tu, material, stock, ton kho, xuat kho, nhap kho, transfer, return, material balance | Warehouse / Materials | `frontend/src/modules/warehouse/WarehousePage.tsx` | `/materials`, `/warehouses`, `/stock`, `/stock/receive|issue|transfer|return-to-warehouse|return-to-subcontractor`, `/projects/:id/material-balances`; handler `Stock`, `stockAction`, `MaterialBalances`; service `CreateStockMovement`; repo `CreateStockMovement`, `ProjectMaterialBalances` | `materials`, `warehouses`, `stock_movements`, `site_material_balances` | Preserve material balance and stock movement traceability. Construction nodes may block when material is insufficient. |
| operations, daily report, site report, issue, van de, nhat ky | Operations | `frontend/src/modules/operations/OperationsPage.tsx` | `/projects/:id/daily-reports`, `/projects/:id/issues`, `/issues/:id`; handlers `ProjectDailyReports`, `CreateDailyReport`, `ProjectIssues`, `CreateIssue`, `UpdateIssue` | `operation_daily_reports`, `issues`, `audit_logs` | Project-scoped access matters. Closed/deleted issue semantics should stay consistent. |
| quality, QA/QC, ITP, RFI, NCR, recurring inspection, corrective action, reinspection | Quality workspace | `frontend/src/modules/quality/QualityPage.tsx`, quality primary surface in `WorkItemPrimarySurface.tsx` | `/projects/:id/quality` plus quality ITP/RFI/NCR/recurring action routes; service/repo quality lifecycle methods | `quality_itp_plans`, `quality_itp_items`, `quality_inspection_requests`, `quality_ncrs`, `construction_recurring_plans`, `construction_recurring_occurrences`, `rework_requests`, `evidence_records` | Rejected RFI creates NCR/rework atomically. NCR close requires accepted reinspection and completed rework. Recurring actions respect evidence gates. |
| acceptance, nghiem thu, checklist, pass, fail, punch list | Acceptance | `frontend/src/modules/acceptance/AcceptancePage.tsx` | `/projects/:id/acceptances`, `/acceptances/:id`, `/pass`, `/fail`, `/create-rework`; handlers `ProjectAcceptances`, `CreateAcceptance`, `GetAcceptance`, `AcceptanceAction`, `CreateReworkFromAcceptance`; service/repo `AcceptanceAction`, `CreateReworkFromAcceptance` | `acceptance_records`, `acceptance_checklist_items`, `rework_requests` | Acceptance failures can create rework. Do not silently skip acceptance history. |
| rework, lam lai, sua loi, punch, replanning, ke hoach lai | Rework / Replanning | `frontend/src/modules/rework/ReworkPage.tsx` | `/projects/:id/reworks`, `/reworks/:id/approve|complete`, `/projects/:id/replannings`, `/replannings/:id/approve`; handlers `ProjectReworks`, `CreateRework`, `ApproveRework`, `CompleteRework`, replanning handlers | `rework_requests`, `replanning_requests`, `acceptance_records`, `pipeline_run_nodes` | Rework completion should preserve acceptance history and not skip workflow blockers. |
| handover, ban giao, close project, closure, handover checklist, readiness snapshot | Handover package | `frontend/src/modules/handover/HandoverPage.tsx`, handover primary surface in `WorkItemPrimarySurface.tsx` | handover list/options/detail/create/submit/check/checklist/complete routes; service/repo package lifecycle methods | `handover_records`, `handover_checklist_items`, exact final acceptance, evidence/documents, quality/accounting/warehouse state | Checklist booleans are derived, not self-attested. Exact final acceptance, required evidence/documents, QA/rework/material/finance blockers and fresh readiness must pass before `handed_over`. |
| accounting, budget, ngan sach, payment, receipt, thu chi, advance | Accounting | `frontend/src/modules/accounting/AccountingPage.tsx` | `/projects/:id/budget`, `/budgets/:id/approve`, `/payments`, `/payments/:id/approve`; handlers `ProjectBudget`, `CreateBudget`, `ApproveBudget`, `CreatePayment`, `ApprovePayment`; service/repo `PaymentAction` | `accounting_budgets`, `payments`, `advance_requests`, `audit_logs` | Preserve financial approval and actor trail. |
| debt, cong no, payable, receivable, phai tra, phai thu | Debt / AR AP | `frontend/src/modules/debt/DebtPage.tsx` | `/payables`, `/payables/:id/approve|pay`, `/receivables`, `/receivables/:id/approve|receive`; handlers `ListPayables`, `CreatePayable`, `ApprovePayable`, `PayPayable`, receivable handlers; service/repo `PayableAction`, `ReceivableAction` | `payables`, `receivables`, `payments` | Pipeline construction can depend on receivable thresholds. Keep payable/receivable statuses and payment links consistent. |
| staff, nhan su, employee, user, role, permission, RBAC, phan quyen | Staff / RBAC | `frontend/src/modules/staff/StaffPage.tsx`, `frontend/src/modules/rbac/RolesPage.tsx`, `frontend/src/modules/rbac/PermissionsPage.tsx` | `/staff`, `/roles`, `/permissions`, `/roles/:id/permissions`, `/users/:id/roles`; permission helpers in handlers/repository | `staff`, `users`, `roles`, `permissions`, `role_permissions`, `user_roles`, `project_members` | Preserve auth/RBAC and project-scoped permission semantics. |
| audit, log, workflow playback, history, ai lam gi | Audit Log | `frontend/src/modules/audit/AuditPage.tsx` | `/audit-logs`, `/projects/:id/audit-logs`; handler/service `AuditLogs`; repo `AddAudit`, `AuditLogs` | `audit_logs`, many domain tables | Changes in domain actions should add/keep audit entries where local pattern already does. |
| reports, bao cao, project report, warehouse report, procurement report, accounting report, staff report | Reports | `frontend/src/modules/reports/ReportsPage.tsx` | `/reports/projects`, `/reports/warehouse`, `/reports/procurement`, `/reports/accounting`, `/reports/staff`; handler `report`; service/repo `Report` | Aggregates across domain tables | Check scoped visibility and stale status labels. |
| settings, cau hinh, system settings | Settings | `frontend/src/modules/settings/SettingsPage.tsx` | `/settings`; handler `Settings` | `system_settings` | Keep settings read/write behavior consistent with existing endpoint shape. |
| migrations, schema, seed, database, bang, SQL | Database / Migrations | No single frontend owner | `backend/internal/shared/migrations`, `backend/cmd/migrate`, repository SQL in `backend/internal/core` | `backend/migrations`, `backend/seed/seed.sql` | Never use Flyway. Add a new ordered migration rather than editing applied files. |
| docker, compose, make up, restart, admin service, ports | Docker Ops | No feature page | `docker-compose.yml`, `Makefile`, backend/frontend Dockerfiles | PostgreSQL volume, Redis, backend, admin service | Preserve `make up` and `make restart` as down -> build -> run; keep frontend service/container as `admin`. |

## Production-Grade Feature Surfaces Added After V10

- Workflow forms: `frontend/src/modules/workflow-forms`, `/workflow-form-templates`, `/workflow-form-template-library`, `/workflow-form-versions/:id/*`, `/workflow-form-instances/:id/*`, tables `workflow_form_*`, migrations `V10`, `V11`, `V12`, `V13`, `V14`, `V16`.
- Evidence and protected files: `frontend/src/modules/work-items/EvidencePanel.tsx`, `EvidenceGallery.tsx`, `/work-items/:id/evidence/upload`, `/evidence/:id/file|accept|reject`, tables `evidence_records`, `documents`, migration `V15`.
- Planning workspaces: `frontend/src/modules/planning-daily-reports`, `meeting-minutes`, `planning-checklists`, routes `/planning/daily-reports`, `/planning/meeting-minutes`, `/planning/checklists`.
- Construction work item panels: `SubcontractorReconciliationPanel.tsx`, `OwnerClientReconciliationPanel.tsx`, `MaterialSnapshotPanel.tsx`, `SurplusHandlingPanel.tsx`, `IncomingAcceptancePanel.tsx`, `OutgoingAcceptancePanel.tsx`.
- Commercial/material/quality migrations: `V17` through `V26` cover unit price lifecycle, subcontractor/client checklist forms, construction daily report, reconciliation, material snapshots, surplus, incoming acceptance, and outgoing acceptance hardening.
- Accounting/debt panels: `frontend/src/modules/debt/InputInvoicePanel.tsx`, `OutputInvoicePanel.tsx`, `InputPaymentPlanPanel.tsx`, `OutputBillingPlanPanel.tsx`, `PayableReconciliationPanel.tsx`, `ReceivableReconciliationPanel.tsx`.
- Finance/report migrations: `V27` through `V35` cover input/output invoice PDFs, payment schedules, advances, AP/AR reconciliation indexes, cashflow, tax, and profit report indexes.
- Core/admin hardening: `V36` partner catalog and `V37` auth session lifecycle.

## Production-Grade Operating Gates

- Mandatory mapping bootstrap applies before feature, bugfix, review, root-cause, or code-changing work.
- Mapping maintenance applies after code changes. Update mapping immediately when code changes affect feature/function/helper/UI/route/contract/table/status/domain behavior; otherwise state why no mapping update was required.
- Do not add placeholder-only, demo-only, or misleading UI for production features. If a feature cannot be completed end to end, defer the incomplete surface instead of shipping a fake affordance.
- Start every feature or bugfix by routing through this map, then read the relevant domain map and owner code. Check backend route, handler, service, repository, frontend route/module, shared types/status labels, migrations, and tests when the surface crosses those boundaries.
- Durable features need real persistence, validation, permissions, actor/audit or activity history where the local pattern requires it, user feedback, loading/empty/error states, and backend/frontend contract agreement.
- UI permission states, navigation, and action readiness must mirror backend permission checks but never replace backend enforcement.
- Project-scoped and user-visible lists must enforce visibility and non-deleted/non-cancelled/non-inactive semantics at the API/query level.
- Report filters, dashboard cards, warnings, and CSV exports must use the same source-backed query semantics.

## Backend Search Hints

- Load `backend-function-map.md` for route -> handler -> service -> repository -> table/function ownership.
- Route first: `rg -n "keyword|RouteName" backend/internal/core/routes.go`.
- Handler next: `rg -n "func \\(h \\*Handler\\).*Keyword|/route-fragment|TableName" backend/internal/core/handlers.go`.
- Service next: `rg -n "func \\(s \\*Service\\).*Keyword|DomainAction" backend/internal/core/service.go`.
- Repository next: `rg -n "func \\(r \\*Repository\\).*Keyword|table_name|DomainAction" backend/internal/core/repository_domain.go backend/internal/core/repository.go`.
- Migration/table next: `rg -n "table_name|column_name|CREATE TABLE|ALTER TABLE" backend/migrations`.

## Frontend Search Hints

- Load `frontend-ui-utility-map.md` for route -> page -> component/helper/API/type ownership.
- Route/page first: `rg -n "path:|<PageName>|/api-fragment" frontend/src/app/router.tsx frontend/src/modules`.
- API calls: `rg -n "get\\(|post\\(|put\\(|delete\\(|/api-fragment" frontend/src/modules frontend/src/lib`.
- Shared presentation/status: `rg -n "status|label|EnumChip|StatusBadge|toast" frontend/src/components frontend/src/lib frontend/src/modules`.
- Work-item UI is shared across `my-work`, project detail, and activity surfaces; inspect `frontend/src/modules/work-items` before duplicating components.

## Root-Cause Triage Hints

- Load `root-cause-triage-map.md` before bugfixes, regressions, slow flows, stale UI, failed status transitions, missing rows, upload/download issues, permissions, or report discrepancies.
- Always split evidence by backend route/API behavior, frontend UI behavior, database state, runtime state, and mapping assumptions.
- If root cause is unresolved, stop at diagnosis/review and ask for clarification or approval of the unresolved risk before editing.

## High-Risk Domain Invariants

- Pipeline templates are reusable definitions; pipeline runs are project-specific runtime copies.
- Publishing and cloning templates must not mutate historical project runs.
- Approval actions must preserve actor, status, comments, and timestamps.
- Workflow form published versions must remain immutable; runtime instances keep stable references.
- `required_role_ids` must remain a multi-role array across templates, runtime steps, work items, and UI.
- BOQ approval can generate procurement; do not bypass approval states.
- BOQ procurement generation must respect final unit-price locks.
- BOQ final price must trace to a selected qualified supplier quotation or an explicit manual-price waiver reason.
- Procurement receive status must be reconciled from submitted incoming acceptance quantities; stock movement remains an explicit warehouse action.
- `projects.contract_amount` is a compatibility snapshot from the active signed main-client contract; `contracts.current_amount` plus signed `contract_versions` are the source of truth.
- Contract milestone obligations and retention releases must create exactly one traceable AP/AR source record per action and preserve audit.
- Procurement receive, stock receive/issue/transfer/return, and site material balances must preserve material traceability.
- Material snapshots and incoming acceptance must not create implicit warehouse movement unless the explicit local contract says so.
- Submitted construction daily-report workflow forms sync idempotently into the operations ledger and retain the exact form/work-item/evidence source plus structured answers.
- Active ITP revisions are immutable; rejected inspection history remains linked through NCR, rework, corrective evidence, and reinspection before closure.
- Manual AP/AR and generic payment creation cannot accept client-owned status or derived balance fields; server normalization and actor audit are mandatory.
- Handover packages preserve exact acceptance/checklist/evidence sources and a readiness snapshot; historical handovers are backfilled with the same required checklist.
- Acceptance failure can create rework; rework completion must not erase acceptance history.
- Handover completion must respect readiness blockers.
- Accounting/payables/receivables/payments/invoices/advances must reconcile by project and source document.
- Cashflow, tax, profit, and CSV exports must stay source-backed and permission-scoped.
- Partner catalog changes must preserve downstream snapshot fields.
- Auth refresh/revoke lifecycle must remain DB-backed and protected.
- User-visible lists must exclude deleted/cancelled/inactive records at the API/query level.
