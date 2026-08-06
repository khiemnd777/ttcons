# Reporting Analytics Domain Map

## Owners

- Ordinary project-finance report: `frontend/src/modules/project-finance/ProjectFinancePage.tsx` with shared `AsyncProjectPicker.tsx`/`ProjectScopeActions.tsx`; backend `handlers_project_finance.go`, `service_project_finance.go`, `repository_project_finance.go`, with project-option paging shared internally through report handler/service/repository helpers.
- Frontend modules: `frontend/src/modules/dashboard`, `frontend/src/modules/reports`, `frontend/src/modules/accounting/AccountingBooksPanel.tsx`, `frontend/src/modules/accounting/CashflowPanel.tsx`
- Report workspace/panels: `ReportsPage.tsx`, `reportRegistry.ts`, `ReportFilters.tsx`, `OperationalReportPanel.tsx`, `TaxReportPanel.tsx`, `ProfitReportPanel.tsx`, `CashflowPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers_reports.go`, `service_reports.go`, `repository_reports.go`, `types_reports.go` (same `package core`); shared route registration remains in `routes.go`
- Shared UI/contracts: `frontend/src/components/DataTable.tsx`, `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/format.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Project-management finance: `/project-finance/projects?search&cursor&limit&selected_id`, `/projects/:id/project-finance` and `/projects/:id/project-finance/export`.
- Dashboard: `/reports/dashboard`
- Report options: `/reports/options?search&cursor&limit&selected_id` returns only non-cancelled projects inside actor scope as `{projects:{items,has_more,next_cursor,selected}}`; limit defaults to 20 and is capped at 50.
- Operational reports: `/reports/projects`, `/reports/warehouse`, `/reports/procurement`, `/reports/accounting`, `/reports/staff`; each accepts optional `project_id` and has a matching `/export` route.
- Finance reports: `/reports/cashflow`, `/reports/cashflow/export`, `/reports/tax`, `/reports/tax/export`, `/reports/profit`, `/reports/profit/export`
- Accounting ledger reports: `/accounting/reports/trial-balance`, `/accounting/reports/general-ledger`, `/accounting/reports/profit-loss`, `/accounting/reports/vat`

## Source Domains

- Project/planning: project status, work items, activity, planning forms.
- Commercial/procurement: BOQ, procurement, quantities, variance.
- Materials/warehouse: stock movements, material balances, surplus.
- Quality/handover: acceptance, rework, handover blockers.
- Accounting/finance: AP, AR, invoices, payments, cashflow, tax, profit.
- Admin/governance: staff and permission-aware visibility.

## Invariants

- Ordinary `/reports` catalog exposes project, procurement, warehouse and staff reports only. Legacy accounting/cashflow/tax/profit APIs stay available for admin/history support but their panels are not registered for normal users.
- Project-management finance reporting lives under `Tài chính công trình`: eligible main-client milestone revenue, confirmed material-purchase cost and provisional profit. The UI/API must carry the incomplete-cost-scope warning and redact management monetary totals without `project_finance.report_view`.
- Project-finance CSV serializes the same material-cost rows returned by the workspace; it must not query legacy AP/AR/payment/invoice/ledger tables as a substitute.
- Global Project Finance keeps draft and applied project scope separate: only the explicit Apply action writes `project`, while Reset removes it and returns to the no-project state. Workspace data, create/action context and CSV always use the applied project ID; the project-scoped route remains fixed to its route ID.
- Reports and Finance share the asynchronous picker/page presentation and internal request/cursor mechanics, not permission routes. Reports reads `/reports/options`; Finance reads `/project-finance/projects`, which additionally filters every list and selected lookup by `project_finance.view`. Neither surface eagerly loads all projects.
- Report APIs enforce project/user visibility.
- Frontend routes `/reports` and `/reports/:reportKey` expose only catalog entries allowed by `report.view`, `report.finance.view`, or `accounting.view`; the base/invalid/unavailable key redirects to the first visible report. Exactly one report panel is lazy-loaded and mounted, so hidden reports do not issue API reads.
- `/reports/options`, list results and CSV exports use the same actor project scope. Report `project_id` and option `selected_id` are UUID-validated before repository access. The options repository applies the same `ProjectAccessFilter` to code/name search, keyset pages and the selected-ID lookup; an inaccessible selection returns `selected: null` without exposing its name. Report services retain the single authoritative project-permission check before report queries, avoiding a duplicate DB permission waterfall without weakening RBAC.
- Operational CSV exports serialize exactly the rows returned by the corresponding scoped report query with stable report-specific columns.
- Project report issue and pending-approval counts are independent aggregates per project; joining those one-to-many tables before counting is forbidden.
- Warehouse report aggregates stock movements and site surplus in separate CTEs before joining materials. Accounting aggregates payables, receivables and budgets independently and uses ordinary `SUM`; `SUM(DISTINCT amount)` is forbidden because equal legitimate obligations must all count.
- Staff report includes only active staff membership in accessible, non-cancelled projects and counts distinct accessible projects. Full-company staff remains the `staff.view` Staff module, not a report-scope bypass.
- Dashboard remains the existing source-backed `Tổng quan điều hành` contract and is labelled `Điều hành` in global navigation. It follows `Việc của tôi`, which is now the authenticated landing page. Dashboard data, permissions, action hierarchy and non-mutating report behavior are unchanged.
- `DashboardReport` computes its ten existing scalar metrics in one materialized scoped-project statement, then runs the four chart queries sequentially and returns the first error instead of silently substituting empty chart data. Dashboard and JSON reports use the bounded report concurrency guard; exports use the separate export guard. Metric predicates and response fields remain unchanged.
- Dashboard calls `/projects` only for actors with `project.view`; report-only actors receive an explicit permission state while `/reports/dashboard` and `/approvals/pending` remain usable. Loading, API errors and true empty results must remain visually distinct.
- Dashboard project attention is a presentation subset of the scoped non-cancelled `/projects` response (`blocked`, open issues or pending approvals). The project-status bar uses each returned count divided by the returned status total; `active_projects` is labelled `Đang triển khai` because it includes planning, in-progress, acceptance and handover states.
- UI filters and CSV exports use the same query semantics.
- The VAT period purchase ledger, sales ledger and declaration CSV are exported from the exact period-detail source used by `TaxWorkspace`; an unprepared declaration export falls back to the current canonical summary, while a prepared/filed/locked export uses the latest immutable return snapshot.
- The ordinary Reports workspace owns one compact global `Công trình` filter inside the page header above both the catalog and active report, without explanatory scope wording or a standalone decorative card. Wide desktop aligns it beside the page purpose while narrower screens stack it below; Apply remains primary and Reset remains a quiet workspace action. It separates draft from applied state, persists only applied `project_id` in the URL across report navigation, and keeps CSV as an active-report action using the same applied query. Its async picker searches code/name after 300ms, loads 20 actor-scoped rows per cursor page, shows code for duplicate-name disambiguation and resolves a direct URL selection separately; it never eagerly loads the full option set. Project choices come from `/reports/options`, never the broader `/projects` endpoint; legacy hidden finance panels retain their existing filter controls.
- Operational report presentation is responsive without changing report semantics: below `md`, project/procurement/warehouse/staff rows render as report-specific cards with mobile sorting; from `md` upward they remain tables. Search, sorting, pagination, applied project filter, permission-aware source links, loading/error/empty states and CSV scope share the same row/query contract.
- Cashflow is payment-ledger based unless changed deliberately.
- `CashflowPanel` keeps report/export query parity. Project Accounting defaults the date range to the project start date (or full-history fallback), while global Accounting defaults to the current month; pending amount, unclassified count and non-cash count are visible report-contract warnings rather than frontend-derived guesses.
- Tax reporting uses canonical `invoice_tax_lines` and requires a live posted source journal for the default `posted` view; a direct invoice status value is not evidence of posting. Posted tax adjustments join the same source-backed aggregation, while source reversals remove the original invoice tax contribution. Counterparty tax-code warnings remain based on the invoice partner tax identifier, not the configured VAT rate code. Profit remains invoice-ledger based unless changed deliberately.
- The accounting Trial Balance, General Ledger, P&L and VAT ledger are posted-journal/source-tax reports scoped through `ProjectAccessFilter`. Trial Balance and P&L include reversal journals as independent offsetting entries; VAT reporting excludes tax lines whose source journal is not posted or has been reversed by the canonical source transition.
- Dashboard and `/reports/accounting` budget values read `accounting_budget_balances`; `used_budget` means recognized actual cost, not legacy mutable `accounting_budgets.used_amount`.
- Warnings are part of the report contract, not decorative UI.
- Tax warnings for missing tax code/VAT arithmetic and the profit unclassified-cost warning can set the controlled ledger search to their exact backend warning flag. Aggregate-only or excluded-row warnings remain informational because their rows are not present in the active ledger.
- Warehouse aggregate reports treat a completed source/target transfer as net zero across all warehouses while per-warehouse position queries subtract from the source and add to the target; legacy transfer rows without a target remain outbound-only.

## Cross-Domain Checks

- Pair with the source domain before changing report metrics, denominators, filters, statuses, warning logic, or CSV columns.
- Check dashboard/report visibility against auth, project membership, and backend permission helpers.
- Keep warning semantics tied to source documents or reconciliation records, not frontend-only guesses.
