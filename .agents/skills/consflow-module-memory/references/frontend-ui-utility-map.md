# Consflow Frontend UI And Utility Map

Use this map after `module-keyword-map.md` for React/Vite admin UI, frontend helpers, API usage, status labels, domain types, and presentation behavior.

## Mandatory Frontend Workflow

1. Start in `frontend/src/app/router.tsx` for route, permission guard, and page ownership.
2. Read the page/module under `frontend/src/modules/<feature>`.
3. Read shared components in `frontend/src/components` and shared module helpers in `frontend/src/modules/_shared` before adding a new primitive.
4. Read API helpers in `frontend/src/lib/api/client.ts`; do not duplicate fetch/token/refresh/SSE/blob logic.
5. Read shared types in `frontend/src/lib/types/domain.ts` and status/format utilities in `frontend/src/lib/utils`.
6. For work-item execution surfaces, inspect `frontend/src/modules/work-items` and `frontend/src/modules/projects/ProjectWorkspaceLayout.tsx` before changing layout, navigation, primary panels, evidence, forms, activity, or workflow sidebar behavior.
7. Update this map when adding, renaming, moving, deleting, or changing UI surfaces, helper contracts, API usage patterns, or shared presentation behavior.

## Frontend Route Map

| Route/surface | Page owner | Shared dependencies | Backend owner |
|---|---|---|---|
| `/login` | `modules/auth/LoginPage.tsx` | `lib/auth/AuthContext.tsx`, `lib/api/client.ts` | `/auth/login`, `/auth/me`, refresh/logout routes |
| `/dashboard` | `modules/dashboard/DashboardPage.tsx` | `components/StatCard.tsx`, status/format helpers | `/reports/dashboard` |
| `/projects` | `modules/projects/ProjectsPage.tsx` | `RecordFormDialog`, `SimpleForm`, `partnerUtils`, project/workspace navigation | `/projects`, `/pipeline-templates`, `/partners` |
| `/projects/:id` workspace | `modules/projects/ProjectWorkspaceLayout.tsx`, `ProjectDetailPage.tsx` | work-item cards, activity, evidence, project sidebar, workflow sidebar | `/projects/:id/operations`, `/projects/:id/audit-logs`, `/projects/:id/start-pipeline` |
| `/projects/:id/work-items/:workItemId`, `/work-items/:id` | `modules/work-items/WorkItemDetailPage.tsx` | `WorkItemPrimarySurface`, `WorkflowFormPanel`, evidence/activity panels, navigation/presentation helpers | `/work-items/:id`, work-item action/evidence/form routes |
| `/my-work` | `modules/my-work/MyWorkPage.tsx` | `WorkItemCard`, activity attention panel, SSE status | `/me/work-items`, `/me/activity-attention`, SSE stream |
| `/pipeline-builder`, `/pipeline-builder/:id` | `modules/pipeline-builder/PipelineBuilderPage.tsx`, `components/*`, `components/diagram/*` | diagram routing, hit testing, edge persistence, inspector panels | pipeline template/node/step/edge routes |
| `/workflow-forms` | `modules/workflow-forms/WorkflowFormsPage.tsx` | form library/version dialogs and status labels | workflow form template/version routes |
| `/planning/daily-reports` | `modules/planning-daily-reports/PlanningDailyReportsPage.tsx` | work-item navigation/presentation | `/planning/daily-reports` |
| `/planning/meeting-minutes` | `modules/meeting-minutes/MeetingMinutesPage.tsx` | work-item navigation/presentation | `/planning/meeting-minutes` |
| `/planning/checklists` | `modules/planning-checklists/PlanningChecklistsPage.tsx` | work-item navigation/presentation | `/planning/checklists` |
| project/global planning | `modules/planning/PlanningPage.tsx` | pipeline runtime controls, audit log, work-item links | `/projects/:id/pipeline-run`, run node/step action routes |
| BOQ | `modules/boq/BOQPage.tsx` | `CurrencyText`, status labels, dialogs | BOQ and unit-price routes |
| procurement | `modules/procurement/ProcurementPage.tsx` | dialog/forms/status labels | procurement routes |
| warehouse | `modules/warehouse/WarehousePage.tsx` | stock forms, partner/project selectors | material/warehouse/stock/material-balance routes |
| operations | `modules/operations/OperationsPage.tsx` | daily report and issue dialogs | daily report/issue routes |
| acceptance | `modules/acceptance/AcceptancePage.tsx` | acceptance detail/actions | acceptance routes |
| rework | `modules/rework/ReworkPage.tsx` | rework/replanning dialogs | rework/replanning routes |
| handover | `modules/handover/HandoverPage.tsx` | handover check/complete actions | handover routes |
| accounting | `modules/accounting/AccountingPage.tsx`, `CashflowPanel.tsx` | budgets, payments, cashflow report | budget/payment/report routes |
| debt/payables/receivables | `modules/debt/DebtPage.tsx` plus debt panels | invoice, payment plan, billing plan, reconciliation panels | payable/receivable/payment/invoice routes |
| core data | `modules/core-data/CoreDataPage.tsx`, `partnerUtils.ts` | partner type helpers | partner routes |
| staff/RBAC | `modules/staff/StaffPage.tsx`, `modules/rbac/*` | permission matrix, role/user assignment | staff/roles/permissions routes |
| audit | `modules/audit/AuditPage.tsx` | timeline/status labels | audit routes |
| reports | `modules/reports/ReportsPage.tsx`, `TaxReportPanel.tsx`, `ProfitReportPanel.tsx` | `downloadBlob`, filter fields, warnings | report and CSV export routes |
| settings | `modules/settings/SettingsPage.tsx` | read-only settings table | `/settings` |

