# [Closed] Pipeline Builder Checklist Drag Drop

Status: closed on 2026-06-13. Checklist drag-drop sorting has been implemented and validated; this note is retained as historical reference only.

## Objective

Allow Pipeline Builder node `Checklist / Step` rows to be reordered by drag-drop while preserving the existing persisted `order_index` behavior.

## Completed Scope

- Added drag handle, drag state, drop indicator, and empty/pending states to the node inspector checklist list.
- Reused the existing pipeline step update endpoint to persist reordered `order_index` values.
- Removed the legacy up/down arrow sort controls after drag-drop was available.
- Updated checklist reorder persistence to submit a complete ordered step-id list and update `order_index` through `PUT /pipeline-steps/:id`.

## Validation

- `cd frontend && npm run build`: passed.
- `cd frontend && npm run build` after arrow removal: passed.
- Browser/Playwright verification skipped per repo instruction because it was not explicitly requested.

## Key Files

- `frontend/src/modules/pipeline-builder/components/NodeInspectorPanel.tsx`
- `frontend/src/modules/pipeline-builder/PipelineBuilderPage.tsx`
- `docs/current-states.md`
