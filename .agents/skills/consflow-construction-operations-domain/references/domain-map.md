# Construction Operations Domain Map

## Owners

- Frontend modules: `frontend/src/modules/operations`, `frontend/src/modules/quality`, `frontend/src/modules/work-items`, `frontend/src/modules/my-work`, `frontend/src/modules/projects`
- Work item panels: `WorkflowFormPanel.tsx`, `EvidencePanel.tsx`, `EvidenceGallery.tsx`, `ActivityThread.tsx`, `ActivityComposer.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Operations: `/projects/:id/daily-reports`, `/daily-reports/:id`, `/daily-reports/:id/evidence/upload`, `/projects/:id/issues`, `/issues/:id`
- Quality field execution: `/projects/:id/quality`, quality ITP/RFI/NCR/recurring action routes
- Work items: `/work-items/:id/*`, `/projects/:id/work-items`, `/me/work-items`
- Evidence: `/work-items/:id/evidence`, `/work-items/:id/evidence/upload`, `/evidence/:id/file|accept|reject`
- Activity: `/work-items/:id/activity`, `/projects/:id/activity`

## Tables And Migrations

- `operation_daily_reports`, `issues`, `work_items`, `work_item_activities`
- `quality_itp_plans`, `quality_itp_items`, `quality_inspection_requests`, `quality_ncrs`, `construction_recurring_plans`, `construction_recurring_occurrences`
- `evidence_records`, `documents`
- Form-backed site records: `workflow_form_instances`, `workflow_form_answers`, `workflow_form_events`
- Checklist/report seed forms: `V18`, `V19`, `V20`
- Evidence/storage migrations: `V6`, `V15`, `V16`
- Product-grade QA/QC and daily-report bridge migrations: `V51`, `V52`, `V53`

## Invariants

- Evidence records remain permissioned and checksum-backed.
- Site daily reports preserve did/will, crew, actual quantity, material usage, and photo evidence when present.
- The project construction UI is consolidated at `/projects/:id/operations`: status cards lead into `Nhật ký & tiến độ`, `Checklist chất lượng`, and `Vấn đề & khắc phục`. This is a presentation/read-model consolidation only; daily reports, issues, ITP/RFI/NCR, rework, and replanning keep their existing persistence and transitions.
- The construction timeline maps structured `completed_items.result=done|partial|blocked`, `next_plan_items`, and `blockers` to Đã làm, Đang làm, Sẽ làm, and Vấn đề. Legacy quick reports encode the same three work groups in `work_summary` and must remain readable; an issue record is still distinct from a report blocker.
- Direct construction report create/update is a typed operations command, not a workflow transition. It persists structured work/resource/blocker data plus legacy summaries, may reference one optional source work item, and may link at most 50 evidence records that belong to that same project/work item, are not rejected and are visible to the actor; it does not create a work item or workflow form instance. Update preserves project/reporter identity and emits audit.
- The construction-report UI and its user-facing validation label the optional source work item as `Công việc liên quan`; persistence and API contracts continue to use `work_item_id` and the same work-item evidence semantics.
- The specialized create/edit dialog and detail use the same hierarchy and frontend read/edit model. Structured blocked completed items plus structured `blockers` are the blocker truth in the red card; legacy/free-text `issues_summary` remains a neutral field note so informational statements such as “Không có sự cố an toàn” are not presented as active blockers.
- Existing linked report evidence is selected through the permission-scoped project evidence read. New files and real-camera captures may be queued before a source is selected, but saving uploads requires the source. `/daily-reports/:id/evidence/upload` delegates storage and permission checks to the existing work-item evidence flow, then links the resulting evidence to the report; there is no project-level evidence storage. Manual detail supports edit and evidence-only quick add, always renders `Minh chứng · N`, and workflow-form-backed reports reject direct update/upload so the source form remains authoritative.
- Submitted `builtin_construction_daily_report` forms are idempotently bridged into `operation_daily_reports` by `workflow_form_instance_id`; the ledger preserves its source work item, structured answer snapshot, primary evidence, progress, crew, material, and issue summaries.
- Recurring construction occurrences are generated idempotently per plan/date; complete/skip actions require the configured evidence gate and preserve actor/audit history.
- Field checklist form flows remain separate from reconciliation and final acceptance workflows.
- Work item actions preserve actor, comments, status, and activity history.

## Cross-Domain Checks

- Pair with commercial/procurement when actual quantities, subcontractor checklist data, owner/client checklist data, or reconciliation variance changes.
- Pair with materials/warehouse when material usage, before/after snapshots, surplus, stock issue, stock return, or site balance changes.
- Pair with quality/acceptance/handover when evidence, checklist status, punch list, rework, or acceptance readiness changes.

## Golden Sample

- `GOLDEN-REAL-001` contains accepted evidence for every executed runtime step, three dated construction reports, subcontractor/client reconciliation, material before/after history and a resolved site issue.
