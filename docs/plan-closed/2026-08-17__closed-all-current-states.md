# [Closed] All Current States

Closed: 2026-08-17
Reason: User requested closing all current state entries.

## [Closed] Golden Rev6 Lean Runtime Sample

### Will be done

- Add the additive/idempotent `GOLDEN-REV6-LEAN-001` project/runtime fixture on immutable official Rev6 without rebinding Batch3 or any historical Golden runtime.
- Keep the project at `Khảo sát` with the Rev6 intake form truthfully submitted as `entry_route=survey_required`, one completed intake work item, six ready survey work items, and every later work item pending.
- Preserve exact runtime template/form snapshots: 15 nodes, 51 steps/work items, 50 form instances, and 43 `lean_capture_v2` form instances with none of the five removed universal fields.
- Wire the fixture into both local/dev seed paths, extend Golden Sample validation and mappings, apply only the additive fixture to the current local database, and run non-browser technical verification.

### Done

- Matched the request to Database / Golden Sample, Planning / Pipeline Run, Workflow Forms and Workflow Orchestration; loaded the central, backend, workflow-orchestration, project-planning and database workflow maps.
- Confirmed local Rev6 `40000000-0000-0000-0000-000000000025` is published with 15 nodes, 51 steps, 50 form-backed steps and 43 lean-form step references; the proposed project/run UUIDs ending in `026` and code `GOLDEN-REV6-LEAN-001` are unused.
- Confirmed Batch3 remains an independent Rev5/engine-4 runtime at `survey`; existing Golden seed coverage stops at Rev4.
- Produced the seed review package and received explicit `OK triển khai` approval for the survey-state fixture, stable UUIDs/code, Make seed parity and direct additive application to the current local database.
- Added idempotent `backend/seed/golden_client_workflow_rev6_lean.sql`. It creates project/run `...026` on exact published Rev6/hash, records the submitted `survey_required` intake answer and event/audit history, completes only intake, opens all six Survey work items, and leaves 44 later work items pending.
- Wired the fixture into both `backend/Makefile` seed paths and extended `validate_golden_project.sql` to assert the exact project/run/current-node contract, member parity, 15 nodes, 51 steps/work items, exact form-version snapshots, 49 draft plus one submitted form, 43 lean instances and zero removed universal fields.
- Updated the central, backend, workflow-orchestration and project-planning maps with the canonical Rev6 sample identity, owner file, state/count contract and preservation boundaries.
- Applied only the additive fixture to the current local database. A second application preserved it without duplicates. Local verification confirms Rev6/engine 4 at Survey, 15 nodes, 51 steps, work items `1 completed + 6 ready + 44 pending`, forms `49 draft + 1 submitted`, 43 lean instances and zero universal fields; `survey.dimensions` is ready on `lean_capture_v2`.
- Created a disposable database from the current working tree, applied all 102 migrations, ran the complete `make seed` sequence and observed `GOLDEN_SAMPLE_VALIDATION_PASSED`; the temporary database was removed. Golden fixture manifest validation and `git diff --check` pass; local backend health/readiness both return HTTP 200. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.

## [Closed] Lean Workflow Form Capture

### Will be done

- Publish immutable V2 versions of the 45 official client business forms without the universal `scope_summary`, `work_result`, `linked_evidence_id`, `next_action`, and `confirmed` fields; retain only step-specific business inputs.
- Give the two invoice-only forms a required status selector and require invoice details only when the invoice is actually issued or received.
- Publish official workflow Revision 6 for future/not-started projects, remapping only the 45 business forms to V2 while preserving the Rev5 intake form, design branch decision form, built-in forms, and every historical/runtime reference.
- Keep evidence in the canonical work-item evidence surface, keep follow-up work in real work items/activities/blockers, and treat the submit action as the explicit confirmation.
- Add the approved fixed mobile `Lưu nháp` / `Chốt hồ sơ` action bar, focused regressions and required Consflow mapping updates, then run non-browser technical verification.

### Done

- Matched the request to Workflow Forms / Work Items / Workflow Orchestration and loaded the central, frontend, backend, workflow-orchestration and impact-analysis maps plus the database migration checklist.
- Confirmed V41 cross-joins five generic fields into 45 client forms (225 declarations, including 135 required declarations), while current code has no targeted business consumer for those field keys beyond generic storage/PDF rendering.
- Confirmed workflow submission already preserves actor, timestamp, revision, snapshot, workflow event, work-item activity and audit metadata; evidence and completion remain governed by the canonical completion-requirements contract.
- Identified V102 as the next migration after the existing uncommitted V101 work and reserved unused Rev6 template/node/step/edge UUID namespaces without modifying the active edits.
- Produced the lean-form review package and Mockup as Text, then received explicit `OK triển khai theo review package này` approval.
- Added V102 with 45 immutable `lean_capture_v2` form definitions. The five universal fields are absent, all forms retain at least one effective required input, and the two invoice forms use a required status plus conditional invoice number and semantic VND currency amount.
- Published immutable Rev6 `40000000-0000-0000-0000-000000000025` by cloning Rev5's 15 nodes, 51 steps and Connector semantics; 43 applicable step occurrences now reference lean forms while intake, branch-decision, built-in forms, Rev5 and every existing runtime/history reference remain unchanged.
- Updated local/dev seed parity to restore V41 business-form mappings before cloning later revisions, then reapply V92 and V102. A fresh temporary PostgreSQL database passed the complete seed/golden validation and retained one published Rev6 template, 45 lean forms and 43 Rev6 lean step references; the database was removed.
- Added the fixed narrow-screen `Lưu nháp` / contextual submit bar using the same form action state and callbacks as the desktop rail, with safe-area padding, responsive focus targeting and no action bar for submitted/upcoming/read-only forms.
- Added focused PostgreSQL migration and frontend source regressions and updated the central, backend, frontend, workflow, planning and accounting mappings.
- Technically verified V102 on a fresh isolated PostgreSQL schema, including idempotent raw reapplication, immutable Rev5 references, conditional invoice rules and Rev6 remapping. The focused migration test and the migration suite excluding the pre-existing V26 reconstruction test pass; fresh-DB core/backend packages pass.
- Frontend verification on the current source passes all 123 tests and the TypeScript/Vite production build. `migrate-check` recognizes all 102 migrations and `git diff --check` passes. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
- Rebuilt and restarted the localhost stack from the current working tree. PostgreSQL, Redis and Redis Cache are healthy; backend/admin are up on ports 8080/5173, storage preflight passed, startup skipped all 102 checksum-matching migrations, and `/api/health`, `/api/ready` plus the admin origin returned HTTP 200.
- Verified the current Batch3 runtime remains bound to Rev5 with its exact V1 `client_site_measurement_record` instance, proving the rollout does not silently rewrite an in-progress project's form. New/not-started Rev6 runs receive the lean forms.

