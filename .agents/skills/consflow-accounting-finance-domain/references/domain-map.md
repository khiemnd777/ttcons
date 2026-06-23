# Accounting Finance Domain Map

## Owners

- Frontend modules: `frontend/src/modules/accounting`, `frontend/src/modules/debt`, `frontend/src/modules/reports`
- Debt panels: `InputInvoicePanel.tsx`, `OutputInvoicePanel.tsx`, `InputPaymentPlanPanel.tsx`, `OutputBillingPlanPanel.tsx`, `PayableReconciliationPanel.tsx`, `ReceivableReconciliationPanel.tsx`
- Report panels: `CashflowPanel.tsx`, `TaxReportPanel.tsx`, `ProfitReportPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Budgets/payments: `/projects/:id/budget`, `/budgets/:id/approve`, `/payments`, `/payments/:id/approve`
- Payables: `/payables`, `/payables/:id/approve|pay`, `/payables/:id/payment-schedules`, `/payment-schedules/:id/approve`, `/payables/:id/advance-requests`, `/advance-requests/:id/approve|settle`, `/payables/:id/payments`, `/payables/:id/input-invoices/upload`, `/payables/:id/reconciliation`
- Receivables: `/receivables`, `/receivables/:id/approve|receive`, `/receivables/:id/payment-schedules`, `/receivable-payment-schedules/:id/approve`, `/receivables/:id/advance-receipts`, `/client-advance-receipts/:id/approve|settle`, `/receivables/:id/payments`, `/receivables/:id/output-invoices/upload`, `/receivables/:id/reconciliation`
- Invoice files/actions: `/projects/:id/input-invoices`, `/input-invoices/:id/file|verify|reject`, `/projects/:id/output-invoices`, `/output-invoices/:id/file|issue|void`
- Reports: `/reports/cashflow`, `/reports/tax`, `/reports/profit` and export routes

## Tables And Migrations

- `accounting_budgets`, `payables`, `receivables`, `payments`, `advance_requests`, `payment_allocations`
- `input_invoices`, `output_invoices`
- `payable_payment_schedules`, `receivable_payment_schedules`, `client_advance_receipts`
- Related documents: `documents`, `evidence_records`, `partners`
- Relevant migrations: `V1`, `V27`, `V28`, `V29`, `V30`, `V31`, `V32`, `V33`, `V34`, `V35`, `V36`

## Invariants

- AP/AR records reconcile to invoices, payments/receipts, advances, settlements, and source documents.
- Input invoice PDFs belong to payables; output invoice PDFs belong to receivables.
- Cashflow, tax, and profit reports must stay source-backed and permission-scoped.
- CSV exports must use the same filters and source logic as UI reports.

## Cross-Domain Checks

- Pair with commercial/procurement when payables, vendor invoices, advances, payment schedules, or procurement-linked source documents change.
- Pair with quality/acceptance/handover when receivables, output invoices, client billing, or handover readiness depends on acceptance state.
- Pair with admin/governance when partner catalog, tax codes, permissions, or audit semantics affect financial records.
