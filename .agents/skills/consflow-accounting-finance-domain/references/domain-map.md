# Accounting Finance Domain Map

## Approved Project Management Boundary — 2026-10-05

- Consflow manages projects, commercial/operational source dossiers and reconciliation of imported MISA results. MISA is authoritative for actual receipts/payments, debt, bookkeeping and tax. No production business history exists in this preproduction retirement.
- V150 removes internal books/accounts/periods/journals, AP/AR/payments/invoices/budgets, VAT filing, old material-cost/cash-request tracking and payment-retention release records. Old professional accounting and money-report APIs/UI are removed, not retained in an archive or read-only branch. Applied V1–V149 SQL/checksums remain technical installation history.
- V151 implements approved direct source reconciliation: no installment proposal, proposal queue, advance recovery or retention release flow. Evidence-confirmed source bases replace proposals as the target of MISA matching and review. This map and current owner code are the durable contract; closed plan records are historical verification evidence only.
- Construction is not gated by collected money. Contract amounts, milestones, contractual retention percentages/amounts remain project information; neither source eligibility nor dossier acknowledgement proves payment.
- First searches: `CreateAccountingBasis|AccountingOperationsWorkspace|AllocateAccountingMISA|ReviewAccountingMISA|basis_id|accounting_reconciliation|source.accounting_handoff_close_ready`.

## Owners And Entry Points

- Source handoff: `accounting_handoff.go`, `accounting_handoff_detail.go`, `handlers_project_finance.go`, `service_project_finance.go`; current contract structs in `types_project_finance.go`.
- MISA/source reconciliation: `accounting_reconciliation.go`, `accounting_misa.go`, `accounting_operations_workspace.go`, `handlers_accounting_operations.go`; routes in `routes_finance.go`.
- Frontend: `modules/project-finance/AccountingHandoffPage.tsx`, `AccountingHandoffDetailDialog.tsx`, `AccountingHandoffActionDialog.tsx`, `AccountingOperationsDialog.tsx`, `accountingOperations.ts`. `ProjectFinancePage.tsx` delegates directly to handoff; there is no mode selection or legacy body.
- Company work: `my_work_queue.go`, `AccountingWorkLane.tsx`, `AccountingWorkCard.tsx`, `accountingWorkActions.ts`; these are handoff dossiers, not payment-proposal tasks.
- Shared source navigation: `recordNavigation.ts`, `RecordLinks.tsx`, `RecordSourcePage.tsx`; only existing current source records have targets. Retired payable/receivable/payment discussion scopes are rejected.
- Project options: `handlers_reports.go`, `service_reports.go`, `repository_reports.go`; `AsyncProjectPicker.tsx`, `ProjectScopeActions.tsx`. Search/list/selected use identical actor/project permission scope and bounded stable cursors.
- Generic backend helpers: `repository_value_helpers.go`, `service_date_helpers.go`; test database helper `isolated_database_test.go`. These are not ledger adapters.

## API, Schema And Permissions

- Handoff UI: `/accounting`, `/projects/:id/accounting`; project choices `GET /project-finance/projects`; current workspace `GET /projects/:id/project-finance`, dossier detail/create/action routes. Only `accounting_handoff.view` authorizes choices; no `project_finance.*` fallback.
- Direct reconciliation: current `/projects/:id/accounting-operations`, `/projects/:id/accounting-bases`, `misa-imports`, `misa-allocations`, `misa-reviews`. Allocation/review carry `basis_id`, not `request_id`.
- Tables: `accounting_handoffs`, `accounting_reconciliation_bases`, `accounting_misa_batches`, `accounting_misa_records`, `accounting_misa_allocations`, `accounting_misa_reviews`. Sources remain procurement requests and contract milestones with exact project/partner IDs.
- Permissions: `accounting_handoff.view|submit|acknowledge`, `accounting_reconciliation.view|manage`, existing single `accounting_misa.reconcile`. There are no invented separate MISA import/allocate/review grants. Company observer scope uses the explicit `project.reconciliation.view` marker and enumerated actions, never Admin/Director identity or wildcard mutation access.
- V150 withdraws exact obsolete standard/demo and official Rev1–Rev8 templates. Current Rev9 remains active; old definitions are immutable authoring records, not runnable legacy workflows.
- Migration retirement guards reject unexpected business data. Fresh install and empty local upgrade must reach the same schema. Seed is additive master-data only, never fake completed finance or a direct project insertion.

## Handoff And Company Work Invariants