## [Closed] Amend Submitted Workflow Form

### Will be done

- Allow an authorized actor to amend a submitted workflow-form instance from work-item detail through a server-provided capability and a new atomic amendment endpoint.
- Require an amendment reason, optimistic revision check and non-empty field diff; persist the new revision plus workflow event, work-item activity and audit metadata in one transaction.
- Keep the completed work item, pipeline node/run and prior downstream automation unchanged; preserve prior signatures, seal applications and PDF exports as immutable revision history and require revision-specific confirmations for future PDFs.
- Add the approved `Chỉnh sửa` action above `Mở công trình`, an edit-reason dialog, inline amendment mode, validation/toasts and readable before/after history.
- Update backend/frontend/workflow/admin mapping, add focused regression coverage, and run non-browser backend/frontend technical verification.

### Done

- Matched the request to Work Items / Workflow Form Instance / Workflow Orchestration and loaded the central, backend, frontend, root-cause, workflow and admin-governance maps.
- Confirmed the current frontend and backend both lock `submitted` form instances; the schema has partial reopen vocabulary but no amendment route/service/repository flow, and current workflow-form logs do not capture changed fields.
- Traced submission side effects and confirmed an amendment must not reuse normal submit orchestration because it can duplicate generated downstream work and rerun auto-advance/branch behavior.
- Produced the impact review and Mockup as Text, then received explicit `OK triển khai` approval for revisioned amendment, unchanged runtime completion state, server-authoritative RBAC and immutable historical confirmations/PDFs.
- Added server-owned `can_amend` capability and `POST /workflow-form-instances/:id/amend`. The service requires assigned work-item action scope, the snapshotted required permission, a mandatory reason, exact expected revision, valid submitted answers and a non-empty diff.
- Added the atomic repository amendment: it locks the submitted instance, stores labeled before/after values, increments the form revision, synchronizes the work-item answer snapshot and writes workflow event, work-item activity and audit in one transaction without changing work-item/node/run state or reprocessing downstream work.
- Added V101 revision-scoped signature/seal uniqueness and current-revision confirmation/PDF selection. Historical signatures, seals and PDF exports remain visible and immutable; prior-revision PDFs cannot be deleted.
- Added the approved right-rail `Chỉnh sửa` action above `Mở công trình`, mandatory-reason dialog, editable amendment mode, unchanged form validation, no-op/stale warnings, save/cancel controls, revision-aware confirmation/PDF history and exact before/after rendering under the workflow-form history filter.
- Updated central/backend/frontend/root-cause/workflow/admin mappings and added focused frontend plus isolated-PostgreSQL amendment regressions. The database test proved completed work stays completed, revision 2 accepts new confirmation evidence, old PDF deletion fails, new PDF selects only the current revision, and stale/no-op amendments fail; its temporary database was removed.
- Technically verified the current source with the 101-migration checksum check, full `go test ./...`, all 121 frontend tests, the TypeScript/Vite production build and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
- Rebuilt the localhost images and applied V101 to the main local database. A concurrently added V102 migration then failed its independent `expected 46 Rev6 step occurrences ... found 43` assertion, so its source was preserved and the backend was restored with `AUTO_MIGRATE=false`; admin/backend/PostgreSQL/Redis are running and `/api/health` plus `/api/ready` return HTTP 200. Default auto-migrating restart remains blocked by V102 until that separate active plan fixes it.
- The separate Lean Workflow Form Capture plan corrected V102's source-backed expectation to 43 applicable occurrences, added its regression/seed coverage and applied V102 successfully; the earlier auto-migration blocker is resolved.

## [Closed] Simplify Site Risk Capture

### Will be done

- Remove the `Nhóm` editor/read-only presentation from `client_site_risk_assessment.risk_items` on desktop and mobile without mutating the published workflow-form definition.
- Default a missing per-row `category` to `other` server-side before workflow-form validation and persistence, while preserving any existing explicit category value.
- Omit the hidden category from newly generated workflow-form PDFs, preserve existing immutable PDF artifacts and historical answer data, and add focused backend/frontend regression coverage.
- Update the central, frontend, backend, workflow-orchestration and project-planning maps, then run non-browser technical verification.

### Done

- Confirmed from V41 and current source that `category` is a required nested select rendered generically from `validation_json.item_fields`, rejected when absent, and persisted inside the `risk_items` JSON answer rather than a dedicated database column.
- Confirmed targeted searches find no assignment, transition, severity, report or other downstream business logic that consumes `risk_items.category`.
- Produced the review package and Mockup as Text, then received explicit `OK triển khai` approval for the scoped UI, server-default and new-PDF behavior while preserving historical values/artifacts.
- Added a non-mutating effective structured-list definition for exact form `client_site_risk_assessment`, removing `category` from editable, mobile and read-only presentation while leaving the published V1 schema untouched.
- Added server-owned answer defaults before generic validation: a missing/blank site-risk category becomes `other`, while an explicit existing category such as `mep` remains unchanged. The value continues to persist inside the existing `risk_items` JSON answer, so no migration or API-shape change was required.
- Removed the category from newly generated site-risk PDFs without changing stored answers, immutable payload hashing or existing PDF artifacts; visually rendered the focused PDF and confirmed the three remaining fields are aligned, legible and unclipped.
- Added focused frontend, backend normalization, PostgreSQL persistence and PDF regressions; updated central/frontend/backend/workflow/planning mappings. Browser/UAT remains intentionally skipped under the repository rule.
- Technically verified the current working tree with full `go test ./...`, all 118 frontend tests, the TypeScript/Vite production build and `git diff --check`. The persistence regression passed against a fresh isolated PostgreSQL database after all 100 migrations; that temporary database was removed. UI remains `NOT UX ACCEPTED` because Browser/UAT was not requested.
- Rebuilt the localhost stack from the verified working tree through `make restart`. PostgreSQL, Redis and Redis Cache are healthy; backend and admin are up on ports 8080/5173, startup skipped all 100 already-applied migrations, storage preflight passed, and `/api/health` plus `/api/ready` returned HTTP 200.

