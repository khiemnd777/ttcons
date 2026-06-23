# Workflow Orchestration Domain Map

## Owners

- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Frontend routes: `frontend/src/app/router.tsx`
- Frontend modules: `frontend/src/modules/pipeline-builder`, `frontend/src/modules/planning`, `frontend/src/modules/workflow-forms`, `frontend/src/modules/work-items`, `frontend/src/modules/my-work`, `frontend/src/modules/projects`
- Shared frontend contracts: `frontend/src/lib/api/client.ts`, `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`
- Migrations: `V1`, `V2`, `V3`, `V4`, `V5`, `V6`, `V7`, `V8`, `V9`, `V10`, `V14`, `V15`, `V16`

## Routes

- Template builder: `/pipeline-templates`, `/pipeline-nodes`, `/pipeline-steps`, `/pipeline-edges`, `/pipeline-edge-conditions`
- Runtime: `/projects/:id/start-pipeline`, `/projects/:id/pipeline-run`, `/pipeline-run-nodes/:id/*`, `/pipeline-run-steps/:id/submit`
- Work items: `/work-items/:id`, `/work-items/:id/start|complete|block|resolve-blocker|request-review|request-approval|approve|reject|request-change`
- Activity/evidence: `/work-items/:id/activity`, `/work-items/:id/evidence`, `/evidence/:id/file|accept|reject`
- Workflow forms: `/workflow-form-templates`, `/workflow-form-template-library`, `/workflow-form-versions/:id/publish|archive`, `/workflow-form-instances/:id/draft|submit|sign|pdf-exports`
- Artifacts: `/workflow-form-signatures/:id/visual`, `/workflow-form-pdf-exports/:id/file`

## Tables

- `pipeline_templates`, `pipeline_template_nodes`, `pipeline_template_edges`, `pipeline_template_steps`
- `pipeline_runs`, `pipeline_run_nodes`, `pipeline_run_edges`, `pipeline_run_steps`
- `work_items`, `work_item_assignments`, `work_item_activities`, `activity_mentions`, `activity_read_states`
- `approval_rules`, `approval_requests`, `approval_actions`
- `workflow_form_templates`, `workflow_form_versions`, `workflow_form_fields`, `workflow_form_instances`, `workflow_form_answers`, `workflow_form_events`, `workflow_form_signatures`, `workflow_form_pdf_exports`
- `evidence_records`, `documents`, `audit_logs`

## Invariants

- Template edits do not mutate existing run copies.
- Published workflow form versions are immutable runtime references.
- `required_role_ids` remains an array everywhere.
- Work item completion respects required forms, evidence, signatures, approvals, and blockers.
- Signatures and PDFs preserve revision, actor, hash, checksum, and protected file access.

## Cross-Domain Checks

- Pair with the source business domain when workflow forms create or summarize planning, construction, commercial, material, acceptance, or finance records.
- Check template authoring, runtime copies, work-item gates, activity, and frontend status labels together when changing workflow transitions.
- Do not let convenience UI bypass approval, required-form, evidence, signature, PDF, material, receivable, or blocker gates.
