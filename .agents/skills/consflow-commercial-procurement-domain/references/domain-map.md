# Commercial Procurement Domain Map

## Current Accounting Retirement Boundary — V150/V151

- Internal obligation opening and retained-money release actions/routes/permissions/tracking tables are removed. Contract versions/signatures/dispatch/milestone eligibility/variations and commercial retention percentages/amounts remain project source information.
- accounting_reconciliation.go confirms expected comparison bases from ordered procurement or active eligible main-client/subcontractor milestones and protected evidence. There is no installment proposal/reservation/payment command or source-net cap on accountant-confirmed expected value.
- Source dimensions/project/partner/currency/direction and evidence are checked under transaction; commercial sources never mutate from MISA matching. Procurement planning estimates/budget_status remain operational context, not accounting balances.
- Current handoff readiness remains same-project supplier/PO/acceptance/stock context; no internal APAR/invoice/journal source is created.

## Project Manager Display Name

- Current `pm` presentation is Quản lý dự án. ContractsPage, contractsPresentation and shared status labels use this name in variation approval notices. Keep event keys/statuses, grants, stored `next_roles` and source transitions intact; V149 role compatibility is owned by Admin Governance/Workflow maps.
- Design-release handoff guidance names Quản lý dự án without rebinding the release handed to QS or modifying historical events/snapshots. First searches: `Quản lý dự án|appRoleDisplayName|consflow_role_label_key`.

## Accounting Proposal Source Boundary


## Required-Approval Work Item Gate Recovery

- `procurement.require_approved` is a project-wide fail-closed gate: every non-cancelled request must be `approved|ordered|partially_received|received`. It does not infer one source from Work Item titles or change procurement state. Blocked inspection returns the exact ordered request records and routes the oldest blocker through the canonical Procurement deep link; the normal server capability still determines whether the opened `ProcurementDialog` offers approval or read-only guidance.
- Source approval/order remains independently audited and does not auto-complete the Work Item. After source mutation, the Work Item is refetched and the actor explicitly retries completion. No BOQ price, quotation, stock, acceptance, payable, journal or finance side effect was added. First searches: `automateClientInputProcurementGate|requiredAutomationPresentation|useRecordDeepLink|procurementActionFor`. Regression: `TestLegacyOfficialProcurementGateExposesExactBlockingRequest` plus Work Item navigation tests.

## Procurement Actions In Discussion

- Shared source owner: `ProcurementDialog.tsx` serves the canonical purchase page, procurement topic cards and supported responsible Work Item picker. `ProcurementDetail.tsx` owns the read-only composition; `ProcurementSnapshot.tsx` renders immutable historical header/line values. The next source action is submit, approve or order based on server actor/project capabilities and status; linked structured approval must be complete before purchase approval.
- A ready `waiting_approval|submitted` detail exposes `approval_context` for all current active App accounts that actually pass `procurement.approve` plus project scope. This list explains who can act; it is not an assigned approver, does not turn the Work Item assignee into an approver, and never authorizes the mutation. Incomplete linked structured approvals retain their separate blocking message instead of receiving the source-approval candidate list.
- `procurement_workspace.go` owns detail capabilities, locked header updates and procurement snapshot composition. Header writes accept only title/supplier/PO/date, preserve source amount/budget/status/approval references, and reject a supplier that contradicts snapshotted BOQ/quotation lines. V139 `edit_version` and optional `expected_version` protect review-to-write consistency, including a header change between reviewing and approving. Version rejection does not erase the frontend draft.
- Successful BOQ generation and submit/approve/order append exactly one source-event snapshot in the source transaction. Header saves, manual empty draft creation and draft creation from contract variations do not publish cards. Historic snapshots require current source view permission; source metadata in the feed carries no price or PO content. The existing receive/stock/accounting flows remain authoritative and are not performed by discussion replies.
- When header save succeeds but a subsequent formal action fails, the UI retains the saved version and clearly reports that the action did not complete. Unsupported task families never opt into an unrelated Work Item auto-start. First searches: `ProcurementWorkspace|UpdateProcurementHeader|insertProcurementDiscussionUpdateTx|procurementMutationWorkItemId`. Checks: procurement workspace integration plus source generation/approval, snapshot visibility, concurrency, no stock/payable creation and frontend behavior/render tests.
- Task deep links must recheck the exact Work Item after the dialog mounts even when `ProjectTaskContext` has populated a warm shared cache. While that check is loading or failed, source actions stay disabled with explicit status; once ready, source status plus actor capability controls the single CTA. First searches: `procurementTaskVerificationQueryOptions|discussionWorkDetailLoadState|ProcurementApprovalPanel`.

## Exact Record Source Navigation