## [Closed] Project Explorer Default Collapsed

### Will be done

- Keep every `/projects` explorer row collapsed on initial load and whenever a new loaded page has no still-visible manually expanded project.
- Preserve the existing manual multi-row expand/collapse interaction and retain manually expanded project IDs across authoritative refetches while those IDs remain visible.
- Add focused frontend regression coverage, update the frontend/project-planning maps, and run non-browser frontend tests/build.

### Done

- Confirmed from the supplied Edge screenshot and current source that the explorer effect automatically expands `rows[0].id` whenever no expanded project remains visible.
- Produced the review package and Mockup as Text, then received explicit `OK triển khai` approval for the default-collapsed behavior across initial load, search, filter and pagination changes.
- Removed the first-row fallback while preserving project-ID-based manual multi-row expansion and pruning only IDs that disappear from the authoritative loaded page.
- Added focused source-contract regression coverage and updated the frontend/project-planning maps for the new user-controlled default-collapsed contract. Backend/API/database behavior remains unchanged.
- Technically verified the current frontend source with the focused explorer test (3/3), the complete frontend suite (105/105), the TypeScript/Vite production build and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.

## [Closed] Actionable Work Item Completion Requirements

### Will be done

- Turn work-item completion requirements into keyboard-accessible actions that navigate to the exact workflow-form, evidence, or action surface when an on-screen target exists.
- For photo/document requirements, open the existing evidence uploader with the required type selected and focus the file input; for workflow forms/checklists, focus the first incomplete field or the form submit action.
- Show explicit resolution guidance instead of a fake action when a requirement has no direct field or section, including permission, automation, approval, and unknown future requirement states.
- Preserve backend `completion_requirements` as the only completion-gate contract; add focused frontend coverage, update Consflow mapping, and run non-browser frontend tests/build.

### Done

- Confirmed from the supplied Edge screenshots and current source that requirement rows are static `div` elements with generic descriptions and no navigation/focus target.
- Confirmed the backend already returns canonical `workflow_form`, `checklist`, `document`, and `photo` keys; no API, schema, migration, or backend completion-gate change is required.
- Produced the review package and Mockup as Text, then received explicit `OK triển khai` approval.
- Added a typed page-local navigation request and keyboard-accessible full-row actions. Form/checklist requirements focus and temporarily highlight the exact checklist/first-invalid field or the form submit/source fallback; photo/document requirements open or view the evidence surface with the required upload type selected; blocker/automation/approval gates focus `Việc cần làm` only when the actor has a matching action.
- Added explicit `Cách xử lý` guidance to every row. Permission-only, completion-payload-only, unknown, and unavailable-form requirements remain non-clickable instead of advertising a target that does not exist.
- Added focused presentation/wiring regressions and updated the central, frontend, and workflow-orchestration maps. Backend/API/database behavior remains unchanged.
- Frontend verification on the current working tree: `npm test` and `npm run build` pass; browser/UAT verification was not run under the repository rule, so the UI remains `NOT UX ACCEPTED` until a separate browser acceptance pass.

## [Closed] Workflow Form PDF Open And Delete

### Will be done

- Replace the duplicated protected-blob popup flow with one shared helper that retains exactly one user-opened tab, detaches its opener, loads the authenticated blob and navigates that tab without a second popup.
- Add an audited, retryable workflow-form PDF deletion lifecycle that preserves immutable export metadata, physically deletes the unreferenced Local/S3 object and blocks deletion while an outgoing acceptance references the export.
- Exclude completed deletions from form detail, protected download and PDF-derived counts; expose server-backed delete readiness to the work-item UI.
- Add the approved responsive PDF row actions and shared confirmation dialog states, focused backend/frontend regression coverage and required Consflow mapping updates.
- Run non-browser technical verification, including current-source tests/build, migration validation, API deletion checks and visual rendering of a generated PDF. Browser/UAT remains skipped unless explicitly requested.

### Done

