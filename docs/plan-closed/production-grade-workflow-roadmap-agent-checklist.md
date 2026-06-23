# [Closed] Production-Grade Workflow Roadmap Agent Checklist

## Objective

Historical implementation record for the Google Sheet roadmap work in Consflow. This closed plan is not durable agent memory or a required source of truth; durable module, feature, invariant, and routing knowledge belongs in `AGENTS.md` and `.agents/skills/**/references/*.md`.

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
| Nền tảng | Done | CI, migration checksum check, request-id logging, validation Make targets, deployment/rollback checklist, backup/restore procedure, and seed policy implemented. |
| Dữ liệu lõi | Done | Partner catalog, canonical partner links, and snapshot preservation implemented for project/material/procurement/accounting/invoice/acceptance flows. |
| Auth / RBAC | Done | Permission-aware navigation/routes/actions, DB-backed session refresh/revoke lifecycle, and RBAC assignment matrix implemented. |
| Lõi module nghiệp vụ | Done | Remaining generic CRUD helper usage in core business endpoints was replaced with named handler-service-repository use cases, actor defaults, audit trail, and project-scope safeguards. |
| Admin UX | Done | Dense operational UX, loading/error/retry, empty states, search/sort/pagination, filters, and action readiness were improved across core admin operation modules. |
| Shared UI | Done | Shared list/table/form/dialog patterns are now upgraded for the core admin operation screens before further UI duplication. |
| Business UI | Done | Work-item runtime UI now exposes workflow form, checklist, evidence, signature, and PDF/document artifact components as business-facing panels. |
| Workflow thành nghiệp vụ | Done | Workflow runtime now exposes typed business form summaries across pipeline steps and work items, with start-time required form mapping validation. |
| Giao diện hoá nghiệp vụ kế hoạch | Done | Planning now has a business-facing form workspace with form metrics, filters, required/optional status, next actions, and preserved pipeline runtime controls. |
| Hệ quy đổi Workflow thành nghiệp vụ | Done | Workflow form template/version governance is now exposed through API/UI, published versions remain immutable, and Pipeline Builder can keep mappings to older published versions. |
| Daily report kế hoạch | Done | Planning daily reports now have project/date/user/owner/status/search filters, did/will/blocker summaries, and direct work-item form access backed by workflow form runtime data. |
| Biên bản họp | Done | Meeting minutes now have a read-only planning workspace with project/date/chair/owner/status/search filters, attendees/decisions/action-items summaries, generated work-item counts, and direct work-item form access backed by workflow form runtime data. |
| Checklist kế hoạch | Done | Planning checklists now have a read-only workspace with project/date/checked-by/assignee/item-status/evidence/form-status/search filters, item/evidence summaries, and direct work-item form access backed by `builtin_checklist` workflow form runtime data. |
| Chữ ký ảo | Done | Internal workflow-form signatures already have immutable actor/time/revision/hash records; Work Item UI now exposes immutable signature detail, purpose duplication guard, and signing acknowledgement. |
| Hình ảnh | Done | Real evidence upload/storage was already backed by V15; Work Item and Project Files now expose gallery filters, protected preview/open, full metadata detail, and evidence accept/reject actions through existing permissioned APIs. |
| Đơn giá | Done | BOQ unit-price lifecycle is backed by V17 expected/norm/final state, immutable price events, final-lock procurement propagation, and readiness UI. |
| Checklist thầu phụ | Done | Dedicated subcontractor checklist form and evidence flow implemented; reconciliation is tracked separately. |
| Checklist chủ đầu tư | Done | Dedicated owner/client checklist and signoff flow implemented; reconciliation is tracked separately. |
| Daily report thi công | Done | Added did/will, crew, actual quantity, material usage rows, and site photo evidence. |
| Đối soát checklist thầu phụ | Done | Durable subcontractor checklist reconciliation with BOQ mapping, confirmed quantity, variance, decision, activity, and audit. |
| Đối soát checklist chủ đầu tư | Done | Durable owner/client checklist reconciliation with acceptance mapping, client decision, required action, final decision, activity, and audit. |
| Vật tư trước/sau | Done | Durable before/after material snapshot per work item, with submit-time site balance usage application and no stock movement generation. |
| Lưu kho vật tư tồn | Done | Durable surplus handling with custody location, condition, evidence, partial/handled balance status, and submit-time stock movement linkage. |
| Nghiệm thu đầu vào | Done | Separate incoming acceptance records/items with procurement/material/evidence references, submit audit, and no automatic stock movement. |
| Nghiệm thu đầu ra | Done | Durable outgoing acceptance records/checklist with work item link, workflow form/PDF references, signature/document gates, submit/chốt result, punch list/rework history, and handover-compatible final pass. |
| PDF hoá đơn đầu vào | Done | Protected input invoice PDF upload/download linked to payable, optional procurement request, and document metadata. |
| PDF hoá đơn đầu ra | Done | Protected output invoice PDF upload/download linked to receivable, optional client receipt payment, and document metadata. |
| Thanh toán kỳ/ứng trước đầu vào | Done | Durable input payment schedules, advance requests, partial payment allocations, advance settlement, and payable reconciliation implemented. |
| Thanh toán kỳ/ứng trước đầu ra | Done | Durable output billing schedules, client advance receipts, partial receipt allocations, advance settlement, issued invoice links, and receivable reconciliation implemented. |
| Công nợ đầu vào | Done | Payable aging, vendor ledger, invoice/payment reconciliation, project-scoped AP list, and reconciliation warnings implemented. |
| Công nợ đầu ra | Done | Receivable aging, client ledger, invoice/receipt reconciliation, project-scoped AR list, and reconciliation warnings implemented. |
| Dòng tiền thu/chi | Done | Cash-basis inflow/outflow time series by project/date/category with payment ledger and CSV export. |
| Báo cáo thuế | Done | Tax/VAT ledger by invoice period, source document, counterparty tax code, warning checks, and CSV export. |
| Báo cáo lợi nhuận | Done | Project P&L with invoice-based revenue, direct/indirect cost, gross/net margin, warning checks, and CSV export. |

## Production-Grade Implementation Checklist

### 0. Agent Start Checklist