- `Bàn giao kế toán` is the source dossier workspace. Source amounts are reference context, not authoritative financial totals; no cost/revenue/profit/VAT/cashflow calculations.
- Global project selection stays draft until explicitly applied to the `project` query; reset returns to no-project state. Fixed project routes never show the global selector.
- One handoff per project/source. `handed_off` stores an immutable snapshot; Accounting acknowledges or returns with a required reason. Only returned dossiers can be handed off again; the new snapshot does not erase events/audit.
- Procurement sources require approved-or-later eligibility and ordered-or-later readiness with supplier/PO. Main-client milestones require active contract, valid source state and required linked acceptance. Missing links do not prove no delivery or business event occurred.
- Optional MISA reference is user-entered dossier context. Acknowledgement is not financial approval or proof of payment; no MISA API sync is claimed.
- Final Rev9 `source.accounting_handoff_close_ready` requires physical handover plus every current candidate acknowledged. No-candidate is ready after physical handover. No automatic acknowledgement or payment is performed.
- Physical handover checks final acceptance, unresolved issues/NCR/rework/actions, surplus handling and mandatory documents; it does not query internal money tables or AP/AR booleans.
- Both handoff view/acknowledge authorize observing company dossiers; only global system `accounting` role makes pending handed-off dossiers personal responsibility. Admin/Developer may inspect under all/company lane without acquiring Accounting ownership.
- Handoff statuses map to personal queue categories only; generic Work Item filters do not become Accounting statuses. Current actions, actor identity, audit and same-source invalidation have one owner.
- Accounting observer excludes configuration/staff/roles/Audit/Event Logs/Settings. Company observation never grants commercial/project/PM approvals.

## Direct MISA Reconciliation Invariants

- An immutable, evidence-confirmed basis fixes source snapshot, exact partner, project, direction, currency and expected comparison amount. It is not a proposal, obligation, payment command or reservation.
- Matching uses `basis_id` and active confirmed MISA cash transactions with identical project/partner/direction/currency. Many-to-many partial matching is allowed; aggregate matching cannot exceed an imported transaction.
- Matched amount may exceed expected basis amount; signed difference is allocated amount minus basis amount. Difference never blocks project work and does not infer paid/unpaid state.
- Invalid/unavailable source, evidence or current source permission yields source_available=false and current reviewed=false. Block new matching/review; retain immutable basis and historical reviews. Source restoration can restore current review only with an unchanged fingerprint.
- Source-only detail allows accounting_reconciliation.view plus procurement.view/contract.view and exact project scope. handoff_access=false suppresses stored handoff snapshot/status/actions; historical handoff read still requires accounting_handoff.view.
- Project/record locking, caller operation UUIDs and unique constraints protect concurrent matching and retries. Revocation requires a reason and invalidates the reviewed fingerprint.
- Mutation authorization rechecks active/non-deleted actor, exact operation/source permission and canonical EvidenceDetail visibility after blocking eligibility locks, immediately before writes. Repository.querySource/withReadTransaction reuse the held transaction for read authorization without borrowing another pooled connection; writes remain owned by the caller. Revoke intentionally requires project/MISA permission while allowing unavailable-basis recovery.
- Review fingerprint derives immutable basis plus active allocations; changed matching makes old review stale. Review records acceptance of comparison, not cash execution.
- CSV import remains the explicit Consflow format, at most 2 MB/2000 rows, exact active partner IDs and one project/entity per batch, protected original file/digest/actor/date and replacement history. It is not arbitrary native MISA parsing or API sync.
- Balance-at-date records and cash transactions are separate. Missing data is not an unpaid balance. Allocated observations cannot be replaced until matching is revoked.
- The `Có chênh lệch` UI filter uses nonzero signed difference regardless of review status or allocation coverage; acknowledgement of a comparison never removes its difference. Balance-at-date rows use `Số dư phải thu`/`Số dư phải trả`, while transaction rows use `Thu`/`Chi`. These are imported observations, not internal obligations or payment execution.
- Protected source files use actual authorized endpoints and exact record/revision metadata with shared `EvidenceRecordViewerDialog`; never manufacture an evidence ID or bypass the standard viewer.
- History amounts/reference amounts remain decimal strings; shared currencyInput formatter renders large foreign-currency decimals and negative differences without Number coercion. Frontend convenience validation does not replace canonical backend NUMERIC caps.

## Developer Sample Data And Validation

- All profiles are book/account/period independent. Blueprint v5 removes book fields from options/request/result/idempotency.
- Completed sample creates actual current design/BOQ/contracts/construction/reconciliation/acceptance/physical handover and acknowledged handoff sources; each Rev9 source gate is validated before commit. It creates no payments, journals or fabricated MISA cash.
- Unknown/missing actions surface completed blockers; completing a sample cannot skip a source requirement. Planning progress, sample identities, role assignments and idempotency remain isolated from real users.
- Source validation and finance retirement tests, current reconciliation DB tests, sample integration, fresh/upgrade/guard/seed tests cover technical behavior. isolated_database_test.go creates schema_migrations in the fixture schema before running migrations and asserts all table owners resolve there. Builds and isolated tests do not prove browser/UAT acceptance.
- Editable money uses `CurrencyInput` or semantic currency; current source reference amounts remain read-only. Standard dialogs/viewers are shared at all entrypoints.
