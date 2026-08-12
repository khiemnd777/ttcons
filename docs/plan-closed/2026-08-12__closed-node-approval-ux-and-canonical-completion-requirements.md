# [Closed] Node Approval UX And Canonical Completion Requirements

## Will be done

- Expose actor-scoped node-approval capabilities from the backend and keep every approve/reject/request-change action authoritative, project-scoped and audit-traceable.
- Reuse one shared approval decision component on Dashboard, Project Workflow and the `Chờ tôi duyệt` workbench lane without converting node approvals into synthetic work items.
- Make work-item detail render backend-computed completion requirements so workflow forms, checklist payloads, documents and photos cannot produce contradictory readiness messages.
- Preserve published Rev5/V92 and every existing runtime snapshot; this plan requires no schema migration or runtime backfill.
- Add focused backend/frontend regressions, update central/detail/domain maps, then run formatting, tests and the production frontend build. Browser UAT is not authorized in this implementation turn.

## Done

- Confirmed the existing node-approval action endpoints, structured policy enforcement, requester separation, project scope and audit persistence are present and technically reusable.
- Confirmed `/approvals/pending` returns raw visible requests without actor capabilities, Dashboard only navigates, Project Workflow only requests/advances/blocks, the shared `ApprovalPanel` is unused, and `/my-work` currently models only work-item decisions.
- Confirmed `WorkItemDetailDialog` incorrectly treats every `checklist_required` flag as missing uploaded checklist evidence, while the backend completion gate treats the exact submitted workflow form as satisfying that gate for current engines; `WorkflowFormPanel` simultaneously renders `Checklist: Không áp dụng` when the form has no `checklist_items` field.
- Produced the cross-layer impact matrix, review package and Mockup as Text; received explicit approval for all six implementation and validation points.
- Added actor/project/current-stage pending-approval contracts, preserved mutation-time authorization/audit, and reused one decision dialog on Dashboard, Project Workflow and a distinct node-approval lane under `Chờ tôi duyệt`.
- Added backend-computed `completion_requirements` as the shared detail/completion-gate source, removed the frontend aggregate-evidence checklist inference, and hid the workflow-form checklist pill when the schema has no checklist field.
- Added backend PostgreSQL regressions plus frontend presentation tests and updated central, backend, frontend, triage and workflow-domain maps. No schema migration or runtime/template rewrite was required.
- Technically verified the two focused PostgreSQL regressions, the non-DB core suite, all 62 frontend tests and the current production frontend build. The full PostgreSQL backend suite reaches an unrelated pre-existing old-template (`...020`) assertion failure because edge `...0007` is `always` in the shared seeded database while that historical test expects `procurement_approved`; the approved scope explicitly excludes old IDs/templates/engines. Browser/UX acceptance remains pending because Browser UAT was not authorized in this turn.

## Closure

- Closed on 2026-08-12 after the user requested `đóng plan, commit, push`.
- Delivery status at closure: `IMPLEMENTED`, `TECHNICALLY VERIFIED`, `NOT UX ACCEPTED`.