- Reproduced the blank-open defect from source and the supplied Edge state: the first `noopener` tab may return a null handle, then a second `window.open` runs after the blob await and is popup-blocked, leaving the original tab blank.
- Verified the protected PDF endpoint independently returns HTTP 200, `application/pdf`, 1,829 bytes and `%PDF-1.4`; rendered the retrieved page and confirmed the workflow-form content is present.
- Confirmed there is no current delete UI/API, the export row is database-immutable, and outgoing acceptance can reference `workflow_form_pdf_exports.id`.
- Confirmed all three duplicate exports on the reported work item have the same checksum and currently have no outgoing-acceptance references.
- Produced the impact review, safe-delete contract and Mockup as Text; received explicit `OK triển khai` approval.
- Added V93 with a separate `workflow_form_pdf_export_deletions` lifecycle so immutable export metadata remains intact. Deferred database guards reject both deleting a referenced PDF and creating an acceptance reference to a pending/deleted PDF.
- Added protected `DELETE /workflow-form-pdf-exports/:id` with the same work-item action/current-node/permission gates as generation. Local/S3 object deletion is retryable, storage failures remain visible as `failed`, success writes one workflow event, work-item activity and audit row, and repeat deletion is idempotent.
- Updated active form/detail/report queries and outgoing-acceptance options: completed deletions are hidden, pending deletion cannot be downloaded/linked, failed deletion remains retryable, and the UI receives server-backed reference count/delete readiness.
- Added `Xoá`/`Thử xoá lại`, linked-record disabled guidance and a shared confirmation dialog to the PDF rows. Read-only work keeps deletion disabled.
- Added `openProtectedBlob.ts` and moved workflow PDF/signature plus evidence consumers to exactly one synchronous viewer; the helper explicitly reports popup blocking and never opens a second window after `await`.
- Updated backend/frontend/root-cause/workflow/quality mappings for the route, V93 lifecycle, acceptance invariant and shared protected-viewer contract.
- Technically verified the current source with full `go test ./...`, all 76 frontend tests, TypeScript/Vite production build, `git diff --check`, and a fresh PostgreSQL migration/integration run. The integration covers failed-storage retry, physical object deletion, retained immutable metadata/tombstone/audit, idempotent repeat deletion and acceptance-reference conflict. The temporary test databases were removed. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
- Rebuilt the localhost Compose stack from the verified source. V93 is recorded as `workflow form pdf export deletion`; the final backend startup skipped all 93 already-applied migrations, passed `storage_preflight provider=local`, and both `/api/health` and `/api/ready` returned HTTP 200.

## [Closed] External Storage Boundary Redesign

### Will be done

- Replace the implicit container-local upload directory with an explicit storage contract: blank/default provider uses `STORAGE_PATH`, while `s3` uses only the configured S3 backend.
- Keep the development filesystem default under the repository at `storage/`, bind it into Docker without a Docker-managed volume, and ignore it from Git.
- Remove temporary filesystem staging from S3 writes and prevent multipart uploads from spilling into the container filesystem.
- Make production Compose and the Kubernetes runtime S3-only, while retaining an explicit external filesystem source only for the one-time storage migration command.
- Preserve the existing Docker volume and the 35 database references to provider `local`; do not delete or rewrite legacy data automatically.
- Add focused backend/storage/config regression coverage, update deployment validation and Consflow mapping, then run non-browser technical verification.

### Done

- Confirmed the PDF failure is an API 500 caused by the backend process lacking write permission on the Docker-managed `/app/uploads` volume, not an RBAC/API permission denial.
- Confirmed generated workflow PDFs are produced in memory before storage, so PDF generation does not require a container-local temporary file.
- Confirmed S3 writes currently use `os.CreateTemp` and Fiber multipart parsing can spill large uploads to the process temporary directory; both paths are included in the approved redesign.
- Confirmed the current database has 35 `local` storage references across workflow evidence/signature/PDF and invoice records; the approved cutover preserves those records and the old volume for an explicit migration instead of deleting either.
- Produced the storage/runtime/deployment review package and received explicit `OK triển khai` approval.
- Replaced `UPLOAD_DIR` with the fail-closed storage contract: empty/`default` resolves to persisted provider `local` and requires `STORAGE_PATH`; production requires `s3`, whose runtime does not create/register local storage.
- Removed implicit service-local storage fallback, made startup probe every active provider, rewound seekable multipart sources after MIME detection and configured Fiber to keep bounded multipart bodies in memory instead of process temp files.
- Removed S3 `os.CreateTemp` staging; S3 now hashes and size-checks the seekable source, rewinds it and streams it to the AWS SDK while preserving bounded retries.
- Moved development storage to ignored repo-root `storage/`, bind-mounted it at `/storage` under the host UID/GID, removed the Docker upload named volume from runtime configuration and changed seed fixture copy to write directly on the host.
- Made production Compose and Kubernetes application runtime S3-only with no object-storage mount; retained an explicit external read-only `STORAGE_PATH` only for the operator migration Job/binary.
- Exported the 17 legacy volume objects into `storage/` with matching SHA-256 for all 17 files. The original `ttcons_consflow_uploads` volume remains intact with 17 files and is not mounted by the new backend.
- Rebuilt the local stack with revision `60824c0` plus the current working tree. Backend runs as host `501:20`, mounts only the host bind at `/storage`, and logged `storage_preflight provider=local status=ready` before listening.
- Re-ran the originally failing workflow-form PDF action through the API for instance `12a97195-ea63-456e-a38f-9941bd605a6f`; it returned HTTP 201 and created a 1,829-byte ignored host file owned by `khiemnguyen:staff` with mode `0640`. This added one legitimate PDF-export record; the 35 pre-existing local metadata references were preserved.
- Added focused config/runtime/S3/multipart regressions and updated central/backend/root-cause/workflow mapping plus deployment/runbook documentation. Full `go test ./...`, development and production Compose config validation, Kubernetes deterministic render validation and `git diff --check` pass. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
- Added the ignored local `.env` and repository template `.env.example`; root and backend Makefiles now load that file explicitly, local Compose consumes its service/runtime variables without hidden storage fallbacks, and production remains S3-only through `.env.prod.example`.
- Preserved the existing Compose project name `ttcons` so local PostgreSQL continues using `ttcons_consflow_postgres`. The first restart exposed and corrected both an accidental project-name mismatch and a partially configured S3 region under the default provider; the unstarted temporary `consflow` containers/network and their unused empty PostgreSQL volume were removed without touching the `ttcons` volume.
- Final `.env` verification passed with mode `0600`, exact key parity against `.env.example`, local/default and production/S3 Compose render checks, and a rebuilt `ttcons` stack. PostgreSQL/Redis are healthy, backend/admin are up, storage preflight reports `provider=local status=ready`, all 92 migrations are already applied, backend runs as `501:20`, and its sole object mount is the host bind `storage/ -> /storage`.

## [Closed] Dashboard Approval Ownership Clarity

### Will be done

