# Active Plan: Production-Grade Workflow Roadmap Agent Checklist

## Objective

Maintain a durable, interruption-safe checklist for implementing the Google Sheet roadmap into Consflow without mismatch, forgotten domain invariants, or hallucinated scope. This plan is the source of truth for follow-up agents when continuing long-running Production-Grade implementation work.

## Latest User Context

- Source roadmap: Google Sheet `Lộ trình Triển khai Tính năng Workflow`, tab `Roadmap`, spreadsheet id `1Hm-hU3bBFLmN_7x7Oh3ue-W4Zbgml4Zm-XltIrQ0Kzo`.
- Ignore roadmap columns: `Thời gian dự kiến`, `Phụ thuộc`, `Trạng thái`, `Ưu tiên`.
- Main need: analyze the roadmap against the current repo and create an agent-facing checklist/memory for long implementation.
- Added core invariant from user: Pipeline Builder step `Role phụ trách` must support multiple roles, not a single role.

## Current Repo Baseline

- Backend: Go/Fiber API, PostgreSQL, Redis, custom SQL migrations.
- Frontend: React/Vite admin UI with Pipeline Builder, project detail, planning, BOQ, procurement, warehouse, operations, acceptance, rework, handover, accounting, debt, staff/RBAC, audit, reports, settings.
- Route owner: `backend/internal/core/routes.go`.
- Backend domain owners: `backend/internal/core/handlers.go`, `backend/internal/core/service.go`, `backend/internal/core/repository.go`, `backend/internal/core/repository_domain.go`.
- Migration owner: `backend/migrations`, custom runner in `backend/internal/shared/migrations`.
- Frontend route owner: `frontend/src/app/router.tsx`.
- Frontend feature owners: `frontend/src/modules/*`.
- Shared frontend owners: `frontend/src/components`, `frontend/src/modules/_shared`, `frontend/src/lib`.
- Existing form helper: `frontend/src/modules/_shared/SimpleForm.tsx`.
- Existing toast helper: `frontend/src/components/ToastProvider.tsx`.
- Existing table helper: `frontend/src/components/DataTable.tsx`.

## Non-Negotiable Domain Invariants

1. Pipeline templates are reusable definitions; pipeline runs are project-specific runtime copies.
2. Publishing, editing, or cloning templates must not mutate historical project runs unexpectedly.
3. Pipeline Builder step `required_role_ids` is a multi-role array. UI must never collapse it to one `responsible_role_id`.
4. Approval actions must preserve actor, status, comments, and timestamps.
5. BOQ approval can generate procurement; procurement must not be generated from unapproved BOQ lines.
6. Procurement receive and warehouse movement must preserve stock balance and traceability.
7. Acceptance failure can create punch list/rework; rework completion must not erase acceptance history.
8. Handover completion must respect readiness blockers.
9. Accounting, payable, receivable, payment, invoice, and advance states must reconcile by project and source document.
10. User-visible admin lists must hide deleted/cancelled/inactive records at API/query level, not only in frontend.
11. Protected endpoints must remain under `middleware.AuthRequired`.
12. Database changes must be new custom SQL migrations. Never edit an already-applied migration and never use Flyway.

## Roadmap Fit Assessment Memory

