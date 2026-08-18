# Quality Acceptance Handover Domain Map

## Owners

- Frontend modules: `frontend/src/modules/quality`, `frontend/src/modules/acceptance`, `frontend/src/modules/rework`, `frontend/src/modules/handover` (`HandoverPage.tsx`, `HandoverCreateDialog.tsx`, `HandoverWorkspace.tsx`, `handoverTypes.ts`), `frontend/src/modules/work-items`
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
- Related forms/artifacts: `workflow_form_instances`, `workflow_form_pdf_exports`, `workflow_form_pdf_export_deletions`, `workflow_form_signatures`, `evidence_records`
- Relevant migrations: `V1`, `V25`, `V26`, `V51`, `V52`, `V54`-`V56`, `V62`-`V64`, `V67`, `V93`; official Rev4 responsibility/type snapshots: `V86`, `V87`

## Invariants

- Acceptance and handover pages remain the canonical quality/handover ledgers. A work-item `execution_target` may open either workspace with presentation-only `?task=` context; the shared task banner/dialog supplies workflow gates without cloning acceptance criteria, evidence, result, rework or handover records and without weakening their source-domain transitions.
- ITP revisions preserve `definition_hash`; only one active revision exists per project/family and approved/active/superseded revisions are immutable.
- RFI inspection requires project-owned evidence. A rejected RFI creates a linked NCR and draft rework record atomically instead of losing the failed inspection history.
- NCR follows `open -> root_cause -> corrective_action -> reinspection -> closed`; closure requires an accepted/conditional reinspection RFI and completed linked rework.
- Quality recurring occurrences are idempotent per plan/date and evidence-gated when completed or skipped.
- Incoming acceptance records deliveries and quality decisions but does not automatically create warehouse movement.
- A submitted incoming-acceptance item can prefill an explicit multi-line stock receipt. Linked receipt movements are capped by its remaining accepted quantity, preserve the acceptance item as source and do not rewrite acceptance history.
- Outgoing acceptance preserves final result, signature/document gates, punch list/rework history, and handover compatibility.
- Any outgoing acceptance reference to `workflow_form_pdf_exports.id` blocks that PDF's deletion regardless of acceptance status. V93 enforces both directions with deferred constraint triggers so a deletion/acceptance race cannot commit an acceptance pointing at an object being removed; the immutable export row remains available to historical acceptance joins.
- `AcceptanceEditor.tsx` is the shared checklist editor/validation owner for the project acceptance workspace and work-item outgoing acceptance panel. Runtime checklist criteria are authored directly in the acceptance; `Công việc liên quan`, workflow forms and PDFs are references and do not auto-generate criteria. Create/edit uses a criterion list plus selected-criterion editor. V67 associates at most 10 ordered, non-rejected, same-project evidence records per criterion; when the acceptance has a related work item every evidence record must belong to it. The first association remains mirrored into legacy `acceptance_checklist_items.evidence_id`, and deletion of an association never deletes the evidence record. Existing project evidence may be linked without a related work item; new camera/file uploads remain work-item scoped and therefore require one. The page retains one state-driven CTA and one explicit conclusion dialog; it does not merge checklist rows with ITP criteria or auto-create rework after a failed acceptance.
- Field quality/remediation is reduced to `Kiểm tra` and `Cần xử lý`; ITP/RFI/NCR/issue/rework/replanning are hidden technical compatibility history, not additional user steps. Legacy project routes redirect to `tab=checks|actions` and original technical routes remain valid for audit/advanced consumers.
- A simplified check owns its criteria snapshot independently of its optional template. `construction_check_evidence` separates pre-inspection current-state `context` from completion `result`; both roles preserve evidence ownership/history, and only the first result is mirrored into the legacy `construction_checks.evidence_id` field. Association removal never deletes the underlying evidence record.
- Failed simplified checks create one action plus linked issue/rework and optional replanning atomically. Passing verification closes the technical issue/rework; failed verification reopens the action without deleting prior evidence/events.
- V65 may attach a simplified check or direct action to an operations daily report. The relationship is provenance only: it must not rewrite ITP/RFI/NCR/acceptance history, bypass the atomic failed-check action path, or replace `construction_action_events` as the khắc phục audit trail. Check-generated actions inherit report provenance through `source_check_id` rather than duplicating it.
- The inline report composer is presentation/orchestration only: it must call the same canonical check/action create routes with their existing permission, evidence, issue/rework/replanning and handover-blocker behavior. A compact form is not a new quality transition, and partial retry must never recreate a child that already succeeded.
- Pending RFI-backed checks and technical NCR/legacy issue action backfills are view-only on the field surface. Editable canonical actions preserve immutable source/evidence/impact/status and synchronize safe metadata to linked issue/rework rows transactionally, so technical audit history and handover blocker semantics do not drift.
- Rework completion does not erase acceptance history.
- Issue updates whose request body contains `status`, rework `approved`/`done`, and replanning approval persist their audit event in the same transaction and fail closed. An invalid audit actor/target or audit insert failure rolls back the status transition. Issue updates without `status` persist metadata edits and attempt the same single `issue_updated` event best-effort; issue/rework/replanning draft creation also remains best-effort, and every audit failure is logged with structured fields.
- Handover completion requires a submitted package, the exact linked final acceptance to be completed/pass or conditional-pass, all required checklist items verified from accepted evidence/active documents or system sources, no open simplified action/issue/NCR/rework/material-surplus blockers, and a fresh readiness snapshot. For Rev7 and older runtimes, unconfirmed project material costs and pending cash requests remain handover blockers. Only an active official-family revision >=8 runtime separates them: physical handover can complete first, records `finance_separated=true` in the readiness snapshot, and the Rev8 `closure.finance` source gate then blocks final workflow closure until finance is terminal.
- Handover creation previews and locks the exact selected final-acceptance source, prefills available representatives and does not let the client attest checklist/readiness values. The user-facing entity is `Hồ sơ bàn giao`; the project page presents history filters and the hồ sơ list before an inline workspace for the selected/newest record with `Việc cần làm`, `Checklist` and `Thông tin hồ sơ`. `Xem chi tiết` selects and scrolls/focuses this workspace even when the record is already active. It leads with one state-derived next action and maps snapshot blocker text to navigation links for Acceptance, Operations, Warehouse or Tài chính công trình. These links are presentation-only; the UI does not enforce a single active package or mutate blocker sources.
- Readiness-owned checklist sources (`final_acceptance`, `warehouse_closure`, `ap_reconciliation`, `ar_reconciliation`) expose a read-only `Xem nguồn` dialog. It must show the exact backend criterion, result/count/amount/time from the last stored readiness snapshot, state that source data may have changed since that check, and offer only permission-allowed navigation to the source module. Evidence/document-linked checklist rows remain manual verification sources and must never be relabelled as automatic. They also expose read-only `Xem nguồn` from the exact item-linked record, including locked handed-over hồ sơ; internally stored `local|s3` evidence continues through the protected evidence-file route, which authorizes before provider-registry `Open` and streams without exposing an object key or filesystem path. `external`, `none` and unknown providers are not treated as protected evidence. Document URLs are opened only when present and safe, placeholder/missing targets are explicit, and `Cập nhật liên kết` is limited to mutable hồ sơ plus handover verification permission.
- Handover checklist presentation maps automatic finance rows to `Chi phí vật tư công trình` and `Đề nghị thu/chi nội bộ`. New item codes are `project_material_costs` and `project_cash_requests`; stored legacy `ap_reconciliation`/`ar_reconciliation` codes keep compatibility but receive the new business labels and route to Tài chính công trình. Raw codes remain tooltip-only and unknown future codes fall back to `Nội dung khác`.

