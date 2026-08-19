# [Closed] Evidence And Workflow Plans

Closed: 2026-08-19
Reason: User requested closing all active plans.

## [Closed] Project evidence stage and work-item columns

### Will be done

- Split the project evidence list's combined source line into explicit `Giai đoạn` and `Đầu việc` columns beside `Hồ sơ`.
- Keep the first action track, sender/time/status columns, server filters, cursor paging, evidence viewer and review permissions unchanged.
- Truncate long file, metadata, stage, work-item and sender text with ellipsis; expose the complete value through hover tooltip and accessible focus/labels.
- Use the seven-column list from `xl` upward and labelled stacked rows below `xl` so the added context does not compress the table.
- Add focused presentation regressions, update durable frontend/project maps, run frontend tests/build/bundle budget and diff checks; skip browser verification unless explicitly requested.

### Done

- Review package and responsive Mockup as Text approved by the user, including the ellipsis and tooltip follow-up.
- Confirmed the existing `/projects/:id/evidence` response already supplies `run_node_name` and `work_item_title`; no API, backend, database or migration change is required.
- Confirmed the implementation owner is the `listContext="project"` branch in the shared `EvidenceGallery`; default gallery list/card consumers remain out of scope.
- Replaced the combined `Hồ sơ / nguồn` presentation with explicit `Hồ sơ`, `Giai đoạn` and `Đầu việc` columns while preserving the blank first action track and sender/time/status order.
- Added the seven-track `xl` grid plus labelled stacked rows below `xl`; missing historical stage/work-item/sender values render as `—` without changing the source contract.
- Added one-line ellipsis for project file metadata and source/sender cells, native hover tooltips with complete values, and focusable accessible labels for the standalone text cells.
- Added focused column, overflow-tooltip and responsive regressions; updated the manual action-column contract plus durable frontend/project maps.
- Technically verified the current working tree: 198/198 frontend tests passed, the TypeScript/Vite production build passed, the 250 KiB bundle budget passed with 124 chunks, and `git diff --check` passed before this documentation-only completion note. Backend tests and migrations were not run because the change is frontend-only; browser/UAT was skipped under repository policy, so the UI remains `NOT UX ACCEPTED`.

## [Closed] Workflow select `Khác` free text

### Will be done

- Add a focused frontend helper and inline field UI so `client_survey_work_package.style_direction` shows `Nhập phong cách khác` when `Khác` is selected.
- Keep the existing `style_direction` answer key and published form/runtime references unchanged; store the trimmed custom text as that answer value.
- Keep draft save permissive, but fail closed on submit/amend when `Khác` has no custom text.
- Preserve ordinary select validation and PDF fallback behavior for every unrelated field.
- Add focused frontend/backend regressions, update durable maps, run the frontend build and backend tests.
- Expand the same behavior through an exact `template_code + field_key` registry for the approved BOQ, dispatch, construction, subcontractor, planning and procurement workflow selects.
- Reuse the existing `scope_detail` when `handover_scope=other` and the existing material row `note` when `source=other`; keep blocker impact and hidden risk category behavior unchanged.
- Keep every published form version, runtime reference, API route, answer payload key and migration unchanged while verifying PDF/read-only/downstream presentation.

### Done

- Review package and Mockup as Text approved by the user.
- Root cause confirmed: Rev8 copies the enum-only field, the frontend renders only a select, and backend normalization rejects custom option text.
- Route, handler and request payload confirmed unchanged; no migration is required.
- Added the scoped frontend adapter and inline required detail control for the exact 12 reviewed `template_code.field_key` pairs, including reopen, option-switch, read-only and 300-character behavior.
- Added backend normalization so draft may retain `other`, submit/amend requires custom text, custom text is trimmed under the existing answer key, and unrelated selects remain enum-strict.
- Reused `builtin_owner_client_checklist.scope_detail` and construction material-row `note` as conditionally required existing fields; blocker impact and site-risk category behavior remain unchanged.
- Clearing the owner checklist's parent scope now also clears any stale dependent `scope_detail` inline error.
- Added focused registry/dependent-field coverage and downstream construction-ledger assertions.
- Expanded the central/frontend/backend/root-cause and workflow/planning/construction/commercial/quality-handover maps for the exact registry and dependent rules.
- Verified the expanded working tree at base `fe369ac`: 177/177 frontend tests passed, `go test ./...` passed, the TypeScript/Vite production build passed, and `git diff --check` passed. The PostgreSQL-backed downstream assertion is present but skipped because `CONSFLOW_TEST_DATABASE_URL` is not configured; browser/UAT remains intentionally skipped under repository policy.
- The user approved the review package and Mockup as Text for the exact multi-form expansion, including current Rev8 and historical accessible forms, same-key custom text capped at 300 characters, and reuse of `scope_detail`/`note` without changing blocker or risk-category behavior.

