# Active Plan: Production-Grade Workflow Roadmap Agent Checklist

## Objective

Maintain a durable, interruption-safe checklist for implementing the Google Sheet roadmap into Consflow without mismatch, forgotten domain invariants, or hallucinated scope. This plan is the source of truth for follow-up agents when continuing long-running Production-Grade implementation work.

## Latest User Context

- Source roadmap: Google Sheet `Lộ trình Triển khai Tính năng Workflow`, tab `Roadmap`, spreadsheet id `1Hm-hU3bBFLmN_7x7Oh3ue-W4Zbgml4Zm-XltIrQ0Kzo`.
- Ignore roadmap columns: `Thời gian dự kiến`, `Phụ thuộc`, `Trạng thái`, `Ưu tiên`.
- Main need: analyze the roadmap against the current repo and create an agent-facing checklist/memory for long implementation.
- Added core invariant from user: Pipeline Builder step `Role phụ trách` must support multiple roles, not a single role.
- Production-grade expectation from user: do not ship MVP-style, placeholder-only, superficial, or half-finished behavior. If a feature cannot be completed end-to-end in the current station, defer it to a dedicated station rather than adding misleading UI.

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
13. Do not add placeholder-only UI for production features. File upload, evidence, PDF, signature, accounting, and workflow form features must be backed by real contracts, persistence, validation, permissions, and user feedback when they are introduced.

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
- After completing each implementation station, always report the next recommended station in the final response so the user does not need to ask.
- Keep each station production-grade. Avoid MVP placeholders; defer incomplete capabilities to a full follow-up station with backend/frontend/storage/validation coverage as needed.
- Move this plan to `docs/plan-closed/` only when the user says `Đóng plan` or `Đóng lại plan`.

## Done

