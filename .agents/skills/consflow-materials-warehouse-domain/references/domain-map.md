# Materials Warehouse Domain Map

## Owners

- Frontend modules: `frontend/src/modules/warehouse`, `frontend/src/modules/work-items`
- Work item panels: `MaterialSnapshotPanel.tsx`, `SurplusHandlingPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Catalog and stock: `/materials`, `/warehouses`, `/stock`
- Stock actions: `/stock/receive`, `/stock/issue`, `/stock/transfer`, `/stock/return-to-warehouse`, `/stock/return-to-subcontractor`
- Site balances: `/projects/:id/material-balances`
- Work item material snapshots: `/work-items/:id/material-snapshot`
- Surplus handling: `/work-items/:id/surplus-handlings`, `/surplus-handlings/:id`, `/surplus-handlings/:id/submit`

## Tables And Migrations

- `materials`, `warehouses`, `stock_movements`, `site_material_balances`
- `material_before_after_snapshots`, `material_before_after_snapshot_items`
- `site_surplus_handlings`
- Related evidence: `evidence_records`, `documents`
- Relevant migrations: `V1`, `V6`, `V23`, `V24`, `V33`

## Invariants

- Every stock operation keeps source, target, actor, quantity, material, and project traceability.
- Material snapshot submit applies site balance usage but does not create warehouse movement by itself.
- Surplus handling can be partial and must keep remaining/handled state consistent.
- Returning only part of the available site material leaves `surplus_status='partial'` when a positive surplus remains; it becomes `handled` only when no surplus remains. Stock SQL binds quantities explicitly as numeric to avoid PostgreSQL parameter type ambiguity.
- Warehouse report and stock list must agree on active material and movement state.

## Cross-Domain Checks

- Pair with procurement when receive/order flows create stock movements or material obligations.
- Procurement `receive` now reconciles submitted incoming-acceptance quantities into `partially_received`/`received`; it does not create `stock_movements`. Warehouse receipt remains an explicit stock action with its own balance ledger and audit.
- Pair with construction operations when site usage, before/after snapshots, or surplus handling changes work item state.
- Pair with quality/acceptance/handover when incoming acceptance, outgoing acceptance, or handover blockers depend on material state.

## Golden Sample

- `GOLDEN-REAL-001` traces procurement acceptance into explicit receive/issue/return movements, six site balances, a submitted material snapshot and six surplus-return histories; final surplus is zero and every balance is `handled`.
