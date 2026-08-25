# [Closed] Chuẩn hóa tự động bắt đầu đầu việc trên mọi module nghiệp vụ

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-25, closed by user on 2026-08-25)

## Will be done

- Add one explicit `work_item_context` / `start_on_success` contract for meaningful business mutations opened from an actionable Work Item; generic module mutations without that context remain unchanged.
- Validate project, actor permission, current runtime Node, execution workspace/action and Work Item status in the backend; atomically transition `pending|ready|needs_changes -> in_progress`, preserve `in_progress|blocked`, and fail closed for future, mismatched or terminal work.
- Apply the contract across workflow forms, planning/handoff, BOQ-commercial-procurement-contract, warehouse/material, construction operations, acceptance/reconciliation, handover and project finance without changing ledger balances, approvals or completion gates.
- Standardize the project task banner, module CTA, truthful transition toast and actor-owned cache invalidation; retain manual `Bắt đầu` only for generic/checklist work without a mapped business mutation.
- Add focused backend/frontend regression coverage, update every affected central/detail/domain map and validate the current dirty working tree without reverting overlapping user changes. No schema migration or historical backfill is planned; Browser/UAT remains skipped unless separately authorized.

## Done

- User approved the cross-module impact analysis, fail-closed status matrix, explicit task-context boundary, fallback manual-start rule, no migration/backfill decision and responsive Mockup as Text with `OK triển khai theo review package này`.
- Confirmed the current integration split: workflow-form first write and direct construction daily-report create already start linked Work Items transactionally; acceptance and several work-item-scoped warehouse/reconciliation records carry explicit Work Item IDs; generic BOQ, commercial, contract, handover and project-finance mutations do not share one task-context contract.
- Loaded the mandatory central/backend/frontend/root-cause mappings plus Workflow Orchestration, Project Planning, Commercial Procurement, Materials Warehouse, Construction Operations, Quality/Acceptance/Handover, Accounting/Finance and backend/frontend/impact-analysis guardrails.
- Added one shared backend `WorkItemMutationContext` and module policy registry. Explicit canonical module mutations validate the same project, actor action scope, required permission, action/workspace, start schedule and exact current Pipeline Node; source mutation plus `pending|ready|needs_changes -> in_progress`, Run Step sync, activity and audit commit or roll back together. `in_progress|blocked` remain unchanged and terminal/future/mismatched contexts fail closed.
- Applied the transaction bridge to BOQ/procurement generation, supplier quotations/selections, procurement transitions, contract save/version/sign/dispatch/scan/activation, posted stock documents, Planning construction handoff, work-item-owned material/acceptance/reconciliation/construction check/action records, physical handover and project finance. Existing Workflow Form and daily-report first-write paths remain intact; no schema migration or historical backfill was added.
- Added the shared frontend `workItemMutationContext.ts` bridge to every canonical module workspace. The task query is presentation context only; actual mutations send nested `work_item_context`, success copy is emitted only from backend `work_item_transition.auto_started`, and Work Item/Operations/My Work/workflow-action queries are invalidated without a second `/start` request.
- Updated the project task banner to explain the automatic transition and removed the separate generic `Bắt đầu` CTA for domain/form-owned execution while retaining it for unmapped generic Work Items. Direct acceptance/material/reconciliation and construction panels use the same truthful response/toast path.
- Added backend unit coverage for explicit intent and exact module policy matching plus frontend payload/notice/consumer coverage. Updated central backend/frontend/root-cause maps and all affected Workflow, Planning, Commercial, Warehouse, Construction, Acceptance/Handover and Project Finance domain maps.
- Current dirty working-tree verification at HEAD `7a37ebf` passed: full backend `go test ./...`; 269/269 frontend tests; TypeScript/Vite production build; 250 KiB gzip bundle budget with 142 chunks; and `git diff --check`.

## Skipped / remaining acceptance

- Browser/UAT was not authorized for this implementation and remains `NOT UX ACCEPTED`.

## Closure

- The user requested `Đóng plan, commit, push` on 2026-08-25.
- Durable lifecycle, routing and domain knowledge has already been promoted into the central/detail/domain maps; this file is historical only.
