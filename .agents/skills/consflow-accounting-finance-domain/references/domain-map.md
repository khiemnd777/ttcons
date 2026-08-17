# Accounting Finance Domain Map

## Owners

- Ordinary project-finance UI: `frontend/src/modules/project-finance/ProjectFinancePage.tsx`, shared `frontend/src/modules/_shared/AsyncProjectPicker.tsx` and `ProjectScopeActions.tsx`, routed at `/accounting` and `/projects/:id/accounting`
- Ordinary project-finance backend: `handlers_project_finance.go`, `service_project_finance.go`, `repository_project_finance.go`, `types_project_finance.go`; project-option parsing/paging/search helpers live in `handlers_reports.go`, `service_reports.go` and `repository_reports.go`
- Frontend modules: `frontend/src/modules/accounting` (`AccountingPage.tsx`, `AccountingBooksPanel.tsx`, `ManualJournalDialog.tsx`, `TaxWorkspace.tsx`, `AccountingCreateDialog.tsx`, `AccountingDetailDialog.tsx`, `CashflowPanel.tsx`), `frontend/src/modules/debt`, `frontend/src/modules/contracts`, `frontend/src/modules/reports`
- Debt panels: `InputInvoicePanel.tsx`, `OutputInvoicePanel.tsx`, `InputPaymentPlanPanel.tsx`, `OutputBillingPlanPanel.tsx`, `PayableReconciliationPanel.tsx`, `ReceivableReconciliationPanel.tsx`
- Report panels: `CashflowPanel.tsx`, `TaxReportPanel.tsx`, `ProfitReportPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: source finance methods in `handlers.go`, `service.go`, `repository_domain.go`, `types.go`; ledger owners `handlers_accounting.go`, `types_accounting.go`, `accounting_core.go`, `accounting_dimensions.go`, `accounting_posting.go`, `accounting_payment_reversal.go`, `accounting_tax.go`, `accounting_reports.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Ordinary project finance: `/project-finance/projects?search&cursor&limit&selected_id`, `/projects/:id/project-finance`, `/projects/:id/project-finance/options`, `/projects/:id/project-finance/export`, create material-cost/cash-request routes and record action routes. The global project-options route requires `project_finance.view` and applies both actor project scope and per-project permission to list/search/cursor/selected resolution.
- The routes below are legacy professional-accounting support. They remain backend-compatible but are not ordinary Accounting/Reports navigation for non-admin roles.
- Budgets/payments: `/projects/:id/budget`, `/budgets/:id/approve`, `/payments`, `/payments/:id/approve|reverse`
- Ledger: `/accounting/accounts`, `/accounting/periods`, `/accounting/journal-entries`, `/projects/:id/accounting/journal-entries`, `/accounting/journal-entries/:id/reverse`
- Ledger reports/tax/reconciliation: `/accounting/reports/trial-balance|general-ledger|profit-loss|vat`, `/accounting/tax-codes`, `/accounting/tax-periods`, `/accounting/tax-periods/:id`, `/accounting/tax-periods/:id/export|prepare|file|lock|start-amendment|adjustments|payments`, `/accounting/tax-adjustments/:id/reverse`, `/accounting/reconciliation-issues`, `/accounting/reconciliation-issues/:id/resolve`
- Payables: `/payables`, `/payables/:id/approve|pay`, `/payables/:id/payment-schedules`, `/payment-schedules/:id/approve`, `/payables/:id/advance-requests`, `/advance-requests/:id/approve|settle`, `/payables/:id/payments`, `/payables/:id/input-invoices/upload`, `/payables/:id/reconciliation`
- Receivables: `/receivables`, `/receivables/:id/approve|receive`, `/receivables/:id/payment-schedules`, `/receivable-payment-schedules/:id/approve`, `/receivables/:id/advance-receipts`, `/client-advance-receipts/:id/approve|settle`, `/receivables/:id/payments`, `/receivables/:id/output-invoices/upload`, `/receivables/:id/reconciliation`
- Invoice files/actions: `/projects/:id/input-invoices`, `/input-invoices/:id/file|verify|reject`, `/projects/:id/output-invoices`, `/output-invoices/:id/file|issue|void`
- Reports: `/reports/cashflow`, `/reports/tax`, `/reports/profit`, `/reports/accounting` and matching export routes
- Contract sources: `/projects/:id/contracts`, contract version/milestone/variation/retention actions; milestones and retention releases materialize draft AP/AR records.
- Handover closure: `/projects/:id/handover`, `/projects/:id/handover/options`, handover submit/check/complete and checklist verification routes consume finance readiness without rewriting the finance ledger.

