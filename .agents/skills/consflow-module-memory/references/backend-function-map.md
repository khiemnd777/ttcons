# Consflow Backend Function Map

Use this map after `module-keyword-map.md` for backend/API/schema/function work. This is a routing cache for fast ownership discovery, not a substitute for reading code.

## Mandatory Backend Workflow

1. Start in `backend/internal/core/routes.go` to confirm the API path and handler.
2. Read the handler in `backend/internal/core/handlers.go` for request parsing, actor extraction, response envelope, and service call.
3. Read the matching service in `backend/internal/core/service.go` for validation, permission checks, normalization, file handling, and domain transition orchestration.
4. Read repository functions in `backend/internal/core/repository_domain.go` or generic helpers in `backend/internal/core/repository.go` for SQL and state persistence.
5. Check request/response structs in `backend/internal/core/types.go`.
6. Check migrations in `backend/migrations` and seed data in `backend/seed/seed.sql` when tables, statuses, permissions, defaults, indexes, or demo data are involved.
7. Update this map or the central map when adding, renaming, moving, or deleting backend routes/functions/helpers.

## Stable Backend Layers

| Layer | Owner files | Purpose | Mapping update trigger |
|---|---|---|---|
| Route registration | `backend/internal/core/routes.go` | API paths, auth protection, handler binding | Any route/path/method/handler change |
| Handlers | `backend/internal/core/handlers.go` | Fiber request parsing, actor/context extraction, response envelope | New handler, action endpoint, upload/download endpoint, request parsing change |
| Services | `backend/internal/core/service.go` | Business validation, permission checks, transitions, normalization, file/PDF handling | New transition, validation, helper, permission rule, generated file behavior |
| Repositories | `backend/internal/core/repository_domain.go`, `backend/internal/core/repository.go` | SQL persistence, transactions, query filters, audit/activity writes | SQL shape, table ownership, soft-delete filter, transaction, report query, derived state |
| Shared backend | `backend/internal/shared` | config, DB, cache, auth middleware, migration runner, response envelope | Auth, migration, response, DB/cache behavior changes |
| Types | `backend/internal/core/types.go` | API requests, filters, payload types, file metadata | Request/response contract change |
| Files/PDF | `backend/internal/core/pdf_generator.go`, service file helpers | Generated PDFs, protected downloads, signature visuals, invoice/evidence files | File storage, MIME, PDF, signature, download behavior |
| Activity stream | `backend/internal/core/activity_stream.go` | SSE activity and attention streams | Realtime stream event/data/visibility changes |

## Backend Function Groups

