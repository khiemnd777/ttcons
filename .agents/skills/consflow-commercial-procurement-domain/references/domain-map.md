# Commercial Procurement Domain Map

## Owners

- Frontend modules: `frontend/src/modules/boq`, `frontend/src/modules/procurement`, `frontend/src/modules/work-items`
- Reconciliation panels: `SubcontractorReconciliationPanel.tsx`, `OwnerClientReconciliationPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- BOQ: `/projects/:id/boq`, `/boq-items/:id`, `/boq-items/:id/unit-price/expected|lock-norm|lock-final`, `/projects/:id/boq/submit|approve|generate-procurement`
- Supplier quotation: `/projects/:id/supplier-quotations`, `/projects/:id/supplier-quote-comparison`, `/supplier-quotations/:id`, `/supplier-quotations/:id/submit|qualify`, `/supplier-quotation-items/:id/select`
- Procurement: `/procurement-requests`, `/procurement-requests/:id`, `/procurement-requests/:id/submit|approve|order|receive`
- Reconciliation: `/work-items/:id/subcontractor-reconciliation`, `/work-items/:id/owner-client-reconciliation`

## Tables And Migrations

- `boq_items`, `boq_unit_price_events`
- `procurement_requests`, `procurement_request_items`
- `supplier_quotations`, `supplier_quotation_items`, `supplier_quote_selections`
- `subcontractor_checklist_reconciliations`, `subcontractor_checklist_reconciliation_items`
- `owner_client_checklist_reconciliations`, `owner_client_checklist_reconciliation_items`
- Related stock table: `stock_movements`
- Relevant migrations: `V1`, `V17`, `V21`, `V22`

## Invariants

- Approved and final-locked BOQ prices drive procurement values; final price traces to a selected qualified quote or an explicit manual waiver reason.
- Submitted quotation items and selection decisions are commercial history; selection locks the BOQ final price and is blocked after procurement exists.
- Procurement generation groups eligible BOQ lines by selected supplier and snapshots quotation item, supplier, price revision, and final-price event.
- Ordering requires supplier, order reference, and expected delivery date. Partial/full receive status is derived from submitted incoming acceptance quantities and does not implicitly create stock movement.
- Procurement receive is auditable and stock-aware.
- Subcontractor reconciliation maps confirmed quantity and variance to BOQ/work item evidence.
- Owner/client reconciliation maps client decision and required action to acceptance/work item evidence.

## Cross-Domain Checks

- Pair with materials/warehouse when procurement receive, stock movement, material balance, or surplus handling can change.
- Pair with quality/acceptance when owner/client reconciliation, outgoing acceptance, punch list, or handover readiness depends on commercial records.
- Pair with accounting/finance when procurement, invoices, payables, advances, payments, or vendor ledgers depend on BOQ or purchase data.