- Quotation detail project authorization depends on `repository.go::safeTable("supplier_quotations")`; a missing exact table entry caused HTTP 500 before the existing project/permission check. Keep the bounded allowlist and handler permission checks intact.
- `GetSupplierQuotationDetail` includes item count, pre-tax sum and total sum from the exact quotation's persisted items, consistent with listing. The detail total must not silently fall back to zero because the API omitted its aggregate fields. The procurement integration test covers this read contract without changing selection, price approval or stored amounts.
- BOQ, supplier quotation and procurement details now use typed new-tab source links. BOQ purchase tabs open the exact procurement ID; quote links use the parent `selected_quotation_id`/line `quotation_id` projected from supplier_quotations, never supplier_quotation_item IDs. Canonical pages accept exact record links independently of table filters. Supplier catalog links use partner IDs; free-text PO remains plain text. Contract sources also resolve exact IDs before first-row selection. Navigation never auto-starts, approves, locks prices, creates procurement or posts stock.
- Shared owners: `modules/_shared/{recordNavigation,RecordLinks,useRecordDeepLink,RecordSourcePage}`. First searches: `recordPath|relatedRecordTargets|useRecordDeepLink`; see frontend/backend detail maps and `recordNavigation.test.ts` for the navigation contract.

## Discussion Source Detail Reuse

- Detail presentation uses shared `RecordSummary`/`RecordSection`; BOQ preserves all four tabs and optional caller-owned actions, Procurement owns its source status/value summary in both entry points, and supplier quotation detail adopts the same hierarchy. Procurement and quotation use container-responsive `RecordTable` cards below 48rem. Requested/accepted/stocked/awaiting quantities remain separate; `pending` item labels become `Chờ tiếp nhận`, waiver reasons remain accessible in a disclosure. Numeric BOQ/quotation quantity display normalizes number-shaped API values without changing quantity/price data. No new actions, source queries, approvals or stock/finance effects.

- `ActivityTopicDetails.tsx` reuses exported `BOQPage::BOQDetail` (four read-only tabs, optional price-management callback omitted) and `ProcurementDetail` after exact source/project/status checks. The ordinary BOQ page retains its price-management callback. Procurement topic opening is intercepted by `ActivityTopicProvider` to use the shared authorized action dialog above; the read-only detail renderer itself does not mutate any source. First searches: `ActivityTopicDetails|ActivityTopicProvider|BOQDetail|ProcurementDetail|activityTopicRecord`.

## Owners

- Frontend modules: `frontend/src/modules/boq`, `frontend/src/modules/procurement`, `frontend/src/modules/contracts`, `frontend/src/modules/work-items`
- Reconciliation panels: `SubcontractorReconciliationPanel.tsx`, `OwnerClientReconciliationPanel.tsx`
- Backend routes: `backend/internal/core/routes.go` bootstrap plus bounded `backend/internal/core/routes_*.go` registrars
- Backend logic: `backend/internal/core/handlers.go` foundation plus bounded `backend/internal/core/handlers_*.go` owners, `service_commercial_procurement.go`, `service_reconciliations.go`, `service_partners.go`, `repository_domain.go` foundation plus bounded `repository_*.go` owners, `types.go` foundation plus bounded `types_*.go` contracts
- Contracts: `frontend/src/lib/types/domain.ts` compatibility barrel plus bounded `frontend/src/lib/types/domain/*.ts` contracts, `frontend/src/lib/utils/status.ts`

## Routes

- BOQ: `/projects/:id/boq`, `GET|PUT /boq-items/:id`, `POST /boq-items/:id/revise`, `/boq-items/:id/unit-price/expected|lock-norm|lock-final`, `/projects/:id/boq/submit|approve|generate-procurement`
- Supplier quotation: `/projects/:id/supplier-quotations`, `/projects/:id/supplier-quote-comparison`, `/supplier-quotations/:id`, `/supplier-quotations/:id/submit|qualify`, `/supplier-quotation-items/:id/select`
- Procurement: `/procurement-requests`, paged project options `GET /procurement-requests/projects?search&cursor&limit&selected_id`, `/procurement-requests/:id`, `/procurement-requests/:id/submit|approve|order|receive`
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
- The procurement workspace is action-first and permission-aware. It may link to the incoming-acceptance work item or create an explicit receipt from a submitted acceptance, but it must not label acceptance-derived `received` as already stocked. Total is presented read-only as a value derived from purchase lines; the primary form/filter does not ask users to re-enter total or `budget_status`, and compatibility update payloads preserve stored fields.
- The global Procurement project picker is a read-only, searchable scope selector. Its options require `procurement.view` together with existing project access, exclude cancelled projects and preserve selected-id authorization; selecting a project only changes the list filter and never creates, approves, orders, receives or posts stock.
- The procurement report remains a legacy/source aggregate and may group stored budget status after applying accessible-project and optional `project_id` scope; the operational Procurement workspace no longer promotes budget status as an editable or primary filter.
- Subcontractor reconciliation maps confirmed quantity and variance to BOQ/work item evidence.
- Owner/client reconciliation maps client decision and required action to acceptance/work item evidence.
- The contract workspace may reorganize and localize lifecycle records, but must keep signed-version immutability, variation-to-draft-version behavior and existing capability gates visible in the available actions.