- Rename the Dashboard pending-approval panel so it reads as a scoped management queue rather than personal work.
- Show the requester's name and the current required approval roles on every row, with truthful fallbacks for incomplete legacy/test records.
- Keep server `capabilities` and `available_actions` authoritative; show an explicit read-only state when the actor cannot decide.
- Add focused frontend regression coverage, update central/frontend/workflow/reporting mapping, and run non-browser frontend verification.

### Done

- Confirmed from the supplied Edge screenshot and current source that Dashboard loads actor-scoped `/approvals/pending` but omits the existing `requested_by_name` and `decision_context.required_roles` fields.
- Confirmed the backend API, approval policy, requester separation, project scope and decision authorization do not need to change.
- Built the repeatable source-level signal `CURRENT_BUG_REPRODUCED` and confirmed the existing approval presentation tests pass before implementation.
- Produced the review package and Mockup as Text, then received explicit `Duyệt làm đi` approval for the scoped UI fix, fallback wording, unchanged RBAC and unchanged fixture data.
- Renamed the Dashboard panel to `Yêu cầu đang chờ phê duyệt`, clarified its management scope, and added requester/current-role context with explicit missing-metadata fallbacks on every row.
- Kept `/approvals/pending`, server `capabilities` and `available_actions` unchanged; rows without decision capability now show `Chỉ theo dõi` while source navigation remains available.
- Added shared requester/role presentation helpers, focused helper/source-contract regression coverage, and updated central/frontend/workflow/reporting mapping for the clarified Dashboard contract.
- Technically verified the current frontend source with all 64 tests, TypeScript/Vite production build, bundle budget (121.5 KiB gzip eager entry, 109 JavaScript chunks under the 250 KiB ceiling) and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.

## [Closed] Responsive Shell Branding And Mobile Navigation

### Will be done

- Reuse the approved Thủ Thiêm Workflow brand lockup at the top of the Pipeline Builder and project-workspace desktop sidebars, with the same permission-aware `/` home behavior as the standard AppShell.
- Add an accessible mobile navigation drawer to the standard AppShell so the permission-filtered `Công việc`, `Workflow`, and `Quản trị` groups remain reachable below the desktop breakpoint.
- Preserve the existing mobile `Mở cây nghiệp vụ` / `Mở cây quy trình` project control and keep Pipeline Builder canvas responsiveness outside this scoped change.
- Add focused regression coverage, update central/frontend/root-cause mapping, and run non-browser frontend verification. Browser/UAT remains skipped unless explicitly requested in the same turn.

### Done

- Confirmed from the supplied 430×932 Edge screenshot and current source that the AppShell desktop sidebar is hidden below `lg` without any mobile menu trigger or replacement navigation.
- Confirmed Project Workspace already exposes a mobile tree toggle, while Pipeline Builder has a fixed desktop-first template sidebar; these are separate from the AppShell navigation defect.
- Produced the evidence-backed review package and Mockup as Text, then received explicit `OK triển khai` for the combined branding and mobile-navigation scope.
- Added shared `BrandHomeLink` behavior and reused it for standard AppShell, Pipeline Builder and project-workspace desktop brand rows plus compact AppShell/project mobile marks.
- Added an AppShell mobile hamburger and modal left drawer that reuses the desktop permission-filtered navigation, active-route states, user/logout context and powered-by footer; backdrop, close, Escape and navigation dismiss it, body scroll is locked and keyboard focus is contained/restored.
- Preserved the project mobile tree control, placed its return action alongside that control and aligned the desktop 320px sidebar brand row with a 64px project header. Pipeline Builder remains desktop-first beyond the approved brand addition.
- Added a focused failing-then-passing source-contract regression for mobile navigation and focus-sidebar branding; focused test and TypeScript no-emit check pass.
- Updated central/frontend/root-cause mapping for the shared brand-home owner, responsive AppShell navigation contract, focus-shell brand placement and mobile-menu triage path.
- Technically verified the current frontend source with all 42 tests, the production TypeScript/Vite build, the bundle budget (121.3 KiB gzip eager entry, 105 JavaScript chunks under the 250 KiB ceiling) and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.

## [Closed] Client Workflow Revision 4 And Definition PDF

### Will be done

- Publish immutable official Rev4 template `40000000-0000-0000-0000-000000000023` from Rev3 without mutating published Rev3 or existing project runtime snapshots.
- Add Purchasing to the BOQ-without-design pricing step, separate QA/QC quality-dossier ownership from Supervisor field-report ownership, and make the approved document/photo evidence gates explicit for future Rev4 runs.
- Keep optional client-node cross-reference data on the template definition and export it with node/step business codes plus checklist/document/photo flags; exclude runtime/UAT reminders, statuses and test results from the template PDF.
- Fix PDF connector-label placement so branch labels stay visually attached to their own route, then add focused regression coverage and visually inspect the rendered Rev4 PDF.
- Update destructive seed replay, Golden validation and the central/backend/workflow/commercial/construction/quality mapping files, then run focused and full backend/database verification.

### Done

- Audited the client notes against Rev3 PDF, source and local PostgreSQL data; confirmed Rev3 has 15 nodes, 16 connectors and 51 stable coded steps.
- Confirmed template `...022` is published and immutable, template `...023` is unused, and projects 020/021/022 already have historical runtime snapshots that must remain unchanged.
- Proved the BOQ/QA-QC/Supervisor role and evidence-flag gaps from V85 and the database, and proved the branch-edge data is correct while PDF label placement can drift away from the owning connector.
- Rendered the source PDF at high resolution and retracted the earlier false clipping finding: page 6 is complete, so no pagination/clipping change is in scope.
- Produced the cross-domain review package and approved Mockup as Text; received explicit `OK triển khai` for Rev4, report-only client mapping, runtime/UAT exclusion and the PDF label fix.
- Added V86/V87: published immutable Rev4 `...023`, preserved Rev3 and all existing runs, added report-only client references, corrected Purchasing/QS and QA/QC/Supervisor responsibility/evidence gates, and corrected the quality dossier from `daily_report` to `document_upload`.
- Extended template repository/clone/hash handling for report presentation metadata and extended the on-demand PDF with node/step business codes, evidence flags and Appendix A while excluding runtime/UAT content.
- Added a failing-then-passing connector-label regression; labels now try every own-route anchor before offsetting and use a leader line back to the owning connector when collision avoidance requires it.
- Updated Makefile replay, Golden validation and central/backend/workflow/commercial/construction/quality/root-cause maps. All 87 migrations plus destructive seed replay passed on an isolated database; the temporary database was removed.
- Applied V86/V87 to local PostgreSQL and verified Rev4 has 15 nodes, 16 edges, 51 steps and 14 client references while projects 020/021/022 retain their existing engine-2 run snapshots.
- Exported and visually inspected all 7 pages of `output/pdf/workflow-client-workflow-hoan-thien-cong-trinh-chinh-thuc-rev-4.pdf`; no clipping was observed, branch labels remain attached, Appendix A is complete and extracted text contains no runtime/UAT content.
- Full `go test ./...` including PostgreSQL-backed integration coverage passed on a separate disposable database; focused PDF/hash tests, migration manifest check and Golden fixture generator check also passed. Temporary test databases were removed. Browser/UAT was not run in this implementation turn.

