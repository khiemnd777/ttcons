# Materials Warehouse Domain Map

## Owners

- Frontend modules: `frontend/src/modules/warehouse`, `frontend/src/modules/work-items`
- Navigation: global `frontend/src/layouts/AppShell.tsx`, project-scoped `frontend/src/modules/projects/projectWorkspaceNavigation.ts`
- Work item panels: `MaterialSnapshotPanel.tsx`, `SurplusHandlingPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Catalog and stock: `GET|POST /materials`, `PUT /materials/:id`, `/warehouses`, `/stock`, `/stock/positions`, `/stock/movements`
- Multi-line stock documents: `GET|POST /stock/documents`, `GET|PUT /stock/documents/:id`; audited physical-count corrections: `POST /stock/adjustments`
- Stock actions: `/stock/receive`, `/stock/issue`, `/stock/transfer`, `/stock/return-to-warehouse`, `/stock/return-to-subcontractor`
- Site balances and receipt sources: `/projects/:id/material-balances`, `POST /projects/:id/material-balances/:materialId/adjustments`, `/projects/:id/warehouse-receipt-options`
- Work item material snapshots: `/work-items/:id/material-snapshot`
- Surplus handling: `/work-items/:id/surplus-handlings`, `/surplus-handlings/:id`, `/surplus-handlings/:id/submit`

## Tables And Migrations

- `materials`, `warehouses`, `stock_documents`, `stock_document_items`, `stock_movements`, `site_material_balances`, `site_material_adjustments`
- `material_before_after_snapshots`, `material_before_after_snapshot_items`
- `site_surplus_handlings`
- Related evidence: `evidence_records`, `documents`
- Relevant migrations: `V1`, `V6`, `V23`, `V24`, `V33`, `V61`, `V132`

## Invariants

- Warehouse remains the canonical stock ledger. Presentation-only `?task=` context may link a workflow item into the warehouse workspace, but it never creates or posts stock, bypasses incoming acceptance or changes balance/movement traceability; all stock mutations remain source-domain actions.
- Every stock operation keeps source, target, actor, quantity, material, and project traceability.
- Soft-deleting the creator/submitting account never changes or deletes a stock document, stock movement, site balance or material snapshot. Dynamic stock-document creator and material-snapshot submitter names keep their original fields plus companion deletion flags for the shared deleted-account label; active warehouse/member selectors continue to exclude deleted identities.
- Posted stock documents are multi-line and atomic. Transfer documents require distinct active source/target warehouses, lock both balances in stable order and count the same quantity out of the source and into the target.
- Posted document metadata (date/note) may be updated in place with audit. Any project/type/warehouse/source/item/quantity change requires a correction reason, atomically reverses the old movements, posts one replacement document, links both documents and keeps the old document as `corrected`; adjustment documents themselves are append-only.
- Physical stock correction never overwrites a balance. `POST /stock/adjustments` serializes the warehouse/material pair and posts `adjustment_in` or `adjustment_out` from the current ledger quantity to the non-negative target. Stock positions, receipt caps, warehouse reports and ledgers include those movement types with the same sign semantics.
- Material name/category/supplier metadata remains editable. Material code and unit are editable only before any operational reference exists; after stock/document/site/BOQ/procurement/quotation/acceptance/snapshot/surplus usage, both are server-locked. Every new stock line must use the active material master's exact unit.
- Site `issued_quantity` remains movement-derived. A usage correction may set only a non-negative `used_quantity` no greater than issued, recalculates surplus/status and appends `site_material_adjustments` plus audit; stock-document reversal corrections append separate issued correction rows and fail closed if the final issued quantity would be below used.
- Submitted incoming acceptance is only a selectable receipt source. A linked receipt cannot exceed `accepted_quantity - previously linked receive movements`; acceptance never posts stock by itself.
- Procurement accepted-vs-stocked progress also recognizes historical explicit receive movements linked to the originating `boq_item`; this compatibility source is capped at accepted quantity and blocks duplicate acceptance-linked receipts.
- The stock workspace has four explicit sources of truth: warehouse positions, posted documents/ledger, project site/surplus balance and active material catalog.
- The standard App shell exposes `/warehouse` under `warehouse.view`, immediately below `Đối tác`; the project business tree keeps `/projects/:id/warehouse`. Global navigation does not make project site/surplus data global: `Tại công trình & dư` still requires a selected project, and posted documents apply the project filter when one is selected.
- Material creation presents `default_supplier_partner_id` as `Nhà cung cấp tham khảo` because a selected quotation supplier still takes precedence; `default_supplier` is the conditional `Tên nhà cung cấp khác` fallback. The naming is presentation-only and does not change supplier validation, snapshots or procurement grouping.
- Material `standard_price` remains schema/API compatibility for historical integrations but is absent from the primary Warehouse create form and material table. Warehouse owns quantity, custody and movement traceability; users do not re-declare accounting valuation here.
- Material snapshot submit applies site balance usage but does not create warehouse movement by itself.
- Surplus handling can be partial and must keep remaining/handled state consistent.
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
