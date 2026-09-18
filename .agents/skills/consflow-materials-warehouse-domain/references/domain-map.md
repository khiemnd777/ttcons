# Materials Warehouse Domain Map

## Proposal Context From Existing Stock

- V145 proposals may link exact posted `issue` document IDs from the same project. Save checks warehouse permission and identity; submit rechecks posted state. `AccountingOperationsWorkspace` lists these documents separately, without requiring a proposal to exist. Corrected/cancelled or cross-project issues are not eligible.
- `accounting_requests.go` / `AccountingOperationsDialog.tsx` own the reference adapter, not stock or valuation. No implicit payable, cost, receipt or movement is created by linking existing stock. First searches: `stock_document_ids|referencesValid|AccountingOperationsWorkspace`. Warehouse canonical document viewer remains the destination; movement quantities and physical-unit invariants remain unchanged.

## Accounting Receipt Trace

- Read-only Accounting detail uses `accounting_handoff_detail.go::accountingHandoffStockReferences` to locate posted receipt documents through typed procurement/acceptance headers, exact acceptance-item lines or historical BOQ-linked receipt movements. EXISTS avoids duplicate documents when several lines resolve to the same receipt; both source and document are project-scoped. Warehouse permission is required independently of handoff visibility.
- This projection never posts stock, values inventory or creates AP. No visible linked receipt means only that no authorized link was found. Canonical `/stock/documents/:id` and exact record navigation remain the source detail owners. Test owner: `accounting_handoff_detail_test.go`.

## Exact Record Source Navigation

- Movement summary formats PostgreSQL numeric strings in Vietnamese: `800.000` displays as `800`, preserving fractional quantities with commas. Do not print SQL decimal strings as Vietnamese thousands or change stored quantities to repair display.
- Stock document detail accepts exact record links and exposes typed source, prior-document and replacement-document references. Movement previews open `/source-records/stock_movement/:id` using authorized ledger reads; only the persisted stock_document_id links to a stock document. Incoming acceptance references use a separate read-only `/source-records/incoming_acceptance/:id` viewer with the existing source permission/Work Item checks. New links never use the Warehouse `receipt` create intent and never post stock or change quantities.
- Shared owners: `modules/_shared/{recordNavigation,RecordLinks,useRecordDeepLink,RecordSourcePage}`. First searches: `recordPath|relatedRecordTargets|useRecordDeepLink`; see frontend/backend detail maps and `recordNavigation.test.ts` for the navigation contract.

## Discussion Stock Movement Topics

- `StockDocumentDetail` leads with document code/type/status, a distinct warehouse direction, recording/source metadata and shared responsive `RecordTable` lines. Correction/replacement notices remain visible and empty documents have an explicit empty state. Discussion movement previews use `TopicRecordSummary` plus grouped source/target/document fields; their IDs and source checks remain movement-specific. Shared styling never posts, corrects or recalculates warehouse data.

- `ActivityTopicDetails.tsx` inspects an exact stock movement from authorized `GET /stock/movements?project_id=` using persisted activity scope/id plus a project check. Display fields come from the movement ledger (material, quantity/unit, source/destination warehouse, document code, reason and timestamp). A movement ID is never passed to the stock-document detail route. Viewing performs no stock/document mutation. First searches: `activityTopicPath|activityTopicRecord|stock_movement` in the shared activity topic owners.

## Owners