## [Closed] Redesign `Xem minh chứng`

### Will be done

- Replace the oversized empty document preview with a compact document-first surface and one primary `Mở tệp` action while preserving the large in-app image preview.
- Recompose persisted evidence metadata into source, review and collapsible technical groups; keep draft evidence free of nonexistent persistence metadata.
- Preserve protected-file opening, review permissions/transitions, loading/retry/decode handling and nested-dialog Escape behavior.
- Keep the shared viewer consistent across Project Hồ sơ, work-item evidence and construction-report evidence.
- Add focused presentation regressions, update durable frontend/project/construction maps, run focused tests, frontend build and diff checks; skip browser verification unless explicitly requested.

### Done

- Review package, Mockup as Text and generated visual mockup approved by the user.
- Current cause confirmed in source: documents and photos share the same dark minimum-height media canvas, metadata is rendered as a flat stack of equal-weight cards, and the document action is separated into the footer.
- Backend API, evidence persistence, storage provider, permission and review-state contracts confirmed unchanged.
- Implemented the media-aware shared shell: protected photos retain the in-app dark preview, while documents use a light file-first surface with one primary `Mở tệp` action and an explicit unavailable state.
- Moved persisted status to the header, grouped source and sender/reviewer context, collapsed technical metadata, and placed permission/state-gated `Đóng` / `Từ chối` / `Chấp nhận` actions in the fixed footer.
- Kept the draft viewer metadata-safe with `Chưa upload`, preserved image loading/retry/decode handling, protected file opening and nested-dialog Escape behavior.
- Updated focused presentation coverage plus central/frontend/project/construction durable maps without overwriting the active Survey-style plan changes.
- Verified the final source with 172 frontend tests, `npm run build` and `git diff --check`; all passed. Browser/UAT verification was intentionally skipped under repository policy, so the UI remains `NOT UX ACCEPTED`.

## [Closed] In-app PDF evidence reader

### Will be done

- Recognize PDF evidence from MIME/filename before the business evidence type so legacy `photo + application/pdf` records do not enter the image decoder.
- Load protected `local|s3` PDF bytes on demand through the existing authenticated evidence endpoint and render the resulting blob URL inside the shared evidence dialog.
- Keep external PDFs and non-PDF documents on the existing `Mở tệp` path; preserve the current image viewer, review permissions/transitions, metadata composition and protected-tab fallback.
- Add responsive loading/error/retry/empty PDF states, revoke blob URLs on close/change and expose a secondary open-in-new-tab action after the protected PDF is ready.
- Add focused media-classification and presentation regressions, update durable frontend/project/construction maps, run frontend tests/build/bundle budget and diff checks; skip browser verification unless explicitly requested.

### Done

- Review package and Mockup as Text approved by the user.
- Current limitation confirmed from source and screenshot: protected PDFs open in a raw tab, the shared dialog has only photo/document surfaces, and `evidence_type=photo` currently overrides an `application/pdf` MIME value.
- Backend API, evidence storage, checksum, permission and review-state contracts confirmed unchanged; no migration is required.
- Added the pure shared media classifier so PDF MIME/filename takes precedence over the legacy evidence type, while normal image/document behavior and external-PDF file-first handling remain unchanged.
- Added the responsive native-browser PDF surface with loading, empty, render/download error, retry and accessible iframe title states.
- Added on-demand authenticated protected-PDF loading to the canonical record viewer, normalized the blob MIME when required, revoked object URLs on record change/close and exposed a ready-blob `Mở tab` action.
- Updated project/work-item evidence galleries and stored construction-report evidence options to label/open protected PDFs as `Xem PDF` in the same viewer without changing review actions or draft persistence.
- Added focused media-classification/viewer regressions and updated the central/frontend/project/construction durable maps; backend mapping remained unchanged because no endpoint, payload or persistence behavior changed.
- Verified the final source state with 177 frontend tests, `npm run build`, the 250 KiB bundle budget gate (124 chunks) and `git diff --check`; all passed. Browser/UAT verification was intentionally skipped under repository policy, so the UI remains `NOT UX ACCEPTED`.