| Roadmap item | Current fit | Required direction |
| --- | --- | --- |
| Nền tảng | Mostly present | Harden env, CI, observability, backup/restore, deployment checklist. |
| Dữ liệu lõi | Partial | Add partner/vendor/client/catalog models where needed. |
| Auth / RBAC | Partial | Add permission-aware UI, stronger session/token lifecycle, coverage audit. |
| Lõi module nghiệp vụ | Partial | Refactor generic CRUD into clearer handler-service-repository use cases. |
| Admin UX | Partial | Improve dense operational UX, errors, loading, empty states, filters. |
| Shared UI | Partial | Upgrade table/form/dialog patterns before duplicating UI. |
| Business UI | Partial | Add workflow-form, checklist, evidence, signature, PDF/document components. |
| Workflow thành nghiệp vụ | Partial | Map workflow step types into typed business form instances. |
| Giao diện hoá nghiệp vụ kế hoạch | Partial | Build business-facing planning forms, not only raw pipeline controls. |
| Hệ quy đổi Workflow thành nghiệp vụ | Partial | Introduce form template/version/instance layer. |
| Daily report kế hoạch | Partial | Add did/will fields, owner, date/user/project filters. |
| Biên bản họp | Missing | New meeting minutes module/form with action items and project/workflow links. |
| Checklist kế hoạch | Partial | Typed checklist template/items/assignee/evidence. |
| Chữ ký ảo | Missing | Internal signature/stamp model with actor/time/immutability. |
| Hình ảnh | Partial | Real upload/storage/gallery/metadata/permission handling. |
| Đơn giá | Partial | Unit-price lifecycle: expected, norm locked, final locked. |
| Checklist thầu phụ | Missing | Dedicated subcontractor checklist form and evidence flow. |
| Checklist chủ đầu tư | Missing | Dedicated owner/client checklist and signoff flow. |
| Daily report thi công | Partial | Add did/will, crew, actual quantity, site photo evidence. |
| Đối soát checklist thầu phụ | Missing | Reconcile subcontractor checklist with BOQ, actual quantity, internal confirmation. |
| Đối soát checklist chủ đầu tư | Missing | Reconcile owner checklist with internal acceptance and project outputs. |
| Vật tư trước/sau | Partial | Add before/after material snapshots by work item or construction item. |
| Lưu kho vật tư tồn | Partial | Add custody location, condition, photo, storage/return record. |
| Nghiệm thu đầu vào | Partial | Separate supplier/subcontractor incoming acceptance from generic acceptance. |
| Nghiệm thu đầu ra | Partial | Owner/client outgoing acceptance with checklist, signature, document output. |
| PDF hoá đơn đầu vào | Missing | Import/export PDF invoice linked to payable/procurement/document. |
| PDF hoá đơn đầu ra | Missing | Import/export PDF invoice linked to receivable/payment/document. |
| Thanh toán kỳ/ứng trước đầu vào | Partial | Payment schedule, advance request, partial pay, settlement. |
| Thanh toán kỳ/ứng trước đầu ra | Partial | Billing milestone, advance receipt, partial receive, invoice issue. |
| Công nợ đầu vào | Partial | Payable aging, vendor ledger, invoice/payment reconciliation. |
| Công nợ đầu ra | Partial | Receivable aging, client ledger, invoice/receipt reconciliation. |
| Dòng tiền thu/chi | Partial | Time-series cashflow by project/date/category with export. |
| Báo cáo thuế | Missing | Tax/VAT ledger by invoice period and source document. |
| Báo cáo lợi nhuận | Missing | Project P&L with revenue, direct cost, indirect cost, gross/net margin. |

## Production-Grade Implementation Checklist

### 0. Agent Start Checklist

- Read `AGENTS.md`.
- Read this active plan in `docs/current-states.md`.
- Read `.agents/skills/consflow-module-memory/references/module-keyword-map.md`.
- Identify affected owner paths before editing.
- If changing code or behavior, present review package and wait for explicit user confirmation.
- If changing UI, present Mockup as Text and wait for explicit user confirmation.
- Record new progress in this plan under `Done` before ending a long turn.

### 1. Foundation Hardening

- Add CI pipeline for backend tests and frontend build.
- Add migration dry-run/checksum verification workflow.
- Add structured logging and request ids.
- Add deployment and rollback checklist.
- Add backup/restore procedure for PostgreSQL.
- Add seed/demo data policy separate from production data.

### 2. RBAC And Actor Safety

- Audit every protected route for global or project-scoped permission checks.
- Make frontend navigation and row actions permission-aware.
- Add tests for forbidden cross-project access.
- Add session/token refresh/revoke behavior beyond demo JWT basics.
- Ensure actor id/name is recorded in all domain transitions.

### 3. Shared UI Refactor

- Upgrade `DataTable` to support sorting, filtering, pagination, loading, empty, error, and action columns consistently.
- Upgrade form layer to support schema-driven fields, validation, conditional fields, file fields, and multi-select fields.
- Keep toast behavior dismissible with warning summaries for invalid fields.
- Reuse dialog/modal patterns for create/view/edit flows.
- Do not create one-off controls when a shared pattern exists.

### 4. Workflow Form Core

- Add tables for form templates, form versions, form instances, form fields/answers, and form events.
- Map pipeline template steps to form template definitions.
- When a pipeline run starts, preserve runtime copies or stable references safely.
- Add typed step outputs: checklist, meeting minutes, daily report, signature, photo evidence, invoice, acceptance.
- Ensure `required_role_ids` remains multi-role at template, run, work item, and UI layers.

### 5. Planning Phase Features

- Implement meeting minutes form: attendees, agenda, decisions, action items, due dates, assignees.
- Implement planning daily report with `đã làm` and `sẽ làm`.
- Implement planning checklist with assignee, status, evidence, comments.
- Implement internal signature for planning reports.
- Implement image evidence upload and gallery.
- Implement unit-price lifecycle: expected, norm locked, final locked.

### 6. Construction Phase Features