## [Closed] Adaptive Role Workbench

### Will be done

- Make `Việc của tôi` the authenticated default landing page and retain the existing executive Dashboard under the user-facing label `Điều hành`.
- Turn `/my-work` into one adaptive role-aware workbench instead of separate duplicated role pages, using the actor-scoped work-item queue as the only task source of truth.
- Default to work the actor can perform now, expose explicit `Đang làm` and read-only `Sắp tới` lanes, and keep future-node work non-actionable through backend capabilities and normal `WorkItemAction` gates.
- Show the complete `Dự án › Node › Bước` context on every work card and selected-work preview while preserving the shared work-item detail dialog and its one-primary-action hierarchy.
- Add focused backend/frontend regression coverage, update the central/detail/domain mapping files, and run non-browser verification. Browser/UAT remains skipped unless explicitly requested in the same turn.

### Done

- Audited the existing Dashboard, HomeRedirect, My Work frontend, `/me/work-items` handler/service/repository contract, work-item capability logic, RBAC/project scope, shared work-item detail dialog, realtime invalidation and focused tests.
- Produced the cross-domain impact matrix, evidence-backed review package and approved Mockup as Text.
- Received explicit approval for one adaptive role workbench, `/my-work` as the default landing page, `Điều hành` as the existing Dashboard label, complete project/node/step context and read-only future work.
- Made authenticated root open `/my-work`, placed `Việc của tôi` first in global work navigation and retained the unchanged report dashboard under `Điều hành`.
- Added actor-scoped `Làm ngay`, `Đang làm`, `Chờ tôi duyệt`, `Bị chặn` and `Sắp tới · chỉ xem` buckets; default state is `all + need_action`, with secondary queue/status/deadline/type/priority/sort controls under `Bộ lọc thêm`.
- Added server `readiness`, node/step order context and source-backed `in_progress`/`upcoming` summary counts. Planned-future and engine-v2 future-node work stays visible but is excluded from current actions/decisions.
- Added shared complete `Dự án › Node › Bước` presentation to work cards, selected preview and detail; responsive layout puts the selected next-work panel first on narrow screens.
- Made future detail read-only across action, activity, evidence review/upload and workflow form edit/submit/sign/PDF controls, with matching backend current-time/current-node guards on work actions and mutation services.
- Added backend readiness/future/context tests and frontend context/default-route tests; updated central, frontend, backend, workflow, governance and reporting maps.
- Technically verified current working tree with backend `go test ./...`, frontend `npm test`, `npm run build`, bundle-budget check and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.

## [Closed] Client Workflow Revision 3 And Contract Execution

### Will be done

- No mandatory implementation or verification work remains. Keep this completed plan in active memory until the user explicitly asks to close it.

### Done

- Audited client notes against source, database data for projects 020/021/022 and current Browser behavior using Admin and Accounting.
- Produced and received explicit approval for the cross-domain review package, implementation phases, acceptance decisions and Mockup as Text.
- Confirmed the client workflow lineage: revision 1 is template `40000000-0000-0000-0000-000000000020`; published revision 2 is immutable template `40000000-0000-0000-0000-000000000021`; revision 3 must use a new template row in the same family.
- Confirmed `SignatureCaptureDialog` and immutable `workflow_form_signatures` already provide visual signature, actor/role/time, revision, payload hash, checksum and protected file access; contract integration will reuse this infrastructure rather than duplicate it.
- Added custom migrations V84/V85 for stable node/step business codes, project entry mode, protected project documents, immutable contract-signature links, paper dispatch/returned scan, staged outside-plan variations, contract RBAC and official engine-3 Rev3 template `40000000-0000-0000-0000-000000000022`.
- Implemented the reviewed Rev3 topology: client-BOQ projects skip survey/meeting/design, outbound contract gates all takeoff/procurement/deployment work, Node 7 negotiation is one step, design review is merged, contract/field/QA-QC roles are separated, and existing 020/021/022 runtimes remain unchanged.
- Reused the existing signature dialog and workflow signature persistence for App signing. Contract activation now requires the current version's App signature, paper dispatch and returned signed scan; project contract UI warns when input/output signed scans are missing.
- Implemented outside-plan variation priority and the auditable Proposal -> QS -> Purchasing -> PM/Director -> procurement stages, including construction-work-item provenance and role-targeted activity mentions.
- Updated Project intake, Pipeline Builder, Contracts and Operations UI plus shared types/status labels; production frontend build and the complete non-DB Go suite pass on the current source.
- Added `contract_officer` / `Bộ phận HĐ` seed parity and untouched `GOLDEN-REV3-BOQ-001` project 023. A full destructive seed replay plus Golden validator passes on isolated database `consflow_rev3_test3`: Rev3 has 15 nodes, 51 unique coded steps/work items and 49 draft forms, while project 022 remains on engine 2.
- Applied all 85 migrations locally, passed the full DB-backed core integration suite on a clean isolated database and passed the migration checksum/manifest check. Updated central/backend/frontend and all affected domain mapping files.
- Recovered V85 to the exact already-applied checksum `93358db...` after Docker's fail-closed startup check exposed an accidental post-apply edit; clean database test6 then applied all 85 migrations successfully. Accounting's isolated-schema test helper now retains `public` extension functions without weakening table isolation.
- Browser UAT on the final rebuilt Docker source passed: project 023 shows/locks `client_boq` intake and Rev3; Pipeline Builder shows 15 nodes plus immutable node/step business codes; Contracts shows missing input/output scan reminders and the four execution checkpoints; Operations opens the mandatory urgent/normal variation proposal; `Bộ phận HĐ` sees exactly five role-scoped contract work items; and the App-sign action opens the existing immutable `SignatureCaptureDialog` locked to `Người phê duyệt`.
- Browser UAT found and closed two defects before handoff: an existing project's intake selector remained editable despite backend locking, and a legacy active contract was presented as deployment-ready without Rev3 files. The selector is now disabled after runtime creation, and readiness counts only the current contract version while labeling legacy active contracts explicitly. No browser warning/error was observed; only normal Vite development/debug messages appeared.
- Final current-source verification passed: frontend production build, `go test ./...`, full DB-backed `go test ./internal/core/... -count=1` on fresh `consflow_rev3_test6` (23.187s), focused current-version readiness regression, 85-migration checksum check, full seed/Golden validation on isolated `consflow_rev3_test3`, Docker backend/admin rebuild, API health and `git diff --check`.