## [Closed] Survey work-package draft/submit validation

### Will be done

- Make nested `client_survey_work_package.dimension_items.area` optional without mutating its published workflow-form definition or historical runtime references.
- Accept an omitted or 1–160 character `Khu vực`; keep the outer `Kích thước chính` list, `Kích thước` and `ĐVT` requirements unchanged.
- Allow draft saves to retain short incomplete text while submit/amend continues to enforce minimum lengths; keep schema keys, types, enums, maximum lengths and numeric ranges fail-closed in both modes.
- Preserve backend validation details in the shared API error and render exact workflow-field errors inline with a dismissible warning toast.
- Add focused backend/frontend regressions, update durable maps, run format, focused tests, the frontend build and diff checks; skip browser verification unless explicitly requested.

### Done

- Review package and Mockup as Text approved by the user.
- Root cause confirmed from the screenshot, backend 400 logs, persisted form schema and source: both draft and submit normalize text with minimum-length checks, while frontend structured-list validation checks required values only and the API client drops backend field details.
- Confirmed the scoped compatibility approach requires no migration and preserves published form versions, runtime instances, answer keys, PDF history and workflow transitions.
- Added the exact frontend/backend effective rule for `client_survey_work_package.dimension_items.area`: no required marker, blank or 1–160 characters accepted, while the outer list plus `dimension` and `unit` remain required and unrelated forms keep their published rules.
- Split text minimum-length enforcement by mutation mode: draft keeps short text after structural/type/maximum/range validation; submit/amend enforces minimum lengths at top level and inside structured lists.
- Added structured-cell submit validation and exact inline messages for required/min/max/type/range/enum/date issues on desktop and mobile.
- Preserved response-envelope validation details in `ApiError`, mapped backend `details.fields` into workflow-form errors and used the existing dismissible warning toast for save/submit/amend validation failures.
- Added focused backend/frontend regressions and updated the central, frontend, backend, root-cause, workflow-orchestration and planning maps.
- Technically verified the working tree based on `fe369acb0d4b5ae0dcba5ebffa7b2d0103427962`: 181/181 frontend tests passed, the TypeScript/Vite production build passed, the 250 KiB bundle budget passed with 124 chunks, `go test ./...` passed and `git diff --check` passed. `CONSFLOW_TEST_DATABASE_URL` is not configured, so PostgreSQL-backed integration cases remain skipped by their existing harness. No migration was added or run; browser/UAT was skipped under repository policy, so the UI remains `NOT UX ACCEPTED`.

## [Closed] Completion evidence guidance

### Will be done

- Keep backend `completion_requirements` and `details.missing_evidence` authoritative when a work-item completion action is rejected.
- For the shared `Hoàn tất` action, route a missing photo/document requirement to the existing evidence surface, open the correctly typed uploader, then scroll, focus and highlight it.
- Put `Minh chứng` before `Trao đổi & lịch sử` and make it the default supporting tab whenever the evidence tab exists; preserve primary-evidence surfaces without duplicating the tab.
- Keep unrelated work-item errors, workflow transitions, evidence gates, permissions, API contracts and persistence unchanged.
- Add focused regressions, update durable work-item/frontend/root-cause/workflow maps, run focused frontend tests, the frontend build and diff checks; skip browser verification unless explicitly requested.

### Done

- Review package and both Mockup as Text revisions approved by the user.
- Root cause confirmed: the backend returns `details.missing_evidence`, the shared API client preserves those details and evidence requirement navigation already works, but the work-item action error handler only emits a toast.
- Added a shared error-to-requirement mapper that treats backend `details.missing_evidence` as authoritative and reuses the existing evidence navigation target, including uploader type, open, scroll, focus and highlight behavior.
- Connected only the shared `complete` action error path to that mapper; unrelated work-item actions and errors keep the existing toast-only behavior.
- Reordered the shared supporting tabs so `Minh chứng` is first and selected by default whenever it exists; primary-evidence layouts continue to avoid a duplicate evidence tab and fall back to their first available tab.
- Added focused regressions for stale client requirement state, unrelated errors, completion-error dispatch and evidence-first/default tab layout.
- Updated the central, frontend, root-cause and workflow-orchestration maps. Backend, API contract, persistence and database mappings were unchanged because this implementation adds no backend, payload, storage or migration behavior.
- Technically verified the working tree based on `fe369acb0d4b5ae0dcba5ebffa7b2d0103427962`: 184/184 frontend tests passed, the TypeScript/Vite production build passed, the repeatable completion-navigation signal passed and `git diff --check` passed. Backend tests and migrations were not run because this change is frontend-only; browser/UAT was skipped under repository policy, so the UI remains `NOT UX ACCEPTED`.

