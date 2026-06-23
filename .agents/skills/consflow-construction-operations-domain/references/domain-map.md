# Construction Operations Domain Map

## Owners

- Frontend modules: `frontend/src/modules/operations`, `frontend/src/modules/work-items`, `frontend/src/modules/my-work`, `frontend/src/modules/projects`
- Work item panels: `WorkflowFormPanel.tsx`, `EvidencePanel.tsx`, `EvidenceGallery.tsx`, `ActivityThread.tsx`, `ActivityComposer.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Operations: `/projects/:id/daily-reports`, `/projects/:id/issues`, `/issues/:id`
- Work items: `/work-items/:id/*`, `/projects/:id/work-items`, `/me/work-items`
- Evidence: `/work-items/:id/evidence`, `/work-items/:id/evidence/upload`, `/evidence/:id/file|accept|reject`
- Activity: `/work-items/:id/activity`, `/projects/:id/activity`

## Tables And Migrations

- `operation_daily_reports`, `issues`, `work_items`, `work_item_activities`
- `evidence_records`, `documents`
- Form-backed site records: `workflow_form_instances`, `workflow_form_answers`, `workflow_form_events`
- Checklist/report seed forms: `V18`, `V19`, `V20`
- Evidence/storage migrations: `V6`, `V15`, `V16`

## Invariants

- Evidence records remain permissioned and checksum-backed.
- Site daily reports preserve did/will, crew, actual quantity, material usage, and photo evidence when present.
- Field checklist form flows remain separate from reconciliation and final acceptance workflows.
- Work item actions preserve actor, comments, status, and activity history.

## Cross-Domain Checks

- Pair with commercial/procurement when actual quantities, subcontractor checklist data, owner/client checklist data, or reconciliation variance changes.
- Pair with materials/warehouse when material usage, before/after snapshots, surplus, stock issue, stock return, or site balance changes.
- Pair with quality/acceptance/handover when evidence, checklist status, punch list, rework, or acceptance readiness changes.