## [Closed] Project Finance Management Refactor

### Will be done

- No mandatory implementation or verification work remains. Keep this completed plan in active memory until the user explicitly asks to close it.

### Done

- Confirmed the business boundary: MISA remains the professional accounting/tax/double-entry system; Consflow does not integrate with MISA and does not duplicate professional books, VAT declarations, accounting periods, or journal correction workflows.
- Completed the required local mapping bootstrap, cross-domain impact review, evidence review, API/UI architecture, Mockup as Text, and received explicit approval of implementation points 1–7.
- Selected separate `project_material_costs` and `project_cash_requests` management sources so legacy payments, invoices, journals, ledgers, and accounting-period data remain historical and isolated from the new management report.
- Implemented V83 schema constraints, project-finance RBAC, future-template wording and non-destructive legacy permission isolation; active/historical pipeline runs are not rewritten.
- Implemented source-validated/audit-atomic material-cost and internal cash-request lifecycles, permission-redacted management summary, same-source CSV export, and handover readiness based on confirmed costs plus processed requests.
- Replaced ordinary global/project Accounting routes and navigation with the five-tab `Tài chính công trình` workspace; removed professional finance panels from the normal Reports catalog and rerouted finance work items/handover blockers.
- Added responsive create/detail/confirmation UI, mandatory reasons for `Yêu cầu bổ sung`, explicit external-system handoff wording, demo data, shared types/statuses and regression coverage.
- Browser UAT exposed and closed two in-scope usability defects: the action-heavy header compressed its purpose text at 1280px and global project context was lost on reload. The finance header now stays stacked until `2xl`, the shared selector has an accessible label, and `/accounting` persists its selected project in the `project` query parameter.
- Latest-source verification passed: frontend tests 31/31, production build, bundle budget (entry 120.1 KiB gzip; project-finance chunk 7.6 KiB gzip), full backend suite on an isolated PostgreSQL database, custom migration/complete Golden seed validation, Docker rebuild/restart and `/api/health`.
- Browser UAT on the rebuilt source completed the real Golden Project flow across `Ngân Kế Toán` and `Hoàng Long`: create/submit/confirm material cost, report/provisional-profit update, create/submit/approve/handoff/process cash request, required correction reason, role-separated actions, CSV success feedback and a clean task queue. Desktop 1280px and mobile 390×844 had no document-level horizontal overflow; mobile tabs use contained horizontal scrolling, dialogs fit the viewport, selected project survives reload, and console warnings/errors were zero. Scenario-based heuristic UX rating: 8.8/10; this is not a human usability study.

## [Closed] Reliable CTGS Tooltip Regression Fix

### Will be done

- Obtain one direct Edge/manual pointer-hover acceptance signal against the rebuilt source. The in-app Browser session could not produce a CSS `:hover` state from its pointer-move control, so mouse hover remains `NOT UX ACCEPTED` even though the implemented handler and the shared rendered tooltip path are present.
- Close this plan only when the user says `Đóng plan`.

### Done

- User confirmed that hovering the compact CTGS number did not display the approved full-number tooltip.
- Root cause was proven: the served Vite source contained only an HTML `title` attribute, no rendered `role="tooltip"` element; the previous build/tests did not exercise hover behavior.
- User approved points 1–4 and explicitly authorized implementation plus Browser UAT.
- Replaced native `title` with a portalled application tooltip, kept the full immutable CTGS in the accessible label/raw data, and added mouse-hover, keyboard-focus, Escape, touch/pen and outside-pointer handling.
- Rebuilt and restarted the local Docker runtime after the final source change; PostgreSQL and Redis were healthy and backend/admin started successfully.
- Browser UAT passed desktop focus/full-value/viewport placement and Escape closure; at 390×844 it passed tap/click, full-value rendering, outside closure and no horizontal overflow. No browser console warnings/errors were recorded.
- The same Browser session could not synthesize a pointer hover: moving over the proven visible CTGS element left `element.matches(':hover') === false`. This criterion is recorded as skipped by tool limitation, not passed.
- User required click-to-copy for `BF-PAY-ffd22b…58faeb` and approved review points 1–4, including implementation and Browser UAT.
- Made every non-empty CTGS a copy control with an explicit copy icon and accessible label. Click, Enter and Space copy the full immutable CTGS value, while compact display text remains presentation-only; clipboard rejection produces an error toast without changing any journal entry.
- Added focused clipboard and keyboard-key regression coverage. The complete frontend suite passed 31/31 tests, the production build passed, and the bundle-budget gate passed on the final source state.
- Rebuilt and restarted the Docker stack after the final keyboard change; PostgreSQL and Redis became healthy and backend/admin started successfully.
- Browser UAT against the rebuilt `Golden Project – Hoàn thiện Văn phòng An Phát` Sổ Nhật ký chung passed desktop click, Enter, Space, exact full-value clipboard content, success toast, tooltip text and Escape closure. At 390×844 it passed click-to-copy, success toast, tooltip containment and zero document-level horizontal overflow. No browser console warnings/errors were recorded.

