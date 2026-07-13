# Accounting Finance Domain Map

## Owners

- Frontend modules: `frontend/src/modules/accounting`, `frontend/src/modules/debt`, `frontend/src/modules/contracts`, `frontend/src/modules/reports`
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
- Contract sources: `/projects/:id/contracts`, contract version/milestone/variation/retention actions; milestones and retention releases materialize draft AP/AR records.
- Handover closure: `/projects/:id/handover`, `/projects/:id/handover/options`, handover submit/check/complete and checklist verification routes consume finance readiness without rewriting the finance ledger.

## Tables And Migrations

- `accounting_budgets`, `payables`, `receivables`, `payments`, `advance_requests`, `payment_allocations`
- `input_invoices`, `output_invoices`
- `payable_payment_schedules`, `receivable_payment_schedules`, `client_advance_receipts`
- `contracts`, `contract_versions`, `contract_milestones`, `contract_variations`, `contract_retentions`, `contract_retention_releases`
- Related documents: `documents`, `evidence_records`, `partners`
- Relevant migrations: `V1`, `V27`-`V36`, `V48`, `V54`-`V56`

## Invariants

- AP/AR records reconcile to invoices, payments/receipts, advances, settlements, and source documents.
- API creation of manual AP/AR is server-owned: new records start `draft`, paid/received amounts start at zero, remaining balance is computed from normalized obligation fields, and the actor/audit trail is recorded. Client-supplied status or derived balances are ignored by typed contracts.
- Generic payment/receipt creation always starts `draft`; approval is the only path that sets approval actor/time. AP/AR-linked payments should use the dedicated allocation endpoints so reconciliation stays correct.
- Input invoice PDFs belong to payables; output invoice PDFs belong to receivables.
- Cashflow, tax, and profit reports must stay source-backed and permission-scoped.
- CSV exports must use the same filters and source logic as UI reports.
- Active signed contract versions are the commercial source of truth. `projects.contract_amount` is only the active main-client compatibility snapshot.
- A milestone can open AP/AR only after the contract is active and the milestone is eligible; acceptance-required milestones must reference an approved pass/conditional-pass acceptance.
- Retention held at milestone obligation time is reconciled through append-only release rows; each release creates a traceable AP/AR source record and cannot exceed outstanding retention.
- The contract workspace presents milestone obligations as Vietnamese payable/receivable labels without exposing raw obligation identifiers; eligibility, obligation creation and retention release still use the existing permission and server transition gates.
- Handover readiness reads AP/AR remaining balances and pending payment status; it snapshots those facts but never force-closes financial records.

## Cross-Domain Checks

- Pair with commercial/procurement when payables, vendor invoices, advances, payment schedules, or procurement-linked source documents change.
- Pair with quality/acceptance/handover when receivables, output invoices, client billing, or handover readiness depends on acceptance state.
- Pair with admin/governance when partner catalog, tax codes, permissions, or audit semantics affect financial records.

## Golden Sample

- `GOLDEN-REAL-001` is the reconciled review fixture for four AP and five AR obligations, schedules, settled advances, nine terminal payments/receipts, allocations, verified input invoice, issued output invoice and released contract retention. AP/AR remaining balances are zero before handover.