- Read Google Sheet roadmap tab `Roadmap`.
- Compared roadmap against current backend routes, migrations, domain service/repository, frontend routes, and major modules.
- Identified current repo as broad demo/full-stack foundation with many partial modules but missing typed business-form layer.
- Added user-provided invariant: step `Role phụ trách` must be multiple.
- Created this interruption-safe agent checklist in `docs/current-states.md`.
- Implemented first roadmap batch for Pipeline Builder step multi-role UI.
- Changed Pipeline Builder step form state/payload from single `responsible_role_id` to `required_role_ids: string[]`.
- Added reusable chip multi-select behavior in `NodeInspectorPanel` and reused it for both node permissions and step roles.
- Added inline/warning validation so step save requires a checklist name and at least one responsible role.
- Fixed the Pipeline Builder step role dropdown being clipped by the dialog footer by rendering that role menu inline while keeping node permissions as an overlay menu.
- Implemented Shared UI Refactor 2A for `DataTable`: opt-in search, sort, pagination, loading, error/retry, alignment, and default sort support.
- Applied the enhanced `DataTable` pattern to Projects, Procurement, and BOQ lists without changing backend API contracts.
- Implemented Shared UI Refactor 2B for `SimpleForm`: production-grade field metadata, helper text, two-column layout, conditional visibility, disabled rules, multi-select array submission, visible-field validation, and optional reset-on-submit.
- Applied the enhanced `SimpleForm` pattern to Operations daily report and issue forms without adding placeholder-only fields or changing backend API contracts.
- Implemented Workflow Form Core 3A schema/API/runtime foundation with new migration `backend/migrations/V10__workflow_form_core.sql`.
- Added versioned workflow form templates, form versions, fields, runtime instances, answers, and events.
- Added built-in published form definitions for checklist, daily report, meeting minutes, and generic submission.
- Mapped pipeline template steps to default form versions and create runtime form instances when starting a pipeline run.
- Added authenticated workflow form APIs: read form by work item, save draft, and submit form instance with backend schema validation and audit/event logging.
- Added Work Item dialog `Workflow Form` panel backed by real API persistence, with draft/submit states, inline validation, event trail, and no placeholder upload/PDF/signature controls.
- Added backend regression coverage for workflow form answer validation and submitted form satisfying checklist gate.
- Implemented Workflow Form Runtime UX 3B: enriched pipeline run step response with work item id/status/evidence count and workflow form instance/status/type/required summary.
- Updated Planning/Pipeline runtime UI so form-backed steps open the real Work Item dialog and workflow form panel instead of using an empty direct submit as the primary action.
- Kept step completion available only when no required form exists or the required workflow form is already submitted; backend evidence/form gates remain authoritative.
- Added backend regression coverage for enriched runtime step contract after pipeline start.
- Implemented Workflow Form Authoring 3C so Pipeline Builder can explicitly bind workflow form versions to checklist/steps.
- Added authenticated `GET /workflow-form-templates` catalog endpoint for active workflow form templates with latest published version, field count, required field keys, and field summaries.
- Added backend publish validation that rejects templates with `form_required=true` steps missing a published active workflow form version, without mutating template status.
- Preserved explicit `form_required=false` authoring intent so backend does not silently reattach a default form when an admin clears an optional form.
- Updated Pipeline Builder step add/edit dialog with workflow form required toggle, form version selector, loading/empty/error states, inline validation, and step-list form status chips.
- Extended Pipeline Builder local validation to flag required steps without a form and steps pointing at inactive/unpublished form versions.
- Added backend regression coverage for workflow form catalog rows and publish rejection when a required step has no published form mapping.
- Implemented Planning Forms 4A for production-grade Meeting Minutes workflow forms.
- Added migration `backend/migrations/V11__meeting_minutes_form_v2.sql` to publish `builtin_meeting_minutes` v2 with structured attendees, decisions, action items, confirmation, and schema-level validation metadata.
- Extended workflow form field support with `structured_list` while preserving existing v1 form instances and template step references.
- Extended work item source typing with `workflow_form_action_item` so submitted meeting minutes can generate traceable follow-up work items.
- Added backend validation for structured list rows, required cells, date/select schemas, min/max item counts, and text length rules.
- Added submit-time meeting minutes action item generation inside the workflow form transaction, including work item records, role assignments, activity trail, and generated id payload on the parent work item.
- Added generic structured-list rendering in `WorkflowFormPanel` so meeting minutes attendees, decisions, and action items are edited as dense row/card controls instead of raw JSON or textarea placeholders.
- Added backend regression coverage for structured-list validation and meeting minutes submit creating linked action item work items.
- Implemented Planning Forms 4B for production-grade Planning Daily Report workflow forms.
- Added migration `backend/migrations/V12__planning_daily_report_form_v2.sql` to publish `builtin_daily_report` v2 with report date, reporter, planning scope, progress percent, completed items, next-plan items, blockers/risks, and confirmation.
- Reused structured-list workflow form validation/rendering for daily report `Đã làm`, `Sẽ làm`, and `Blocker / Risk` sections without adding placeholder-only upload/PDF/signature fields.
- Added submit-time daily report next-plan generation inside the workflow form transaction, including linked work items, role assignments, activity trail, and generated id payload on the parent work item.
- Added backend regression coverage for daily report v2 selection and submit creating linked next-plan work items.
- Implemented Planning Forms 4C for production-grade Planning Checklist workflow forms.
- Added migration `backend/migrations/V13__planning_checklist_form_v2.sql` to publish `builtin_checklist` v2 with check date, checker, planning scope, structured checklist items, overall comment, and confirmation.
- Extended structured-list validation to support real `evidence_reference` cells scoped to the same project/work item and excluding rejected evidence.
- Added checklist submit hard gate: checklist v2 can save draft with blocked/follow-up rows, but submit is rejected until every checklist item is `done` or `not_applicable`.
- Preserved checklist auditability by storing submitted `checklist_items` in the parent work item's completion payload, not only a generic submitted marker.
- Extended `WorkflowFormPanel` structured-list UI with evidence dropdown cells, blank select defaults, rejected-evidence filtering, and checklist-specific inline submit warnings.
- Added backend regression coverage for checklist v2 catalog selection, invalid evidence rejection, blocked checklist submit rejection, and successful checklist submit payload persistence.
- Implemented Planning Forms 4D for production-grade internal workflow form signatures.
- Added migration `backend/migrations/V14__workflow_form_signatures.sql` with `workflow_form_signatures`, immutable update/delete trigger, per-user/per-purpose uniqueness, signed payload hash, and `workflow_form_events` support for `signed`.
- Added authenticated `POST /workflow-form-instances/:id/sign` action that only signs submitted forms, enforces existing work item actor/permission rules, writes signature/event/activity/audit records in one transaction, and returns refreshed form detail.
- Added signature payload hashing from the submitted form snapshot, signer identity, purpose, roles, comment, form revision, project, and work item context.
- Extended workflow form detail responses with `signatures`.
- Added `WorkflowFormPanel` internal signature section with signer list, timestamp, short hash, comment, draft disabled state, duplicate-current-user disabled state, and sign action.
- Added backend regression coverage for sign-before-submit rejection, successful signing, duplicate signing rejection, event/activity/audit persistence, and DB immutability trigger.
- Implemented Planning Forms 4E for production-grade real image/evidence upload and gallery.
- Added migration `backend/migrations/V15__evidence_file_storage.sql` with evidence storage provider/key/checksum metadata, checksum format validation, and storage-key indexes.
- Added backend upload configuration via `UPLOAD_DIR` and `MAX_UPLOAD_BYTES`, increased Fiber body limit from that config, and mounted Docker volume `consflow_uploads` at `/app/uploads`.
- Added authenticated multipart `POST /work-items/:id/evidence/upload` and protected `GET /evidence/:id/file`; binary files are served only after existing work-item visibility checks and storage-path containment checks.
- Added service-side upload validation for file presence, size, detected MIME allowlist, evidence-type/MIME consistency, safe storage paths, SHA-256 checksum generation, and DB failure cleanup of orphan files.
- Extended evidence records and frontend types with `storage_provider`, `storage_key`, and `checksum_sha256` while keeping legacy JSON evidence/file_url behavior compatible.
- Added `EvidencePanel` in the Work Item dialog with real multipart upload, inline validation/warning toasts, protected thumbnail/blob fetching with token, checksum/file metadata display, and open-file action.
- Added backend regression coverage for upload storage metadata, checksum, stored file bytes, protected file lookup, and invalid MIME rejection.
- Implemented Planning Forms 4F for production-grade workflow form visual signature stamps and PDF export artifacts.
- Added migration `backend/migrations/V16__workflow_form_pdf_exports.sql` with visual signature storage metadata on immutable workflow form signatures, append-only immutable `workflow_form_pdf_exports`, storage/checksum constraints, and `pdf_exported` workflow form events.
- Extended workflow form signing so new signatures require a canvas PNG data URL plus normalized vector strokes; the PNG is stored as a protected local asset and the vector strokes are embedded in the immutable signature payload for PDF stamping.
- Added protected endpoints for visual signature files and workflow form PDF files, both gated by existing work item visibility checks and protected storage path checks.
- Added backend PDF generation for submitted workflow forms using the submitted form snapshot, answers, current signatures, payload hash, visual signature stamp boxes, and stored PDF artifact metadata.
- Added `SignatureCaptureDialog` with canvas pointer capture, clear state, purpose/comment controls, stroke capture, PNG export, and submit gating.
- Extended `WorkflowFormPanel` with `Ký / Đóng dấu PDF`, visual signature open action, `Generate PDF`, and protected PDF artifact open action.
- Added backend regression coverage for PDF byte generation with signature stamp text, visual signature metadata/storage lookup, PDF artifact metadata/storage lookup, event/audit persistence, and PDF export immutability when DB-backed tests are enabled.
- Implemented Planning Forms 4G for production-grade BOQ unit-price lifecycle.
- Added migration `backend/migrations/V17__boq_unit_price_lifecycle.sql` with `expected_unit_price`, `norm_unit_price`, `final_unit_price`, lock actor/time/comment metadata, price revision tracking, immutable `boq_unit_price_events`, procurement item price-source metadata, and BOQ trigger protection for norm/final locks.
- Preserved backward compatibility by keeping `boq_items.unit_price` as the effective price used by existing `total_amount` and legacy readers, synchronized from final/norm/expected lifecycle state.
- Added backend actions for `POST /boq-items/:id/unit-price/expected`, `POST /boq-items/:id/unit-price/lock-norm`, and `POST /boq-items/:id/unit-price/lock-final`, with validation, permission checks, audit logging, immutable price events, and final-lock update protection.
- Updated BOQ create/update flow so legacy `unit_price` input maps to `expected_unit_price` while normal BOQ edits cannot bypass norm/final lifecycle rules.
- Updated procurement generation from BOQ so approved BOQ rows must be final locked before purchase requests are generated; generated procurement items now store `boq_price_source`, `boq_price_revision_no`, and `boq_price_event_id`.
- Rebuilt `BOQPage` with lifecycle metrics, expected/norm/final unit-price display, final-lock status, dedicated Unit Price Lifecycle dialog, price history, and separate update/lock actions.
- Added frontend BOQ lifecycle types and backend regression coverage for price validation plus BOQ-to-procurement final-lock behavior when DB-backed tests are enabled.