## Developer Sample Data

- A completed sample materializes one coherent BOQ → supplier quotation/selection → procurement chain. Final BOQ price remains locked to the selected quotation item; procurement links the same supplier/material/BOQ source.
- Sample partners and materials use unique project-derived codes and explicit simulated notes. Planning profiles do not pre-create unfinished commercial ledgers.

## Cross-Domain Checks

- Pair with materials/warehouse when procurement receive, stock movement, material balance, or surplus handling can change.
- Pair with quality/acceptance when owner/client reconciliation, outgoing acceptance, punch list, or handover readiness depends on commercial records.
- Pair with accounting reconciliation when procurement/contract source dossiers feed immutable comparison bases or imported MISA matching; no invoice/APAR/payment ledger owner remains.

## Current Sample Data


## Rev3 Contract Execution And Outside-Plan Variations

- The current contract version may be marked App-signed only by linking a same-project immutable `workflow_form_signature` with purpose `approved_by`; the link stores actor/time and is unique for both signature and contract version. `SignatureCaptureDialog` remains the only visual capture implementation.
- V103 adds monotonic `documents.revision_no` plus `release_status|released_at|released_by` per project/target/type series. A new protected document release supersedes the prior released row without deleting or mutating its stored artifact; Rev7 contract/document gates read canonical release/lifecycle records.
- Execution readiness is a fail-closed sequence: App sign -> paper dispatch -> upload/receive signed scan -> activate. `contract_dispatches` preserves recipient, method, tracking, sent/received actors/times and returned document. Project contract list rows expose `signed_scan_received` only for the current version so the UI can remind missing input/output scans.
- An outside-plan variation records `priority=urgent|normal`, optional construction `source_work_item_id`, and optional V88 `source_construction_action_id`. Contract authors may still create a draft, while the narrow construction proposal endpoint requires an active main-client receivable contract and eligible construction source, rejects duplicate active proposals per action, and atomically creates then submits to `qs_review`. Technical remediation is independent. Status continues `draft/submitted -> qs_review -> purchasing_sourcing -> pending_approval -> approved -> procurement_created`; urgent never skips a role gate, events are append-only, and procurement generation stays idempotent through `procurement_request_id`.
- Rev3 moves input takeoff/price sourcing after outbound-contract readiness. The single negotiation node precedes the input-contract/advance node; BOQ/procurement approval invariants remain unchanged.
- Rev4 `...023` keeps those gates and adds `Mua hàng` beside `QS` only on `boq_without_design.pricing`; this responsibility snapshot does not approve the BOQ, lock final prices or generate procurement by itself.
- BOQ prices, supplier quotation prices/discounts, procurement values, contract/version/milestone/retention amounts and variation value deltas are monetary inputs and must use shared `CurrencyInput` or semantic `currency`, honoring the quotation/contract currency when present. Quantities, tax percentages, payment-term days and schedule ordering remain `number`.
- BOQ release/procurement generation, supplier quotation/qualification/selection, input-contract procurement transitions and contract save/version/sign/dispatch/scan/activation accept explicit Work Item mutation context from the canonical task route. The source mutation and `ready -> in_progress` transition are one transaction and the backend matches the exact official action family (including payable vs receivable contract direction). No action skips BOQ approval, price locking, procurement, signature, paper dispatch, scan or activation gates; calls without context keep their ordinary module behavior.

## Notification Platform Boundary

- Subcontractor and owner/client reconciliation rows expose a separate shared-viewer action for their persisted work-item evidence ID/name. Viewing uses the existing protected evidence-file authorization and never changes confirmed quantity, variance, BOQ/acceptance linkage, client decision, required action, comment, reconciliation status, source attribution or any stock/quality/finance downstream state.

- V126 catalogs BOQ, quotation, procurement, contract and variation notification candidates disabled. No commercial recipient is currently inferred from a role label or sent a notification. Each future enablement must emit from the authoritative approval/selection/milestone transaction, preserve idempotency and reuse the exact decider/source-owner policy without bypassing price lock, procurement generation, signature or contract activation gates.