## Work Item Execution Surfaces

| Keywords | Owner files | Notes |
|---|---|---|
| work item detail, primary panel, action readiness | `modules/work-items/WorkItemDetailPage.tsx`, `WorkItemPrimarySurface.tsx`, `workItemPresentation.ts` | Shared between global and project-scoped work item detail. Check backend `WorkItemDetail`, `WorkItemAction`, permissions, evidence/form gates. |
| workflow sidebar, project workspace navigation | `modules/projects/ProjectWorkspaceLayout.tsx` | Uses `/projects/:id/operations` full view or `view=workflow-sidebar`. Preserve non-work-item project navigation. |
| evidence upload/review/gallery | `modules/work-items/EvidencePanel.tsx`, `EvidenceGallery.tsx` | Uses `postForm`, protected `downloadBlob`, accept/reject actions. |
| activity composer/thread/attachments | `modules/work-items/ActivityComposer.tsx`, `ActivityThread.tsx`, `ActivityAttachments.tsx`, `modules/my-work/MyWorkPage.tsx` | Uses activity routes, mentions, attention, read states, SSE streams. |
| workflow form draft/submit/sign/PDF | `modules/work-items/WorkflowFormPanel.tsx`, `SignatureCaptureDialog.tsx` | Uses form instance routes, signature visual, PDF export protected downloads. |
| reconciliation panels | `SubcontractorReconciliationPanel.tsx`, `OwnerClientReconciliationPanel.tsx` | Quantity/price business panels tied to work items and BOQ/acceptance data. |
| material/quality panels | `MaterialSnapshotPanel.tsx`, `SurplusHandlingPanel.tsx`, `IncomingAcceptancePanel.tsx`, `OutgoingAcceptancePanel.tsx` | Cross warehouse/acceptance/workflow gates; load materials/warehouse/quality domain maps. |
| work item links | `workItemNavigation.ts` | Preserve project-scoped return paths and `returnTo` behavior. |

## Shared UI Components

