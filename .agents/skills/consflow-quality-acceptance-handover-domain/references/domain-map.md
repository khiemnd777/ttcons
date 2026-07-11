# Quality Acceptance Handover Domain Map

## Owners

- Frontend modules: `frontend/src/modules/quality`, `frontend/src/modules/acceptance`, `frontend/src/modules/rework`, `frontend/src/modules/handover`, `frontend/src/modules/work-items`
- Work item panels: `IncomingAcceptancePanel.tsx`, `OutgoingAcceptancePanel.tsx`, `EvidencePanel.tsx`, `WorkflowFormPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Quality workspace: `/projects/:id/quality`
- ITP: `/projects/:id/quality/itp`, `/quality/itp/:id/revise|submit|approve|activate`
- RFI: `/projects/:id/quality/rfi`, `/quality/rfi/:id/submit|schedule|inspect`
- NCR: `/projects/:id/quality/ncr`, `/quality/ncr/:id/root-cause|corrective-action|reinspection|close`
- Recurring execution: `/projects/:id/quality/recurring-plans`, `/quality/recurring-plans/:id/generate`, `/quality/recurring-occurrences/:id/complete|skip`
- Incoming acceptance: `/projects/:id/incoming-acceptances`, `/incoming-acceptances/:id`, `/incoming-acceptances/:id/submit`, `/work-items/:id/incoming-acceptances`
- Outgoing acceptance: `/projects/:id/acceptances`, `/acceptances/:id`, `/acceptances/:id/submit|pass|conditional-pass|fail|create-rework`, `/work-items/:id/outgoing-acceptance`
- Rework/replanning: `/projects/:id/reworks`, `/reworks/:id/approve|complete`, `/projects/:id/replannings`, `/replannings/:id/approve`
- Handover: `/projects/:id/handover`, `/projects/:id/handover/options`, `/handover/:id`, `/handover/:id/submit|check|complete`, `/handover-checklist-items/:id`

## Tables And Migrations

- `quality_itp_plans`, `quality_itp_items`, `quality_inspection_requests`, `quality_ncrs`
- `construction_recurring_plans`, `construction_recurring_occurrences`
- `incoming_acceptance_records`, `incoming_acceptance_items`
- `acceptance_records`, `acceptance_checklist_items`
- `rework_requests`, `replanning_requests`, `handover_records`
- Product-grade handover package: `handover_checklist_items`, `handover_records.readiness_snapshot_json`, exact final-acceptance/form source, submit/check/complete actor timestamps
- Related forms/artifacts: `workflow_form_instances`, `workflow_form_pdf_exports`, `workflow_form_signatures`, `evidence_records`
- Relevant migrations: `V1`, `V25`, `V26`, `V51`, `V52`, `V54`-`V56`

## Invariants

- ITP revisions preserve `definition_hash`; only one active revision exists per project/family and approved/active/superseded revisions are immutable.
- RFI inspection requires project-owned evidence. A rejected RFI creates a linked NCR and draft rework record atomically instead of losing the failed inspection history.
- NCR follows `open -> root_cause -> corrective_action -> reinspection -> closed`; closure requires an accepted/conditional reinspection RFI and completed linked rework.
- Quality recurring occurrences are idempotent per plan/date and evidence-gated when completed or skipped.
- Incoming acceptance records deliveries and quality decisions but does not automatically create warehouse movement.
- Outgoing acceptance preserves final result, signature/document gates, punch list/rework history, and handover compatibility.
- Rework completion does not erase acceptance history.
- Handover completion requires a submitted package, the exact linked final acceptance to be completed/pass or conditional-pass, all required checklist items verified from accepted evidence/active documents or system sources, no open issue/NCR/rework/material-surplus/finance blockers, and a fresh readiness snapshot. Completion rechecks readiness before writing `handed_over` and `completed_at`.

## Cross-Domain Checks

- Pair with construction operations when acceptance consumes work-item evidence, checklist results, site reports, or rework activity.
- Pair with materials/warehouse and procurement when incoming acceptance or handover readiness depends on deliveries, stock, site balance, or surplus.
- Pair with accounting/finance when handover blockers depend on unpaid AP/AR, invoices, advances, settlements, or reconciliation warnings.

## Golden Sample

- `GOLDEN-REAL-001` preserves ITP Rev.01/Rev.02, pass/fail/reinspection RFI history, a closed NCR linked to completed rework, recurring QA occurrences, final pass acceptance and a handed-over package with 8/8 verified checklist items.