| Keywords | Routes | Handler/service entry points | Repository/function owners | Domain map |
|---|---|---|---|---|
| auth, login, refresh, logout, session, JWT | `/auth/login`, `/auth/refresh`, `/auth/logout`, `/auth/me` | `Login`, `Refresh`, `Logout`, `Me`; service `Login`, `RefreshSession`, `LogoutByRefreshToken`, `Me`, `signToken` | `FindUserByEmail`, `UserRoles`, `UserPermissions`, refresh-session helpers | `consflow-admin-governance-domain` |
| RBAC, permission, project scope, role assignment | `/roles`, `/permissions`, `/roles/:id/permissions`, `/users/:id/roles` | role/user handlers; service pass-through where local pattern applies | `UserHasAnyPermission`, `UserHasAnyProjectPermission`, `UserProjectPermissions`, `ReplaceRolePermissions`, `ReplaceUserRoles`, `ProjectAccessFilter` | `consflow-admin-governance-domain` |
| partners, catalog, vendor/client snapshots | `/partners`, `/partners/:id` | `ListPartners`, `CreatePartner`, `UpdatePartner`; service `ListPartners`, `CreatePartner`, `UpdatePartner`, `applyPartnerSnapshot`, `requireActivePartner` | `ListPartners`, generic `Insert`/`Update` | `consflow-admin-governance-domain` |
| projects, members, project detail | `/projects`, `/projects/:id` | `ListProjects`, `CreateProject`, `GetProject`, `UpdateProject`, `DeleteProject`; service `ListProjects`, `CreateProject`, `ProjectDetail`, `UpdateProject`, `DeleteProject` | `ListProjects`, `GetProjectDetail`, `GetProjectSummary`, `RecordProjectID` | `consflow-project-planning-domain` |
| project operations dashboard/sidebar | `/projects/:id/operations` | `ProjectOperations`; service `ProjectOperations` | `ProjectOperations`, `ProjectOperationsWorkflowSidebar`, `ProjectOperationsActivity`, `ProjectOperationsApprovals`, `ProjectOperationsEvidence`, `ProjectOperationsAudit`, `ProjectOperationsDomainCounts` | `consflow-project-planning-domain`, `consflow-workflow-orchestration-domain` |
| pipeline template builder | `/pipeline-templates`, `/pipeline-nodes`, `/pipeline-steps`, `/pipeline-edges`, `/pipeline-edge-conditions` | `ListTemplates`, `CreateTemplate`, `UpdateTemplate`, `PublishTemplate`, `CloneTemplate`, node/step/edge handlers | `GetTemplateDetail`, `CreateTemplateWithNodes`, `CloneTemplate`, `TemplatePublishFormIssues` | `consflow-workflow-orchestration-domain` |
| workflow forms, versions, signatures, PDFs | `/workflow-form-templates`, `/workflow-form-template-library`, `/workflow-form-versions/:id/*`, `/workflow-form-instances/:id/*`, protected signature/PDF routes | workflow-form handlers; service `WorkflowFormCatalog`, `WorkflowFormTemplateDetail`, `CreateWorkflowFormVersion`, `SaveWorkflowFormDraft`, `SubmitWorkflowForm`, `SignWorkflowForm`, `GenerateWorkflowFormPDFExport`, validation helpers | `WorkflowFormCatalog`, `WorkflowFormTemplateLibrary`, `WorkflowFormVersionDetail`, `SaveWorkflowForm`, `SignWorkflowForm`, `CreateWorkflowFormPDFExport`, `DefaultWorkflowFormVersionID` | `consflow-workflow-orchestration-domain` |
| pipeline runtime, run node, run step, advance/block/approval | `/projects/:id/start-pipeline`, `/projects/:id/pipeline-run`, `/pipeline-run-nodes/:id/*`, `/pipeline-run-steps/:id/submit` | `StartPipeline`, `GetPipelineRun`, `StartRunNode`, `SubmitRunStep`, `RequestNodeApproval`, `AdvanceRunNode`, `BlockRunNode`; service counterparts | `StartPipeline`, `GetPipelineRun`, `SubmitRunStep`, `RequestNodeApproval`, `AdvanceRunNode`, `BlockRunNode`, `nextRunNode`, `runEdgeConditionMet`, `handoverReadyCondition` | `consflow-workflow-orchestration-domain` |
| work item actions, evidence, activity, mentions, attention | `/me/work-items`, `/work-items/:id`, `/work-items/:id/start|complete|block|resolve-blocker|request-review|request-approval|approve|reject|request-change|evidence|activity`, `/activities/:id/read`, SSE streams | work-item handlers; service `WorkItemDetail`, `WorkItemAction`, `AddWorkItemEvidence`, `AddWorkItemEvidenceUpload`, `AddWorkItemActivity`, `MarkActivityRead`, `MyActivityAttention` | `GetWorkItemDetail`, `ApplyWorkItemTransition`, `ApplyWorkItemDecision`, `RequestWorkItemApproval`, `AddWorkItemEvidence`, `AddWorkItemActivity`, `WorkItemAccessProfile`, `CanViewWorkItem` | `consflow-workflow-orchestration-domain`, `consflow-construction-operations-domain` |
| planning reports, meeting minutes, checklists | `/planning/daily-reports`, `/planning/meeting-minutes`, `/planning/checklists` | planning handlers; service filter builders and list methods | `PlanningDailyReports`, planning query helpers | `consflow-project-planning-domain` |
| BOQ, unit price lifecycle, procurement generation | `/projects/:id/boq`, `/boq-items/:id/*`, `/projects/:id/boq/generate-procurement` | BOQ handlers; service `ProjectBOQ`, `CreateBOQ`, `UpdateBOQ`, unit-price lock/update methods, `GenerateProcurementFromBOQ` | `ListProjectBOQ`, `GetBOQItemDetail`, `CreateBOQItem`, `UpdateBOQItem`, `changeBOQUnitPrice`, `GenerateProcurementFromBOQ` | `consflow-commercial-procurement-domain` |
| procurement requests, order, receive | `/procurement-requests`, `/procurement-requests/:id/*` | procurement handlers; service `ProcurementAction` | `GetProcurementDetail`, `ListProjectProcurementRequests`, `ListProjectProcurementRequestItems`, `ProcurementAction` | `consflow-commercial-procurement-domain`, `consflow-materials-warehouse-domain` |
| materials, warehouses, stock, balances | `/materials`, `/warehouses`, `/stock`, `/stock/*`, `/projects/:id/material-balances` | material/warehouse/stock handlers; service `CreateMaterial`, `CreateWarehouse`, `CreateStockMovement` | `CreateStockMovement`, `ProjectMaterialBalances`, `ListActiveMaterials`, `ListActiveWarehouses`, `CreateWarehouse` | `consflow-materials-warehouse-domain` |
| material snapshot, surplus handling | `/work-items/:id/material-snapshot`, `/work-items/:id/surplus-handlings`, `/surplus-handlings/:id/*` | service `MaterialSnapshot`, `SaveMaterialSnapshot`, `SubmitMaterialSnapshot`, `SurplusHandlings`, `CreateSurplusHandling`, `UpdateSurplusHandling`, `SubmitSurplusHandling` | material snapshot and surplus repository helpers in `repository_domain.go` | `consflow-materials-warehouse-domain` |
| operations daily reports, issues | `/projects/:id/daily-reports`, `/projects/:id/issues`, `/issues/:id` | operations handlers; service `ProjectDailyReports`, `CreateDailyReport`, `ProjectIssues`, `CreateIssue`, `UpdateIssue` | `ListProjectDailyReports`, `CreateDailyReport`, `ListProjectIssues`, `CreateIssue`, `UpdateIssue` | `consflow-construction-operations-domain` |
| incoming acceptance | `/projects/:id/incoming-acceptances`, `/incoming-acceptances/:id`, `/work-items/:id/incoming-acceptances` | service `ProjectIncomingAcceptances`, `WorkItemIncomingAcceptances`, `GetIncomingAcceptance`, `CreateIncomingAcceptance`, `UpdateIncomingAcceptance`, `SubmitIncomingAcceptance` | `ListIncomingAcceptancesForProject`, `ListIncomingAcceptancesForWorkItem`, `GetIncomingAcceptance`, `SaveIncomingAcceptance`, `SubmitIncomingAcceptance` | `consflow-quality-acceptance-handover-domain`, `consflow-materials-warehouse-domain` |
| outgoing acceptance, pass/fail, rework creation | `/projects/:id/acceptances`, `/acceptances/:id/*`, `/work-items/:id/outgoing-acceptance` | service `ProjectAcceptances`, `WorkItemOutgoingAcceptance`, `CreateAcceptance`, `SubmitAcceptance`, `AcceptanceAction`, `CreateReworkFromAcceptance` | `ListProjectAcceptanceRecords`, `ListAcceptanceRecordsForWorkItem`, `GetAcceptanceDetail`, `SaveAcceptance`, `SubmitAcceptance`, `AcceptanceAction`, `CreateReworkFromAcceptance` | `consflow-quality-acceptance-handover-domain` |
| rework, replanning | `/projects/:id/reworks`, `/reworks/:id/*`, `/projects/:id/replannings`, `/replannings/:id/approve` | service `ProjectReworks`, `CreateRework`, `UpdateReworkStatus`, `ProjectReplannings`, `CreateReplanning`, `ApproveReplanning` | `ListProjectReworks`, `CreateRework`, `UpdateReworkStatus`, `ListProjectReplannings`, `CreateReplanning`, `ApproveReplanning` | `consflow-quality-acceptance-handover-domain` |
| handover | `/projects/:id/handover`, `/handover/:id/check`, `/handover/:id/complete` | service `ProjectHandover`, `CreateHandover`, `HandoverCheck`, `CompleteHandover` | `ListProjectHandover`, `CreateHandover`, `HandoverCheck`, `CompleteHandover`, `handoverReadyCondition` | `consflow-quality-acceptance-handover-domain` |
| budget, payments, payables, receivables, invoices, schedules, advances, reconciliation | `/projects/:id/budget`, `/payables*`, `/receivables*`, `/payments*`, `/input-invoices*`, `/output-invoices*` | finance/accounting handlers; service `CreateBudget`, `ApproveBudget`, payable/receivable schedule/advance/payment/invoice methods, `PayableAction`, `ReceivableAction`, `PaymentAction` | payable/receivable detail, reconciliation, schedule, advance, payment, invoice, allocation helpers in `repository_domain.go` | `consflow-accounting-finance-domain` |
| reports, dashboard, CSV exports | `/reports/dashboard`, `/reports/projects`, `/reports/warehouse`, `/reports/procurement`, `/reports/accounting`, `/reports/staff`, `/reports/cashflow`, `/reports/tax`, `/reports/profit`, export routes | report handlers; service `Dashboard`, `Report`, `CashflowReport`, `TaxReport`, `ProfitReport` | `DashboardReport`, `Report`, `CashflowReport`, `TaxReport`, `ProfitReport` | `consflow-reporting-analytics-domain` |
| audit logs | `/audit-logs`, `/projects/:id/audit-logs` | `AuditLogs`, `ProjectAuditLogs`; service `AuditLogs` | `AddAudit`, `AuditLogs` and domain action audit calls | `consflow-admin-governance-domain`, source domain |
| settings | `/settings` | `Settings` | settings query helpers or generic repository pattern | `consflow-admin-governance-domain` |