- Implement subcontractor checklist form.
- Implement owner/client checklist form.
- Implement construction daily report with did/will, crew, actual quantity, materials used, photos.
- Implement subcontractor checklist reconciliation against BOQ/actual/internal confirmation.
- Implement owner checklist reconciliation against acceptance/internal output.
- Implement material before/after reports by work item or construction item.
- Implement surplus material storage workflow with location, condition, photo, and movement linkage.

### 7. Acceptance And Quality

- Split incoming acceptance and outgoing acceptance where the workflow requires different actors/documents.
- Add acceptance checklist item editing and evidence attachment UI.
- Add fail -> punch list -> rework -> re-acceptance loop with preserved history.
- Add signature and PDF export for acceptance records.
- Add tests for acceptance fail and rework history.

### 8. Documents, PDF, And Storage

- Add file storage adapter and upload endpoint.
- Add document metadata: project, target type/id, file type, visibility, checksum, uploader, created time.
- Add PDF invoice import for input and output invoices.
- Add PDF export for meeting minutes, reports, acceptance, handover, invoices.
- Enforce document visibility and project scope in backend.

### 9. Procurement And Warehouse

- Add procurement request item UI, not only request header UI.
- Ensure procurement receive can create/trigger traceable stock movement.
- Add warehouse movement history and project/material ledger.
- Add stock availability tests for issue/return/transfer.
- Add surplus handling workflow from site balance to warehouse/subcontractor return.

### 10. Accounting And Debt

- Add invoice tables/forms linked to payables and receivables.
- Add payment schedule for input/output contracts.
- Add advance request and settlement UI.
- Support partial payment/receipt, not only pay/receive all remaining amount.
- Add payable and receivable aging reports.
- Add reconciliation checks among invoices, payments, payables, receivables, and source workflow items.

### 11. Reporting

- Add project cashflow time-series report.
- Add tax/VAT report by period and document source.
- Add project P&L report with revenue, direct cost, indirect cost, profit, margin.
- Add export CSV/PDF for operational and accounting reports.
- Add report access-control tests.

### 12. Test Coverage

- Backend: route permission tests for major modules.
- Backend: migration tests for new schema.
- Backend: transaction tests for BOQ -> procurement -> warehouse.
- Backend: transaction tests for acceptance -> rework -> re-acceptance.
- Backend: accounting tests for payable/receivable/payment/invoice reconciliation.
- Frontend: component tests for shared form/table/multi-select.
- Frontend: build must pass after each UI batch.
- Browser checks are skipped by default unless user explicitly requests browser verification.

## Double-Check Gates Before Any Feature Batch

1. Is this a template definition change, runtime pipeline change, or both?
2. Does it affect `required_role_ids` or actor assignment? If yes, preserve multi-role arrays.
3. Does it require a new migration? If yes, add a new `V<N>__description.sql`.
4. Does it need new backend permission keys or project-scoped checks?
5. Does frontend use shared API client and shared UI patterns?
6. Does save/edit show success, warning, and error toasts?
7. Does every list exclude deleted/cancelled/inactive records at API level?
8. Does every state transition preserve actor/time/comment/audit where needed?
9. Does the change require report/dashboard contract updates?
10. Are backend and frontend status labels/enums aligned?

## Suggested Implementation Order

1. Fix Pipeline Builder step multi-role UI because it is a core workflow definition invariant.
2. Refactor shared form/table primitives enough to support long roadmap features.
3. Add workflow form core schema and typed form instance model.
4. Implement planning forms: meeting minutes, did/will report, checklist, signature, image evidence.
5. Implement construction forms: subcontractor/client checklist, daily report, material before/after.
6. Implement document/PDF infrastructure.
7. Implement acceptance incoming/outgoing and re-acceptance loop.
8. Implement invoice/payment schedule/advance settlement.
9. Implement tax, cashflow, P&L reports.
10. Add CI, broader tests, ops hardening continuously across all batches.

## Will Be Done

- Keep this plan updated as the long roadmap implementation progresses.
- Add review packages before each code-changing batch.
- Add Mockup as Text before each UI-changing batch.
- Track completed batches under `Done`.
- Move this plan to `docs/plan-closed/` only when the user says `Đóng plan` or `Đóng lại plan`.

## Done

- Read Google Sheet roadmap tab `Roadmap`.
- Compared roadmap against current backend routes, migrations, domain service/repository, frontend routes, and major modules.
- Identified current repo as broad demo/full-stack foundation with many partial modules but missing typed business-form layer.
- Added user-provided invariant: step `Role phụ trách` must be multiple.
- Created this interruption-safe agent checklist in `docs/current-states.md`.

## Validation Status

- No source code changed.
- No tests/build run for this documentation-only update.
- Browser verification not run, per repo rule unless explicitly requested.