## Cross-Domain Checks

- Pair with construction operations when acceptance consumes work-item evidence, checklist results, site reports, or rework activity.
- Pair with materials/warehouse and procurement when incoming acceptance or handover readiness depends on deliveries, stock, site balance, or surplus.
- Pair with accounting/finance when handover blockers depend on unconfirmed project material costs or internal cash requests that have not reached `processed`; legacy AP/AR is no longer an ordinary closure gate.

## Golden Sample

- `GOLDEN-REAL-001` preserves technical ITP/RFI/NCR/rework history behind one optional active template, four checks, four role-based evidence associations and one resolved action; the remaining pending check has context evidence and is not a handover blocker, while open actions are. Final acceptance and the handed-over package remain complete with 8/8 verified checklist items.

## Rev3 QA/QC Boundary

- `QA/QC` and `Giám sát` are distinct Rev3 responsibility labels. QA/QC owns preparation/coordination of the quality dossier with the client and acceptance history; Supervisor owns field checklist/photo reporting. Neither label grants the other's backend permission or replaces work-item role/project-scope checks.
- Rev4 `...023` encodes the dossier side as `planning.client_quality_file`: only `QA/QC` is assigned, both checklist and document evidence are required, and V87 marks it `document_upload` rather than the misleading `daily_report`. Giám sát remains on the separate construction item-report step; Rev4 does not rewrite historical runs or acceptance evidence.
- Paper documents sent because a client does not use the App remain contract dispatch/scan records. They do not replace QA/QC acceptance evidence, immutable workflow form history, or handover readiness sources.
- Rev7 `outbound_contract.returned_scan` is form-less: Contract Dispatches owns the returned signed scan and `contract.require_execution_ready` verifies App signature, received scan and active contract. Incoming/owner/subcontractor acceptance actions similarly check their canonical submitted reconciliation/acceptance records before completion.
- Rev8 combines subcontractor checklist plus quantity dossier into one immutable work-package form and combines owner reconciliation plus final acceptance into one composite source checklist. Canonical reconciliation, acceptance result, evidence and rework tables remain authoritative and are not replaced by the combined form.
- Rework budget impact is monetary and uses semantic `currency`/shared `CurrencyInput`; timeline impact, acceptance quantities and checklist counts remain ordinary numeric fields.