## Shared Backend Helpers

| Helper group | Owner functions/files | Use when | Mapping update trigger |
|---|---|---|---|
| Generic CRUD/query | `Repository.Query`, `QueryOne`, `Exec`, `Insert`, `Update`, `GetByID`, `ListTable` in `repository.go` | Local pattern uses generic table access | Generic SQL contract or soft-delete behavior changes |
| Status and domain errors | `StatusError` in `errors.go`, `response.Fail` in shared response | Returning typed HTTP errors | Error code/status behavior changes |
| Auth middleware | `backend/internal/shared/middleware/auth.go` | Protected routes and user context | JWT/user context/permission extraction changes |
| Response envelope | `backend/internal/shared/response/response.go` | `response.OK`, `Created`, `Fail` | Envelope shape changes; update frontend API client and types |
| Migration runner | `backend/internal/shared/migrations/runner.go` | Schema migrations and checksum behavior | Runner semantics, checksum, ordering, or migration metadata changes |
| Upload/download storage | service helpers `storeEvidenceUpload`, `storeInputInvoicePDF`, `storeOutputInvoicePDF`, `storeGeneratedFile`, `storedFileResult` | Evidence, invoice PDFs, signatures, generated workflow PDFs | File path, storage key, MIME, permission, or download behavior changes |
| Workflow form validation | service `validateWorkflowFormAnswers`, `normalizeWorkflowFormValue`, structured list helpers | Form draft/submit/sign/PDF behavior | Field type, validation, required gate, signature/PDF changes |
| Permission gates | repository `CanViewWorkItem`, `CanActOnWorkItem`, `CanDecideWorkItem`, service `requireWorkItemPermission`, finance/acceptance access helpers | User action readiness, backend authorization | RBAC/project-scope logic changes |
| Automation | `RunClientWorkflowAutomationForWorkItem`, `executeClientWorkflowAutomation`, `automateClient*`, `ensure*ScaffoldTx` | Official client workflow auto-scaffold and downstream state creation | New automation action, scaffold target, status, or source document behavior |

## Backend Root Searches

- Route owner: `rg -n "route-fragment|HandlerName" backend/internal/core/routes.go`.
- Handler owner: `rg -n "func \\(h \\*Handler\\).*Name|route-fragment|service\\.Name" backend/internal/core/handlers.go`.
- Service owner: `rg -n "func \\(s \\*Service\\).*Name|RepositoryFunction|table_name|status_value" backend/internal/core/service.go`.
- Repository owner: `rg -n "func \\(r \\*Repository\\).*Name|table_name|status_value|JOIN table_name" backend/internal/core/repository_domain.go backend/internal/core/repository.go`.
- Types/contracts: `rg -n "RequestName|ResponseType|json:\\\"field_name\\\"" backend/internal/core/types.go`.
- Migrations: `rg -n "table_name|column_name|status_value|CREATE TABLE|ALTER TABLE|CREATE INDEX" backend/migrations`.
- Tests: `rg -n "FeatureName|route-fragment|table_name" backend/internal backend`.

