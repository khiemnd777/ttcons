# Materials Warehouse Domain Map

## Owners

- Frontend modules: `frontend/src/modules/warehouse`, `frontend/src/modules/work-items`
- Work item panels: `MaterialSnapshotPanel.tsx`, `SurplusHandlingPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Catalog and stock: `/materials`, `/warehouses`, `/stock`, `/stock/positions`, `/stock/movements`
- Multi-line stock documents: `/stock/documents`, `/stock/documents/:id`
- Stock actions: `/stock/receive`, `/stock/issue`, `/stock/transfer`, `/stock/return-to-warehouse`, `/stock/return-to-subcontractor`
- Site balances and receipt sources: `/projects/:id/material-balances`, `/projects/:id/warehouse-receipt-options`
- Work item material snapshots: `/work-items/:id/material-snapshot`
- Surplus handling: `/work-items/:id/surplus-handlings`, `/surplus-handlings/:id`, `/surplus-handlings/:id/submit`

## Tables And Migrations

- `materials`, `warehouses`, `stock_documents`, `stock_document_items`, `stock_movements`, `site_material_balances`
- `material_before_after_snapshots`, `material_before_after_snapshot_items`
- `site_surplus_handlings`
- Related evidence: `evidence_records`, `documents`
- Relevant migrations: `V1`, `V6`, `V23`, `V24`, `V33`, `V61`

## Invariants

- Warehouse remains the canonical stock ledger. Presentation-only `?task=` context may link a workflow item into the warehouse workspace, but it never creates or posts stock, bypasses incoming acceptance or changes balance/movement traceability; all stock mutations remain source-domain actions.
- Every stock operation keeps source, target, actor, quantity, material, and project traceability.
- Posted stock documents are multi-line and atomic. Transfer documents require distinct active source/target warehouses, lock both balances in stable order and count the same quantity out of the source and into the target.
- Submitted incoming acceptance is only a selectable receipt source. A linked receipt cannot exceed `accepted_quantity - previously linked receive movements`; acceptance never posts stock by itself.
- Procurement accepted-vs-stocked progress also recognizes historical explicit receive movements linked to the originating `boq_item`; this compatibility source is capped at accepted quantity and blocks duplicate acceptance-linked receipts.
- The stock workspace has four explicit sources of truth: warehouse positions, posted documents/ledger, project site/surplus balance and active material catalog.
- Material creation presents `default_supplier_partner_id` as `Nhà cung cấp tham khảo` because a selected quotation supplier still takes precedence; `default_supplier` is the conditional `Tên nhà cung cấp khác` fallback. The naming is presentation-only and does not change supplier validation, snapshots or procurement grouping.
- Material snapshot submit applies site balance usage but does not create warehouse movement by itself.
- Surplus handling can be partial and must keep remaining/handled state consistent.
- Returning only part of the available site material leaves `surplus_status='partial'` when a positive surplus remains; it becomes `handled` only when no surplus remains. Stock SQL binds quantities explicitly as numeric to avoid PostgreSQL parameter type ambiguity.
- New surplus submissions accept only a value currently enabled by the database-backed `surplus_handling_options` registry setting. The work-item API returns those server-owned choices for presentation; disabling a choice does not rewrite historical `site_surplus_handlings` rows or material balances.
- Warehouse report and stock list must agree on active material and movement state. Report stock movement totals and site-surplus totals are aggregated separately before joining active materials so multiple movements cannot multiply multiple site balances; optional project filtering and CSV use the same actor scope.

## Cross-Domain Checks

- Pair with procurement when receive/order flows create stock movements or material obligations.
- Procurement `receive` now reconciles submitted incoming-acceptance quantities into `partially_received`/`received`; it does not create `stock_movements`. Warehouse receipt remains an explicit stock action with its own balance ledger and audit.
- Submitted incoming acceptance and posted stock documents can be selected as same-project source evidence for `project_material_costs`. This reference is read-only: material-cost creation/confirmation never posts, reverses or changes warehouse movements/balances.
- Pair with construction operations when site usage, before/after snapshots, or surplus handling changes work item state.
- Pair with quality/acceptance/handover when incoming acceptance, outgoing acceptance, or handover blockers depend on material state.
- Material `standard_price` is a monetary input and uses semantic `currency`/shared `CurrencyInput`; stock, acceptance, snapshot and surplus quantities remain numeric quantity fields and must not be converted to currency.

## Golden Sample

- `GOLDEN-REAL-001` traces procurement acceptance into explicit receive/issue/return movements, six site balances, a submitted material snapshot and six surplus-return histories; final surplus is zero and every balance is `handled`.
