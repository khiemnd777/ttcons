# Quality Acceptance Handover Domain Map

## Owners

- Frontend modules: `frontend/src/modules/acceptance`, `frontend/src/modules/rework`, `frontend/src/modules/handover`, `frontend/src/modules/work-items`
- Work item panels: `IncomingAcceptancePanel.tsx`, `OutgoingAcceptancePanel.tsx`, `EvidencePanel.tsx`, `WorkflowFormPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Incoming acceptance: `/projects/:id/incoming-acceptances`, `/incoming-acceptances/:id`, `/incoming-acceptances/:id/submit`, `/work-items/:id/incoming-acceptances`
- Outgoing acceptance: `/projects/:id/acceptances`, `/acceptances/:id`, `/acceptances/:id/submit|pass|conditional-pass|fail|create-rework`, `/work-items/:id/outgoing-acceptance`
- Rework/replanning: `/projects/:id/reworks`, `/reworks/:id/approve|complete`, `/projects/:id/replannings`, `/replannings/:id/approve`
- Handover: `/projects/:id/handover`, `/handover/:id/check|complete`

## Tables And Migrations

- `incoming_acceptance_records`, `incoming_acceptance_items`
- `acceptance_records`, `acceptance_checklist_items`
- `rework_requests`, `replanning_requests`, `handover_records`
- Related forms/artifacts: `workflow_form_instances`, `workflow_form_pdf_exports`, `workflow_form_signatures`, `evidence_records`
- Relevant migrations: `V1`, `V25`, `V26`

## Invariants

- Incoming acceptance records deliveries and quality decisions but does not automatically create warehouse movement.
- Outgoing acceptance preserves final result, signature/document gates, punch list/rework history, and handover compatibility.
- Rework completion does not erase acceptance history.
- Handover completion is blocked until quality, material, procurement, and finance readiness are satisfied.

## Cross-Domain Checks

- Pair with construction operations when acceptance consumes work-item evidence, checklist results, site reports, or rework activity.
- Pair with materials/warehouse and procurement when incoming acceptance or handover readiness depends on deliveries, stock, site balance, or surplus.
- Pair with accounting/finance when handover blockers depend on unpaid AP/AR, invoices, advances, settlements, or reconciliation warnings.
