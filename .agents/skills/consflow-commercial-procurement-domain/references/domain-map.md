# Commercial Procurement Domain Map

## Owners

- Frontend modules: `frontend/src/modules/boq`, `frontend/src/modules/procurement`, `frontend/src/modules/contracts`, `frontend/src/modules/work-items`
- Reconciliation panels: `SubcontractorReconciliationPanel.tsx`, `OwnerClientReconciliationPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- BOQ: `/projects/:id/boq`, `GET|PUT /boq-items/:id`, `POST /boq-items/:id/revise`, `/boq-items/:id/unit-price/expected|lock-norm|lock-final`, `/projects/:id/boq/submit|approve|generate-procurement`
- Supplier quotation: `/projects/:id/supplier-quotations`, `/projects/:id/supplier-quote-comparison`, `/supplier-quotations/:id`, `/supplier-quotations/:id/submit|qualify`, `/supplier-quotation-items/:id/select`
- Procurement: `/procurement-requests`, `/procurement-requests/:id`, `/procurement-requests/:id/submit|approve|order|receive`
- Reconciliation: `/work-items/:id/subcontractor-reconciliation`, `/work-items/:id/owner-client-reconciliation`
- Contracts: `/projects/:id/contracts`, `/contracts/:id` and version/milestone/variation/retention action routes

## Tables And Migrations

- `boq_items`, `boq_unit_price_events`
- `procurement_requests`, `procurement_request_items`
- `supplier_quotations`, `supplier_quotation_items`, `supplier_quote_selections`
- `subcontractor_checklist_reconciliations`, `subcontractor_checklist_reconciliation_items`
- `owner_client_checklist_reconciliations`, `owner_client_checklist_reconciliation_items`
- Related stock tables: `stock_documents`, `stock_document_items`, `stock_movements`
- Relevant migrations: `V1`, `V17`, `V21`, `V22`, `V61`; official workflow responsibility snapshot: `V86`

## Invariants

- BOQ, contracts and procurement remain the sole business ledgers for their records. A work-item `execution_target` may open these pages with presentation-only `?task=` context; `ProjectTaskContext` does not duplicate a commercial form, grant permission or perform a transition. Source-domain status/actions and workflow completion gates remain independently authoritative.
- Approved and final-locked BOQ prices drive procurement values; final price traces to a selected qualified quote or an explicit manual waiver reason.
- BOQ lifecycle fields are server-owned: create always starts at draft; content and expected price change only in draft/revised; norm lock requires submitted; approval requires every submitted line norm-locked; final lock requires approved. A revise request needs an audit reason, clears the approver, and is blocked after final lock or while any non-cancelled procurement request is linked. A norm-locked revised line keeps its commercial fields immutable and permits descriptive metadata only.
- V103 adds immutable project-level `boq_releases`. Each successful serialized project BOQ approval supersedes the prior current release and snapshots all approved rows, item count, total amount, actor/time and SHA-256 definition hash with a monotonic release number. Rev7 source gates read that release instead of asking users to retype BOQ/version/price data in workflow forms.
- Rev8 keeps BOQ, supplier quotations, quote selection, procurement requests and payable-direction contracts as the sole commercial ledgers. Its one-step BOQ packages still use `procurement.generate_from_boq`; the input-contract package checks both an approved procurement and an approved/signed/active source contract and never fabricates either record from workflow-form answers.
- Commercial workflow `other` free text is limited to `client_boq_owner_submission.sent_channel`, `client_hardcopy_dispatch.delivery_method`, `client_owner_soft_file_submission.delivery_channel`, `client_procurement_takeoff_review.takeoff_source`, and the registered subcontractor work-package forms. Draft may retain the sentinel; submit/amend stores trimmed custom text under the same answer key with a 300-character cap. These presentation/answer compatibilities do not approve a BOQ, create procurement, mutate contract dispatch records or weaken canonical commercial transitions.
- Submitted quotation items and selection decisions are commercial history; selection locks the BOQ final price and is blocked after procurement exists.
- Staff soft-delete preserves BOQ unit-price events, contract-variation events, procurement requester references and subcontractor/owner reconciliation attribution. Existing actor/submitter/requester name fields gain companion deletion flags for presentation only; no price lock, procurement status, reconciliation variance, contract transition or source-document link changes, and active commercial selectors exclude deleted identities.
- BOQ list/detail expose the newest non-cancelled procurement request plus `active_procurement_count`; count greater than one is an integrity warning, not proof of uniqueness. Price history is bounded and paired with total/truncation metadata.
- Procurement generation is permission-gated and serialized with BOQ lifecycle transitions, excludes lines already linked to active procurement, groups eligible BOQ lines by selected supplier and snapshots quotation item, supplier, price revision, and final-price event. Generation does not create stock movements, payables or receivables.
- Ordering requires supplier, order reference, and expected delivery date. Partial/full receive status is derived from submitted incoming acceptance quantities and does not implicitly create stock movement.
- Procurement receive is auditable and stock-aware: its acceptance-reconciliation audit event is distinct from the later warehouse receipt event, and list/detail contracts expose accepted quantity separately from explicitly stocked quantity.
- The procurement workspace is action-first and permission-aware. It may link to the incoming-acceptance work item or create an explicit receipt from a submitted acceptance, but it must not label acceptance-derived `received` as already stocked.
- The procurement report groups non-cancelled requests by status and budget status only after applying accessible-project and optional `project_id` scope; its CSV is generated from exactly that row set.
- Subcontractor reconciliation maps confirmed quantity and variance to BOQ/work item evidence.
- Owner/client reconciliation maps client decision and required action to acceptance/work item evidence.
- The contract workspace may reorganize and localize lifecycle records, but must keep signed-version immutability, variation-to-draft-version behavior and existing capability gates visible in the available actions.
- An approved payable with an approved budget creates an accounting commitment. A verified procurement/vendor input invoice recognizes actual cost and reduces that commitment in the append-only budget movement ledger; payment is a separate cash event and must not recognize the same cost again.
- Procurement, contract and invoice source IDs remain traceable on journals and budget movements. Commercial code must not insert or update posted journal lines, static budget actuals or paid balances outside the canonical accounting posting transaction.
- An ordinary project material-cost record may reference a same-project procurement request as management evidence. Creating or confirming that cost never mutates the procurement status, stock ledger, legacy payable/invoice/journal, or MISA; only confirmed records affect the scope-warned project-management profit view.

## Developer Sample Data

- A completed sample materializes one coherent BOQ → supplier quotation/selection → procurement chain. Final BOQ price remains locked to the selected quotation item; procurement links the same supplier/material/BOQ source.
- Sample partners and materials use unique project-derived codes and explicit simulated notes. Planning profiles do not pre-create unfinished commercial ledgers.

## Cross-Domain Checks

- Pair with materials/warehouse when procurement receive, stock movement, material balance, or surplus handling can change.
- Pair with quality/acceptance when owner/client reconciliation, outgoing acceptance, punch list, or handover readiness depends on commercial records.
- Pair with accounting/finance when procurement, invoices, payables, advances, payments, or vendor ledgers depend on BOQ or purchase data.

## Golden Sample

- `GOLDEN-REAL-001` is the review fixture for six approved/final-locked BOQ lines, a three-supplier/eighteen-line quote matrix, six selections, two received procurement requests, three active signed contracts, milestone obligations, variation history and released retention.

## Rev3 Contract Execution And Outside-Plan Variations

- The current contract version may be marked App-signed only by linking a same-project immutable `workflow_form_signature` with purpose `approved_by`; the link stores actor/time and is unique for both signature and contract version. `SignatureCaptureDialog` remains the only visual capture implementation.
- V103 adds monotonic `documents.revision_no` plus `release_status|released_at|released_by` per project/target/type series. A new protected document release supersedes the prior released row without deleting or mutating its stored artifact; Rev7 contract/document gates read canonical release/lifecycle records.
- Execution readiness is a fail-closed sequence: App sign -> paper dispatch -> upload/receive signed scan -> activate. `contract_dispatches` preserves recipient, method, tracking, sent/received actors/times and returned document. Project contract list rows expose `signed_scan_received` only for the current version so the UI can remind missing input/output scans.
- An outside-plan variation records `priority=urgent|normal`, optional construction `source_work_item_id`, and optional V88 `source_construction_action_id`. Contract authors may still create a draft, while the narrow construction proposal endpoint requires an active main-client receivable contract and eligible construction source, rejects duplicate active proposals per action, and atomically creates then submits to `qs_review`. Technical remediation is independent. Status continues `draft/submitted -> qs_review -> purchasing_sourcing -> pending_approval -> approved -> procurement_created`; urgent never skips a role gate, events are append-only, and procurement generation stays idempotent through `procurement_request_id`.
- Rev3 moves input takeoff/price sourcing after outbound-contract readiness. The single negotiation node precedes the input-contract/advance node; BOQ/procurement approval invariants remain unchanged.
- Rev4 `...023` keeps those gates and adds `Mua hàng` beside `QS` only on `boq_without_design.pricing`; this responsibility snapshot does not approve the BOQ, lock final prices or generate procurement by itself.
- Rev4 Operations UAT project `...025` seeds one active main-client receivable contract solely so the construction proposal UI can enter `qs_review`. That synthetic contract is not evidence of App signature, paper dispatch, signed-scan return, upstream execution readiness, or commercial approval completion.
- BOQ prices, supplier quotation prices/discounts, procurement values, contract/version/milestone/retention amounts and variation value deltas are monetary inputs and must use shared `CurrencyInput` or semantic `currency`, honoring the quotation/contract currency when present. Quantities, tax percentages, payment-term days and schedule ordering remain `number`.
