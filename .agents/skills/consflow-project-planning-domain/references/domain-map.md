# Project Planning Domain Map

## Owners

- Frontend modules: `frontend/src/modules/projects`, `frontend/src/modules/planning`, `frontend/src/modules/planning-daily-reports`, `frontend/src/modules/meeting-minutes`, `frontend/src/modules/planning-checklists`
- Shared work item UI: `frontend/src/modules/work-items`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Projects: `/projects`, `/projects/:id`, `/projects/:id/operations`
- Planning: `/projects/:id/start-pipeline`, `/projects/:id/pipeline-run`, `/projects/:id/work-items`
- Planning reports: `/planning/daily-reports`, `/planning/meeting-minutes`, `/planning/checklists`
- Activity: `/projects/:id/activity`, `/projects/:id/activity/stream`, `/me/activity-attention`
- Contract workspace: `/projects/:id/contracts`; activating the signed main-client contract updates the project-level `contract_amount` compatibility snapshot.

## Tables And Migrations

- `projects`, `project_members`, `audit_logs`
- Commercial project master: `contracts`, signed `contract_versions`; `projects.contract_amount` is not the version history.
- Runtime planning: `pipeline_runs`, `pipeline_run_nodes`, `pipeline_run_steps`, `work_items`
- Form-backed planning: `workflow_form_instances`, `workflow_form_answers`, `workflow_form_events`
- Activity: `work_item_activities`, `activity_mentions`, `activity_read_states`
- Relevant migrations: `V1`, `V5`, `V6`, `V7`, `V8`, `V9`, `V10`, `V11`, `V12`, `V13`

## Invariants

- Planning reports are project-scoped and permission-filtered.
- Meeting minutes action items and daily report next-plan outputs remain traceable.
- Planning checklist evidence/status summaries do not replace the underlying submitted form history.
- Project detail and global workspaces must agree on statuses and available actions.

## Cross-Domain Checks

- Pair with workflow orchestration when planning records are backed by workflow form instances, work items, signatures, or PDF exports.
- Pair with construction operations when planning outputs become field execution work or project activity.
- Pair with reporting when changing planning metrics, filters, date scopes, owner scopes, or CSV/reportable fields.