- Frontend modules: `frontend/src/modules/warehouse`, `frontend/src/modules/work-items`
- Navigation: global `frontend/src/layouts/AppShell.tsx`, project-scoped `frontend/src/modules/projects/projectWorkspaceNavigation.ts`
- Work item panels: `MaterialSnapshotPanel.tsx`, `SurplusHandlingPanel.tsx`
- Backend routes: `backend/internal/core/routes.go` bootstrap plus bounded `backend/internal/core/routes_*.go` registrars
- Backend logic: `backend/internal/core/handlers.go` foundation plus bounded `backend/internal/core/handlers_*.go` owners, `service_materials_warehouse.go`, `service_evidence.go`, `repository_materials_warehouse.go`, physical-unit subledger owner `repository_material_stock_units.go`, `repository_domain.go` foundation plus bounded `repository_*.go` owners, `types.go` foundation plus bounded `types_*.go` contracts
- Contracts: `frontend/src/lib/types/domain.ts` compatibility barrel plus bounded `frontend/src/lib/types/domain/*.ts` contracts, `frontend/src/lib/utils/status.ts`

## Routes

- Catalog and stock: `GET|POST /materials`, `PUT /materials/:id`, `/warehouses`, `/stock`, `/stock/positions`, `/stock/movements`
- Reusable physical units: `GET /stock/units`, `GET /stock/units/:id`, `POST /materials/:id/stock-units/initialize`
- Global/warehouse physical-unit reads require `warehouse.view`. A project-filtered unit read may also be used by the authoritative Work Item panels under exact-project `operation.report` or `warehouse.surplus_handle`; it must remain project-filtered and never expose another project or global warehouse inventory through those permissions.
- Multi-line stock documents: `GET|POST /stock/documents`, `GET|PUT /stock/documents/:id`; audited physical-count corrections: `POST /stock/adjustments`
- Stock actions: `/stock/receive`, `/stock/issue`, `/stock/transfer`, `/stock/return-to-warehouse`, `/stock/return-to-subcontractor`
- Site balances and receipt sources: `/projects/:id/material-balances`, `POST /projects/:id/material-balances/:materialId/adjustments`, `/projects/:id/warehouse-receipt-options`
- Work item material snapshots: `/work-items/:id/material-snapshot`
- Surplus handling: `/work-items/:id/surplus-handlings`, `/surplus-handlings/:id`, `/surplus-handlings/:id/submit`

## Tables And Migrations

- `materials`, `warehouses`, `stock_documents`, `stock_document_items`, `stock_movements`, `site_material_balances`, `site_material_adjustments`
- `material_stock_units`, immutable `material_stock_unit_events`, `stock_movement_unit_allocations`
- `material_before_after_snapshots`, `material_before_after_snapshot_items`
- `site_surplus_handlings`
- Related evidence: `evidence_records`, `documents`
- Relevant migrations: `V1`, `V6`, `V23`, `V24`, `V33`, `V61`, `V132`, `V144`

## Invariants

