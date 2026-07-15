# Quality Acceptance Handover Domain Map

## Owners

- Frontend modules: `frontend/src/modules/quality`, `frontend/src/modules/acceptance`, `frontend/src/modules/rework`, `frontend/src/modules/handover`, `frontend/src/modules/work-items`
- Work item panels: `IncomingAcceptancePanel.tsx`, `OutgoingAcceptancePanel.tsx`, `EvidencePanel.tsx`, `WorkflowFormPanel.tsx`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Primary quality presentation: `/projects/:id/operations?tab=checks`; legacy `/projects/:id/quality` redirects to that tab while technical quality APIs remain available
- Simplified checks/actions: `/projects/:id/construction-workspace`, check-template/check creation, `PUT /construction-checks/:id`, check completion, action create, `PUT /construction-actions/:id`, resolution/verify/impact routes
- ITP: `/projects/:id/quality/itp-plans`, `/quality/itp-plans/:id/revise|submit|approve|activate`
- RFI: `/projects/:id/quality/rfis`, `/quality/rfis/:id/submit|schedule|inspect`
- NCR: `/projects/:id/quality/ncrs`, `/quality/ncrs/:id/root-cause|corrective|reinspection|close`
- Recurring execution: `/projects/:id/quality/recurring-plans`, `/quality/recurring-plans/:id/generate`, `/quality/recurring-occurrences/:id/complete|skip`
- Incoming acceptance: `/projects/:id/incoming-acceptances`, `/incoming-acceptances/:id`, `/incoming-acceptances/:id/submit`, `/work-items/:id/incoming-acceptances`
- Outgoing acceptance: `/projects/:id/acceptances`, `/acceptances/:id`, `/acceptances/:id/submit|pass|conditional-pass|fail|create-rework`, `/work-items/:id/outgoing-acceptance`
- Rework/replanning: `/projects/:id/reworks`, `/reworks/:id/approve|complete`, `/projects/:id/replannings`, `/replannings/:id/approve`
- Handover: `/projects/:id/handover`, `/projects/:id/handover/options`, `/handover/:id`, `/handover/:id/submit|check|complete`, `/handover-checklist-items/:id`

## Tables And Migrations

- `construction_check_templates`, `construction_checks`, `construction_check_evidence`, `construction_action_items`, `construction_action_events`
- Technical compatibility: `quality_itp_plans`, `quality_itp_items`, `quality_inspection_requests`, `quality_ncrs`, `issues`, `rework_requests`, `replanning_requests`
- `construction_recurring_plans`, `construction_recurring_occurrences`
- `incoming_acceptance_records`, `incoming_acceptance_items`
- `acceptance_records`, `acceptance_checklist_items`, ordered `acceptance_checklist_item_evidence`
- `rework_requests`, `replanning_requests`, `handover_records`
- Product-grade handover package: `handover_checklist_items`, `handover_records.readiness_snapshot_json`, exact final-acceptance/form source, submit/check/complete actor timestamps
- Related forms/artifacts: `workflow_form_instances`, `workflow_form_pdf_exports`, `workflow_form_signatures`, `evidence_records`
- Relevant migrations: `V1`, `V25`, `V26`, `V51`, `V52`, `V54`-`V56`, `V62`-`V64`, `V67`

## Invariants