## [Closed] Automatic evidence file classification

### Will be done

- Automatically classify selected work-item evidence as Image, PDF, Word or Excel across the shared work-item, Operations and Acceptance upload surfaces.
- Support PNG/JPG/JPEG/WebP/GIF, PDF, DOC/DOCX and XLS/XLSX up to the existing 10 MB limit; keep SVG, HEIC/TIFF and macro-enabled Office files outside this scope.
- Keep workflow evidence semantics unchanged: images persist as `photo`; PDF/Word/Excel persist as `document`, so completion requirements and report photo gates remain fail-closed.
- Verify file bytes/container structure on the backend rather than trusting the browser MIME or filename; preserve protected storage, checksum, permissions, review history and download behavior.
- Show the detected file kind as read-only presentation, keep images/PDFs on their current viewers, and open Word/Excel through the existing protected document action.
- Add focused frontend/backend regressions, update durable central/frontend/backend/root-cause/workflow/construction maps, run frontend/backend tests, build and diff checks; skip browser verification unless explicitly requested.

### Done

- Review package, impact matrix and Mockup as Text approved by the user.
- Root cause confirmed with a repeatable failing signal: the work-item panel does not derive a type after file selection, the shared capture helper recognizes only image/PDF, and backend upload validation whitelists only PNG/JPG/WebP/PDF.
- Database/API compatibility confirmed: `evidence_records.evidence_type=document|photo` and existing MIME/original-filename fields can represent all approved formats without a migration or response-contract change.
- Added the pure shared file classifier and immutable chooser contract for Image, PDF, Word and Excel. PNG/JPG/JPEG/WebP/GIF maps to `photo`; PDF/DOC/DOCX/XLS/XLSX maps to `document`; browser filename fallback is limited to missing or generic MIME values.
- Replaced the work-item upload type dropdown with the detected read-only kind, added inline/toast unsupported-file feedback and kept a photo-requirement warning when the selected document cannot satisfy `Hình ảnh`.
- Reused the same classifier, accepted formats and visible kind labels in the Operations/Acceptance evidence queue while preserving real/system camera capture and the direct construction-report `evidenceType=photo` gate.
- Added backend magic-byte/container verification before protected storage: image/PDF sniffing, DOCX/XLSX ZIP main-part validation, DOC/XLS OLE signatures, macro/unsupported/spoof rejection, rewind, normalized MIME/extension and existing requested-type mismatch protection.
- Kept protected download/view behavior unchanged: images retain preview, PDF retains the in-app reader, and Word/Excel use the existing permissioned `Mở tệp` document path. Size, SHA-256, original filename, visibility, review, RBAC and audit contracts remain unchanged.
- Added pure frontend/backend file-kind regressions plus PostgreSQL-backed DOCX/XLSX storage/checksum/download assertions, and updated central/frontend/backend/root-cause/workflow/construction durable maps.
- Technically verified the working tree based on `fe369acb0d4b5ae0dcba5ebffa7b2d0103427962`: 188/188 frontend tests passed, `go test ./...` passed, the TypeScript/Vite production build passed, the 250 KiB bundle budget passed with 124 chunks, and `git diff --check` passed. `CONSFLOW_TEST_DATABASE_URL` is not configured, so the PostgreSQL-backed upload/download assertions remain skipped by the existing harness. No migration was added or run; browser/UAT was skipped under repository policy, so the UI remains `NOT UX ACCEPTED`.

## [Closed] Internal-only evidence visibility

### Will be done

- Remove the evidence-upload visibility chooser from the shared work-item surface and keep the responsive upload layout dense.
- Make work-item, Operations daily-report and Acceptance uploads submit `internal` visibility.
- Normalize every newly created evidence record to `internal` on the backend even when a caller requests `project`, `client` or another value.
- Preserve the existing database column, historical visibility values, evidence review/storage/checksum/RBAC behavior and activity visibility contract; add no migration.
- Add focused frontend/backend regressions, update durable evidence maps, run frontend tests/build, backend tests and diff checks; skip browser verification unless explicitly requested.

