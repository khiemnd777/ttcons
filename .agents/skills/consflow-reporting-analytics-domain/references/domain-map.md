# Reporting Analytics Domain Map

## Owners

- Frontend modules: `frontend/src/modules/dashboard`, `frontend/src/modules/reports`, `frontend/src/modules/accounting/CashflowPanel.tsx`
- Report workspace/panels: `ReportsPage.tsx`, `reportRegistry.ts`, `ReportFilters.tsx`, `OperationalReportPanel.tsx`, `TaxReportPanel.tsx`, `ProfitReportPanel.tsx`, `CashflowPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Shared UI/contracts: `frontend/src/components/DataTable.tsx`, `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/format.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Dashboard: `/reports/dashboard`
- Report options: `/reports/options` returns only non-cancelled projects inside actor scope.
- Operational reports: `/reports/projects`, `/reports/warehouse`, `/reports/procurement`, `/reports/accounting`, `/reports/staff`; each accepts optional `project_id` and has a matching `/export` route.
- Finance reports: `/reports/cashflow`, `/reports/cashflow/export`, `/reports/tax`, `/reports/tax/export`, `/reports/profit`, `/reports/profit/export`

## Source Domains

- Project/planning: project status, work items, activity, planning forms.
- Commercial/procurement: BOQ, procurement, quantities, variance.
- Materials/warehouse: stock movements, material balances, surplus.
- Quality/handover: acceptance, rework, handover blockers.
- Accounting/finance: AP, AR, invoices, payments, cashflow, tax, profit.
- Admin/governance: staff and permission-aware visibility.

## Invariants

- Report APIs enforce project/user visibility.
- Frontend routes `/reports` and `/reports/:reportKey` expose only catalog entries allowed by `report.view`, `report.finance.view`, or `accounting.view`; the base/invalid/unavailable key redirects to the first visible report. Exactly one report panel is lazy-loaded and mounted, so hidden reports do not issue API reads.
- `/reports/options`, list results and CSV exports use the same actor project scope. A selected `project_id` is UUID-validated and permission-checked in both handler and service before repository filtering.
- Operational CSV exports serialize exactly the rows returned by the corresponding scoped report query with stable report-specific columns.
- Project report issue and pending-approval counts are independent aggregates per project; joining those one-to-many tables before counting is forbidden.
- Warehouse report aggregates stock movements and site surplus in separate CTEs before joining materials. Accounting aggregates payables, receivables and budgets independently and uses ordinary `SUM`; `SUM(DISTINCT amount)` is forbidden because equal legitimate obligations must all count.
- Staff report includes only active staff membership in accessible, non-cancelled projects and counts distinct accessible projects. Full-company staff remains the `staff.view` Staff module, not a report-scope bypass.
- Dashboard presents the existing source-backed contract as `Tổng quan điều hành`: action counts first, then pending approvals and already-scoped attention projects, followed by status composition and payment-ledger finance. It does not derive new business totals or mutate approval/workflow state.
- Dashboard calls `/projects` only for actors with `project.view`; report-only actors receive an explicit permission state while `/reports/dashboard` and `/approvals/pending` remain usable. Loading, API errors and true empty results must remain visually distinct.
- Dashboard project attention is a presentation subset of the scoped non-cancelled `/projects` response (`blocked`, open issues or pending approvals). The project-status bar uses each returned count divided by the returned status total; `active_projects` is labelled `Đang triển khai` because it includes planning, in-progress, acceptance and handover states.
- UI filters and CSV exports use the same query semantics.
- Report filter controls separate draft from applied state, persist applied state in the URL, and offer explicit apply/reset/export. Project choices come from `/reports/options`, never the broader `/projects` endpoint.
- Cashflow is payment-ledger based unless changed deliberately.
- `CashflowPanel` keeps report/export query parity. Project Accounting defaults the date range to the project start date (or full-history fallback), while global Accounting defaults to the current month; pending amount, unclassified count and non-cash count are visible report-contract warnings rather than frontend-derived guesses.
- Tax and profit are invoice-ledger based unless changed deliberately.
- Warnings are part of the report contract, not decorative UI.
- Tax warnings for missing tax code/VAT arithmetic and the profit unclassified-cost warning can set the controlled ledger search to their exact backend warning flag. Aggregate-only or excluded-row warnings remain informational because their rows are not present in the active ledger.
- Warehouse aggregate reports treat a completed source/target transfer as net zero across all warehouses while per-warehouse position queries subtract from the source and add to the target; legacy transfer rows without a target remain outbound-only.

## Cross-Domain Checks

- Pair with the source domain before changing report metrics, denominators, filters, statuses, warning logic, or CSV columns.
- Check dashboard/report visibility against auth, project membership, and backend permission helpers.
- Keep warning semantics tied to source documents or reconciliation records, not frontend-only guesses.