- ITP revisions preserve `definition_hash`; only one active revision exists per project/family and approved/active/superseded revisions are immutable.
- RFI inspection requires project-owned evidence. A rejected RFI creates a linked NCR and draft rework record atomically instead of losing the failed inspection history.
- NCR follows `open -> root_cause -> corrective_action -> reinspection -> closed`; closure requires an accepted/conditional reinspection RFI and completed linked rework.
- Quality recurring occurrences are idempotent per plan/date and evidence-gated when completed or skipped.
- Incoming acceptance records deliveries and quality decisions but does not automatically create warehouse movement.
- A submitted incoming-acceptance item can prefill an explicit multi-line stock receipt. Linked receipt movements are capped by its remaining accepted quantity, preserve the acceptance item as source and do not rewrite acceptance history.
- Outgoing acceptance preserves final result, signature/document gates, punch list/rework history, and handover compatibility.
- `AcceptanceEditor.tsx` is the shared checklist editor/validation owner for the project acceptance workspace and work-item outgoing acceptance panel. Runtime checklist criteria are authored directly in the acceptance; `Công việc liên quan`, workflow forms and PDFs are references and do not auto-generate criteria. Create/edit uses a criterion list plus selected-criterion editor. V67 associates at most 10 ordered, non-rejected, same-project evidence records per criterion; when the acceptance has a related work item every evidence record must belong to it. The first association remains mirrored into legacy `acceptance_checklist_items.evidence_id`, and deletion of an association never deletes the evidence record. Existing project evidence may be linked without a related work item; new camera/file uploads remain work-item scoped and therefore require one. The page retains one state-driven CTA and one explicit conclusion dialog; it does not merge checklist rows with ITP criteria or auto-create rework after a failed acceptance.
- Field quality/remediation is reduced to `Kiểm tra` and `Cần xử lý`; ITP/RFI/NCR/issue/rework/replanning are hidden technical compatibility history, not additional user steps. Legacy project routes redirect to `tab=checks|actions` and original technical routes remain valid for audit/advanced consumers.
- A simplified check owns its criteria snapshot independently of its optional template. `construction_check_evidence` separates pre-inspection current-state `context` from completion `result`; both roles preserve evidence ownership/history, and only the first result is mirrored into the legacy `construction_checks.evidence_id` field. Association removal never deletes the underlying evidence record.
- Failed simplified checks create one action plus linked issue/rework and optional replanning atomically. Passing verification closes the technical issue/rework; failed verification reopens the action without deleting prior evidence/events.
- V65 may attach a simplified check or direct action to an operations daily report. The relationship is provenance only: it must not rewrite ITP/RFI/NCR/acceptance history, bypass the atomic failed-check action path, or replace `construction_action_events` as the khắc phục audit trail. Check-generated actions inherit report provenance through `source_check_id` rather than duplicating it.
- The inline report composer is presentation/orchestration only: it must call the same canonical check/action create routes with their existing permission, evidence, issue/rework/replanning and handover-blocker behavior. A compact form is not a new quality transition, and partial retry must never recreate a child that already succeeded.
- Pending RFI-backed checks and technical NCR/legacy issue action backfills are view-only on the field surface. Editable canonical actions preserve immutable source/evidence/impact/status and synchronize safe metadata to linked issue/rework rows transactionally, so technical audit history and handover blocker semantics do not drift.
- Rework completion does not erase acceptance history.
- Handover completion requires a submitted package, the exact linked final acceptance to be completed/pass or conditional-pass, all required checklist items verified from accepted evidence/active documents or system sources, no open simplified action/issue/NCR/rework/material-surplus/finance blockers, and a fresh readiness snapshot. Completion rechecks readiness before writing `handed_over` and `completed_at`.
- The project handover page is an inline readiness workspace for the selected/newest available package: it leads with the state-derived next action, groups checklist items, and maps snapshot blocker text to navigation links for Acceptance, Operations, Warehouse, Debt or Accounting. These links are presentation-only; P1 does not enforce a single active package or mutate blocker sources.

## Cross-Domain Checks

- Pair with construction operations when acceptance consumes work-item evidence, checklist results, site reports, or rework activity.
- Pair with materials/warehouse and procurement when incoming acceptance or handover readiness depends on deliveries, stock, site balance, or surplus.
- Pair with accounting/finance when handover blockers depend on unpaid AP/AR, invoices, advances, settlements, or reconciliation warnings.

## Golden Sample

- `GOLDEN-REAL-001` preserves technical ITP/RFI/NCR/rework history behind one optional active template, four checks, four role-based evidence associations and one resolved action; the remaining pending check has context evidence and is not a handover blocker, while open actions are. Final acceptance and the handed-over package remain complete with 8/8 verified checklist items.