## Tables And Migrations

- Ordinary management records: `project_material_costs`, `project_cash_requests`; migration `V83`.
- `accounting_budgets`, `payables`, `receivables`, `payments`, `advance_requests`, `payment_allocations`, `receipt_allocations`
- `accounting_books`, `accounting_accounts`, `accounting_periods`, `journal_entries`, `journal_entry_lines`, `accounting_posting_rules`
- `budget_movements`, `payment_reversals`, `accounting_backfill_issues`
- `tax_codes`, `tax_periods`, `invoice_tax_lines`, `tax_adjustments`, `vat_return_snapshots`, `tax_period_events`
- `input_invoices`, `output_invoices`
- `payable_payment_schedules`, `receivable_payment_schedules`, `client_advance_receipts`
- `contracts`, `contract_versions`, `contract_milestones`, `contract_variations`, `contract_retentions`, `contract_retention_releases`
- Related documents: `documents`, `evidence_records`, `partners`
- Relevant migrations: `V1`, `V27`-`V36`, `V48`, `V54`-`V56`, `V70`, `V71`, `V73`-`V83`; existing V90 project search/order indexes support the Finance picker without a new migration.

## Invariants

- Product boundary: Consflow ordinary users manage workflow and project-management finance only. MISA owns professional bookkeeping/tax outside Consflow; there is no MISA integration or synchronization.
- Global `/accounting` uses a draft searchable project choice and applies it only through the `project` query parameter after explicit confirmation. It must not eagerly load `/projects` or auto-select the first project; Reset removes the applied project and returns to the no-project empty state. Workspace reads, creates, actions and CSV use only the applied ID. `/projects/:id/accounting` remains fixed to the route project and never renders the global selector.
- Finance project choices come from `/project-finance/projects`, not `/reports/options`. Search is server-side by code/name with bounded cursor pages; list rows and direct `selected_id` resolution apply identical actor-scope, non-cancelled and `project_finance.view` checks, so an unauthorized selected project returns `selected: null` without leaking its name.
- Only `project_material_costs.status='confirmed'` contributes to management cost. Draft/submitted/needs-changes records remain visible blockers but do not alter provisional profit.
- Provisional management profit is eligible main-client milestone revenue minus confirmed material-purchase cost. It must always disclose that labor, subcontractor, equipment, overhead, management cost and tax are excluded.
- A project material cost must link at least one same-project procurement, incoming-acceptance, stock-document or document source. Cash-request generic source IDs are type-whitelisted and same-project validated.
- `project_cash_requests` is an internal request/handoff workflow. `processed` means Accounting handled the dossier in the external professional system; it is not proof of cash movement, double-entry posting, invoice issuance or tax filing in Consflow.
- Management report totals and CSV require `project_finance.report_view`; workspace responses redact management monetary totals without it. Operational transitions are actor-audited atomically and `needs_changes` requires a reason.
- Legacy accounting, Debt and finance-report backends remain immutable/history-compatible and admin-only by ordinary RBAC. Do not derive new management material cost or cash-request completion from legacy AP/AR/payment/invoice/ledger tables.
- AP/AR records reconcile to invoices, payments/receipts, advances, settlements, and source documents.
- Every posted journal has at least two lines and equal debit/credit totals. The deferred balance trigger and open-period trigger enforce this at transaction commit; posted headers/lines are immutable and are corrected only through a linked reversal journal.
- A manual CTGS persists its optional `source_document_no` on the immutable journal header; journal detail presents source kinds as Vietnamese accounting document labels instead of leaking internal source codes.
- Long migration/backfill CTGS numbers remain immutable accounting identifiers. Journal, General Ledger and VAT list surfaces may compact only their visual presentation; compacted values expose the full number through an application-rendered portal tooltip on mouse hover, keyboard focus and touch/pen tap, while assistive labels, detail, search and export flows retain the same full value.
- Copying a CTGS is presentation-only: click, Enter or Space copies the full immutable accounting identifier, never the compact display string; explicit success/failure feedback must not mutate the posted journal.
- Source transitions and their journals are one atomic transaction. Payable approval records budget commitment; input-invoice verification records cost/VAT/AP and actual budget; output-invoice issue records AR/revenue/VAT; payment approval records cash plus AP/AR allocation or an explicit generic income/expense account.
- Direct `pay` on payable and direct `receive` on receivable are blocked. Settlement must pass through a Thu/Chi voucher with allocations so the cash journal, source balance, schedule/advance and budget movement cannot drift.
- Budget balances are derived from append-only movements: `committed` is an open obligation, `actual` is recognized cost, `paid` is cash paid, and `available = approved - committed - actual`. Negative available balance is visible; approval/posting must not hide overspend with a floor at zero.
- Payment reversal is atomic and immutable: it requires a reason, reverses allocations/source balances, records offsetting budget movements, posts a reversal journal and cancels the original voucher without deleting history.
- Accounting periods gate all posting/reversal by entry date. Close/reopen requires a reason and permission and writes audit. No API may mutate a closed period's journal directly.
- VAT lines are generated from input/output invoices and feed the VAT ledger. Tax code, input/output account, taxable amount, VAT amount and deductible amount remain source-backed dimensions, not frontend calculations.
- A VAT return can be prepared only when every in-range invoice is posted with a canonical tax line and posted TK 1331/3331 turnover reconciles to the tax dimensions. Prepare creates an immutable snapshot; file records the authority reference; lock closes the declaration; later correction opens the next amendment number and creates another snapshot without rewriting history.
- An open or amended VAT period exposes a provisional payable/credit calculation including prior-period carried credit; prepared, filed and locked screens present the immutable latest snapshot, so the UI must not mix a live preview with a filed amount.
- VAT adjustments are append-only, require a reason and balanced linked CTGS, and are corrected only by an explicit adjustment plus journal reversal. A VAT payment belongs to one filed/locked tax period, posts Nợ TK 3331/Có TK tiền on approval, appears in cashflow, and must not create budget commitment, actual cost or paid-budget movement.
- Once a VAT return snapshot exists, input/output invoice status changes in that source period are blocked until the controlled supplemental-declaration workflow is used. V81 covers VAT only; TNDN and TNCN are not implemented by this lifecycle.
- V77 normalizes legacy voucher terminal statuses `paid|received` to canonical `approved` while preserving `paid_at`, and clears a legacy `target_type` only when its `target_id` never existed. Complete finance constraints are validated only after these deterministic repairs; unknown partial references must still fail migration rather than inventing a source.
- `backfill_accounting_ledger()` is idempotent. It posts only deterministic historical sources and writes ambiguous sources to `accounting_backfill_issues`; an open issue can become `resolved` only with a same-project posted CTGS, or `accepted` only with an explanatory note and no journal link.
- API creation of manual AP/AR is server-owned: new records start `draft`, paid/received amounts start at zero, remaining balance is computed from normalized obligation fields, and the actor/audit trail is recorded in the same fail-closed transaction. Client-supplied status or derived balances are ignored by typed contracts.
- Generic payment/receipt creation always starts `draft` and is atomic with its audit event; approval is the only path that sets approval actor/time. AP/AR-linked payments should use the dedicated allocation endpoints so reconciliation stays correct.
- Budget creation is typed and server-owned: clients provide only code/name/planned amount; draft approved/used/remaining amounts start at zero, and approval derives approved amount from planned amount plus remaining from approved minus used. Both creation and approval write audit in the same transaction and roll back on audit failure.
- Generic `/payments` creation is reserved for non-AP/AR `outbound` and `inbound` records. Legacy `other` rows remain readable and enter reconciliation when they lack a deterministic posting direction; new supplier/subcontractor/client/advance payments must use dedicated payable/receivable endpoints with allocation context.
- Payable and receivable list handlers both combine actor project visibility with a validated explicit `project_id` filter; project-scoped UI must not rely on client filtering.
- The Debt UI owns AP/AR work queues, complete aging (`not_due`, `1_30`, `31_60`, `61_90`, `90_plus`, `settled`) and invoice/schedule/advance/payment/reconciliation detail. User-facing text spells these out as `công nợ phải trả` and `công nợ phải thu`; AP/AR remains technical shorthand in code/contracts/maps. Accounting owns budget, generic phiếu approval/list and cashflow presentation.
- The project Debt page accepts presentation-only `?kind=payables|receivables&queue=...` source-navigation queries. It applies only a kind the user may view and never turns query parameters into an API mutation, allocation, approval or reconciliation action; Handover uses this to disclose the current module behind stored readiness counts.
- Debt create/detail presentation may derive live opening balances and promote one state-aware next action, but approval and pay/receive actions still call canonical routes; allocations, invoices, advances, schedules and reconciliation remain owned by their existing debt panels and server calculations.
- Accounting create/detail presentation may combine budget and generic phiếu entry into one permission-aware dialog, but it must keep `supplier|subcontractor|client_receipt|advance` out of generic payment creation and direct AP/AR work to Debt. A linked payable/receivable phiếu uses `Mở công nợ` to navigate to the Debt workspace without mutating allocation or reconciliation state.
- Input invoice PDFs belong to payables; output invoice PDFs belong to receivables. Both persist opaque `local|s3` storage provider/key metadata through the shared storage registry under V70. V71 adds `NOT VALID` future-write checks requiring invoice keys to be non-null, nonblank and equal to `btrim(storage_key)` without rewriting legacy invalid rows. Both load detail before transaction commit, delete a newly written object only on a typed pre-commit failure, preserve it when commit outcome is unknown, enforce deleted-record guards, and authorize project/accounting access before opening and streaming a protected file.
- Cashflow, tax, and profit reports must stay source-backed and permission-scoped.
- Accounting report totals payables, receivables and budgets independently per accessible project before joining them. Equal monetary amounts are separate obligations and must all count; never use `SUM(DISTINCT amount)` as a join-deduplication technique. Optional project filtering and CSV reuse the same query.
- CSV exports must use the same filters and source logic as UI reports.
- Active signed contract versions are the commercial source of truth. `projects.contract_amount` is only the active main-client compatibility snapshot.
- A milestone can open AP/AR only after the contract is active and the milestone is eligible; acceptance-required milestones must reference an approved pass/conditional-pass acceptance.
- Retention held at milestone obligation time is reconciled through append-only release rows; each release creates a traceable AP/AR source record and cannot exceed outstanding retention.
- The contract workspace presents milestone obligations as Vietnamese payable/receivable labels without exposing raw obligation identifiers; eligibility, obligation creation and retention release still use the existing permission and server transition gates.
- Handover readiness requires zero unconfirmed project material costs and zero cash requests outside `processed|cancelled`. Older `ap_reconciliation` and `ar_reconciliation` checklist codes are compatibility aliases for these new facts and never force-close legacy financial records.

## Cross-Domain Checks

- Pair with commercial/procurement when payables, vendor invoices, advances, payment schedules, or procurement-linked source documents change.
- Pair with quality/acceptance/handover when receivables, output invoices, client billing, or handover readiness depends on acceptance state.
- Pair with admin/governance when partner catalog, tax codes, permissions, or audit semantics affect financial records.
- Every editable monetary value in accounting, project finance, AP/AR, invoices, schedules, advances, settlements, taxes and journal debit/credit uses shared `CurrencyInput` or semantic `currency`; native/generic number fields remain reserved for non-money numerics. Preserve explicit currency, negative adjustment rules, decimal precision and the existing numeric API payload.
- V102's lean workflow forms do not post or reconcile invoices. `client_output_invoice_issue` and `client_input_invoice_closure` V2 require only an operational invoice status by default; number and semantic VND currency amount become visible and required only for `issued` or `received`. Canonical invoice/AR/AP routes, files, transitions, journals, tax and reconciliation remain separate and authoritative.

## Golden Sample

- `GOLDEN-REAL-001` is the reconciled review fixture for four AP and five AR obligations, schedules, settled advances, nine terminal payments/receipts, allocations, verified input invoice, issued output invoice and released contract retention. AP/AR remaining balances are zero before handover.