- Read `AGENTS.md`.
- Read this active plan in `docs/current-states.md`.
- Read `.agents/skills/consflow-module-memory/references/module-keyword-map.md`.
- If the user says `tiếp tục triển khai`, treat it only as permission to continue the workflow by preparing the next review package and Mockup as Text when UI is involved. Do not edit code, create migrations, update UI, update docs, or run implementation-side formatting/build/test commands until the user explicitly confirms implementation after the review package, such as `OK triển khai`, `Duyệt, làm đi`, `Confirm implement`, or `Triển khai theo review package này`.
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
- Implement incoming acceptance by work item for supplier/subcontractor/procurement deliveries, with line decisions, evidence, audit, and no implicit stock movement.

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
- Implemented Construction Phase 5A for production-grade subcontractor checklist workflow forms.
- Added migration `backend/migrations/V18__subcontractor_checklist_form.sql` to publish `builtin_subcontractor_checklist` with subcontractor identity, work package, construction area, structured checklist rows, quantity claim/confirmation cells, optional evidence references, owner response, and confirmation.
- Kept subcontractor checklist under existing `form_type='checklist'` to reuse the current workflow form runtime, checklist submit gate, work item completion payload, signature, PDF, and evidence validation behavior without widening form-type constraints.
- Extended structured-list validation/rendering to support numeric row cells for claimed and confirmed quantities.
- Updated Pipeline Builder construction presets/default form selection so steps named for subcontractor checklist bind to `Checklist thầu phụ` when the catalog is loaded, while other checklist steps still use the generic checklist form.
- Added backend fallback form-version selection for pipeline step create/update and template creation so `Checklist thầu phụ` can resolve to the subcontractor checklist even when frontend catalog data is not available.
- Added backend regression coverage for structured-list numeric cells and subcontractor checklist catalog, blocked submit rejection, invalid quantity rejection, invalid evidence rejection, successful submit, and completion payload persistence when DB-backed tests are enabled.
- Implemented Construction Phase 5B for production-grade owner/client checklist workflow forms.
- Added migration `backend/migrations/V19__owner_client_checklist_form.sql` to publish `builtin_owner_client_checklist` with client/company representatives, handover scope, structured checklist rows, client decision, evidence reference, severity, required action, final client note, and confirmation.
- Kept owner/client checklist under existing `form_type='checklist'` to reuse workflow form runtime, checklist submit gate, work item completion payload, signature, PDF, and evidence validation behavior.
- Extended backend fallback form-version selection for pipeline step create/update and template creation so `Checklist chủ đầu tư`, `client checklist`, or `owner checklist` resolves to the owner/client checklist form.
- Updated Pipeline Builder acceptance presets/default form selection so steps named for owner/client checklist bind to `Checklist chủ đầu tư` when the catalog is loaded.
- Added backend regression coverage for owner/client checklist catalog, default form selection, blocked submit rejection, invalid evidence rejection, successful submit, and completion payload persistence when DB-backed tests are enabled.
- Deferred owner checklist reconciliation against acceptance/internal output to the next dedicated station.
- Implemented Construction Phase 5C for production-grade construction daily report workflow forms.
- Added migration `backend/migrations/V20__construction_daily_report_form.sql` to publish `builtin_construction_daily_report` with report date/reporter, construction scope, weather, progress percent, site photo evidence, completed work rows, crew rows, report-only material usage rows, next-plan rows, blockers, and confirmation.
- Kept construction daily report under existing `form_type='daily_report'` while using a separate template code from `builtin_daily_report` so planning daily report v2 remains the default for non-construction daily report steps.
- Generalized daily-report next-plan item generation so construction daily reports produce `construction_daily_report` metadata and comments while planning daily reports keep existing `planning_daily_report` behavior.
- Updated backend and Pipeline Builder default form selection so `Daily report thi công`, construction node daily-report steps, or site/construction report names bind to `Daily report thi công` when available.
- Added backend regression coverage for construction daily report catalog, default form selection, invalid evidence rejection, invalid quantity rejection, successful submit, report-only material usage persistence, and next-plan work item generation when DB-backed tests are enabled.
- Deferred warehouse stock deduction/reconciliation from `material_usage_items`; material usage rows are report evidence only in this station.
- Implemented Construction Phase 5D for production-grade subcontractor checklist reconciliation.
- Added migration `backend/migrations/V21__subcontractor_checklist_reconciliation.sql` with durable reconciliation headers and item rows linked to project, work item, submitted workflow form revision, BOQ item snapshot, evidence, confirmed quantity, variance, decision, activity, and audit trail.
- Added authenticated work item reconciliation APIs: read, save draft, and submit under existing work item auth with project permission checks for `boq.update` or `operation.report`.
- Kept reconciliation as separate auditable evidence and did not mutate submitted workflow form answers, BOQ, procurement, warehouse stock, payable, or payment state.
- Added service/repository validation for submitted `builtin_subcontractor_checklist` forms, same-project BOQ mapping, same-work-item evidence, nonnegative quantities, duplicate item rejection, and immutable submitted reconciliation.
- Added `SubcontractorReconciliationPanel` inside the Work Item dialog under `WorkflowFormPanel`, with loading/error/submit-required states, BOQ select, claim/confirmed/variance display, decision select, line comments, draft save, submit, and dismissible toast feedback through the existing toast provider.
- Added frontend domain types for subcontractor reconciliation response, header, and item contracts.
- Added backend regression coverage for reconciliation default generation, invalid BOQ rejection, persistence, variance calculation, workflow form answer immutability, work item activity, audit log, and post-submit edit rejection when DB-backed tests are enabled.
- Implemented Construction Phase 5E for production-grade owner/client checklist reconciliation.
- Added migration `backend/migrations/V22__owner_client_checklist_reconciliation.sql` with durable reconciliation headers and item rows linked to project, work item, submitted workflow form revision, evidence, same-project acceptance records, client decision, severity, required action, due date, final reconciliation decision, activity, and audit trail.
- Added authenticated work item owner/client reconciliation APIs: read, save draft, and submit under existing work item auth with project permission checks for `acceptance.approve`, `acceptance.reject`, or `operation.report`.
- Kept reconciliation as separate auditable evidence and did not mutate submitted workflow form answers, acceptance status/result, rework, handover, stock, accounting, or PDF state.
- Added service/repository validation for submitted `builtin_owner_client_checklist` forms, same-project acceptance mapping, same-work-item evidence, valid client/final decisions, submit-time internal acceptance requirements, duplicate item rejection, and immutable submitted reconciliation.
- Added `OwnerClientReconciliationPanel` inside the Work Item dialog under `WorkflowFormPanel`, with loading/error/submit-required states, acceptance select, client decision/status badges, required action display, final decision select, line comments, draft save, submit, and dismissible toast feedback through the existing toast provider.
- Added frontend domain types for owner/client reconciliation response, header, and item contracts.
- Added backend regression coverage for default owner/client reconciliation generation, draft save without acceptance, missing/invalid acceptance rejection on submit, persistence, workflow form answer immutability, work item activity, audit log, and post-submit edit rejection when DB-backed tests are enabled.
- Implemented Construction Phase 5F for production-grade material before/after snapshots.
- Added migration `backend/migrations/V23__material_before_after_snapshots.sql` with durable snapshot headers and item rows linked to project, work item, material, site balance, before/after evidence, quantities, condition, source, comments, activity, and audit trail.
- Added authenticated work item material snapshot APIs: read, save draft, and submit under existing work item auth with project permission checks for `operation.report` or `warehouse.issue`.
- Kept draft saves report-only; submit is immutable and transactionally increments `site_material_balances.used_quantity`/recomputes surplus for balance-affecting rows without creating `stock_movements`.
- Added service/repository validation for active materials, same-work-item evidence, nonnegative before/added/after/waste quantities, computed used quantity, site balance existence, submit-time available balance, duplicate item rejection, and post-submit edit rejection.
- Added `MaterialSnapshotPanel` inside the Work Item dialog under checklist reconciliation panels, with material/source/condition controls, before/added/after/waste quantities, computed used quantity, evidence before/after selectors, draft save, submit, and dismissible toast feedback through the existing toast provider.
- Added frontend domain types for material snapshot response, header, item, material catalog, and project material balance contracts.
- Implemented Construction Phase 5G for production-grade surplus material handling.
- Added migration `backend/migrations/V24__site_surplus_handling.sql` with durable surplus handling headers linked to project, work item, site material balance, material, optional material snapshot, evidence, warehouse, and stock movement.
- Added authenticated work item surplus handling APIs: read by work item, create draft, update draft, and submit under existing work item auth with submit restricted to `warehouse.surplus_handle`.
- Kept draft saves report-only; submit is immutable and transactionally creates `return_to_warehouse` or `return_to_subcontractor` stock movement, links the movement back to the handling record, updates site balance, and records activity/audit.
- Changed stock return balance status semantics so partial surplus handling sets `surplus_status='partial'` and only zero-surplus handling sets `surplus_status='handled'`.
- Added service/repository validation for same-project site balance, active warehouse, same-work-item evidence, positive quantity, submit-time surplus availability, required custody location/evidence on submit, valid condition/type, and post-submit edit rejection.
- Added `SurplusHandlingPanel` inside the Work Item dialog under `MaterialSnapshotPanel`, with per-balance draft controls, warehouse/location/evidence/condition fields, save draft, submit, and submitted handling history.
- Added frontend domain types for surplus handling records, response payload, and warehouses.
- Implemented Construction Phase 5H for production-grade incoming acceptance.
- Added migration `backend/migrations/V25__incoming_acceptance.sql` with durable incoming acceptance headers and item rows linked to project, optional work item, procurement request/item, material, warehouse, evidence, source, result, quantities, condition, line decision, activity, and audit trail.
- Added authenticated incoming acceptance APIs: list by work item, list by project, create draft, update draft, get detail, and submit under existing project/work item auth and project permission checks.
- Kept draft saves report-only; submit is immutable and records result/activity/audit without creating `stock_movements`, mutating procurement, or changing warehouse/site material balance.
- Added service/repository validation for same-project work item/procurement/evidence, active material/warehouse, nonnegative delivered/accepted/rejected quantities, accepted+rejected within delivered, line decisions, required defect evidence/note for conditional/rejected rows, derived result, and post-submit edit rejection.
- Added `IncomingAcceptancePanel` inside the Work Item dialog under `SurplusHandlingPanel`, with source/procurement/warehouse/date/result header, material/procurement item line controls, accepted/rejected quantity validation, condition/decision/evidence/defect controls, draft save, submit, toasts, and submitted acceptance history.
- Added frontend domain types for incoming acceptance records/items/response and procurement request/item option payloads.
- Added backend regression coverage proving incoming acceptance draft/submit does not create stock movements, derives `conditional_pass`, persists item quantities, records activity/audit, and rejects post-submit edits.
- Implemented Acceptance Phase 6A for production-grade outgoing acceptance.
- Added migration `backend/migrations/V26__outgoing_acceptance_hardening.sql` to harden existing `acceptance_records` and `acceptance_checklist_items` with acceptance codes, work item/form/PDF links, acceptance date/location/representatives/scope, signature/document status, submitted actor/time, structured checklist decisions, severity, evidence, required action, and summary JSON.
- Added authenticated outgoing acceptance APIs: list by work item, list by project, create draft, update draft, get detail, submit, pass, conditional-pass, fail, and create rework under existing project/work item auth and project permission checks.
- Kept draft saves mutable/report-only; submit is immutable and enforces signature/document completion or waiver, no pending checklist rows, and real evidence/action requirements for conditional/fail rows.
- Added service/repository validation for same-project work item, workflow form, PDF export, and evidence references, submitted workflow form linkage, result/checklist consistency, and post-submit edit rejection.
- Added `OutgoingAcceptancePanel` inside the Work Item dialog under `IncomingAcceptancePanel`, with workflow form/PDF selectors, representative/scope/signature/document fields, checklist decision/severity/evidence/action rows, draft save, submit, and submitted acceptance action history.
- Upgraded `AcceptancePage` with typed outgoing acceptance list/detail, checklist count, work item/form/PDF/signature/document visibility, submitted-only pass/conditional/fail actions, and failed-completed-only rework action.
- Added frontend domain types for outgoing acceptance records/checklist/response.
- Added backend regression coverage proving outgoing acceptance submit locks edits, blocks pass when checklist has fail rows, fail creates punch list/rework history, duplicate rework is rejected, and final pass remains compatible with handover readiness checks.
- Implemented Documents/PDF Phase 7A for production-grade input invoice PDF handling.
- Added migration `backend/migrations/V27__input_invoice_pdf.sql` with durable `input_invoices` linked to project, payable, optional procurement request, document metadata, protected local storage metadata, checksum, status, upload/verify/reject actor fields, and active-list indexes.
- Added authenticated input invoice APIs for list by payable, list by project, upload PDF, protected file open, verify, and reject under existing project accounting permissions.
- Kept invoice upload from mutating payment schedules or payable payment state; verify/reject only changes invoice status and audit metadata.
- Added service/repository validation for same-project payable/procurement/document references, PDF-only MIME detection, upload size/checksum/storage-path containment, and protected file lookup.
- Added `InputInvoicePanel` inside payable detail with upload dialog, inline validation/warning toasts, protected PDF open action, verify/reject actions, invoice totals, and refreshed payable invoice counters.
- Upgraded `DebtPage` payable rows with invoice count/verified total and typed payable/receivable/input-invoice frontend contracts.
- Added backend regression coverage for input invoice upload, protected file storage bytes/checksum, generated document linkage, payable/project invoice lists, verify/reject transitions, audit logs, and cross-project procurement rejection when DB-backed tests are enabled.
- Implemented Documents/PDF Phase 7B for production-grade output invoice PDF handling.
- Added migration `backend/migrations/V28__output_invoice_pdf.sql` with durable `output_invoices` linked to project, receivable, optional client receipt payment, document metadata, protected local storage metadata, checksum, status, upload/issue/void actor fields, and active-list indexes.
- Added authenticated output invoice APIs for list by receivable, list by project, upload PDF, protected file open, issue, and void under existing project accounting permissions.
- Kept output invoice upload from mutating receivable/payment state; issue/void only changes invoice status and audit metadata.
- Added service/repository validation for same-project receivable/payment/document references, `client_receipt` payment linkage, PDF-only MIME detection, upload size/checksum/storage-path containment, and protected file lookup.
- Added `OutputInvoicePanel` inside receivable detail with upload dialog, inline validation/warning toasts, protected PDF open action, issue/void actions, invoice totals, and refreshed receivable invoice counters.
- Upgraded `DebtPage` receivable rows with invoice count/issued total and typed output-invoice frontend contracts.
- Added backend regression coverage for output invoice upload, protected file storage bytes/checksum, generated document linkage, receivable/project invoice lists, issue/void transitions, audit logs, and cross-project payment rejection when DB-backed tests are enabled.
- Implemented Accounting/Debt Phase 8A for production-grade input payment schedules and advances.
- Added migration `backend/migrations/V29__input_payment_schedule_advance.sql` with `payable_payment_schedules`, `payment_allocations`, and payable/payment metadata on `advance_requests`.
- Added authenticated input payment APIs for listing/creating/approving payable schedules, listing/creating/approving/settling payable advances, and listing/creating payable-scoped payments.
- Extended payment approval so payable payment allocations update payable paid/remaining state, schedule paid/remaining state, advance paid state, and audit logs atomically without double-counting advance settlement.
- Added `InputPaymentPlanPanel` inside payable detail with schedule, advance, payment, and settlement tables/dialogs, inline validation/warning toasts, status chips, and refreshed payable/payment query invalidation.
- Removed the direct payable-list `Pay` action so users create partial payments from the detail dialog instead of one-click full settlement.
- Added seed data and backend regression coverage for schedule approval, advance approval, settlement, partial payment allocation, advance payment recording, overpay validation, and audit logging.
- Implemented Accounting/Debt Phase 8B for production-grade output billing schedules and client advances.
- Added migration `backend/migrations/V30__output_billing_schedule_advance.sql` with `receivable_payment_schedules`, `client_advance_receipts`, and `receipt_allocations`.
- Added authenticated output billing APIs for listing/creating/approving receivable schedules, listing/creating/approving/settling client advances, and listing/creating receivable-scoped receipt payments.
- Extended payment approval so receipt allocations update receivable received/remaining state, schedule received/remaining state, client advance cash state, and audit logs atomically without double-counting approved client advances.
- Added `OutputBillingPlanPanel` inside receivable detail with schedule, client advance, receipt, and settlement tables/dialogs, inline validation/warning toasts, status chips, issued-invoice linking, and refreshed receivable/payment query invalidation.
- Removed the direct receivable-list `Receive` action so users create partial receipts from the detail dialog instead of one-click full settlement.
- Added backend regression coverage for output schedule approval, client advance approval, settlement, partial receipt allocation, advance receipt cash recording, over-receipt validation, and audit logging.
- Implemented Accounting/Debt Phase 8C for production-grade input debt aging and reconciliation.
- Added migration `backend/migrations/V31__payable_aging_reconciliation_indexes.sql` with payable aging and reconciliation indexes.
- Extended `/payables` with optional `project_id`, due/effective due date, aging bucket, overdue days, advance/payment aggregates, and reconciliation status while keeping project access filtering at API level.
- Added authenticated `/payables/:id/reconciliation` snapshot with payable summary, warning checks, and vendor ledger rows from payable, input invoices, approved advances, payment allocations, and advance settlement evidence.
- Added `PayableReconciliationPanel` inside payable detail with reconciliation metric tiles, warning states, and vendor ledger table.
- Upgraded `DebtPage` payable list with project-scoped AP query, aging summary cards, due/aging/reconciliation columns, and payable due date capture on create.
- Added backend regression coverage for project-scoped payable aging list output and payable reconciliation snapshot warnings/ledger when DB-backed tests are enabled.
- Implemented Accounting/Debt Phase 8D for production-grade output debt aging and reconciliation.
- Added migration `backend/migrations/V32__receivable_aging_reconciliation_indexes.sql` with receivable aging, receipt allocation, and client advance indexes.
- Extended `/receivables` with optional `project_id`, due/effective due date, aging bucket, overdue days, issued invoice totals, receipt/advance aggregates, and reconciliation status while keeping project access filtering at API level.
- Added authenticated `/receivables/:id/reconciliation` snapshot with receivable summary, warning checks, and client ledger rows from receivable, output invoices, approved client advances, approved receipt allocations, and advance settlement evidence.
- Added `ReceivableReconciliationPanel` inside receivable detail with reconciliation metric tiles, warning states, and client ledger table.
- Upgraded `DebtPage` receivable list with project-scoped AR query, aging summary cards, due/aging/reconciliation columns, and receivable due date capture on create.
- Added backend regression coverage for project-scoped receivable aging list output and receivable reconciliation snapshot warnings/ledger when DB-backed tests are enabled.
- Implemented Accounting/Cashflow Phase 9A for production-grade cash-basis thu/chi reporting.
- Added migration `backend/migrations/V33__cashflow_report_indexes.sql` with a project/status/occurred-at payment index for cashflow queries.
- Added authenticated `/reports/cashflow` JSON report with date range, project, group-by, and status filters, including opening balance, inflow, outflow, net flow, closing balance, pending payments, time-series, category totals, and ledger rows.
- Added authenticated `/reports/cashflow/export` CSV export from the same report source.
- Added `CashflowPanel` inside Accounting with filters, summary cards, time-series table, category table, ledger, reload, and CSV export.
- Added backend regression coverage for scoped cashflow summary calculations and CSV serialization.
- Implemented Reporting/Tax Phase 9B for production-grade VAT reporting from input/output invoice source documents.
- Added migration `backend/migrations/V34__tax_report_indexes.sql` with invoice period/status/tax-code indexes for tax report queries.
- Added authenticated `/reports/tax` JSON report with date range, project, group-by, source, and posted/all filters, including VAT summary, period rows, counterparty rows, ledger rows, and warnings.
- Added authenticated `/reports/tax/export` CSV export from the same tax ledger source.
- Added `TaxReportPanel` inside Reports with project/date/source/status filters, summary cards, warning states, period table, MST/counterparty table, ledger, reload, and CSV export.
- Added backend regression coverage for tax CSV serialization and scoped posted VAT summary calculations when DB-backed tests are enabled.
- Implemented Reporting/P&L Phase 9C for production-grade invoice-based project profit reporting.
- Added migration `backend/migrations/V35__profit_report_indexes.sql` with invoice and payable classification indexes for P&L report queries.
- Added authenticated `/reports/profit` JSON report with date range, project, group-by, and posted/all filters, including P&L summary, period rows, project rows, cost breakdown, ledger rows, and warnings.
- Added authenticated `/reports/profit/export` CSV export from the same P&L ledger source.
- Added `ProfitReportPanel` inside Reports with project/date/status filters, summary cards, warning states, project table, period table, cost breakdown, ledger, reload, and CSV export.
- Added backend regression coverage for P&L CSV serialization and scoped posted profit summary calculations when DB-backed tests are enabled.
- Implemented Foundation Hardening 10A for production-grade repo validation and ops readiness.
- Added `.github/workflows/ci.yml` to run backend format check, migration checksum validation, backend tests, frontend build, and whitespace diff checks.
- Added read-only migration validation via `go run ./cmd/migrate -check`, `cd backend && make migrate-check`, and root `make migrate-check`.
- Added root Make validation targets for backend tests, frontend build, migration check, Docker Compose config, and combined test flow while preserving the existing `up`/`restart` Docker sequence.
- Added request-id middleware and request-id log formatting to the Fiber server without changing API routes or response contracts.
- Added ops runbooks for deployment/rollback, PostgreSQL backup/restore, and seed/demo data policy in `docs/ops/`.
- Implemented Dữ liệu lõi as a production-grade partner/client/vendor/catalog foundation.
- Added migration `backend/migrations/V36__core_partner_catalog.sql` with `partners`, partner type metadata, active filtering, permissions, and nullable partner links for projects, materials, procurement request items, payables, receivables, input/output invoices, and incoming acceptance records.
- Added authenticated `/partners` list/create/update APIs with `catalog.view` and `catalog.manage` permissions, active/type filters, partner snapshot helpers, and type validation for supplier/client/subcontractor usage.
- Preserved historical text snapshot fields while allowing canonical `partner_id` links to populate client/vendor/supplier names and tax codes on create/upload/acceptance flows.
- Added the frontend `Dữ liệu lõi` page under system navigation with partner create/edit/detail dialogs, filters, active states, and dense data table behavior.
- Added partner selectors to Projects, Materials, Payables, Receivables, input/output invoice upload dialogs, and incoming acceptance supplier/subcontractor fields while keeping manual entry fallback.
- Updated frontend domain contracts and seed/demo data with deterministic partners and linked sample projects/materials/debts/procurement rows.
- Implemented Auth / RBAC as a production-grade session and authorization hardening station.
- Added migration `backend/migrations/V37__auth_session_lifecycle.sql` with DB-backed auth sessions, refresh-token hashes, access token ids, expiry, revoke timestamps, and active-session indexes.
- Moved refresh/logout to HttpOnly refresh-cookie flow while keeping protected business APIs behind `middleware.AuthRequired` with DB session validation.
- Added configurable access-token TTL, refresh-token TTL, refresh-cookie secure flag, and SameSite mode.
- Added access tokens with `session_id` and `jti`; refresh rotates both refresh token hash and access token id, and logout revokes the session so existing access tokens are invalidated.
- Added frontend API credential handling, automatic one-time refresh retry for API/file requests, and auth failure cleanup in `AuthContext`.
- Added permission-aware app navigation, guarded routes with an explicit "Không đủ quyền" state, and role-aware landing redirect to avoid sending limited users to inaccessible Dashboard routes.
- Upgraded RBAC UI with role permission matrix assignment grouped/searchable by module, permission list search/loading/error states, and staff role assignment dialog for global or project-scoped roles.
- Replaced append-only role/permission assignment endpoints with validated transaction-backed snapshot replacement and enriched role/staff list responses for UI state.
- Added DB-backed regression coverage for session refresh/revoke middleware invalidation and RBAC snapshot replacement.

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
- `gofmt -w backend/internal/core/service.go backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Construction Phase 5A.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Construction Phase 5A; DB-backed integration tests remain gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Construction Phase 5A; Vite chunk-size warning remains.
- `git diff --check` passed after Construction Phase 5A.
- Custom migration runner was not run after Construction Phase 5A; applying `V18` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Construction Phase 5B.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Construction Phase 5B; DB-backed integration tests remain gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Construction Phase 5B; Vite chunk-size warning remains.
- `git diff --check` passed after Construction Phase 5B.
- Custom migration runner was not run after Construction Phase 5B; applying `V19` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Construction Phase 5C.
- `GOCACHE="$PWD/backend/.gocache" go test ./backend/...` passed after Construction Phase 5C; DB-backed integration tests remain gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Construction Phase 5C; Vite chunk-size warning remains.
- `git diff --check` passed after Construction Phase 5C.
- Custom migration runner was not run after Construction Phase 5C; applying `V20` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/types.go backend/internal/core/routes.go backend/internal/core/handlers.go backend/internal/core/service.go backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Construction Phase 5D.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Construction Phase 5D.
- `cd frontend && npm run build` passed after Construction Phase 5D; Vite chunk-size warning remains.
- `git diff --check` passed after Construction Phase 5D.
- Custom migration runner was not run after Construction Phase 5D; applying `V21` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/types.go backend/internal/core/routes.go backend/internal/core/handlers.go backend/internal/core/service.go backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Construction Phase 5E.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Construction Phase 5E.
- `cd frontend && npm run build` passed after Construction Phase 5E; Vite chunk-size warning remains.
- `git diff --check` passed after Construction Phase 5E.
- Custom migration runner was not run after Construction Phase 5E; applying `V22` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/types.go` passed after Construction Phase 5F.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Construction Phase 5F.
- `cd frontend && npm run build` passed after Construction Phase 5F; Vite chunk-size warning remains.
- `git diff --check` passed after Construction Phase 5F.
- Custom migration runner was not run after Construction Phase 5F; applying `V23` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/types.go` passed after Construction Phase 5G.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Construction Phase 5G.
- `cd frontend && npm run build` passed after Construction Phase 5G; Vite chunk-size warning remains.
- `git diff --check` passed after Construction Phase 5G.
- Custom migration runner was not run after Construction Phase 5G; applying `V24` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/types.go` passed after Construction Phase 5H.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Construction Phase 5H.
- `cd frontend && npm run build` passed after Construction Phase 5H; Vite chunk-size warning remains.
- `git diff --check` passed after Construction Phase 5H.
- Custom migration runner was not run after Construction Phase 5H; applying `V25` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/types.go` passed after Acceptance Phase 6A.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Acceptance Phase 6A; DB-backed integration tests remain gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Acceptance Phase 6A; Vite chunk-size warning remains.
- `git diff --check` passed after Acceptance Phase 6A.
- Custom migration runner was not run after Acceptance Phase 6A; applying `V26` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/types.go` passed after Documents/PDF Phase 7A.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Documents/PDF Phase 7A; DB-backed input invoice integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Documents/PDF Phase 7A; Vite chunk-size warning remains.
- `git diff --check` passed after Documents/PDF Phase 7A.
- Custom migration runner was not run after Documents/PDF Phase 7A; applying `V27` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/types.go` passed after Documents/PDF Phase 7B.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Documents/PDF Phase 7B; DB-backed output invoice integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Documents/PDF Phase 7B; Vite chunk-size warning remains.
- `git diff --check` passed after Documents/PDF Phase 7B.
- Custom migration runner was not run after Documents/PDF Phase 7B; applying `V28` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/types.go backend/internal/core/handlers.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/repository.go backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Accounting/Debt Phase 8A.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Accounting/Debt Phase 8A; DB-backed payment schedule/advance integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Accounting/Debt Phase 8A; Vite chunk-size warning remains.
- `git diff --check` passed after Accounting/Debt Phase 8A.
- Custom migration runner was not run after Accounting/Debt Phase 8A; applying `V29` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/types.go backend/internal/core/handlers.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/repository.go backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Accounting/Debt Phase 8B.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Accounting/Debt Phase 8B; DB-backed output billing/advance integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Accounting/Debt Phase 8B; Vite chunk-size warning remains.
- `git diff --check` passed after Accounting/Debt Phase 8B.
- Custom migration runner was not run after Accounting/Debt Phase 8B; applying `V30` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go` passed after Accounting/Debt Phase 8C.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Accounting/Debt Phase 8C; DB-backed payable reconciliation integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Accounting/Debt Phase 8C; Vite chunk-size warning remains.
- `git diff --check` passed after Accounting/Debt Phase 8C.
- Custom migration runner was not run after Accounting/Debt Phase 8C; applying `V31` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go` passed after Accounting/Debt Phase 8D.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Accounting/Debt Phase 8D; DB-backed receivable reconciliation integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Accounting/Debt Phase 8D; Vite chunk-size warning remains.
- `git diff --check` passed after Accounting/Debt Phase 8D.
- Custom migration runner was not run after Accounting/Debt Phase 8D; applying `V32` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/types.go` passed after Accounting/Cashflow Phase 9A.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Accounting/Cashflow Phase 9A; DB-backed cashflow integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Accounting/Cashflow Phase 9A; Vite chunk-size warning remains.
- `git diff --check` passed after Accounting/Cashflow Phase 9A.
- Custom migration runner was not run after Accounting/Cashflow Phase 9A; applying `V33` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/types.go` passed after Reporting/Tax Phase 9B.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Reporting/Tax Phase 9B; DB-backed tax report integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Reporting/Tax Phase 9B; Vite chunk-size warning remains.
- `git diff --check` passed after Reporting/Tax Phase 9B.
- Custom migration runner was not run after Reporting/Tax Phase 9B; applying `V34` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/project_operations_integration_test.go backend/internal/core/repository_domain.go backend/internal/core/routes.go backend/internal/core/service.go backend/internal/core/types.go` passed after Reporting/P&L Phase 9C.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Reporting/P&L Phase 9C; DB-backed P&L report integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Reporting/P&L Phase 9C; Vite chunk-size warning remains.
- `git diff --check` passed after Reporting/P&L Phase 9C.
- Custom migration runner was not run after Reporting/P&L Phase 9C; applying `V35` still requires a reachable Consflow PostgreSQL role/database.
- `gofmt -w backend/cmd/migrate/main.go backend/cmd/server/main.go backend/internal/shared/migrations/runner.go` passed after Foundation Hardening 10A.
- `make migrate-check` passed after Foundation Hardening 10A; 35 migration files were validated without connecting to the database.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Foundation Hardening 10A.
- `cd frontend && npm run build` passed after Foundation Hardening 10A; Vite chunk-size warning remains.
- `make docker-config` passed after Foundation Hardening 10A.
- `git diff --check` passed after Foundation Hardening 10A.
- No new SQL migration was added for Foundation Hardening 10A; no database migration apply was run.
- `gofmt -w backend/internal/core/types.go backend/internal/core/service.go backend/internal/core/repository.go backend/internal/core/repository_domain.go backend/internal/core/handlers.go backend/internal/core/routes.go` passed after Dữ liệu lõi.
- `make migrate-check` passed after Dữ liệu lõi; 36 migration files were validated without connecting to the database.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Dữ liệu lõi.
- `cd frontend && npm run build` passed after Dữ liệu lõi; Vite chunk-size warning remains.
- `git diff --check` passed after Dữ liệu lõi.
- Custom migration runner apply was not run after Dữ liệu lõi; applying `V36` still requires a reachable Consflow PostgreSQL role/database.
- Browser verification not run, per repo rule unless explicitly requested.
- `gofmt -w backend/internal/shared/config/config.go backend/internal/shared/middleware/auth.go backend/internal/core/routes.go backend/internal/core/handlers.go backend/internal/core/service.go backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Auth / RBAC.
- `make migrate-check` passed after Auth / RBAC; 37 migration files were validated without connecting to the database.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Auth / RBAC; DB-backed session/RBAC integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `cd frontend && npm run build` passed after Auth / RBAC; Vite chunk-size warning remains.
- `git diff --check` passed after Auth / RBAC.
- Custom migration runner apply was not run after Auth / RBAC; applying `V37` still requires a reachable Consflow PostgreSQL role/database.
- Browser verification not run after Auth / RBAC, per repo rule unless explicitly requested.
- Implemented Lõi module nghiệp vụ as a backend service/repository refactor station.
- Replaced remaining generic CRUD helper call-sites for warehouse create, operations daily reports/issues, rework/replanning, handover, accounting budget, and payments with named handler-service-repository methods.
- Added service-level actor defaults and audit events for the refactored core business creates/status updates.
- Blocked issue `project_id` changes during update so project-scoped permission checks cannot be bypassed by moving a record after authorization.
- Removed the unused generic project-list/project-create/status helper code after the business endpoint call-sites were migrated; generic create/update remains only for Staff/RBAC call-sites outside this station.
- Added DB-backed integration coverage for actor defaults, audit events, project-scope preservation, and lifecycle status updates; tests remain skipped unless `CONSFLOW_TEST_DATABASE_URL` is set.
- `gofmt -w backend/internal/core/handlers.go backend/internal/core/service.go backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Lõi module nghiệp vụ.
- `make migrate-check` passed after Lõi module nghiệp vụ; 37 migration files were validated without connecting to the database.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Lõi module nghiệp vụ; DB-backed business-core integration coverage remains gated by `CONSFLOW_TEST_DATABASE_URL`.
- `git diff --check` passed after Lõi module nghiệp vụ.
- No new SQL migration was added for Lõi module nghiệp vụ; custom migration runner apply was not run.
- Frontend build was not run for Lõi module nghiệp vụ because there were no frontend changes.
- Browser verification not run after Lõi module nghiệp vụ, per repo rule unless explicitly requested.
- Implemented Admin UX as a frontend-only operational screen hardening station.
- Added shared `FilterBar` segmented controls and applied status/type filters to Operations, Warehouse, Rework/Replanning, Handover, and Accounting screens.
- Enabled enhanced `DataTable` behavior on those screens: loading, error/retry, search, sortable columns, pagination, context-aware empty states, and clearer action readiness.
- Disabled create/action buttons when required project/catalog context is missing or when row status is no longer actionable.
- `cd frontend && npm run build` passed after Admin UX; Vite chunk-size warning remains.
- `git diff --check` passed after Admin UX.
- No backend/API changes and no SQL migration were added for Admin UX; custom migration runner apply was not run.
- Browser verification not run after Admin UX, per repo rule unless explicitly requested.
- Implemented Shared UI as a frontend-only shared component station.
- Added `ListSection` for consistent dense table section headings without one-off section chrome.
- Added `RecordFormDialog` on top of shared `Dialog` + `SimpleForm` and converted Operations, Warehouse, Rework/Replanning, Handover, and Accounting create/edit forms away from page-local `FormDrawer` usage.
- Upgraded `SimpleForm` with optional cancel action support so dialog forms have a consistent `[Hủy] [submit]` action row while preserving existing validation and warning toast behavior.
- Upgraded `DataTable` with reusable empty-state actions and optional stable row key selection.
- Added context-aware empty-state create actions to the scoped core admin operation tables where records can be created from that table context.
- `cd frontend && npm run build` passed after Shared UI; Vite chunk-size warning remains.
- `git diff --check` passed after Shared UI.
- No backend/API changes and no SQL migration were added for Shared UI; custom migration runner apply was not run.
- Browser verification not run after Shared UI, per repo rule unless explicitly requested.
- Implemented Business UI as a frontend-only work-item runtime station using existing protected workflow form, evidence, signature, and PDF export APIs.
- Added a business execution summary to the Work Item detail dialog with next action, required evidence, active/accepted evidence, and current status.
- Added workflow form next-action summary for draft/submit/signature/PDF states without changing the existing `draft -> submitted`, sign, or PDF export transitions.
- Upgraded checklist structured-list rendering into a business checklist panel with row counts, Done/N/A progress, needs-action count, evidence count, per-row status, evidence selection, and row actions while preserving the existing `checklist_items` answer schema.
- Improved evidence panel visibility with photo/document/accepted/pending stats and kept protected file upload/open behavior on the existing evidence endpoints.
- Improved workflow form evidence selector labels so checklist rows show evidence name, type, and status.
- `cd frontend && npm run build` passed after Business UI; Vite chunk-size warning remains.
- `git diff --check` passed after Business UI.
- No backend/API changes and no SQL migration were added for Business UI; custom migration runner apply was not run.
- Browser verification not run after Business UI, per repo rule unless explicitly requested.
- Implemented Workflow thành nghiệp vụ as a backend/frontend runtime contract station.
- Added typed workflow form summary fields to pipeline run steps, project/my work item lists, and work item detail responses: instance/status/type/template/version/required/submitted metadata.
- Added start-pipeline validation so a template with `form_required=true` cannot create runtime work items unless the mapped workflow form version is published and active.
- Updated Planning runtime step cards, Work Item cards, Work Item detail metrics, and WorkflowFormPanel copy to show business form identity such as Biên bản họp, Checklist, Daily report, or Submission instead of only generic workflow form wording.
- Preserved existing workflow form status transitions, signature/PDF flow, step completion gating, multi-role `required_role_ids`, and current schema; no SQL migration was added.
- `gofmt -w backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Workflow thành nghiệp vụ.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Workflow thành nghiệp vụ.
- `cd frontend && npm run build` passed after Workflow thành nghiệp vụ; Vite chunk-size warning remains.
- `git diff --check` passed after Workflow thành nghiệp vụ.
- Custom migration runner apply was not run after Workflow thành nghiệp vụ because no migration was added.
- Browser verification not run after Workflow thành nghiệp vụ, per repo rule unless explicitly requested.
- Implemented Giao diện hoá nghiệp vụ kế hoạch as a frontend-only Planning workspace station.
- Added Planning form metrics, search, type/status/required filters, business form queue cards, missing-instance visibility, and form-first open action using the existing Work Item detail dialog.
- Preserved existing pipeline runtime controls for node progress, step completion, approval requests, blockers, and audit playback; no backend status transition was changed.
- `cd frontend && npm run build` passed after Giao diện hoá nghiệp vụ kế hoạch; Vite chunk-size warning remains.
- `git diff --check` passed after Giao diện hoá nghiệp vụ kế hoạch.
- No backend/API changes and no SQL migration were added for Giao diện hoá nghiệp vụ kế hoạch; custom migration runner apply was not run.
- Browser verification not run after Giao diện hoá nghiệp vụ kế hoạch, per repo rule unless explicitly requested.
- Implemented Hệ quy đổi Workflow thành nghiệp vụ as a full-stack workflow form governance station.
- Added protected Workflow Form Library APIs for template library/detail, template create/update/archive, version draft creation, draft field updates, publish, and archive.
- Expanded the Pipeline Builder workflow form catalog to return all active published versions with `latest_published`, so existing template steps can keep historical published `form_version_id` mappings while new steps still default to latest.
- Added `/workflow-forms` admin UI for template search/filter, metadata editing, draft cloning, draft field editing, publishing, and archive guard visibility.
- Preserved runtime workflow form instances, answers, signatures, PDF exports, step completion gates, and existing pipeline status transitions; published versions remain immutable because only draft versions can be edited.
- `gofmt -w backend/internal/core/types.go backend/internal/core/routes.go backend/internal/core/handlers.go backend/internal/core/service.go backend/internal/core/repository_domain.go backend/internal/core/project_operations_integration_test.go` passed after Hệ quy đổi Workflow thành nghiệp vụ.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Hệ quy đổi Workflow thành nghiệp vụ.
- `cd frontend && npm run build` passed after Hệ quy đổi Workflow thành nghiệp vụ; Vite chunk-size warning remains.
- `git diff --check` passed after Hệ quy đổi Workflow thành nghiệp vụ.
- No SQL migration was added for Hệ quy đổi Workflow thành nghiệp vụ because the existing workflow form schema already supports the governance layer; custom migration runner apply was not run.
- Browser verification not run after Hệ quy đổi Workflow thành nghiệp vụ, per repo rule unless explicitly requested.
- Implemented Daily report kế hoạch as a read-only backend/frontend planning report workspace on top of existing `builtin_daily_report` workflow form runtime data.
- Added protected `GET /planning/daily-reports` with project/date/user/owner/status/search filters, project-visibility scoping, and structured completed/next-plan/blocker counts.
- Added `/planning/daily-reports` admin UI with dense filters, metrics, daily report table, detail dialog, and direct Work Item form opening.
- Added a daily report business summary to the Work Item workflow form panel so planning reports surface Đã làm, Sẽ làm, and Blocker/Risk before the generic field controls.
- Preserved existing workflow form draft/submit/sign/PDF behavior, next-plan work-item generation, pipeline step gates, and pipeline status transitions; no new SQL migration was added.
- `gofmt -w backend/internal/core/types.go backend/internal/core/routes.go backend/internal/core/handlers.go backend/internal/core/service.go backend/internal/core/repository_domain.go` passed after Daily report kế hoạch.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Daily report kế hoạch.
- `cd frontend && npm run build` passed after Daily report kế hoạch; Vite chunk-size warning remains.
- `git diff --check` passed after Daily report kế hoạch.
- Custom migration runner apply was not run after Daily report kế hoạch because no migration was added.
- Browser verification not run after Daily report kế hoạch, per repo rule unless explicitly requested.
- Implemented Biên bản họp as a read-only planning workspace on top of existing `meeting_minutes` workflow form runtime data.
- Added protected `GET /planning/meeting-minutes` with project/date/chair/owner/status/search filters, project-visibility scoping, v2 structured-list parsing, and v1 textarea fallback fields.
- Added `/planning/meeting-minutes` admin UI with dense filters, metrics, meeting minutes table, detail dialog, artifacts counts, generated action-item work-item counts, and direct Work Item form opening.
- Added a meeting-minutes business summary to the Work Item workflow form panel so meeting title/date/chair, attendees, decisions, action items, and agenda are visible before generic field controls.
- Preserved existing workflow form draft/submit/sign/PDF behavior, meeting action-item work-item generation, pipeline step gates, and pipeline status transitions; no new SQL migration was added.
- Implemented Checklist kế hoạch as a read-only planning workspace on top of existing `builtin_checklist` workflow form runtime data.
- Added protected `GET /planning/checklists` with project/date/checked-by/assignee/item-status/evidence/form-status/search filters, project-visibility scoping, structured `checklist_items` parsing, and v1 `items_done`/`exceptions` fallback fields.
- Added `/planning/checklists` admin UI with dense filters, metrics, checklist table, detail dialog, item/evidence summaries, artifacts counts, and direct Work Item form opening.
- Added a planning-checklist business summary to the Work Item workflow form panel, scoped to `builtin_checklist` so subcontractor/client checklist flows remain separate.
- Preserved existing workflow form draft/submit/sign/PDF behavior, checklist submit validation, pipeline step gates, and pipeline status transitions; no new SQL migration was added.
- `gofmt -w backend/internal/core/types.go backend/internal/core/routes.go backend/internal/core/handlers.go backend/internal/core/service.go backend/internal/core/repository_domain.go` passed after Checklist kế hoạch.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./...` passed after Checklist kế hoạch.
- `cd frontend && npm run build` passed after Checklist kế hoạch; Vite chunk-size warning remains.
- `git diff --check` passed after Checklist kế hoạch.
- Custom migration runner apply was not run after Checklist kế hoạch because no migration was added.
- Browser verification not run after Checklist kế hoạch, per repo rule unless explicitly requested.
- Implemented Chữ ký ảo as a Work Item trust/audit UI station on top of the existing immutable workflow-form signature records from V14/V16.
- Strengthened the workflow form signature panel with revision, signer roles, payload hash, visual checksum, immutable status, and a detail dialog for actor/purpose/time/hash/checksum/comment review.
- Strengthened the signature capture dialog with an immutable-record acknowledgement, current revision context, and current-user duplicate-purpose guarding before `Ký & lưu`.
- No backend/API/migration changes were needed for Chữ ký ảo because the existing signature and PDF export model already stores actor/time/revision/hash/visual checksum and enforces immutability.
- `cd frontend && npm run build` passed after Chữ ký ảo; Vite chunk-size warning remains.
- `git diff --check` passed after Chữ ký ảo.
- Backend tests were not run after Chữ ký ảo because no backend code changed.
- Custom migration runner apply was not run after Chữ ký ảo because no migration was added.
- Browser verification not run after Chữ ký ảo, per repo rule unless explicitly requested.
- Implemented Hình ảnh as a frontend evidence gallery/detail/review station on top of the existing V15 local storage and protected evidence file APIs.
- Added a shared evidence gallery/detail UI for Work Item evidence and Project Files, with photo/document/status filters, search, protected preview/open, full metadata, original filename, checksum, storage, uploader/reviewer, and work item context.
- Connected evidence accept/reject actions to existing `POST /evidence/:id/accept` and `POST /evidence/:id/reject`, keeping backend permission checks as source of truth and invalidating work item/project views after review.
- No backend/API/migration changes were needed for Hình ảnh because real upload/storage/protected download/checksum/permission handling already exists.
- `cd frontend && npm run build` passed after Hình ảnh; Vite chunk-size warning remains.
- `git diff --check` passed after Hình ảnh.
- Backend tests were not run after Hình ảnh because no backend code changed.
- Custom migration runner apply was not run after Hình ảnh because no migration was added.
- Browser verification not run after Hình ảnh, per repo rule unless explicitly requested.
- Implemented Đơn giá closeout as frontend BOQ lifecycle readiness hardening on top of the existing V17 unit-price lifecycle.
- Added BOQ readiness summary for expected, norm locked, final locked, approved, approved-final, and blocker states, with direct links into the Unit Price Lifecycle dialog for approved rows that still need final lock.
- Disabled `Sinh mua sắm` when approved BOQ rows are not final locked or when no approved final-locked rows exist, keeping the frontend aligned with the existing backend final-lock procurement gate.
- Improved the Unit Price Lifecycle dialog with next-action guidance, procurement impact copy, lock metadata, and clearer history context.
- No backend/API/migration changes were needed for Đơn giá closeout because V17 already provides lifecycle fields, immutable events, lock endpoints, and procurement price propagation.
- `cd frontend && npm run build` passed after Đơn giá; Vite chunk-size warning remains.
- `git diff --check` passed after Đơn giá.
- Backend tests were not run after Đơn giá because no backend code changed.
- Custom migration runner apply was not run after Đơn giá because no migration was added.
- Browser verification not run after Đơn giá, per repo rule unless explicitly requested.
- No backend/API/migration changes were needed for the Pipeline Builder multi-role UI, Shared UI 2A, or Shared UI 2B batches.
