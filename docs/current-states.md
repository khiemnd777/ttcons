# Current States

## Published Pipeline Node Inspector

Status: complete

Mục đích chính:
- Cho phép người dùng xem đầy đủ Cấu hình Node của revision workflow đã phát hành mà không mở bất kỳ đường chỉnh sửa nào.

Will be done:
- Render Node Inspector khi một node được chọn ở template `published`, gắn nhãn chỉ xem và giữ nút đóng.
- Khóa các trường cấu hình/approval policy và ẩn toàn bộ thao tác lưu, thêm, sửa, xóa, kéo sắp xếp step hoặc mở dialog chỉnh sửa.
- Bổ sung regression check cho hợp đồng UI này, cập nhật mapping Pipeline Builder và chạy frontend validation không dùng browser.
- Mở từng Checklist / Step ở revision đã phát hành bằng dialog chỉ xem, hiển thị đúng cấu hình Step và Workflow Form hiện hữu.

Done:
- User approved the review package with `Approve`.
- Confirmed the node-detail toggle sets `detailsNodeId`, but `PipelineBuilderPage` only renders Node Inspector when `!isReadOnly`.
- Confirmed backend `ensureDraftTemplate` continues to reject every node/step/edge write for a published revision; no backend contract, migration or runtime pipeline behavior is in scope.
- Published revisions now render the selected Node Inspector with a `Đã phát hành · Chỉ xem` label and an explicit immutability notice.
- Disabled every node/approval control; removed save, step add/edit/delete and step-drag affordances; blocked step edit-dialog display and guarded mutation callbacks.
- Updated central and frontend Pipeline Builder mappings and added a source-level regression check for the read-only Node Inspector contract.
- Validation passed: `cd frontend && npm run check:pipeline-builder` and `git diff --check`. Browser verification was skipped because it was not explicitly requested.
- User approved the Step detail review package with `Approve`.
- Each Step card in a published revision now opens by click, Enter or Space and exposes an explicit `Xem` affordance.
- Reused the Step dialog in `view` mode: it shows the exact stored name, type, roles, form version and required-field summary, disables all controls and offers only `Đóng`.
- Updated the Pipeline Builder regression check and the central/frontend maps for the read-only Step-detail contract.
- Validation passed: `cd frontend && npm run check:pipeline-builder` and `git diff --check`. Browser verification was skipped because it was not explicitly requested.

## Workflow Overall Progress Excludes Skipped Branches

Status: complete

Mục đích chính:
- Tiến độ chung phản ánh nhánh workflow thực tế đã thực hiện, đồng thời công khai số step bị bỏ qua thay vì làm project hoàn tất chỉ đạt 92%.

Will be done:
- Loại runtime step `skipped`/`cancelled` khỏi mẫu số tiến độ thực hiện.
- Hiển thị riêng số step thực hiện và số step đã bỏ qua.
- Giữ tổng coverage template trong dữ liệu workflow, không giả các step bỏ qua thành đã hoàn tất.
- Cập nhật frontend mapping và chạy frontend build, backend regression, agent validation, diff check.

Done:
- Screenshot and source confirmed the old calculation used all 52 runtime steps as denominator, while only 48 executed steps entered the completed numerator.
- User approved `100% · 48/48 step thực hiện · 4 đã bỏ qua` with `OK triển khai`.
- Overall sidebar progress now filters skipped/cancelled steps out of the execution denominator before calculating completion.
- Sidebar displays executable completion and separately discloses skipped count plus total workflow coverage.
- Updated frontend mapping with the execution-progress denominator invariant.
- Validation passed: frontend production build, backend core regression tests, agent artifact validation and `git diff --check`. Browser verification was skipped because it was not explicitly requested.

## Workflow Sidebar Submitted Step Progress

Status: complete

Mục đích chính:
- Đồng bộ bộ đếm và thanh tiến độ của cây workflow với trạng thái hiển thị từng work item, để node có năm step đã thỏa gate hiển thị `5/5` thay vì `0/5`.

Will be done:
- Coi runtime step `submitted` và `completed` là đã hoàn tất khi tính sidebar progress.
- Ưu tiên `completed_steps` và `progress_percent` từ backend timeline khi payload có summary.
- Không tính `skipped` hoặc `cancelled` vào tiến độ thực hiện.
- Đồng bộ icon fallback của step `submitted` với trạng thái hoàn tất.
- Cập nhật frontend mapping và chạy frontend build, backend regression, agent validation và diff check.

Done:
- User confirmed the screenshot expectation: five completed work items in node `Họp báo cáo khảo sát` must display `5/5`.
- Root cause confirmed in `ProjectWorkspaceLayout`: row status prefers `work_item_status=completed`, while node counter reads runtime `step.status=submitted` through a helper that only accepted literal `completed`.
- User approved the review package with `OK triển khai`.
- Updated sidebar completion semantics so `submitted` and `completed` both count as satisfied steps; skipped/cancelled remain outside completed progress.
- Node counters now prefer backend `completed_steps` and `progress_percent` with bounded fallback calculation from runtime steps.
- Updated fallback step icon semantics and frontend mapping for the shared workflow-sidebar contract.
- Validation passed: frontend production build, backend core regression tests, agent artifact validation and `git diff --check`. Browser verification was skipped because it was not explicitly requested.

## Project Workflow Work Item History Visibility

Status: complete

Mục đích chính:
- Giữ toàn bộ work item theo đúng node workflow ngay trên Project Detail để người dùng xem được việc đang xử lý, đã hoàn tất và đã bỏ qua mà không phải chuyển sang UI khác.