| Component/helper | Owner file | Use when | Mapping update trigger |
|---|---|---|---|
| dialogs/modals | `components/Dialog.tsx`, `RecordDetailDialog.tsx`, `modules/_shared/RecordFormDialog.tsx`, `modules/_shared/SimpleForm.tsx` | Create/view/edit flows | Dialog sizing, validation, shared form behavior changes |
| tables/lists/actions | `components/DataTable.tsx`, `RowActions.tsx`, `ListSection.tsx` | Dense admin lists and row actions | Table columns, action API, list rendering conventions change |
| status/enum chips | `components/StatusBadge.tsx`, `EnumChip.tsx`, `lib/utils/status.ts` | Any enum/status display | New/renamed statuses, result values, movement types, activity/evidence types |
| feedback states | `ToastProvider.tsx`, `EmptyState.tsx`, `LoadingSpinner.tsx`, `PermissionDenied.tsx` | Success/warn/error/loading/empty/permission UX | Toast API, dismiss behavior, error display changes |
| page chrome | `PageHeader.tsx`, `StatCard.tsx`, `ProjectBackLink.tsx`, `Timeline.tsx`, `CurrencyText.tsx` | Reused shell widgets | Visual/contract behavior changes |
| project selector | `modules/_shared/ProjectSelector.tsx` | Global/scoped pages needing project context | Project query/filter behavior changes |
| formatting | `lib/utils/format.ts` | Currency/date/percent display | Formatting convention changes |
| auth context | `lib/auth/AuthContext.tsx` | user, loading, login/logout, permission checks | Auth state, permission helper, refresh handling changes |
| API client | `lib/api/client.ts` | API envelope, auth token, refresh, `get/post/put/del/postForm`, blob download, SSE | Any fetch/refresh/envelope/blob/SSE behavior change |
| domain types | `lib/types/domain.ts` | Shared frontend type contracts | Backend response shape, status field, new domain record changes |

## Pipeline Builder Diagram Helpers

| Keywords | Owner files | Notes |
|---|---|---|
| canvas, drag, pan, zoom, keyboard, reconnect | `DiagramCanvas.tsx`, `interactionReducer.ts` | Core interaction state; browser verification only when user explicitly requests it. |
| node card/layer/adorner/ports | `NodeCard.tsx`, `NodeLayer.tsx`, `AdornerLayer.tsx`, `DragPreviewLayer.tsx` | Port hit target and selected state bugs usually live here or in geometry. |
| connector layer/labels/handles | `ConnectorLayer.tsx`, `useConnectorRoutingLayout.ts` | Rendering, label placement, routing cache. |
| routing/geometry/hit testing | `routingEngine.ts`, `router.ts`, `geometry.ts`, `hitTesting.ts`, `diagramConstants.ts`, `diagramTypes.ts` | Use for overlap, path, port, snap, obstacle, and coordinate bugs. |
| edge persistence | `edgePersistence.ts` | Create/update payloads, connector model conversion, duplicate edge keys. |

## Frontend Root Searches

- Route/page: `rg -n "path:|PageName|route-fragment" frontend/src/app/router.tsx frontend/src/modules`.
- API usage: `rg -n "get<|post<|put<|del<|postForm<|downloadBlob|streamSSE|/api-fragment" frontend/src/modules frontend/src/lib frontend/src/components`.
- Component ownership: `rg -n "export function ComponentName|function ComponentName|export const ComponentName" frontend/src`.
- Status/enum labels: `rg -n "statusLabels|enumLabels|statusLabel|enumLabel|status_value" frontend/src/lib frontend/src/components frontend/src/modules`.
- Toast/validation: `rg -n "useToast|toast\\.|submitAttempted|required|invalid|warning|error" frontend/src/modules frontend/src/components`.
- Work item presentation: `rg -n "WorkItemPrimarySurface|workItemPresentation|workItemNavigation|evidence|workflow form|returnTo" frontend/src/modules/work-items frontend/src/modules/projects`.
- Diagram bugs: `rg -n "connector|port|drag|pan|zoom|route|hitTest|nodeRects|edgePersistence" frontend/src/modules/pipeline-builder/components/diagram frontend/src/modules/pipeline-builder/PipelineBuilderPage.tsx`.