## [Closed] Production Readiness And Maintainability Program

### Will be done

- Rerun C1 and C2 J0-J9 with the 15-role Golden Sample matrix in an isolated disposable environment when Browser verification is explicitly authorized in a future turn.
- Provision the selected production S3-compatible provider and migrate existing local objects with inventory, checksum and cross-replica validation before enabling multiple application replicas.
- Select the managed Kubernetes/edge vendor, production cluster/network CIDRs, managed PostgreSQL/Redis/S3 instances and external secret controller; Compose remains the controlled single-host topology until the reviewed Kubernetes overlay and cross-replica gates pass.
- Continue decomposing oversized backend core owners after the reporting slice, without changing public API, schema, RBAC or workflow behavior.
- Update the central/detail/domain maps whenever implementation changes owner files, CI/runtime contracts, audit behavior, storage, or route-loading behavior.

### Done

- Assessed the repository at 78/100 and confirmed the seven readiness gaps against current code, tests, migrations, CI, production Compose, and runbooks.
- Prepared and received user approval for the cross-cutting implementation roadmap and delegated execution.
- Added the aggregate `npm test` command for all committed frontend tests and made it a required CI step before the frontend production build; all 23 tests pass locally.
- Added a PostgreSQL CI service and required DB-backed core/migration tests so those checks cannot silently skip; fixed the PostgreSQL `DISTINCT`/ordering defect exposed by that gate.
- Centralized audit writes: critical governance/workflow/finance/quality transitions are transactional and fail closed, while explicitly non-critical events remain observable best-effort with structured failure logs.
- Split the giant backend integration suite into domain-owned files with shared fixtures, and moved the reporting handler/service/repository/type slice into focused owner files without changing routes, API, schema or RBAC.
- Added the 12-role J0-J9 Golden Sample UAT matrix, isolated-DB/reset contract, evidence requirements and explicit `BLOCKED-COVERAGE` state for missing dedicated demo actors.
- Added `_FILE` production secrets, a pinned-session advisory-locked one-shot migration job, incompatible-rollback detection, safe drain/migrate/start runbooks and secret-safe backup guidance.
- Added internal Prometheus HTTP/Go/process/DB metrics with bounded labels and kept `/metrics` unavailable through public Nginx.
- Added a provider-neutral protected-file storage registry plus local provider and migrated evidence, workflow signatures/PDFs and input/output invoice files behind authorization-before-open streaming. Independent review closed post-commit cleanup data loss, symlink-swap TOCTOU and registry concurrency risks with regression/race coverage.
- Added the portable S3-compatible provider, fail-closed startup write/read/delete preflight, dual-provider legacy reads, V70/V71 provider and storage-key integrity migrations, and a dry-run-by-default local-to-S3 operator command with checksum verification, conditional metadata updates and no source deletion.
- Added a fail-closed portable Kubernetes/Kustomize application baseline with two backend/admin replicas, private Services and ingress NetworkPolicies, one-at-a-time two-node-safe rollout, PDBs, cross-node anti-affinity/zone spread, probes/resources/security contexts, enforced rate limiting, separate runtime DML versus migration DDL/owner credentials, release-unique migration Job, managed PostgreSQL/Redis/S3 and secret-manager contracts, startup S3 preflight, forward-only object-cutover guidance, optional gated HPA/Ingress examples and deterministic offline validation. No cluster was deployed.
- Made evidence gallery/preview and handover source presentation provider-aware: internal `local|s3` objects use protected endpoints, while external, none and unknown providers remain excluded.
- Converted authenticated page modules to route-level lazy loading below auth/permission guards while keeping Login, guards and AppShell eager; added standard/focus-shell loading and recoverable error states plus a CI gzip budget of 250 KiB for the entry and every JavaScript chunk. The production entry is now 119.9 KiB gzip and the largest lazy chunk is 35.8 KiB gzip.
- Corrected the production runbook: Redis Pub/Sub already provides shared SSE fanout, while shared object storage, deployment/runtime choices and cross-replica validation remain required before scaling.
- Updated central, backend, root-cause and affected domain maps for the new owners and operational contracts.
- Passed the full backend suite against an isolated PostgreSQL database, storage/core race checks, `go vet ./...`, all 72 migration manifest checks, 23 frontend tests, the production frontend build and bundle budget, Kubernetes render validation, agent mapping validation and diff hygiene checks; browser verification was not run by policy.
- Ran the authorized Golden Sample Browser UAT in isolated DB `consflow_uat_20260722_115155`: all 12 demo actors authenticated and saw Golden Live, completed fixtures remained unchanged, but C1 failed at J1 because PM lacks the required `planning.edit` permission for draft/evidence mutations; C2 and J2-J9 remain blocked by that shared prerequisite and missing dedicated `Thiết kế`/`Sale`/`Pháp lý` actors. Evidence is stored outside the repo at `/private/tmp/consflow-uat-evidence/20260722_115155`.
- Resolved the confirmed Golden Sample UAT blockers: V72 and seed parity grant PM `planning.view`/`planning.edit` while preserving project/work-item scope; dedicated `Thiết kế`, `Sale` and `Pháp lý` users/staff/roles raise Golden Live membership to 15; validators, runbooks, UAT matrix and maps now cover all actors and rate-limit-safe evidence capture. Migration/seed validation and the full backend suite pass on disposable PostgreSQL databases; Browser rerun remains explicitly pending.