- Warehouse remains the canonical stock ledger. Presentation-only `?task=` context may link a workflow item into the warehouse workspace, but it never creates or posts stock, bypasses incoming acceptance or changes balance/movement traceability; all stock mutations remain source-domain actions.
- Every stock operation keeps source, target, actor, quantity, material, and project traceability.
- `stock_movements` remains the canonical aggregate ledger. For a `physical_unit` material, `material_stock_units` is the exact-unit subledger: every receive/issue/transfer/return allocation total must equal its stock-document line, split descendants preserve root/parent lineage, and all changes append immutable unit events.
- Physical-unit initialization is an explicit, audited one-time conversion. It is allowed only when no material remains at a project and the declared units equal the aggregate ledger balance in every warehouse; a material-scoped advisory lock prevents concurrent stock movement during conversion.
- Only warehouse units with `condition='usable'` and `status='available'` are eligible for issue or transfer. Returned damaged/mixed/unknown units remain part of the aggregate warehouse ledger but enter quarantine and are excluded from reusable quantity.
- Soft-deleting the creator/submitting account never changes or deletes a stock document, stock movement, site balance or material snapshot. Dynamic stock-document creator and material-snapshot submitter names keep their original fields plus companion deletion flags for the shared deleted-account label; active warehouse/member selectors continue to exclude deleted identities.
- Posted stock documents are multi-line and atomic. Transfer documents require distinct active source/target warehouses, lock both balances in stable order and count the same quantity out of the source and into the target.
- Posted document metadata (date/note) may be updated in place with audit. For aggregate materials, any project/type/warehouse/source/item/quantity change requires a correction reason, atomically reverses the old movements, posts one replacement document, links both documents and keeps the old document as `corrected`; adjustment documents themselves are append-only. If a posted document has physical-unit allocations, core stock fields are fail-closed: keep the document immutable and post a new return/transfer document that selects the exact units.
- Physical stock correction never overwrites a balance. `POST /stock/adjustments` serializes the warehouse/material pair and posts `adjustment_in` or `adjustment_out` from the current ledger quantity to the non-negative target for aggregate materials. Tracked physical materials reject this shortcut and require a stock document with exact unit allocations. Stock positions, receipt caps, warehouse reports and ledgers include adjustment movement types with the same sign semantics.
- Material name/category/supplier metadata remains editable. Material code, unit, tracking mode and form factor are editable only before any operational reference exists; after stock/document/site/BOQ/procurement/quotation/acceptance/snapshot/surplus usage, those identity fields are server-locked. Every new stock line must use the active material master's exact unit.
- Site `issued_quantity` remains movement-derived. For aggregate materials, a usage correction may set only a non-negative `used_quantity` no greater than issued, recalculates surplus/status and appends `site_material_adjustments` plus audit; stock-document reversal corrections append separate issued correction rows and fail closed if the final issued quantity would be below used. Tracked physical materials reject direct usage correction and require a Work Item material snapshot with exact per-unit used/waste allocation.
- Submitted incoming acceptance is only a selectable receipt source. A linked receipt cannot exceed `accepted_quantity - previously linked receive movements`; acceptance never posts stock by itself.
- Procurement accepted-vs-stocked progress also recognizes historical explicit receive movements linked to the originating `boq_item`; this compatibility source is capped at accepted quantity and blocks duplicate acceptance-linked receipts.
- The stock workspace has five explicit sources of truth for warehouse actors: warehouse positions, posted documents/ledger, project site/surplus balance, reusable physical units and the active material catalog.
- The standard App shell exposes `/warehouse` under `warehouse.view`, immediately below `Đối tác`; the project business tree keeps `/projects/:id/warehouse`. Global navigation does not make project site/surplus data global: `Tại công trình & dư` still requires a selected project, and posted documents apply the project filter when one is selected.
- The standard `/warehouse` document content inherits `AppShell` Utility Dock end clearance (`8rem + safe-area` on mobile, `4rem` from `sm`). Its stock-table paging remains fully visible above the fixed launcher; this is layout-only and does not alter warehouse queries, stock balances or movement actions.
- Material creation presents `default_supplier_partner_id` as `Nhà cung cấp tham khảo` because a selected quotation supplier still takes precedence; `default_supplier` is the conditional `Tên nhà cung cấp khác` fallback. The naming is presentation-only and does not change supplier validation, snapshots or procurement grouping.
- Material `standard_price` remains schema/API compatibility for historical integrations but is absent from the primary Warehouse create form and material table. Warehouse owns quantity, custody and movement traceability; users do not re-declare accounting valuation here.
- Material snapshot submit applies site balance used/waste quantities but does not create warehouse movement by itself. A tracked material must allocate the exact project units and preserve remaining code/dimensions/condition; a draft may remain incomplete, but submit fails closed on any quantity mismatch.
- Surplus handling can be partial and must keep remaining/handled state consistent.
- Tracked surplus handling stores exact unit allocations in draft and atomically moves/splits those units with the aggregate return movement on submit. Only usable returned units become reusable; other conditions are quarantined.
- Returning only part of the available site material leaves `surplus_status='partial'` when a positive surplus remains; it becomes `handled` only when no surplus remains. Stock SQL binds quantities explicitly as numeric to avoid PostgreSQL parameter type ambiguity.
- New surplus submissions accept only a value currently enabled by the database-backed `surplus_handling_options` registry setting. The work-item API returns those server-owned choices for presentation; disabling a choice does not rewrite historical `site_surplus_handlings` rows or material balances.
- Warehouse report and stock list must agree on active material and movement state. Report stock movement totals and site-surplus totals are aggregated separately before joining active materials so multiple movements cannot multiply multiple site balances; optional project filtering and CSV use the same actor scope.

