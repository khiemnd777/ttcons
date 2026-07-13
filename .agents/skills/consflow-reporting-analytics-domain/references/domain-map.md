# Reporting Analytics Domain Map

## Owners

- Frontend modules: `frontend/src/modules/dashboard`, `frontend/src/modules/reports`, `frontend/src/modules/accounting/CashflowPanel.tsx`
- Report panels: `TaxReportPanel.tsx`, `ProfitReportPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Shared UI/contracts: `frontend/src/components/DataTable.tsx`, `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/format.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Dashboard: `/reports/dashboard`
- Operational reports: `/reports/projects`, `/reports/warehouse`, `/reports/procurement`, `/reports/accounting`, `/reports/staff`
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
- UI filters and CSV exports use the same query semantics.
- Cashflow is payment-ledger based unless changed deliberately.
- Tax and profit are invoice-ledger based unless changed deliberately.
- Warnings are part of the report contract, not decorative UI.
- Warehouse aggregate reports treat a completed source/target transfer as net zero across all warehouses while per-warehouse position queries subtract from the source and add to the target; legacy transfer rows without a target remain outbound-only.

## Cross-Domain Checks

- Pair with the source domain before changing report metrics, denominators, filters, statuses, warning logic, or CSV columns.
- Check dashboard/report visibility against auth, project membership, and backend permission helpers.
- Keep warning semantics tied to source documents or reconciliation records, not frontend-only guesses.