### Done

- Review package, impact matrix and Mockup as Text approved by the user.
- Confirmed from source that the work-item chooser defaulted to `project`, shared Operations/Acceptance capture sent `project`, and the backend accepted explicit `internal|project|client` despite the database default being `internal`.
- Confirmed the approved scope preserves historical evidence rows and keeps activity/message visibility independent.
- Removed the work-item evidence visibility state/control, reduced the responsive uploader to four fields and made both work-item and shared Operations/Acceptance capture submit `internal`.
- Made `validateWorkItemEvidenceRequestForStorage` normalize every new metadata/upload request to `internal`, including callers that request `project`, `client` or an unsupported value; retained the database column/check and every historical row unchanged.
- Added focused frontend source-contract and backend normalization regressions, and updated the central/frontend/backend/workflow/construction/acceptance durable maps.
- Technically verified the working tree based on `fe369acb0d4b5ae0dcba5ebffa7b2d0103427962`: 189/189 frontend tests passed, `go test ./...` passed, the TypeScript/Vite production build passed, the 250 KiB bundle budget passed with 124 chunks, and `git diff --check` passed. `CONSFLOW_TEST_DATABASE_URL` is not configured, so PostgreSQL-backed integration cases remain skipped by their existing harness. No migration was added or run; browser/UAT was skipped under repository policy, so the UI remains `NOT UX ACCEPTED`.

## [Closed] Multi-file evidence upload progress

### Will be done

- Replace the one-file work-item evidence uploader with an explicit row queue and `[+ Thêm]`, capped at 10 files per batch.
- Upload at most three files concurrently through the existing one-file endpoint; after settlement clear every successful row, reset an all-success batch to one blank row, and retain only failed rows for retry without rollback.
- Show per-row byte progress, a weighted overall percentage and distinct queued, uploading, server-processing, successful and failed states.
- Keep the approved Image/PDF/Word/Excel classifier, 10 MB file limit, photo-requirement warning and backend-normalized `internal` visibility unchanged.
- Refresh evidence queries once after each batch, add focused queue/progress regressions, update durable evidence maps, run frontend tests/build/bundle budget and diff checks; skip browser verification unless explicitly requested.

### Done

- Review package, impact analysis and responsive Mockup as Text approved by the user.
- Confirmed the current work-item UI owns one selected file and one mutation, while the existing backend endpoint accepts one multipart file per request. The approved implementation reuses that endpoint per row and needs no API, persistence or migration change.
- Added the responsive row uploader with explicit file/type/name/caption/progress columns on wide screens and labelled row cards below `xl`; `[+ Thêm]` focuses the new chooser and the queue stays capped at 10 rows.
- Added the pure concurrency helper and authenticated XHR multipart transport. A batch sends at most three requests at once, keeps shared refresh-token/error-envelope behavior, reports per-row byte progress and shows `100% · Đang xử lý` until the server response succeeds.
- Implemented fail-open batch continuation with fail-closed rows: accepted records are never rolled back, an all-success batch resets to one blank row, a partial batch removes successful rows and retains only failed metadata/messages for retry, and an all-failed batch retains every failed row.
- Preserved the existing 10 MB and Image/PDF/Word/Excel validation, photo-requirement warning, one-file backend endpoint, protected storage/review/audit semantics and backend-normalized `internal` visibility.
- Added focused queue, weighted-progress, concurrency, partial-result, UI-state and authenticated-progress source regressions; updated the central/frontend/workflow/construction durable maps. Backend and database maps were unchanged because no route, payload, persistence, migration or server behavior changed.
- Technically verified the working tree based on `fe369acb0d4b5ae0dcba5ebffa7b2d0103427962`: 195/195 frontend tests passed, the TypeScript/Vite production build passed, the 250 KiB bundle budget passed with 124 chunks, and `git diff --check` passed before this documentation-only completion note. Backend tests and migrations were not run because the implementation is frontend-only; browser/UAT was skipped under repository policy, so the UI remains `NOT UX ACCEPTED`.
- Follow-up review package and Mockup as Text approved: a fully successful batch must show `Đã tải xong N tệp` and reset to one blank row; a partial batch must clear successful rows and retain only failed rows; an all-failed batch retains every failed row.
- Added the pure failed-row selector plus regression coverage for all-success and partial settlement, updated the toast wording to `Đã tải xong`, and synchronized the central/frontend/workflow durable mappings with the cleanup contract.