## Developer Sample Data

- Completed samples create a project-derived site warehouse, accepted receive document/movement and equal issue document/movement. The resulting site balance records planned=issued=used and zero surplus/closed.
- Evidence and source IDs trace the receive to incoming acceptance and the issue to the completed construction work item. No direct stock quantity overwrite is allowed.

## Cross-Domain Checks

- Pair with procurement when receive/order flows create stock movements or material obligations.
- Procurement `receive` now reconciles submitted incoming-acceptance quantities into `partially_received`/`received`; it does not create `stock_movements`. Warehouse receipt remains an explicit stock action with its own balance ledger and audit.
- Submitted incoming acceptance and posted stock documents remain canonical readiness/quantity evidence attached to their Procurement source. They do not create separate Accounting Handoff rows; handoff transitions never post, reverse or change warehouse movements/balances. Rev8/legacy projects may still select them as old `project_material_costs` evidence.
- Pair with construction operations when site usage, before/after snapshots, or surplus handling changes work item state.
- Pair with quality/acceptance/handover when incoming acceptance, outgoing acceptance, or handover blockers depend on material state.
- Stock, acceptance, snapshot and surplus quantities remain numeric quantity fields. The ordinary Warehouse UI has no editable monetary field; if a compatibility maintenance surface ever edits `standard_price`, it must still use semantic `currency`/shared `CurrencyInput`.

## Golden Sample

- `GOLDEN-REAL-001` traces procurement acceptance into explicit receive/issue/return movements, six site balances, a submitted material snapshot and six surplus-return histories; final surplus is zero and every balance is `handled`.
- Posted stock documents opened from a canonical Warehouse Work Item may carry explicit `work_item_context`; work-item-owned material snapshots and surplus handling derive the same start intent from their persisted link. The stock/source rows, movements, balance/audit and Work Item transition share one transaction. Wrong project/action/workspace, future/terminal work or missing permission rolls back the warehouse mutation; auto-start never changes quantities, invents movements or marks surplus handled.

## Notification Platform Boundary

- Material before/after snapshot, incoming acceptance and surplus-handling selectors expose a separate `Xem` action for the selected protected evidence. The shared viewer is presentation-only: it does not change the selected evidence ID, submit payload, accepted quantities, site-balance usage, surplus handling state, custody, warehouse movement or stock traceability.

- V126 catalogs stock-document completion, material-snapshot review and surplus assignment candidates disabled. Notification projection must never post a movement, change a balance or mark surplus handled. Future producers must emit only after the same authoritative stock/snapshot/surplus transaction succeeds and resolve current requester/custodian/reviewer scope without duplicating warehouse ledgers.

## Design Material Catalog Boundary

- V134 separates active material-master observation into `material.view`. The Design role receives this permission but not `warehouse.view`; `GET /materials` accepts `material.view|warehouse.view|procurement.view`, while every warehouse, stock, site-balance, partner and mutation endpoint keeps its existing gate.
- The standard shell exposes a Design-only `Vật tư` entry at `/warehouse?tab=materials`. `WarehousePage` detects the absence of `warehouse.view`, enables only the `/materials` query and renders only material search/table. It must not fetch or reveal warehouses, stock positions, stock documents, site balances, receipt options, physical units, suppliers or inventory statistics. Warehouse actors use the five-tab `Kho vật tư` workspace, including `Phần dư tái sử dụng`.
- Material visibility is reference data for design choices only. It does not reserve stock, select a supplier, create BOQ/Procurement, change price, post a movement or expose quantities.
