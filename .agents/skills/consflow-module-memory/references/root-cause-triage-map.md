# Consflow Root-Cause Triage Map

Use this map after `module-keyword-map.md` for bugs, regressions, slow flows, stale UI, failed status transitions, data mismatches, permission failures, upload/download issues, and report discrepancies.

## Mandatory Triage Flow

1. Load the central map and the relevant backend/frontend/domain detail maps.
2. State current behavior, expected behavior, evidence source, likely owner files, and first targeted searches before proposing a fix.
3. Split evidence into UI, API, database, runtime workflow state, permissions, and mapping assumptions.
4. Build the smallest repeatable pass/fail signal when local reproduction is feasible.
5. If root cause is not proven, stop at diagnosis/review and ask for clarification or approval of unresolved risk before editing.
6. After a confirmed code change, apply the mapping maintenance gate.

## Symptom Routing

| Symptom keywords | First maps | Owner files | First targeted searches | High-risk checks |
|---|---|---|---|---|
| login fail, refresh loop, logout, session expired, token, cookie | central, backend, frontend, admin governance | `backend/internal/core/service.go`, `handlers.go`, `repository_domain.go`, `shared/middleware/auth.go`, `frontend/src/lib/api/client.ts`, `AuthContext.tsx` | `rg -n "Login|RefreshSession|Logout|refresh|INVALID_REFRESH_TOKEN|setAuthFailureHandler|auth/me" backend frontend/src/lib` | JWT expiry, refresh token DB state, cookie same-site/secure, frontend retry loop |
| permission denied, hidden button, can not approve, RBAC | central, backend, frontend, admin governance, affected domain | `repository_domain.go`, `service.go`, `router.tsx`, page/component owner | `rg -n "permission|hasAnyPermission|CanAct|CanView|CanDecide|require.*Access|UserHas" backend/internal/core frontend/src` | Backend enforcement must match UI affordance; project scope must not be client-only |
| list shows deleted/cancelled/inactive, missing active rows, stale filter | central, backend, frontend, affected domain | route handler/service/repo, page owner, status utils | `rg -n "status <>|active = true|cancelled|deleted|inactive|List.*|queryKey|statusLabels" backend/internal/core frontend/src` | API-level filters, React query keys, status labels, scoped visibility |
| work item cannot complete/start/block/approve | central, backend, frontend, workflow orchestration | `handlers.go`, `service.go`, `repository_domain.go`, `WorkItemDetailPage.tsx`, `WorkItemPrimarySurface.tsx` | `rg -n "WorkItemAction|missingRequiredEvidence|ApplyWorkItemTransition|ApplyWorkItemDecision|request-review|request-approval|complete" backend/internal/core frontend/src/modules/work-items` | Required evidence, required form submission, permission gates, actor/audit/activity |
| workflow form draft/submit/sign/PDF broken | central, backend, frontend, workflow orchestration | workflow form handlers/services/repos, `WorkflowFormPanel.tsx`, `SignatureCaptureDialog.tsx` | `rg -n "SaveWorkflowForm|SubmitWorkflowForm|SignWorkflowForm|PDFExport|validateWorkflowFormAnswers|workflow-form-instances|WorkflowFormPanel" backend/internal/core frontend/src/modules/work-items` | Published versions immutable, required fields, signature visual, protected PDF download |
| evidence upload/download/review broken | central, backend, frontend, workflow orchestration | upload handlers/services, file helpers, `EvidencePanel.tsx`, `EvidenceGallery.tsx` | `rg -n "UploadWorkItemEvidence|EvidenceFile|AcceptEvidence|RejectEvidence|downloadBlob|postForm|evidence" backend/internal/core frontend/src/modules/work-items` | Storage key, MIME, max file size, permissions, protected blob handling |
| project detail/sidebar slow or blank, workflow sidebar wrong step | central, backend, frontend, project planning, workflow orchestration | `ProjectWorkspaceLayout.tsx`, `ProjectDetailPage.tsx`, `ProjectOperations` service/repo | `rg -n "ProjectOperations|workflow-sidebar|ProjectOperationsSidebarResponse|pipeline_run|work_items|WorkflowSidebar" backend/internal/core frontend/src/modules/projects frontend/src/lib/types/domain.ts` | Full vs lightweight response shape, run step loading, project/work item scoping |
| pipeline node not advancing, wrong branch, blocker incorrect | central, backend, workflow orchestration | `service.go`, `repository_domain.go`, `routes.go`, `PlanningPage.tsx` | `rg -n "AdvanceRunNode|SubmitRunStep|nextRunNode|runEdgeConditionMet|workflowFormAnswerEqualsCondition|handoverReadyCondition|nodeBlockers" backend/internal/core frontend/src/modules/planning` | Template/runtime separation, edge conditions, material/receivable/handover blockers |
| pipeline builder connector, drag, adorner, edge persistence bug | central, frontend, workflow orchestration | `PipelineBuilderPage.tsx`, `components/diagram/*` | `rg -n "connector|port|hitTest|routing|edgePersistence|buildCreateEdgePayload|buildUpdateEdgePayload|nodeRects|Adorner" frontend/src/modules/pipeline-builder` | UI-only change still needs Mockup as Text; browser check only if user explicitly asks |
| BOQ price, unit lock, generate procurement wrong | central, backend, frontend, commercial procurement | BOQ handlers/services/repos, `BOQPage.tsx` | `rg -n "BOQ|UnitPrice|LockBOQ|GenerateProcurementFromBOQ|boq_items|unit_price" backend/internal/core backend/migrations frontend/src/modules/boq` | Approval states, final unit price locks, procurement generation audit |
| procurement order/receive wrong, stock mismatch | central, backend, frontend, commercial procurement, materials warehouse | procurement/stock handlers/services/repos, `ProcurementPage.tsx`, `WarehousePage.tsx` | `rg -n "ProcurementAction|ReceiveProcurement|CreateStockMovement|stock_movements|site_material_balances|movement_type" backend/internal/core backend/migrations frontend/src/modules` | Stock traceability, balances, source document, status transition |
| material snapshot, surplus, incoming acceptance issue | central, backend, frontend, materials warehouse, quality | material/incoming/surplus service/repo, work-item panels | `rg -n "MaterialSnapshot|SurplusHandling|IncomingAcceptance|material_snapshot|surplus|incoming_acceptance" backend/internal/core backend/migrations frontend/src/modules/work-items` | Snapshot must not imply warehouse movement unless explicit; acceptance history |
| outgoing acceptance fail/rework/handover issue | central, backend, frontend, quality handover | acceptance/rework/handover service/repo, panels/pages | `rg -n "AcceptanceAction|CreateReworkFromAcceptance|HandoverCheck|CompleteHandover|acceptance_records|rework_requests|handover" backend/internal/core backend/migrations frontend/src/modules` | Acceptance history, rework continuity, handover readiness blockers |
| payables, receivables, invoice, schedule, advance, settlement mismatch | central, backend, frontend, accounting finance | finance handlers/services/repos, debt panels, domain types | `rg -n "Payable|Receivable|InputInvoice|OutputInvoice|PaymentSchedule|Advance|Reconciliation|PaymentAction|allocations" backend/internal/core backend/migrations frontend/src/modules/debt frontend/src/lib/types/domain.ts` | AP/AR reconciliation, source document linkage, actor/audit, project scope |
| cashflow/tax/profit/report/export discrepancy | central, backend, frontend, reporting analytics, source domain | report handlers/services/repos, report panels | `rg -n "CashflowReport|TaxReport|ProfitReport|ReportCSV|DashboardReport|/reports|downloadBlob|warning" backend/internal/core frontend/src/modules/reports frontend/src/modules/accounting` | Query semantics, date/status filters, permission scope, CSV parity |
| toast missing, required fields not highlighted, validation UX bad | central, frontend | page/dialog owner, `ToastProvider.tsx`, shared form/dialog components | `rg -n "useToast|toast\\.|required|invalid|submitAttempted|warning|error|RecordFormDialog|SimpleForm" frontend/src/modules frontend/src/components` | Repo UI gate requires inline invalid fields and dismissible warning/error/success toasts |
| API 404/405/contract drift | central, backend, frontend | `routes.go`, handler/service, frontend API caller, domain types | `rg -n "route-fragment|HandlerName|api-fragment|TypeName|json:\\\"field\\\"" backend/internal/core frontend/src` | Route method/path, response envelope, frontend domain type, status labels |
| migration checksum, schema missing, seed wrong | central, backend, relevant domain | `backend/migrations`, `shared/migrations/runner.go`, `backend/seed/seed.sql` | `rg -n "schema_migrations|checksum|V[0-9]+__|CREATE TABLE|ALTER TABLE|seed_value|table_name" backend` | Never edit applied migrations; add new migration; custom runner only, no Flyway |
| SSE stream stale/offline/reconnect | central, backend, frontend, workflow orchestration | `activity_stream.go`, `api/client.ts`, `MyWorkPage.tsx` | `rg -n "streamSSE|ProjectActivityStream|MyActivityAttentionStream|text/event-stream|reconnecting|offline" backend/internal/core frontend/src` | Auth header, stream visibility, reconnect behavior, read-state updates |
| Docker/make/local stack issue | central, docker ops | `Makefile`, `docker-compose.yml`, Dockerfiles, env config | `rg -n "make up|restart|admin|ports|depends_on|DATABASE_URL|REDIS|VITE_API_URL" Makefile docker-compose.yml backend frontend` | Preserve down -> build -> run sequence; frontend service/container remains `admin` |

## Diagnosis Output Checklist

For bugfixes and unclear behavior changes, include:

- matched feature/domain/function group/UI surface
- mapping files loaded
- current behavior with code/log/screenshot/reproduction evidence label
- expected behavior
- root cause or unresolved root-cause risk
- falsifiable hypotheses checked or still open
- files likely to change
- implementation plan and validation plan
- mapping maintenance impact