## Validation Status

- `cd frontend && npm run build` passed after Shared UI Refactor 2B.
- `cd frontend && npm run build` passed after Workflow Form Core 3A.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Workflow Form Core 3A.
- `cd backend && make fmt` passed after Workflow Form Core 3A.
- `cd frontend && npm run build` passed after Workflow Form Runtime UX 3B.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Workflow Form Runtime UX 3B.
- `cd backend && make fmt` passed after Workflow Form Runtime UX 3B.
- `cd frontend && npm run build` passed after Workflow Form Authoring 3C; Vite chunk-size warning remains.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Workflow Form Authoring 3C.
- `cd backend && make fmt` passed after Workflow Form Authoring 3C.
- `git diff --check` passed after Workflow Form Authoring 3C.
- `cd frontend && npm run build` passed after Planning Forms 4A; Vite chunk-size warning remains.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Planning Forms 4A.
- `cd backend && make fmt` passed after Planning Forms 4A.
- `git diff --check` passed after Planning Forms 4A.
- `cd backend && make fmt` passed after Planning Forms 4B.
- `cd frontend && npm run build` passed after Planning Forms 4B; Vite chunk-size warning remains.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Planning Forms 4B.
- `git diff --check` passed after Planning Forms 4B.
- `cd backend && make fmt` passed after Planning Forms 4C.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Planning Forms 4C.
- `cd frontend && npm run build` passed after Planning Forms 4C; Vite chunk-size warning remains.
- `git diff --check` passed after Planning Forms 4C.
- `cd backend && make fmt` passed after Planning Forms 4D.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Planning Forms 4D.
- `cd frontend && npm run build` passed after Planning Forms 4D; Vite chunk-size warning remains.
- `git diff --check` passed after Planning Forms 4D.
- `cd backend && make fmt` passed after Planning Forms 4E.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Planning Forms 4E.
- `cd frontend && npm run build` passed after Planning Forms 4E; Vite chunk-size warning remains.
- `docker compose config` passed after Planning Forms 4E.
- `git diff --check` passed after Planning Forms 4E.
- Custom migration runner for Planning Forms 4E could not apply locally because the reachable PostgreSQL server returned `FATAL: role "consflow" does not exist`; no database role/database mutation was attempted.
- `cd backend && make fmt` passed after Planning Forms 4F.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Planning Forms 4F.
- `cd frontend && npm run build` passed after Planning Forms 4F; Vite chunk-size warning remains.
- `docker compose config` passed after Planning Forms 4F.
- `git diff --check` passed after Planning Forms 4F.
- Custom migration runner for Planning Forms 4F could not apply locally because the reachable PostgreSQL server returned `FATAL: role "consflow" does not exist`; no database role/database mutation was attempted.
- PDF render-to-PNG validation was not run because `pdftoppm` is not available in PATH and no local migrated DB artifact could be generated; byte-level PDF generation regression coverage passed.
- `cd backend && make fmt` passed after Planning Forms 4G.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Planning Forms 4G.
- `cd frontend && npm run build` passed after Planning Forms 4G; Vite chunk-size warning remains.
- `docker compose config` passed after Planning Forms 4G.
- `git diff --check` passed after Planning Forms 4G.
- Custom migration runner for Planning Forms 4G could not apply locally because the reachable PostgreSQL server returned `FATAL: role "consflow" does not exist`; no database role/database mutation was attempted.
- Browser verification not run, per repo rule unless explicitly requested.
- No backend/API/migration changes were needed for the Pipeline Builder multi-role UI, Shared UI 2A, or Shared UI 2B batches.