Will be done:
- Project Operations trả cả work item terminal cho màn hình lịch sử, nhưng các API danh sách tác nghiệp và action gate vẫn không đưa work item đã hủy vào hàng đợi xử lý.
- Cho phép mở work item đã bỏ qua ở chế độ read-only để xem activity và metadata lịch sử.
- Timeline đếm runtime step `submitted` là đã hoàn thành và tách số bước `skipped`.
- Panel workflow mặc định hiển thị tất cả, có tìm kiếm và bộ lọc Cần xử lý/Chờ duyệt/Bị chặn/Hoàn tất/Đã bỏ qua.
- Badge Công việc dùng tổng số thay vì chỉ số đang mở; empty state phân biệt chưa phát sinh, không khớp bộ lọc và node đã bỏ qua.
- Bổ sung test, cập nhật mapping, chạy backend test, frontend build và diff check.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo mockup này`.
- Confirmed frontend `ProjectDetailPage` filters out `completed` and `cancelled` before rendering the workflow panel and tab count.
- Confirmed Project Operations currently uses the operational list query that excludes cancelled records.
- Confirmed Golden Sample runtime nodes are completed while executed runtime steps use terminal status `submitted`, causing the existing timeline aggregation to report `0/N`.
- Added a Project Operations history query that includes completed/cancelled work items without changing the operational `/projects/:id/work-items` queue contract.
- Kept cancelled work items viewable under the original project/RBAC scope while `CanActOnWorkItem` continues to reject them.
- Updated workflow timeline aggregation so `submitted` and `completed` steps count as complete, skipped/cancelled steps are reported separately and skipped nodes remain at 0% execution progress.
- Rebuilt the Project Detail workflow panel with default `Tất cả`, total count, search, status filters, explicit skipped-branch context, terminal compact cards and status-specific empty states.
- Changed the Project Detail `Công việc` tab count from open-only to all visible project work items and excluded skipped nodes from automatic current-node selection.
- Added a read-only skipped-work-item detail surface that exposes history without primary business actions or activity composer.
- Added focused timeline regression coverage and updated central, backend, frontend, workflow orchestration and project-planning maps.
- Validation passed: backend core/full tests, frontend production build, agent artifact validation and `git diff --check`. Browser verification was skipped because it was not explicitly requested.

## Golden Live Operational Project

Status: complete

Mục đích chính:
- Cung cấp một project Golden chưa hoàn tất để người dùng tự vận hành từng node, step, form, approval và nghiệp vụ từ khảo sát đến bàn giao trên đúng engine-v2 contract.

Will be done:
- Sửa approval policy của official template trong seed về đúng schema V46 (`roles`, `quorum`, `requester_cannot_approve`, `comment_required`, `signature_required`) và mode `any`.
- Bổ sung demo users/staff cho Mua hàng, QA/QC, Đội thi công, Thầu phụ và Chủ đầu tư.
- Tạo `GOLDEN-LIVE-001` ở trạng thái planning 0%, node Khảo sát in-progress, sáu work item ready, các node/step còn lại pending và 51 workflow form draft.
- Không seed answer, evidence, approval hoặc domain ledger cho Live project để người dùng tự tạo qua UI/API.
- Thêm runbook theo node/vai trò/gate và validator đảm bảo project thực sự chưa được thao tác.
- Seed database sạch và local, chạy fixture/database validator, backend tests, migration check và frontend build.

Done:
- User approved the review package with `OK triển khai Golden Live`.
- Confirmed `StartPipeline` creates the first node `in_progress`, first-node work items `ready`, later work items `pending`, all runtime steps `pending`, and exact draft form instances.
- Confirmed current Golden seed approval policy uses incompatible keys and would fail normalization/request-approval for a real engine-v2 run.
- Confirmed the current demo accounts do not cover procurement, QA/QC, construction team, subcontractor or client roles.
- Added five demo users/staff/role assignments: procurement, QA/QC, construction team, subcontractor and client; all use the documented local password and are members of Golden Live.
- Corrected official seeded approval policies to the exact V46 contract with mode `any`, role array, quorum, requester separation, comment and signature flags.
- Restored the 15 contract/quality/handover permissions and role-permission matrix that base seed reset previously removed after migrations.
- Added `backend/seed/golden_live_project.sql`, mirroring `StartPipeline` state: one in-progress survey node, eleven pending nodes, 52 pending run steps, six ready work items, 46 pending work items and 51 untouched draft form instances.
- Golden Live intentionally has zero answers, evidence, approvals, documents, BOQ, quotations, procurement, contracts, stock, quality, acceptance, finance, invoice or handover records after seed.
- Added `backend/seed/GOLDEN_LIVE_RUNBOOK.md` with project scenario, role accounts, node-by-node operating sequence, expected blockers and reset instructions.
- Extended `backend/Makefile` to seed Golden Live before validation and expanded `validate_golden_project.sql` with runtime, untouched-data, policy-contract, RBAC and role-account assertions.
- Updated module-memory, backend, workflow, project-planning and admin-governance maps with Golden Live ownership and invariants.
- Rebuilt a separate V1-V56 PostgreSQL database and ran base + completed Golden + Golden Live + validator successfully.
- Applied `make seed` successfully to the running local database; Golden Live is ready for use at project id `30000000-0000-0000-0000-000000000022`.
- Final validation passed: Golden fixtures, Golden database validator, backend tests, 56 migration checks and frontend production build. Browser verification was skipped because it was not explicitly requested.

## Golden Sample 100% Official Workflow

Status: complete

Mục đích chính:
- Cung cấp một bộ dữ liệu chuẩn để nhìn xuyên suốt toàn bộ Consflow như một công trình thật, dùng đúng template chính thức `40000000-0000-0000-0000-000000000020` và không bỏ sót node, step, form hay ledger nghiệp vụ.

Will be done:
- Tạo flagship `GOLDEN-REAL-001` chạy nhánh Có thiết kế đến hoàn thành/bàn giao/đóng tài chính.
- Tạo companion `GOLDEN-NO-DESIGN-001` chạy nhánh Không thiết kế để bao phủ nhánh loại trừ mà không làm sai workflow thực tế.
- Bao phủ 12 node, 52 step, 51 workflow form, evidence, approvals, signatures, PDF và activity/audit.
- Seed đầy đủ BOQ, báo giá/so sánh, procurement, hợp đồng/version/milestone/variation/retention, kho/vật tư, nhật ký thi công, ITP/RFI/NCR/recurring, nghiệm thu/rework, AP/AR, hóa đơn/thanh toán và handover checklist.
- Thêm validator định lượng cho coverage, quan hệ nguồn, cân bằng kho/tài chính và trạng thái bàn giao cuối.
- Chạy seed trên PostgreSQL đã migrate V56, validator, backend tests, frontend build và diff checks; browser verification chỉ chạy khi được yêu cầu riêng.

Done:
- User approved the review package with `OK triển khai Golden Sample`.
- Confirmed the existing `CF-FULL-001` is tied to demo template `...0010` and lacks V47-V56 commercial/contract/quality coverage.
- Confirmed the official template has 12 nodes, 52 steps, 51 required forms and one evidence-only step, including mutually exclusive design/no-design branches.
- Loaded module-memory, backend/database workflow, project planning, workflow orchestration, commercial/procurement, construction, materials, quality/handover, finance and impact-analysis guidance.
- Chosen a flagship plus companion structure so 100% branch coverage remains truthful to a real project execution.
- Added `backend/seed/golden_project.sql` to rebuild the official V56 form/surface bindings after a local reset, then generate two complete runtime snapshots with 24 total nodes, 104 steps/work items, 92 submitted form instances, 94 accepted evidence records, signatures, approvals, activity and audit.
- Added `backend/seed/golden_project_domains.sql` with six final-locked BOQ lines, three suppliers/eighteen quote items, six selections, two received procurement requests, two incoming acceptances, explicit receive/issue/return stock ledgers, six closed site balances and three construction reports.
- Added `backend/seed/golden_project_closure.sql` with ITP Rev.01/Rev.02, pass/fail/reinspection RFIs, closed NCR, completed rework, three recurring QA occurrences, final pass acceptance and subcontractor/client reconciliation.
- Added `backend/seed/golden_project_finance.sql` with three active contracts, four signed versions, incorporated variation, six paid milestones, three released retentions, four AP, five AR, nine terminal payments/receipts, schedules, settled advances, invoice allocations, material closure and a handed-over 8/8 package.
- Added `backend/seed/validate_golden_project.sql`; `make seed-validate` reports `GOLDEN_SAMPLE_VALIDATION_PASSED` and exact project counts.
- Added eight rendered Golden business PDFs, copied invoice fixtures, a checksum manifest and `make seed-fixtures-check`; visual QA found no clipping, overlap or unreadable Vietnamese text.
- Fixed base seed drift exposed by a clean V56 database: reset now cascades to later domain tables; pipeline templates include required lineage/engine/hash/publish fields; legacy handover rows include `handover_code` and current statuses.
- Updated `backend/Makefile` so `make seed` applies base + Golden layers, validates the suite and copies upload fixtures; applied it successfully to the running local database.
- Rebuilt a separate clean PostgreSQL database through all V1-V56 SQL migrations, then ran every seed layer and validator successfully.
- Updated central/backend/domain maps with Golden Sample ownership, IDs, coverage and validation commands.
- Ran backend tests, migration check for 56 migrations, frontend production build, PDF fixture validation and Golden database validation successfully. Browser verification was skipped because it was not explicitly requested.

## Product-grade Workflow Engine And Business Forms

Status: complete

Mục đích chính:
- Biến pipeline thành hợp đồng thực thi nghiệp vụ: template đã phát hành bất biến, project chỉ chạy đúng phiên bản đã phát hành, và mọi bước nghiệp vụ phải đạt đúng gate trước khi quy trình đi tiếp.

Will be done:
- S0: lập baseline tương thích cho template `40000000-0000-0000-0000-000000000020`, các run hiện hữu và dữ liệu form lịch sử; không sửa submitted/signed/PDF history.
- S1: thêm template lineage, revision, engine version, definition hash, published actor/time; khóa mọi đường ghi vào template đã phát hành.
- S1: mở rộng publish validation cho graph, form version, condition config và permission; project chỉ được start bằng template published.
- S2-S5: runtime snapshot, gate engine, state transition/branch, hard automation, approval và capability-driven UI.
- B5: hoàn tất UAT, regression hardening và sign-off toàn hệ thống theo các vertical slice đã duyệt.
- Mỗi slice cập nhật migration, backend/frontend contract, mapping, automated tests và UAT evidence tương ứng.

Done:
- User approved the complete review package with `OK triển khai theo review package này.`
- Loaded the mandatory module memory, backend/frontend maps, workflow orchestration, project planning, commercial/procurement, construction operations, quality/acceptance/handover, accounting/finance, administration/governance, reporting and impact-analysis domain guidance.
- Confirmed from current code that published pipeline templates remain writable through template/node/step/edge endpoints and `StartPipeline` does not reject a selected draft template.
- Confirmed the current publish gate only validates required workflow-form mappings.
- Began S0-S1 with an additive migration and shared immutable-template/publish-validation design; existing run/form history remains out of mutation scope.
- Added V43 template lifecycle schema: family/revision/source lineage, engine version, definition hash, publish actor/time, plus run engine/hash snapshots.
- Made template creation and revision cloning transactional; clone now creates the next draft revision in the same family.
- Guarded every template/node/step/edge structural write so published revisions are immutable; deleting a published revision archives it.
- Expanded publish validation to graph root/sink/reachability/cycle, required published forms, answer-condition field/value validity, and permission-key existence.
- Updated Pipeline Builder with published read-only mode, `Tạo revision`, local graph validation, and authoring controls for `workflow_form_answer_equals.field_key/value`.
- Filtered Project template choices to published revisions and made backend `StartPipeline` reject draft/archived templates.
- Added V44 runtime completion gates: run-step snapshots of exact required form version, engine-v2 active-node enforcement, exact form submission, complete work items/steps, node approval, and blocked automation checks before advance.
- Removed synthetic legacy checklist completion from engine v2 while preserving compatibility for engine-v1 runs.
- Added direct non-selected branch skipping/cancellation and excluded skipped nodes from runtime progress.
- Added V45 stable work-item surface binding with template/run-step snapshots of `surface_kind` and `domain_action_key`.
- Bound official steps 38/39/40/46 explicitly: subcontractor checklist reconciliation, incoming material acceptance, quantity dossier workflow form, and owner/client reconciliation respectively; this removes the prior title-based misrouting.
- Updated work-item primary-surface resolution to prefer the backend snapshot and retain text heuristics only for legacy/non-pipeline records.
- Added V46 hard automation and structured approval-policy schema, including request policy snapshots, current approval stage, and actor role labels on approval actions.
- Replaced engine-v2 display-name dispatch with stable terminal `domain_action_key` mappings for BOQ/procurement, input/output finance scaffolds, and handover readiness; engine v1 retains legacy behavior.
- Required engine-v2 hard automation to succeed before work-item/legacy-step completion. Blocked/failed outcomes are stored with the same idempotency key and returned as actionable validation blockers instead of being ignored after completion.
- Added work-item automation-event payload and requirement-panel status so users see the exact hard action and blocker; retrying completion safely retries the same idempotent action.
- Implemented structured approval evaluation for `any`, `all`, `quorum`, and ordered role stages with requester separation, required comments, duplicate-approval prevention, row locking, stage progression, and audit.
- Added Pipeline Builder approval-policy controls for mode, ordered roles, quorum, requester separation, and required comments.
- Completed B1 supplier quotation and procurement vertical slice with V47: supplier quotation headers/items, qualification state, immutable submitted quote items, auditable quote selection history, BOQ selected-quote/waiver source, procurement quotation snapshots, supplier/PO/delivery metadata, and supporting indexes/constraints.
- Added project supplier-quotation list/create/detail/update/submit/qualify APIs, BOQ-line comparison API, and qualified-quote selection API. Selection requires approved BOQ + norm lock, validates quote/item compliance and validity, locks final price atomically, writes the immutable BOQ price event, and records audit metadata.
- Manual BOQ final-price lock now requires an explicit quotation-waiver reason. BOQ submit/approve now use transactional state transitions with audit and no longer approve draft rows directly.
- Procurement generation now requires a selected quote or waiver source, groups BOQ lines into one request per supplier, and snapshots quote item, supplier, BOQ price revision/event, quoted price, and variance while retaining the legacy top-level response shape.
- Procurement create/update now validates project, title, budget state, partner catalog, and delivery date and writes audit. Structured approval requests can no longer be silently forced approved by the procurement action endpoint.
- Procurement ordering now requires supplier, PO/order reference, and expected delivery date. Receive derives `partially_received`/`received` only from submitted incoming-acceptance quantities; cumulative accepted quantity cannot exceed requested quantity, and no implicit stock movement is created.
- Added the BOQ `Báo giá nhà cung cấp & so sánh thương mại` workspace with quote editor, supplier catalog, quote-item grid, technical/commercial assessment, comparison table, decision reason, loading/empty/error states, and responsive horizontal tables.
- Reworked the Procurement page around generated requests and real lifecycle actions, with supplier/order metadata, source-price detail, incoming-acceptance progress, status-specific actions, and shared dialogs instead of one-off drawers.
- Added DB-backed integration coverage for quote submission/qualification/selection, BOQ final lock, procurement supplier/quote snapshot, and the legacy manual-waiver generation path.
- Applied V47 successfully through the running backend migration runner and verified schema migration version `47` in PostgreSQL.
- Ran B1 DB-backed integration tests inside the backend Docker network: both targeted scenarios pass.
- Ran `GOCACHE=$PWD/.gocache go test ./...` in `backend`: pass.
- Ran `make migrate-check`: 47 migrations checked successfully.
- Ran `npm run build` in `frontend`: pass; only the existing large chunk warning remains.
- Browser verification for B1 was skipped because it was not explicitly requested.
- Completed B2 contract lifecycle with V48-V50: project-scoped contracts, signed immutable versions, payment milestones, variation/appendix workflow, retention ledger/release history, dedicated contract RBAC, cascade-safe history triggers, and official-workflow surface binding.
- Added contract lifecycle API and service/repository transitions: create/update draft contract, create version, submit/review/approve/sign, activate signed current version, create/qualify milestone, materialize AP/AR, create/submit/approve/reject/incorporate variation, and release retention into traceable AP/AR.
- Kept `projects.contract_amount` as a compatibility snapshot updated only from the active signed main-client contract; signed `contract_versions` and `contracts.current_amount` are now the commercial source of truth.
- Version definition hashes cover amount, scope, commercial/payment terms, dates and document reference. Signed versions cannot be updated; incorporated/rejected variations cannot be updated. Project cascade cleanup remains possible through V49 without exposing a direct history-delete API.
- Milestone obligation creation requires an active contract and eligible milestone. Acceptance-required milestones require an approved pass/conditional-pass acceptance. Obligation creation is idempotent and snapshots contract/milestone source into payable/receivable records.
- Milestone retention creates a held ledger entry. Retention release is warranty-gated, cannot exceed outstanding retention, appends a numbered release row, opens a traceable AP/AR record, and updates held/partial/released state with audit.
- Approved variation incorporation creates the next draft version with revised amount, schedule and scope. It does not silently mutate the active signed version; the new revision must pass review/approve/sign/activate again.
- Added `frontend/src/modules/contracts/ContractsPage.tsx` with project-scoped contract list, capability-gated lifecycle actions, version hash/status table, milestone/AP-AR actions, variation revision controls, retention release dialog, loading/empty/error states and responsive tables.
- Added `/projects/:id/contracts` to router and project workspace navigation under `Thương mại & Vật tư`; added contract domain types and Vietnamese status presentation.
- Added `surface_kind='contract'` to workflow surface contracts. Official template steps 24 and 29-31/33 open the Contract Workspace while preserving the step business form/PDF as a supporting tab; step 32 remains the structured approval surface. Only pending runtime steps are remapped.
- Applied V48, V49 and V50 successfully through the running backend migration runner; PostgreSQL confirms five official template steps on the contract surface and seven contract permission keys.
- Added and ran DB-backed integration coverage for contract create → version review/approve/sign → activation → milestone → receivable/retention → variation/revision → re-sign/re-activate → retention release. The scenario passes inside the Docker network and cleanup is cascade-safe.
- Ran backend core tests, `make migrate-check` for 50 migrations, and frontend production build successfully; only the existing large chunk warning remains.
- Browser verification for B2 was skipped because it was not explicitly requested.
- Completed B3 QA/QC and construction execution with V51-V53: revisioned ITP plans/items, RFI inspection lifecycle, NCR root-cause/corrective/reinspection lifecycle, recurring construction occurrences, dedicated quality RBAC, official-workflow quality surfaces, and an idempotent construction daily-report source bridge.
- Added the project quality workspace API and transactional transitions for ITP create/revise/submit/approve/activate, RFI create/submit/schedule/inspect, NCR create/root-cause/corrective/reinspection/close, recurring plan generation and occurrence complete/skip.
- Rejected RFI inspection now creates a linked NCR and draft rework atomically. NCR closure requires project-owned corrective evidence, an accepted/conditional reinspection RFI, and completed linked rework; actor and audit history are preserved.
- Active/approved/superseded ITP definitions are immutable and hash-backed. Only one active revision exists per project/family; recurring occurrence generation is idempotent by plan/date and respects evidence gates.
- Added `frontend/src/modules/quality/QualityPage.tsx` with capability-gated ITP, RFI, NCR and recurring-work sections, including structured editors, lifecycle actions, loading/empty/error states, project-owned evidence selection and responsive tables.
- Added `/projects/:id/quality`, project workspace navigation, quality frontend contracts/status labels, and the `quality` work-item primary surface. Official workflow steps 36-37 now open the quality workspace while completed historical runtime steps remain unchanged.
- Submitted `builtin_construction_daily_report` forms now sync idempotently into `operation_daily_reports` with workflow-form/work-item/evidence linkage, structured answer snapshot, progress, work, crew, material and issue summaries; the existing next-plan work-item generation remains intact.
- Applied V51, V52 and V53 with the custom migration runner. `make migrate-check` validates all 53 migrations.
- Added DB-backed lifecycle coverage for ITP -> failed RFI -> NCR/rework -> corrective evidence -> accepted reinspection -> close and recurring occurrence completion. Added daily-report bridge assertions covering the complete structured mapping.
- Ran targeted Docker/PostgreSQL integration tests for B3 and both planning/construction daily-report next-plan flows: all pass.
- Ran backend core tests and the frontend production build successfully during B3 implementation; the final full-suite validation remains part of the B3 completion gate below.
- Browser verification for B3 was skipped because it was not explicitly requested.
- Completed B4 financial creation and handover package hardening with V54-V56: typed/server-owned AP/AR creation, draft-only generic payment creation, actor audit, handover package lifecycle, evidence/document checklist, readiness snapshot, new verification permission, official finance/handover workflow surfaces, and historical checklist backfill.
- Manual AP/AR APIs no longer accept client-controlled paid/received/remaining/status fields. Payables derive remaining from gross minus advance/deduction; receivables derive remaining from gross; all new records start draft with zero paid/received and audited creator.
- Generic payment/receipt creation validates project/code/type/method/positive amount, ignores client approval/status fields, and always starts draft. Dedicated payable/receivable payment endpoints continue to own allocation and reconciliation updates.
- Added exact final-acceptance source locking, handover code, workflow-form source, checklist items, accepted-evidence/active-document verification, submit/check/ready/blocked/handed-over lifecycle, checked/completed actor timestamps, and immutable completion snapshot semantics.
- Readiness now checks the linked final acceptance, open issues, NCRs, rework, material surplus, AP/AR remaining balances, pending payments and all required checklist items. Complete always reruns readiness immediately before writing `handed_over`.
- Rebuilt `HandoverPage.tsx` as a dense project workspace with package summary, exact acceptance selection, checklist evidence/document verification, blocker display, capability-gated submit/check/complete actions and required completion comment. Removed self-attested readiness checkboxes.
- Removed client-editable derived balance/status controls from AP/AR and generic payment create forms. Server ownership is explained inline.
- Added finance and handover primary work-item surfaces. Official steps 42, 44-45 and 48-51 open the project debt/finance workspace; step 52 opens the handover package while preserving each workflow form/PDF in the support tab. Only pending runtime steps are remapped.
- Applied V54-V56 with the custom migration runner; V54 backfills legacy `checking` to `submitted`, and V56 gives every historical non-deleted handover the same eight required checklist items.
- DB-backed regression chain passes for input/output invoice PDF, payable/receivable approval, payment schedule, advance, partial payment/receipt allocation, reconciliation ledger/aging, derived-state hardening, blocked handover and acceptance -> verified package -> readiness -> completed handover.
- Frontend production build passes after the B4 workspace/forms/surface changes; only the existing large chunk warning remains.
- Browser verification for B4 was skipped because it was not explicitly requested.
- Completed B5 full impact audit and DB-backed UAT hardening across template/runtime, forms/signatures/PDF, activity/RBAC, BOQ/procurement, contract, stock/materials, QA/NCR/rework, invoice/payment/reconciliation and handover.
- The first full DB suite exposed two production SQL defects that focused tests had not reached: runtime queries ordered `pipeline_run_steps` by a non-existent `created_at`, and site-material balance SQL reused an untyped quantity placeholder across numeric/integer expressions. Both root causes were fixed in repository code.
- Explicit UUID/text casts were added to activity mention insertion SQL after the full suite exposed PostgreSQL parameter inference ambiguity on role mentions.
- UAT fixtures were migrated to the current engine-v2 contract: template lineage is mandatory, node completion fixtures satisfy exact forms/steps/approvals, and official hard automation is selected by `domain_action_key` instead of legacy display-name dispatch.
- Updated checklist UAT to the currently published structured planning-checklist schema and configured test-local upload storage for signature artifact verification.
- Corrected the material ledger regression expectation: a partial return with positive remaining surplus stays `partial`, not `handled`.
- Re-ran every previously failing regression group: all pass.
- Re-ran the entire compiled core test suite with `CONSFLOW_TEST_DATABASE_URL` against PostgreSQL in Docker: `PASS`.
- Final backend `go test ./...`, frontend production build, `make migrate-check` for 56 migrations, agent artifact validation and `git diff --check` all pass. Browser verification was not run because it was not explicitly requested.
- Applied V43, V44, and V45 successfully through the running backend container migration runner.
- Applied V46 successfully through the running backend container migration runner.
- Ran `make migrate-check`: 46 migrations checked successfully.
- Ran `GOCACHE=$PWD/.gocache go test ./...` in `backend`: pass.
- Ran `npm run build` in `frontend`: pass; only the existing large chunk warning remains.
- Browser verification was skipped because it was not explicitly requested.

## Pipeline Builder Back Navigation

Status: complete

Mục đích chính:
- Add a clear `[← Quay lại]` control to the full-screen Pipeline Builder workspace so users can return to the previous page without using the browser chrome.

Will be done:
- Reuse the existing `TemplateSidebar` back-button surface instead of adding a new UI primitive.
- Change the button label from `Back` to `Quay lại`.
- Route the button through browser/app history with a safe `/` fallback when no internal history entry exists.
- Keep backend APIs, template publish/clone/edit behavior, canvas/diagram interactions, workflow gates, and runtime copies unchanged.
- Update the frontend UI mapping because the Pipeline Builder surface behavior changes.
- Run `cd frontend && npm run build`; browser verification remains skipped unless explicitly requested.

Done:
- User approved the review package with `OK`.
- Re-read Consflow module memory, frontend workflow, workflow orchestration domain, central module map, frontend UI map, domain map, `AGENTS.md`, router, app shell, and Pipeline Builder owner files.
- Enabled the existing Pipeline Builder sidebar back-button surface.
- Changed the back button label from `Back` to `Quay lại` and added a Vietnamese title.
- Updated Pipeline Builder back behavior to use browser/app history with `/` as a fallback when no internal history entry exists.
- Updated `frontend-ui-utility-map.md` to record Pipeline Builder sidebar back navigation.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Mandatory Agent Mapping Bootstrap

Status: complete

Mục đích chính:
- Make Consflow agents and subagents load the routing/mapping memory as a mandatory first step for repository work, and keep that memory updated whenever code changes affect features, functions, utilities, helpers, UI, routes, contracts, tables, statuses, or domain invariants.

Will be done:
- Keep scope limited to agent artifacts, mapping references, and documentation; do not change backend or frontend runtime behavior.
- Add a mandatory mapping bootstrap rule to `AGENTS.md`, `consflow-module-memory`, subagent profiles, and agent docs.
- Add a mapping maintenance gate: any map-affecting code change must update the relevant mapping in the same work turn or state why no mapping update is required.
- Split detailed mapping into targeted reference files so agents load the central map first, then detail maps by keyword/domain instead of loading all inventory for every task.
- Add backend function/route/repository/service mapping guidance.
- Add frontend UI/component/helper/API mapping guidance.
- Add root-cause triage keywords and search recipes for faster diagnosis.
- Validate agent artifacts with `.agents/scripts/validate.sh` and targeted `rg` checks.

Done:
- User approved the review package with `Approve`.
- Re-read `AGENTS.md`, `consflow-module-memory`, `consflow-repo-architect`, `consflow-impact-analysis`, central module keyword map, subagent profiles, and agent docs.
- Confirmed current behavior: mapping is implicit/instruction-driven, not engine-level preloaded; runtime sync is not automatic.
- Added mandatory mapping bootstrap and mapping maintenance gate to `AGENTS.md`.
- Updated `consflow-module-memory` so agents must load the central map, then targeted backend/frontend/root-cause/domain maps before scans, diagnosis, review, planning, or edits.
- Added `backend-function-map.md` for backend route, handler, service, repository, helper, type, migration, and search ownership.
- Added `frontend-ui-utility-map.md` for frontend routes, UI surfaces, shared components, helpers, API usage, domain types, status labels, and pipeline diagram helpers.
- Added `root-cause-triage-map.md` for symptom-to-owner routing and focused `rg` recipes.
- Updated subagent profiles so explorer, backend worker, frontend worker, reviewer, and DB reviewer must perform mapping bootstrap and enforce mapping maintenance.
- Updated agent docs and skill README to describe the mandatory mapping policy.
- Ran `bash .agents/scripts/validate.sh`: pass.
- Ran targeted `rg` checks confirming mandatory bootstrap and mapping maintenance gate are present in subagent profiles and mapping docs.
- Ran `git diff --check`: pass.

## Project Operations Lightweight Workflow Sidebar Payload

Status: complete

Mục đích chính:
- Make project-scoped work item detail pages load the workflow sidebar quickly by requesting only the project summary, workflow timeline, run nodes/steps, and minimal work item data needed for sidebar navigation.

Will be done:
- Keep the default `/projects/:id/operations` dashboard contract unchanged.
- Add a lightweight `view=workflow-sidebar` variant for `/projects/:id/operations`.
- Preserve project operations visibility, project/work item scoping, runtime workflow gates, form/evidence/signature/PDF semantics, and audit/activity behavior.
- Batch pipeline run step loading so runtime nodes no longer trigger one step query per node.
- Update `ProjectWorkspaceLayout` to use the lightweight view only on project-scoped work item detail routes.
- Update frontend types for the lighter operations response shape.
- Add or adjust focused tests when a stable backend seam exists.
- Run focused backend validation and `cd frontend && npm run build` when feasible. Browser verification remains skipped unless explicitly requested.

Done:
- User approved the review package with `OK`.
- Re-read Consflow backend workflow, diagnose, workflow orchestration, module memory, AGENTS, route, handler, service, repository, frontend layout, router, and shared type code.
- Confirmed root cause: project-scoped work item detail pages fetch the full project operations dashboard response even though the detail content fetches `/work-items/:id` separately.
- Added `ProjectOperationsFilter` with a validated `view=workflow-sidebar` backend variant.
- Kept default `/projects/:id/operations` full dashboard response unchanged.
- Added lightweight project operations repository response with project summary, visibility, health counts, workflow timeline, pipeline run, summary, and sidebar work items only.
- Added minimal sidebar work-item query that preserves project/work item scoping and workflow form/evidence summary fields.
- Batched `GetPipelineRun` step loading into one query grouped by node, preserving `pipeline_run.nodes[].steps`.
- Updated `ProjectWorkspaceLayout` so project-scoped work item detail routes request `/projects/:id/operations?view=workflow-sidebar`; non-detail project workspace routes still request the full endpoint.
- Added `ProjectOperationsSidebarResponse` frontend type.
- Extended the DB-backed start-pipeline integration test to assert the lightweight operations response shape when `CONSFLOW_TEST_DATABASE_URL` is available.
- Ran `GOCACHE=$PWD/.gocache go test ./internal/core` in `backend`: pass.
- Ran `GOCACHE=$PWD/.gocache go test ./...` in `backend`: pass.
- Ran `npm run build` in `frontend`: pass; Vite reported only the existing large chunk warning.

## Work Item Detail Workflow Sidebar Collapse Scroll

Status: complete

Mục đích chính:
- Keep the work item detail workflow sidebar focused on the currently selected Checklist / Step by expanding only the active node by default and auto-scrolling to the active step.

Will be done:
- Add expand/collapse controls for workflow nodes in the project-scoped work item detail sidebar.
- Default to only the node containing the selected work item step being expanded.
- Collapse other nodes by default when the active work item changes.
- Auto-scroll the sidebar to the active step after expansion.
- Preserve project-scoped work item links, active state, fallback step data, backend APIs, workflow gates, form/evidence/signature/PDF behavior, and action semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `OK triển khai.`
- Re-read Consflow frontend workflow, workflow orchestration, module memory, AGENTS, module keyword map, domain map, UI guardrails, router, shared domain types, and current `ProjectWorkspaceLayout` sidebar code.
- Added workflow node expand/collapse state in `frontend/src/modules/projects/ProjectWorkspaceLayout.tsx`.
- Defaulted the sidebar to expand only the node containing the active work item step, with other nodes collapsed on active work item changes.
- Added node header toggle buttons with expanded/collapsed chevrons and `aria-expanded`.
- Added active step ref and auto-scroll to center the selected step after the active node expands.
- Preserved project-scoped work item links, active state, fallback step data, backend APIs, workflow gates, forms, evidence, signatures, PDFs, and action semantics.
- Ran `cd frontend && npm run build` successfully; Vite reported only the existing large chunk warning.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Detail Workflow Sidebar Step Fallback

Status: complete

Mục đích chính:
- Ensure the work item detail left workflow sidebar shows real Checklist / Step rows, not only workflow nodes, when `pipeline_run.nodes[].steps` is empty in `/projects/:id/operations`.

Will be done:
- Keep using existing `/projects/:id/operations`; do not add backend endpoints or migrations.
- Build sidebar steps from `pipeline_run.nodes[].steps` when available.
- Fallback/merge steps from `operations.work_items` grouped by `run_node_id` and `run_step_id`.
- Preserve project-scoped work item detail links, active step state, workflow gates, forms, evidence, signatures, PDFs, and action semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User confirmed implementation with `OK triển khai`.
- Re-read Consflow frontend workflow, workflow orchestration, module memory, AGENTS, domain map, module keyword map, and UI guardrails.
- Confirmed root cause: sidebar only read `pipeline_run.nodes[].steps`, while available `operations.work_items` already contains `run_node_id`, `run_step_id`, `run_node_name`, and `run_step_name`.
- Updated `frontend/src/modules/projects/ProjectWorkspaceLayout.tsx` to merge `pipeline_run.nodes[].steps` with fallback steps built from `operations.work_items`.
- Added stable step grouping by `run_node_id`, dedupe/merge by `run_step_id`, workflow-order sorting, and fallback node construction when needed.
- Preserved project-scoped work item links, active state, status/evidence/form chips, backend APIs, and runtime workflow semantics.
- Ran `cd frontend && npm run build` successfully; Vite reported only the existing large chunk warning.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Detail Workflow Sidebar Timeline

Status: complete

Mục đích chính:
- Replace the left `Cây nghiệp vụ` menu only on project-scoped work item detail pages with a workflow > Checklist / Step timeline so users can see progress and jump between runtime work items from the same project workspace.

Will be done:
- Keep project workspace navigation unchanged on non-work-item project pages.
- In `/projects/:id/work-items/:workItemId`, render a workflow timeline sidebar using existing `/projects/:id/operations` data.
- Show overall step progress and per-node progress.
- Show node > Checklist / Step rows with status, active state, evidence/form hints, and disabled state for steps without a `work_item_id`.
- Let clicking an enabled step open the corresponding project-scoped work item detail route on the right.
- Preserve backend APIs, workflow gates, form/evidence/signature/PDF behavior, query invalidation, and runtime workflow semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `Approve`.
- Re-read Consflow frontend workflow, workflow orchestration, module memory, and UI guardrail skills.
- Confirmed `/projects/:id/operations` already provides `pipeline_run` and `workflow_timeline`; no backend or migration change is needed for this scope.
- Updated `frontend/src/modules/projects/ProjectWorkspaceLayout.tsx` so project-scoped work item detail pages render a `Workflow` sidebar instead of `Cây nghiệp vụ`.
- Kept the existing `Cây nghiệp vụ` sidebar unchanged for non-work-item project workspace pages.
- Added overall step progress, per-node progress, node > Checklist / Step rows, active state, disabled state for steps without `work_item_id`, and project-scoped links to work item detail pages.
- Preserved backend APIs, route contracts, workflow gates, form/evidence/signature/PDF behavior, and query invalidation semantics.
- Ran `cd frontend && npm run build` successfully; Vite reported only the existing large chunk warning.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Official Workflow Business Forms Product Grade

Status: complete

Mục đích chính:
- Turn official client workflow template `40000000-0000-0000-0000-000000000020` into a production-grade operational workflow where every Node > Step opens the correct business form, checklist, evidence gate, or domain panel.
- Remove user-facing generic wording from production work-item execution UI. Users must see business language, not implementation language such as `Generic submission`, `Workflow Form`, or `Trường dữ liệu`.

Evidence gathered:
- `backend/migrations/V38__client_workflow_template.sql` defines the official client workflow template with 12 nodes and 52 steps.
- In V38, 51/52 steps map to `builtin_generic_submission`; step 2 is evidence-only with no form; step 11 maps to `client_design_branch_decision`.
- `backend/migrations/V40__vietnamese_official_workflow_text.sql` only cleans Vietnamese labels/descriptions and does not replace the generic form mapping.
- Existing business UI/form surfaces in the repo include planning daily report, meeting minutes, planning checklist, evidence, BOQ unit price lifecycle, subcontractor checklist, owner/client checklist, construction daily report, reconciliation panels, material snapshot, surplus handling, incoming/outgoing acceptance, input/output invoices, payment schedules, debt reconciliation, cashflow, tax, and profit reporting.

Will be done:
- Save this active plan as the shared source-of-truth before implementation.
- Produce a review package before any code, migration, or UI change.
- Produce a Mockup as Text before any UI change.
- Classify all 52 steps as one of: keep evidence-only, remap to existing business form, remap to existing domain panel, create a new `client_*` workflow form, or remove generic fallback.
- Design field-level specs for every new `client_*` workflow form: field key, label, type, required flag, validation, help text, signature/PDF needs, and completion gate.
- Add only new migrations for form templates/versions/fields and step remapping. Do not edit already-applied migrations.
- Preserve workflow template/runtime separation. Publishing, remapping, and cloning must not silently mutate submitted or historical project runs.
- Define runtime migration handling for existing draft instances separately from submitted/signed/PDF instances.
- Update work-item UI labels so business users see the current step's business form title, expected input, and next action, not internal generic template names.
- Verify backend/frontend contracts around form detail, form submit, signature, PDF, evidence, required gates, activity/audit, and RBAC.
- Build a UAT script that starts a project from template `40000000-0000-0000-0000-000000000020` and checks every node/step mapping.

Done:
- User confirmed the plan-save direction with `OK`.
- User approved implementation with `OK triển khai với điều phối subagents phù hợp, đúng skills và hiệu quả`.
- Re-read Consflow repo architecture and workflow orchestration guardrails.
- Re-read V38 official workflow template and V40 Vietnamese text cleanup migration.
- Re-read the user-provided business UI/form catalog attachment.
- Confirmed the current plan was not yet present in `docs/current-states.md` before this entry.
- Delegated frontend work-item form-copy cleanup to a frontend subagent; build passed in that subagent's scope.
- Delegated migration/schema safety review to a DB reviewer subagent; reviewer confirmed the safe path is a new V41 migration, published versions, and template-step remap only.
- Added `backend/migrations/V41__official_workflow_business_forms.sql` to seed official `client_*` business forms and remap template `40000000-0000-0000-0000-000000000020` away from `builtin_generic_submission` for future runs.
- Preserved step 2 as evidence-only and preserved step 11 branch routing through `needs_design` on `client_design_branch_decision` version 2.
- Kept runtime instances, submitted answers, signatures, and PDF exports untouched.
- Updated work-item execution UI copy so user-facing execution labels say `Hồ sơ nghiệp vụ`, `Nội dung cần xử lý`, and `Thông tin nghiệp vụ` instead of generic form-engine wording.
- Updated work-item primary panel routing so document-upload steps with a workflow form open the business hồ sơ, incoming acceptance steps open incoming acceptance, procurement material steps with forms open hồ sơ nghiệp vụ, and subcontractor quantity dossier steps open reconciliation.
- Ran `make migrate-check`: 42 migrations checked successfully.
- Applied V41 successfully to the running Docker PostgreSQL stack through `/app/consflow-migrate`: `applied=1 skipped=40`.
- Verified template `40000000-0000-0000-0000-000000000020` in PostgreSQL: 52 total steps, 51 steps with forms, 0 steps on `builtin_generic_submission`, step 2 remains evidence-only, step 11 remains `client_design_branch_decision` v2 with `needs_design`.
- Added `backend/migrations/V42__remap_official_workflow_draft_forms.sql` to remap existing blank draft runtime instances from official workflow runs away from `builtin_generic_submission`.
- Applied V42 successfully to the running Docker PostgreSQL stack through `/app/consflow-migrate`: `applied=1 skipped=41`.
- Verified existing official workflow runtime instances in PostgreSQL: 0 `workflow_form_instances` remain on `builtin_generic_submission`.
- Ran `GOCACHE=$PWD/.gocache go test ./...` in `backend`: pass.
- Ran `npm run build` in `frontend`: pass; Vite reported only the existing large chunk warning.
- Browser/Playwright verification was skipped per repo rule because it was not explicitly requested in this turn.

Source-of-truth mapping:

| # | Node | Step / checklist | Current mapping | Product-grade target | Implementation status |
|---|---|---|---|---|---|
| 1 | Khảo sát | Khảo sát yêu cầu của chủ đầu tư | `builtin_generic_submission` | New `client_survey_requirement_intake` form for owner requirements, scope, stakeholders, constraints, deadline, and notes | Implemented in V41 |
| 2 | Khảo sát | Khảo sát / Chụp / Tải ảnh hiện trạng | No form; photo evidence required | Evidence-only step with guided photo categories, location/area, before-condition notes, and required evidence gate | Evidence-only preserved |
| 3 | Khảo sát | Thiết kế nắm phong cách | `builtin_generic_submission` | New `client_design_style_brief` form for style direction, references, materials, color tone, exclusions, and owner preference | Implemented in V41 |
| 4 | Khảo sát | Ghi nhận kích thước chính | `builtin_generic_submission` | New `client_site_measurement_record` form for measured areas, dimensions, unit, tolerance, measurement owner, and attachment evidence | Implemented in V41 |
| 5 | Khảo sát | Xác định rủi ro mặt bằng | `builtin_generic_submission` | New `client_site_risk_assessment` checklist for structural, MEP, access, safety, schedule, and cost risks | Implemented in V41 |
| 6 | Khảo sát | Biên bản khảo sát | `builtin_generic_submission` | New `client_site_survey_minutes` form with survey conclusion, participants, issues, required attachments, signature, and PDF | Implemented in V41 |
| 7 | Họp báo cáo khảo sát | QS / Bốc khối lượng không thiết kế | `builtin_generic_submission` | BOQ/takeoff business surface plus a takeoff summary form if embedded work-item submission is needed | Implemented in V41 |
| 8 | Họp báo cáo khảo sát | Thiết kế / Lên bản bố trí mặt bằng | `builtin_generic_submission` | New `client_design_layout_submission` form for layout revision, file links, assumptions, owner feedback, and readiness | Implemented in V41 |
| 9 | Họp báo cáo khảo sát | Thiết kế / Lên bản vẽ 3D | `builtin_generic_submission` | New `client_design_3d_submission` form for 3D package, revision, render files, owner comments, and approval readiness | Implemented in V41 |
| 10 | Họp báo cáo khảo sát | Thiết kế / Lên bản vẽ thi công | `builtin_generic_submission` | New `client_construction_drawing_submission` form for drawing set, revision, issued-for status, and attachment evidence | Implemented in V41 |
| 11 | Họp báo cáo khảo sát | Biên bản cuộc họp | `client_design_branch_decision` | Business meeting minutes with participants, decisions, tasks, and routing field `needs_design`; branch decision remains but cannot be the whole meeting form | Implemented in V41 |
| 12 | Thiết kế | Trao đổi / Lên bản vẽ bố trí mặt bằng | `builtin_generic_submission` | New `client_design_review_layout` form for design iteration, comments, blockers, attachment, and next revision | Implemented in V41 |
| 13 | Thiết kế | Trao đổi / Lên bản vẽ 3D | `builtin_generic_submission` | New `client_design_review_3d` form for 3D iteration, comments, acceptance criteria, attachment, and next revision | Implemented in V41 |
| 14 | Thiết kế | Trao đổi / Lên bản vẽ thi công | `builtin_generic_submission` | New `client_design_review_construction_drawing` form with document upload, drawing status, revision, and review notes | Implemented in V41 |
| 15 | Thiết kế | Chốt với Sale / Chủ đầu tư | `builtin_generic_submission` | Owner/client confirmation checklist or new `client_design_owner_approval` form with approval status and signature/PDF | Implemented in V41 |
| 16 | Thiết kế | Đưa bản vẽ qua bên QS | `builtin_generic_submission` | New `client_design_to_qs_handoff` form with handoff package, drawing revision, BOQ assumptions, and receiver confirmation | Implemented in V41 |
| 17 | BOQ (Không thiết kế) | QS lấy giá từ thầu phụ / tổ đội / vật tư | `builtin_generic_submission` | BOQ unit price lifecycle plus supplier price collection fields | Implemented in V41 |
| 18 | BOQ (Không thiết kế) | QS / PM / GĐ kiểm tra đơn giá đầu vào | `builtin_generic_submission` | BOQ price check/approval form with margin, variance, and reviewer decision | Implemented in V41 |
| 19 | BOQ (Không thiết kế) | GĐ rà soát định mức quản lý | `builtin_generic_submission` | Director management-rate review form with approval, conditions, and audit note | Implemented in V41 |
| 20 | BOQ (Không thiết kế) | QS / PM / GĐ gửi BOQ qua chủ đầu tư | `builtin_generic_submission` | BOQ submission package with document upload, sent date, recipient, version, and evidence | Implemented in V41 |
| 21 | BOQ (Có thiết kế) | QS / Sale / PM gửi BOQ qua chủ đầu tư | `builtin_generic_submission` | BOQ-after-design submission package with drawing revision linkage and owner delivery evidence | Implemented in V41 |
| 22 | Đầu Vào | QS / Bốc khối lượng | `builtin_generic_submission` | Procurement input takeoff form or BOQ-derived procurement quantity review | Implemented in V41 |
| 23 | Đầu Vào | Mua hàng / QS mua hàng với tổ đội / thầu phụ / NCC vật tư | `builtin_generic_submission` | Procurement sourcing form for vendor, scope, quotes, lead time, comparison, and selected option | Implemented in V41 |
| 24 | Đầu Vào | Mua hàng / QS ký hợp đồng / hồ sơ tạm ứng | `builtin_generic_submission` | Input contract/advance package using contract document upload and input payment schedule/advance surface | Implemented in V41 |
| 25 | Đầu Vào | PM / GĐ rà soát & kiểm tra đầu vào | `builtin_generic_submission` | Procurement approval checklist with scope, price, contract, advance, risk, and approve/reject decision | Implemented in V41 |
| 26 | Đàm phán giá | Sale / QS / PM / GĐ đàm phán thầu phụ | `builtin_generic_submission` | Negotiation log form with parties, original price, negotiated price, concessions, and decision | Implemented in V41 |
| 27 | Đàm phán giá | QS / PM chốt lại định mức | `builtin_generic_submission` | Final rate/margin confirmation form linked to BOQ unit price lifecycle | Implemented in V41 |
| 28 | Đàm phán giá | GĐ rà soát & chốt báo cáo đơn giá | `builtin_generic_submission` | Director final price approval form with signed decision and audit note | Implemented in V41 |
| 29 | Chốt Hợp đồng với chủ đầu tư | QS lên báo giá & hợp đồng | `builtin_generic_submission` | Owner quote/contract package form with quote version, contract draft, payment terms, and files | Implemented in V41 |
| 30 | Chốt Hợp đồng với chủ đầu tư | Kế toán / Pháp lý kiểm tra hợp đồng | `builtin_generic_submission` | Legal/finance contract review checklist with risk flags, tax/payment terms, and required corrections | Implemented in V41 |
| 31 | Chốt Hợp đồng với chủ đầu tư | Gửi file mềm cho chủ đầu tư | `builtin_generic_submission` | Owner soft-file submission form with document list, delivery channel, recipient, and sent evidence | Implemented in V41 |
| 32 | Chốt Hợp đồng với chủ đầu tư | PM / GĐ rà soát và ký duyệt | `builtin_generic_submission` | Internal contract approval/signature form with PDF/signature gate | Implemented in V41 |
| 33 | Chốt Hợp đồng với chủ đầu tư | In và gửi bản cứng cho chủ đầu tư | `builtin_generic_submission` | Hardcopy dispatch form with print set, courier/hand delivery, receiver, date, and evidence | Implemented in V41 |
| 34 | QA & QC / Thi công | QS / PM / Thiết kế / QA họp & phân công | `builtin_generic_submission` | Meeting minutes plus construction assignment matrix | Implemented in V41 |
| 35 | QA & QC / Thi công | Chốt biên bản cuộc họp | `builtin_generic_submission` | Meeting minutes v2 with PDF/signature, action items, owners, and due dates | Implemented in V41 |
| 36 | QA & QC / Thi công | QA đảm bảo chất lượng / kế hoạch thi công | `builtin_generic_submission` | QA/construction plan checklist with inspection criteria and schedule | Implemented in V41 |
| 37 | QA & QC / Thi công | QA báo cáo liên tục trong quá trình thi công | `builtin_generic_submission` | Existing construction daily report form | Implemented in V41 |
| 38 | Nghiệm thu Đầu Vào | Nghiệm thu thầu phụ / tổ đội | `builtin_generic_submission` | Incoming acceptance panel plus subcontractor checklist/reconciliation | Implemented in V41 |
| 39 | Nghiệm thu Đầu Vào | Nghiệm thu vật tư đầu vào vs đầu ra | `builtin_generic_submission` | Incoming acceptance panel plus material before/after snapshot | Implemented in V41 |
| 40 | Nghiệm thu Đầu Vào | QS hồ sơ kiểm tra khối lượng thầu phụ | `builtin_generic_submission` | Subcontractor reconciliation panel with quantity and pricing check | Implemented in V41 |
| 41 | Nghiệm thu Đầu Vào | GĐ rà soát hồ sơ kiểm tra khối lượng thầu phụ | `builtin_generic_submission` | Director approval checklist/signature for subcontractor quantity dossier | Implemented in V41 |
| 42 | Nghiệm thu Đầu Vào | QS hồ sơ thanh toán cho thầu phụ | `builtin_generic_submission` | Input invoice/payment schedule/payable dossier surface | Implemented in V41 |
| 43 | Nghiệm thu Đầu Vào | GĐ ký nghiệm thu hồ sơ thanh toán | `builtin_generic_submission` | Signed payment acceptance approval with PDF/signature gate | Implemented in V41 |
| 44 | Nghiệm thu Đầu Vào | Kế toán kiểm tra hồ sơ thanh toán | `builtin_generic_submission` | Payable reconciliation/input invoice review panel | Implemented in V41 |
| 45 | Nghiệm thu Đầu Vào | Thanh toán cho thầu phụ | `builtin_generic_submission` | Input payment plan/payment execution surface | Implemented in V41 |
| 46 | Nghiệm thu Đầu Ra | QA / QS / PM / GĐ nghiệm thu & ký duyệt công trình | `builtin_generic_submission` | Outgoing acceptance panel plus owner/client checklist and signature | Implemented in V41 |
| 47 | Nghiệm thu Đầu Ra | GĐ hồ sơ nghiệm thu & ký duyệt | `builtin_generic_submission` | Director outgoing acceptance approval with PDF/signature gate | Implemented in V41 |
| 48 | Nghiệm thu Đầu Ra | Hồ sơ thanh toán & quyết toán & ký duyệt | `builtin_generic_submission` | Output billing/receivable settlement package with document upload and approval | Implemented in V41 |
| 49 | Nghiệm thu Đầu Ra | Xuất hóa đơn | `builtin_generic_submission` | Output invoice PDF panel | Implemented in V41 |
| 50 | Nghiệm thu Đầu Ra | Thanh toán nhiều kỳ | `builtin_generic_submission` | Output billing schedule/payment collection panel | Implemented in V41 |
| 51 | Kết thúc: Bàn giao + Đóng tài chính | Kế toán thầu phụ xuất hóa đơn về công ty | `builtin_generic_submission` | Input invoice PDF/AP closure surface | Implemented in V41 |
| 52 | Kết thúc: Bàn giao + Đóng tài chính | Kế toán xuất hóa đơn cho chủ đầu tư | `builtin_generic_submission` | Output invoice PDF/AR closure plus cashflow/tax/profit closure checks | Implemented in V41 |

Product-grade definition of done:
- 52/52 steps are explicitly classified and implemented with no accidental generic fallback in the official client template.
- 0 user-facing occurrences of `Generic submission`, `Trường dữ liệu`, or `Workflow Form` remain in the work-item execution experience for official client workflow runs.
- Every required form has domain-specific labels, help text, validation, and completion summary.
- Evidence-only steps explain exactly what evidence the user must upload.
- Domain-panel steps open the relevant business panel with clear empty/loading/error states.
- Submitted/signed/PDF historical form instances remain auditable and are not silently rewritten.
- Draft runtime instances have a documented migration/remap path before any data rewrite is implemented.
- Backend tests, frontend build, and migration validation pass. Browser verification remains skipped unless explicitly requested by the user in that turn.

Implementation review package:
- Completed before code changes and approved by the user before implementation.

## Work Item Remove Workflow Position Rail

Status: complete

Will be done:
- Remove the left `Vị trí workflow` rail from the work item detail body.
- Keep `Node`, `Bước`, `Minh chứng`, and `Biểu mẫu` in the existing `Quy trình đang chạy` header stats.
- Change the work item detail body from three columns to two columns: main content and right `Việc cần làm`.
- Do not add a breadcrumb replacement.
- Preserve workflow actions, required gates, evidence, activity, forms, backend APIs, query invalidation, and runtime workflow semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `OK`.
- Re-read Consflow frontend workflow and workflow orchestration skills and target work item detail code.
- Removed the `WorkItemProgressRail` render from the work item detail body.
- Removed the unused `WorkItemProgressRail` component implementation.
- Changed the work item detail body grid from three columns to two columns: main content plus right `Việc cần làm`.
- Kept `Node`, `Bước`, `Minh chứng`, and `Biểu mẫu` in the existing `Quy trình đang chạy` header stats.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Workflow Position Breadcrumb

Status: reverted

Before change snapshot:
- User screenshot: `Microsoft Edge Appshot 2026-07-06T10-39-01.914Z.png`.
- Work item detail body is a three-column grid: left `Vị trí workflow`, center current step/support content, right `Việc cần làm`.
- Left `Vị trí workflow` rail is rendered by `WorkItemProgressRail` in `frontend/src/modules/work-items/WorkItemDetailDialog.tsx`.
- `WorkItemProgressRail` displays four vertical items: `Công trình`, `Node quy trình`, `Bước`, `Đầu việc`.
- `WorkItemWorkspaceHeader` currently shows `Quy trình đang chạy` with title, status/project/priority chips, context stats for `Node`, `Bước`, `Minh chứng`, `Biểu mẫu`, and next action.

Attempted change:
- Remove the left `Vị trí workflow` column from the work item detail body.
- Move the workflow position information into a compact breadcrumb inside the `Quy trình đang chạy` header.
- Change the work item detail body from three columns to two columns: main content and right `Việc cần làm`.
- Preserve workflow actions, required gates, evidence, activity, forms, backend APIs, query invalidation, and runtime workflow semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Attempt done:
- User approved implementation and explicitly requested preserving the before-change reference for comparison.
- Re-read Consflow frontend workflow and workflow orchestration skills, AGENTS.md, current state notes, and target work item detail code.
- Removed the `WorkItemProgressRail` left column from the work item detail body.
- Changed the work item detail body grid from three columns to two columns: main content plus right `Việc cần làm`.
- Added compact `Vị trí workflow` breadcrumb inside `Quy trình đang chạy` with `Công trình > Node > Bước > Đầu việc`.
- Kept `Minh chứng` and `Biểu mẫu` as header stats after moving `Node` and `Bước` into the breadcrumb.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

Revert done:
- User requested `Revert lại đoạn vừa rồi đi`.
- Restored the three-column work item detail body with left `Vị trí workflow`, center content, and right `Việc cần làm`.
- Restored `WorkItemProgressRail` and removed the compact breadcrumb from `Quy trình đang chạy`.
- Restored header stats for `Node`, `Bước`, `Minh chứng`, and `Biểu mẫu`.
- Kept the earlier sticky offset fix for the restored left rail: `xl:top-[5.5rem]`.
- Ran `cd frontend && npm run build` successfully after the revert.

## Work Item Sticky Side Panel Offset

Status: complete

Will be done:
- Retouch only sticky positioning for the side panels inside `frontend/src/modules/work-items/WorkItemDetailDialog.tsx`.
- Move `Vị trí workflow` and `Việc cần làm` sticky panels below the sticky project top bar so their headers remain visible while scrolling.
- Keep the existing three-column work item layout, workflow actions, evidence/forms/activity behavior, backend APIs, query invalidation, and workflow gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `OK`.
- Re-read Consflow frontend workflow and diagnose skills, current state notes, and target `WorkItemDetailDialog` sticky panel code.
- Updated both work item side panels from `xl:top-0` to `xl:top-[5.5rem]` so their headers sit below the sticky project top bar.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Evidence Search Row Layout

Status: complete

Will be done:
- Retouch only the search/filter layout inside `frontend/src/modules/work-items/EvidenceGallery.tsx`.
- Move the `Tìm minh chứng, đầu việc, mô tả` search input to its own full-width row above the evidence filters.
- Keep the evidence filter segmented buttons below the search input.
- Keep evidence data, upload, review, search/filter behavior, backend APIs, query invalidation, and work item workflow gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `làm đi`.
- Re-read Consflow frontend workflow skill and target `EvidenceGallery` code.
- Moved the evidence search input to its own full-width row above the filter group.
- Kept the evidence filter segmented buttons below search without the previous desktop two-column split.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Evidence Filter Affordance

Status: complete

Will be done:
- Retouch only the evidence filter row inside `frontend/src/modules/work-items/EvidenceGallery.tsx`.
- Make the `Tất cả`, `Ảnh`, `Tài liệu`, status filters read as interactive filter controls instead of passive chips or labels.
- Rename the `submitted` filter label from `Đã gửi` to `Đang chờ` to match the current `draft/submitted/pending` filter logic.
- Keep evidence data, upload, review, search, backend APIs, query invalidation, and work item workflow gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `approve`.
- Re-read Consflow frontend workflow skill, UI guardrails, current plan notes, and target `EvidenceGallery` code.
- Added a visible `Lọc minh chứng` filter group around the evidence filter controls.
- Changed evidence filter buttons to clearer segmented filter controls with active state, active icon, `aria-pressed`, and count badges.
- Renamed the `submitted` filter label from `Đã gửi` to `Đang chờ` while keeping the existing `draft/submitted/pending` filter behavior unchanged.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Detail Project Workspace Route

Status: complete

Will be done:
- Add a project-scoped work item detail route under `/projects/:projectId/work-items/:workItemId` so the detail page stays inside the project workspace shell.
- Keep `/work-items/:id` as a fallback route for callers that do not have project context.
- Update work item detail navigation to prefer the project-scoped route when `project_id` is available.
- Remove standalone work item focus mode from `AppShell`; project workspace focus mode remains responsible for project-scoped detail pages.
- Keep backend routes, API contracts, workflow gates, form/evidence/signature/PDF behavior, and query invalidation unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the correction with `Approve`.
- Re-read Consflow frontend/workflow orchestration skills, route shell, current work item detail page, navigation helper, and current work item entry points.
- Added the nested project workspace route `/projects/:id/work-items/:workItemId`.
- Updated `WorkItemDetailPage` to resolve either nested `workItemId` or fallback top-level `id` and to return to the project overview when no `returnTo` is supplied.
- Updated work item navigation links from project detail, my work, planning runtime, planning daily reports, meeting minutes, and planning checklists to pass project context.
- Removed the standalone work item focus mode from `AppShell`; project-scoped work item detail now relies on project focus mode.
- Kept `/work-items/:id` as a fallback route for callers without project context.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Detail Full Page

Status: complete

Will be done:
- Add a full-page work item detail route at `/work-items/:id`.
- Reuse the existing work item detail workspace so dialog and page behavior stay consistent without duplicated domain logic.
- Add return navigation so callers can open the page and return to the original screen.
- Convert current work item open actions from modal state to route navigation where feasible.
- Keep backend routes, API contracts, mutations, query invalidation, and workflow domain gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User selected implementation direction 2: open work item detail as a page and back to the caller.
- Re-read Consflow frontend/workflow orchestration skills, router, dialog shell, current work item dialog, and current callsites.
- Added a reusable work item detail workspace extracted from the dialog body.
- Added `/work-items/:id` page route shell with a `Quay lại` action.
- Added a small helper for work item detail links with safe `returnTo` paths.
- Converted project detail, planning runtime, my work, daily reports, meeting minutes, and planning checklists work-item open actions to navigate to the full-page detail route.
- Added work item focus mode in `AppShell` so `/work-items/:id` uses full page width without the global shell/sidebar.
- Preserved the existing dialog wrapper as a reuse path, but current work item entry points now open the page route.
- Kept backend APIs, workflow actions, form/evidence/signature/PDF mutations, and query invalidation behavior inside the shared detail workspace unchanged.
- Ran `cd frontend && npm run build` successfully after the route/page changes and again after enabling work item focus mode.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Main And Support Single Column Retouch

Status: complete

Will be done:
- Refactor only the `Nội dung chính` and `Thông tin hỗ trợ` areas in the work item detail workspace.
- Keep the overall work item dialog layout unchanged: workflow rail, central content, and right action panel stay as-is.
- Make `WorkflowFormPanel` stack form summary, fields, signature/PDF, and history in one column.
- Make workflow form fields full width inside `Nội dung chính`.
- Change `Thông tin hỗ trợ` selectors from card-like buttons into a proper tab strip.
- Keep backend routes, API contracts, mutations, query invalidation, and domain panels unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User clarified the correct scope: `Nội dung chính` and `Thông tin hỗ trợ`.
- User approved implementation with `làm đi`.
- Re-read Consflow frontend/workflow orchestration skills, current state notes, and target work item UI code.
- Refactored `WorkflowFormPanel` so the workflow form summary, fields, signature/PDF, and events stack in one column.
- Removed the two-column field grid inside `Nội dung chính` so workflow form controls use full width.
- Refactored `WorkItemSupportingTabs` so the support header stacks in one column.
- Replaced card-like support tab buttons with a compact tab strip using tab semantics.
- Kept the overall work item dialog layout, backend APIs, mutations, and query invalidation unchanged.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Support Workspace Retouch

Status: complete

Will be done:
- Refactor only the `THÔNG TIN HỖ TRỢ` area inside the work item detail workspace.
- Keep backend routes, API response contracts, mutations, and query invalidation behavior unchanged.
- Retouch the support shell so evidence, activity, and workflow form/PDF share a clearer workspace hierarchy.
- Make evidence list/gallery-first with compact upload controls.
- Make activity timeline-first with a compact composer entry point.
- Make workflow form/PDF summary-first with validation, form fields, signature, and PDF artifacts separated clearly.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `Approve`.
- Re-read Consflow frontend workflow skill, AGENTS.md, router, UI guardrails, current plan notes, and target work item components.
- Refactored `WorkItemSupportingTabs` into a clearer support workspace shell with status-aware tabs and a next support action.
- Changed work-item activity support to timeline-first with a compact composer toggle.
- Changed work-item evidence support to summary/list-first and moved upload into a compact expandable panel.
- Added a list variant to `EvidenceGallery` while preserving the existing card default for project files.
- Reworked `WorkflowFormPanel` into an action center, processing summary, form fields section, artifacts column, and form event panel.
- Removed the old unused workflow form summary helper after replacing it with the new summary layout.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Workflow Detail Workspace UI

Status: complete

Will be done:
- Redesign `WorkItemDetailDialog` into a clearer workflow detail workspace with a left workflow-position rail, central current-step content, and right action panel.
- Keep existing work item actions, evidence, activity, workflow form, and domain-specific panels without changing backend contracts.
- Move competing footer actions into a single action hierarchy with one primary CTA and secondary actions.
- Add or reuse shared sizing/layout support needed by the dialog.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved implementation with `Làm đi` after reviewing text and image mockups.
- Read Consflow frontend and workflow orchestration skills, UI guardrails, router, current work item detail implementation, and existing dirty diffs.
- Added a `workspace` dialog size for wide operational detail layouts.
- Reworked `WorkItemDetailDialog` into a 3-zone layout: workflow position rail, current-step panel, and right-side action panel.
- Preserved existing APIs and domain panels; no backend contract or route change was introduced.
- Ran `cd frontend && npm run build` successfully.
- Skipped browser verification per repository rule because it was not explicitly requested in this turn.
- Tuned `Vị trí workflow` so it no longer stretches to the body height; it is sticky on wide viewports with internal timeline scrolling.

## Full Workflow Demo Seed

Status: complete

Will be done:
- Extend `backend/seed/seed.sql` with a full demo project that covers all current work item types and workflow form templates.
- Seed a runtime pipeline with nodes, steps, work items, workflow form instances, answers, evidence, activities, approvals, and domain records.
- Seed operational data for material snapshots, surplus handling, incoming/outgoing acceptance, subcontractor/client reconciliation, and finance/debt panels.
- Validate seed syntax and run the nearest feasible backend checks without browser verification.

Done:
- Review package approved by the user with `OK triển khai`.
- Confirmed this is a seed/demo-data change only; no schema, API, or UI contract change is intended.
- Added full workflow/form coverage seed data in `backend/seed/seed.sql`.
- Added seed upload fixtures for workflow form PDF, input invoice PDF, and output invoice PDF.
- Updated `backend/Makefile` so `make seed` copies seed upload fixtures into the configured upload directory/container volume.
- Validated `backend/seed/seed.sql` against the running Postgres container inside a rollback transaction.
- Applied the seed to the running local Docker Postgres with `cd backend && make seed`.
- Ran `cd backend && make test` successfully.

## Project-First Folder UI

Status: complete

Will be done:
- Make `/projects` the primary post-login entry for users with `project.view`.
- Reduce the main sidebar to high-level entry points instead of exposing every project domain module as a global navigation item.
- Convert `frontend/src/modules/projects/ProjectsPage.tsx` from a dense table into a folder/tree explorer for construction projects.
- Convert the project detail overview in `frontend/src/modules/projects/ProjectDetailPage.tsx` to a compact project operations summary that sits inside the project workspace layout.
- Add project focus mode for `/projects/:id/*` so the project business tree becomes the main menu and the global app sidebar/header is hidden.
- Keep existing project-scoped routes and backend APIs unchanged.
- Run `cd frontend && npm run build` after frontend edits.

Done:
- User approved the visual mockup direction: `/projects` as project explorer and project detail as a folder-style business tree.
- Updated `HomeRedirect` so users with `project.view` land on `/projects`.
- Reduced the main sidebar to project-first high-level navigation and removed the project domain modules from the primary global menu.
- Reworked `ProjectsPage` into a searchable/filterable folder tree explorer with expandable construction project rows.
- Reworked `ProjectDetailPage` overview into a folder-based operations workspace with a persistent business tree and compact work table.
- Added `ProjectWorkspaceLayout` focus mode for `/projects/:id/*` with project top bar, fixed business tree menu, and scoped content outlet.
- Nested all project-scoped routes under `ProjectWorkspaceLayout` while preserving their existing URLs and permission guards.
- Updated `AppShell` so project focus mode hides the global sidebar/header, matching the focused `pipeline-builder` behavior.
- Removed the old embedded business tree from `ProjectDetailPage`; the project workspace layout is now the single owner of that menu.
- Added query-tab support for project members/files links from the workspace tree.
- Replaced user-facing `PM` labels with `Phụ trách` and removed internal `Folder` / `Theo quyền` text from project folder rows.
- Kept backend APIs, routes, migrations, and project-scoped module routes unchanged.
- Ran `cd frontend && npm run build` successfully after both project-first and focus-mode changes.

## Consflow Impact Analysis Skill

Status: complete

Will be done:
- Add `consflow-impact-analysis` as a repo-scoped skill for feature overlap and logic collision analysis.
- Add UI metadata for the new skill.
- Wire the new skill into the Consflow skill README, module routing, and repo architecture workflow.
- Validate the skill artifact with targeted file inspection and `rg`.

Done:
- Review package approved by the user.
- Added `.agents/skills/consflow-impact-analysis/SKILL.md`.
- Added `.agents/skills/consflow-impact-analysis/agents/openai.yaml`.
- Wired `consflow-impact-analysis` into `.agents/skills/README.md`.
- Added impact-analysis pairing guidance to `consflow-module-memory`.
- Added cross-feature collision routing to `consflow-repo-architect`.
- Validated `SKILL.md` frontmatter and `openai.yaml` with Ruby YAML checks because the Python validator is missing `PyYAML` in this environment.
