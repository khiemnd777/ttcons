# Current States

## Planning To Construction Handoff

Status: complete

Mục đích chính:
- Bàn giao một lần các hạng mục giám sát đã được chuẩn bị trong Planning sang hàng đợi `Kiểm tra` để Giám sát viên chỉ thực hiện kiểm tra, xử lý/khắc phục và báo cáo tại hiện trường.

Will be done:
- Thêm contract `construction_handoffs`/`construction_handoff_items` với snapshot nguồn, actor/time audit, trạng thái và khóa idempotency; mỗi hạng mục được kích hoạt đúng một work item và một canonical construction check.
- Tạo API đọc preview, lưu/activate handoff atomic và hỗ trợ runtime hiện hữu mà không viết lại lịch sử; nguồn kế hoạch và check được liên kết hai chiều.
- Tách planning/pre-construction với execution trong official workflow revision mới; không mutate published template hoặc runtime copy lịch sử.
- Đồng bộ check/action lifecycle với generated work item và chặn rời node Thi công khi check/action từ bàn giao chưa hoàn tất.
- Thêm Planning handoff panel/dialog, trạng thái readiness và một CTA bàn giao; Thi công hiển thị nguồn `Từ kế hoạch` nhưng giữ manual check cho phát sinh ngoài kế hoạch.
- Cập nhật Golden fixtures/tests và central/backend/frontend/root-cause/workflow/planning/construction mappings; chạy backend format/tests/migration/seed validation, frontend build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the cross-domain review package and Mockup as Text with `OK triển khai theo review package này`.
- Loaded mandatory central/backend/frontend/root-cause maps, workflow/planning/construction domain guardrails, impact analysis, backend/frontend workflow and custom database migration instructions.
- Confirmed PostgreSQL through `database/sql`, custom checksum migration runner and a high-risk schema/transaction/API/UI change owned by `repository_domain.go`, service/handler/routes, workflow runtime, Planning and Operations.
- Confirmed existing dirty files belong to the previous approved dialog-width change and will be preserved.
- Added V66 with `construction_handoffs`/`construction_handoff_items`, snapshot hash/revision/actor audit, generated record links, `construction.handoff.manage`, the dedicated `client_construction_handoff_plan` form and published official workflow revision 2.
- Revision 2 preserves the immutable revision-1 runtime and separates node 9 `Kế hoạch thi công` from node 10 `Thi công`; future project runs use revision 2 while active/historical runs keep their selected template snapshot.
- Added typed read/activate routes and focused `construction_handoff.go` service/repository ownership. Preview prefers the dedicated submitted form and supports current revision-1 QA/assignment form imports; activation atomically/idempotently creates one handoff item, target-node work item and canonical check per source row. The client sends its previewed form revision and backend locks/rechecks it to reject stale-tab activation. Same-form-revision replay returns the existing handoff; a newer form revision replaces only unstarted rows and is rejected after work/evidence/action begins.
- Added `CONSTRUCTION_HANDOFF_REQUIRED` to the engine-v2 planning completion gate. Generated work stays pending until the normal construction node advance; a generated check cannot run early.
- Synchronized generated work lifecycle: check pass completes it, check fail blocks it and creates the canonical action, verification pass completes it, and verification fail keeps it blocked. Planning-generated checks are read-only snapshots; manual checks remain available for unplanned conditions.
- Added `ConstructionHandoffPanel` to Planning with readiness metrics, one bulk CTA, report-width inline editor, participant assignment, inline validation, dismissible toasts and read-only revision view. Operations lists/detail show `Từ kế hoạch · rev n` and preserve the shared perform/remediation dialogs.
- Added DB regression coverage for the planning completion blocker, atomic two-item generation, idempotent replay, guarded form-revision supersession and pass/fail/resolution/verification synchronization.
- Updated central/backend/frontend/root-cause plus planning/workflow/construction domain mappings. The completed Golden historical fixture was intentionally not rewritten to revision 2; disposable DB coverage validates the new revision while preserving the fixture's historical runtime contract.
- Validation passed: frontend production build; backend package tests; all 66 migrations on a fresh disposable PostgreSQL database; revision-2 schema check (`13` nodes, `53` steps, ordered `Kế hoạch thi công -> Thi công -> Nghiệm thu Đầu Vào`); focused DB regression; `git diff --check`.
- Browser verification was skipped because the user did not explicitly request it.

## Information-Dense Dialog Widths

Status: complete

Mục đích chính:
- Cho các dialog chứa nhiều biểu mẫu, bảng, tiêu chí, minh chứng hoặc panel liên quan tận dụng tốt chiều rộng desktop trong khi dialog xác nhận đơn giản vẫn gọn và mobile vẫn responsive.

Will be done:
- Giữ `sm`/`md`; mở rộng semantic sizes dùng chung thành `lg` 800px, `xl` 1120px, `report` 1320px và `workspace` 1480px, luôn giới hạn theo viewport.
- Chuyển form Tạo/Sửa Kiểm tra từ `lg` sang `xl`; giữ Báo cáo ở `report` và các workspace giàu thông tin ở tier hiện hữu để tự nhận chiều rộng mới.
- Không đổi API, validation, trạng thái, permission, footer CTA hoặc responsive single-column behavior.
- Cập nhật central/frontend mappings; chạy frontend build, rà soát Dialog usages và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Approve`.
- Reloaded the mandatory central/frontend mappings, frontend workflow skill and shared Dialog source before implementation.
- Confirmed the root cause is the legacy fixed maxima (672/896/1080/1280px) remaining after information-heavy dialogs adopted multi-column layouts.
- Expanded the shared responsive tiers to `lg` 800px, `xl` 1120px, `report` 1320px and `workspace` 1480px while preserving the existing viewport cap and compact `sm`/`md` tiers.
- Promoted the information-heavy Tạo/Sửa Kiểm tra form from `lg` to `xl`; report create/edit/view and existing rich workspaces automatically receive their approved wider tiers.
- Updated the central and frontend UI maps with the semantic size contract; no backend, API, permission, validation or business-domain mapping changed.
- Validation passed: all explicit Dialog usages were inventoried, the four widths were emitted in the production CSS, frontend production build and `git diff --check` passed.
- Browser verification was skipped because the user did not explicitly request it.

## One-Flow Daily Report Field Actions

Status: complete

Mục đích chính:
- Cho phép ghi báo cáo và chuyển phát sinh thành `Kiểm tra` hoặc `Cần xử lý` trong một dialog, một ngữ cảnh và một primary CTA; không đóng báo cáo hoặc đổi tab để tạo liên kết.

Will be done:
- Đổi form tạo báo cáo sang chế độ nhanh: nhóm công việc dạng tab gọn, blocker có lựa chọn xử lý, minh chứng ở luồng chính và nguồn lực/ghi chú mặc định thu gọn.
- Thêm shared compact composer cho kiểm tra/việc xử lý, dùng trực tiếp trong form tạo và chi tiết báo cáo; tự kế thừa công việc, tiêu đề và minh chứng nguồn.
- Lưu báo cáo rồi tạo các liên kết đã chuẩn bị bằng API V65 hiện hữu; nếu một liên kết lỗi, giữ báo cáo và chuyển đúng draft lỗi sang chi tiết để thử lại mà không tạo trùng.
- Tạo kiểm tra/việc từ báo cáo hiện hữu ngay trong dialog báo cáo, refresh timeline liên quan tại chỗ và không đổi tab.
- Giữ nguyên permission, evidence ownership, fail-to-action, action events, technical compatibility và handover blocker.
- Cập nhật central/frontend/root-cause/construction/quality mappings; chạy frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Approve`.
- Reloaded mandatory central/frontend/root-cause mappings plus frontend workflow, impact analysis and construction/quality domain guardrails.
- Confirmed implementation remains frontend-only over the existing V65 create routes and canonical evidence/status transitions.
- Added a shared compact composer for `Kiểm tra` and `Việc xử lý`; it reuses report work-item/evidence context, supports direct criteria, and keeps optional fields collapsed.
- Reworked report creation into one flow: quick context defaults, tabbed work groups, blocker-level `Chỉ ghi nhận`/`Tạo kiểm tra`/`Giao xử lý`, primary evidence capture, collapsed supplementary data, and one summary CTA.
- Orchestrated report plus linked records from one submit while keeping successful children idempotent; only failed drafts and failed evidence are retained for inline retry.
- Embedded creation in report detail so new checks/actions refresh the related timeline in place without closing the report or changing tabs.
- Reused the canonical construction create commands and preserved permission, evidence ownership, check-to-action transition, action events, technical compatibility, and handover blockers.
- Updated central/frontend/root-cause/construction/quality mappings for the new composer, one-flow behavior and partial-retry path.
- Validation passed with the frontend production build and `git diff --check`; browser verification was skipped because the user did not explicitly request it.

## Daily Report To Construction Resolution Traceability

Status: complete

Mục đích chính:
- Tạo chuỗi truy vết hiện trường có quan hệ nguồn rõ ràng `Nhật ký/Báo cáo -> Kiểm tra -> Cần xử lý -> Khắc phục/Xác nhận`, không suy luận theo ngày hoặc công việc.

Will be done:
- Thêm migration V65 với nguồn báo cáo tùy chọn cho kiểm tra và việc xử lý trực tiếp; việc sinh từ kiểm tra tiếp tục kế thừa nguồn qua `source_check_id` để tránh lưu trùng.
- Mở rộng typed contract, service/repository validation và construction workspace response; nguồn phải cùng công trình, và phải cùng work item khi báo cáo đã khóa work item.
- Bổ sung chi tiết báo cáo với trạng thái liên quan, CTA tạo kiểm tra/việc xử lý, form prefill nguồn/công việc/minh chứng và liên kết ngược trong View kiểm tra/việc xử lý.
- Giữ nguyên fail-to-action atomic transition, technical issue/rework/replanning bridge, permission hiện hữu, lịch sử action events và evidence ownership.
- Cập nhật Golden fixture, integration tests, central/backend/frontend/construction/quality mappings; chạy backend format/tests/migration checks, frontend build và `git diff --check`.
- Không backfill production bằng ngày/work item và không chạy browser verification vì người dùng không yêu cầu trong lượt này.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Reloaded mandatory central/backend/frontend/root-cause mappings plus frontend/backend workflow, impact analysis and construction/quality domain guardrails.
- Confirmed current storage only links `construction_action_items.source_check_id`; reports, checks and direct actions otherwise share contextual project/work-item fields without a causal report relation.
- Added migration V65 with optional daily-report source foreign keys for checks and direct actions, same-project/work-item validation, indexes, and inherited report traceability for actions generated from failed checks.
- Extended the construction workspace contracts to expose report date, weather and direct-versus-check linkage while preserving the existing issue, rework, replanning and action-event lifecycle.
- Added report-list counters and a linked check/action/remediation timeline in report detail, with permission-aware create actions and navigation through the existing shared check/action dialogs.
- Updated create/view flows so checks and actions can start from a daily report with locked work-item context and reusable report evidence; check/action detail links back to its source report.
- Updated Golden fixtures and validators with a complete report -> check -> failed action -> resolved remediation chain; the construction fixture reload is idempotent and leaves no open quality/handover blockers.
- Updated the central, backend, frontend, root-cause, construction-operations and quality/handover maps for the new relation and UI ownership.
- Validation passed: backend formatting/full tests, focused database integration test, 65-migration check, Golden fixture and validator checks, authenticated construction-workspace API smoke check, frontend production build and `git diff --check`.
- Browser verification was skipped because the user did not explicitly request it in this turn.

## Direct Construction Check Criteria And Evidence

Status: complete

Mục đích chính:
- Cho phép người dùng tạo một kiểm tra hoàn chỉnh ngay trong một dialog bằng cách nhập tiêu chí trực tiếp và đính kèm ảnh hiện trạng, không bắt buộc tạo mẫu trước.

Will be done:
- Thêm migration V64 với liên kết nhiều evidence theo vai trò `context`/`result`, backfill evidence kết quả hiện hữu và giữ `construction_checks.evidence_id` làm primary compatibility field.
- Mở rộng create/update check contract với tiêu chí trực tiếp và context evidence; complete check nhận nhiều result evidence, giữ validation cùng project/work item, status và actor/audit.
- Đổi form Tạo/Sửa sang tiêu chí inline thêm/xóa, `Nạp từ mẫu` là lựa chọn phụ, ảnh hiện trạng tùy chọn; View tách gallery ảnh hiện trạng và minh chứng kết quả.
- Giữ RFI-backed check read-only, quyền V63, fail-to-action và handover blocker không đổi.
- Cập nhật seed/validator/tests và central/backend/frontend/construction/quality mappings; chạy backend format/tests/migration/seed validation, frontend build, API smoke và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu trong lượt triển khai này.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Reloaded mandatory central/backend/frontend/construction/quality mappings plus impact-analysis and migration instructions.
- Confirmed the existing worktree contains the approved V62/V63 construction workspace and will be preserved.
- Added V64 `construction_check_evidence` with ordered `context`/`result` roles and legacy primary-result backfill; create/edit accepts direct criteria plus optional context evidence, while completion accepts up to 10 result evidence items.
- Updated the check dialogs so criteria are entered inline, a template only preloads editable rows, create/edit can capture current-state images, and View renders separate current-state and result galleries.
- Preserved pending RFI edit locks, same-project/work-item evidence validation, fail-to-action behavior and the legacy `construction_checks.evidence_id` primary result field.
- Extended the Golden construction fixture with one pending current-state image and three result evidence associations; added validator checks and focused normalization tests.
- Updated central/backend/frontend/root-cause plus construction/quality domain mappings for V64 contracts, ownership and troubleshooting paths.
- Ran backend format/tests, 64-migration check, fixture check, frontend production build and `git diff --check`: pass; Vite reported only the existing large-chunk advisory.
- Applied V64 through the custom migration runner (`applied=1 skipped=63`), reloaded only the Golden construction fixture, rebuilt/recreated backend and confirmed the final startup reports `applied=0 skipped=64`.
- Golden validation returned `GOLDEN_SAMPLE_VALIDATION_PASSED`; authenticated API smoke returned 4 checks, 1 context link, 3 result links and 2 direct criteria on the pending sample, and the typed edit API preserved its criteria/context evidence.
- Browser verification was skipped because the user did not explicitly request it in this turn.

## Construction Check And Action UI Redesign

Status: complete

Mục đích chính:
- Giúp người dùng tại công trường tìm nhanh kiểm tra hoặc việc cần xử lý, xem đầy đủ ngữ cảnh/minh chứng/lịch sử, thực hiện đúng hành động tiếp theo và sửa metadata an toàn mà không viết lại kết quả nghiệp vụ.

Will be done:
- Thiết kế lại List `Kiểm tra` và `Cần xử lý` thành bảng tác nghiệp dense trên desktop, card responsive trên mobile, có search, status queue, bộ lọc, row actions và đầy đủ loading/error/empty/no-result states.
- Tách shared dialog cho Create/Edit/View; giữ transition `Ghi kết quả`, `Gửi khắc phục`, `Xác nhận` theo trạng thái và permission hiện hữu.
- Thêm typed update API cho kiểm tra thủ công còn `pending` và action canonical còn `open`; khóa các trường nguồn/minh chứng/ảnh hưởng/trạng thái và các bản ghi technical backfill.
- Đồng bộ action edit với compatibility issue/rework trong một transaction, ghi actor/audit, giữ nguyên handover blocker và evidence/work-item gates.
- Thêm permission `construction.check.edit`, `construction.action.edit` bằng migration V63 và cấp cho cùng nhóm role đang có quyền request/create tương ứng.
- Cập nhật focused tests, Golden validator, central/backend/frontend/construction/quality mappings; chạy backend format/tests, migration/seed validation, frontend build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu trong lượt triển khai này.

Done:
- User approved the full review package and Mockup as Text with `OK triển khai`.
- Reloaded the mandatory central/backend/frontend/construction/quality/workflow maps plus impact-analysis, migration and git workflow instructions.
- Confirmed the current worktree already contains the approved V62 simplified construction workspace changes; unrelated/current user changes will be preserved.
- Replaced the compact check/action card grids with operational status queues, search and domain filters, dense desktop tables, mobile cards, overdue/urgent emphasis, row actions and contextual empty/no-result states.
- Added shared check dialogs for template creation, create/edit/view and evidence-gated pass/fail completion; check detail now shows criteria, context, protected evidence and its linked action.
- Added shared action dialogs for create/edit/view, resolution submission and verification; action detail now shows immutable source context, before/after protected evidence, impact state and the complete event timeline.
- Added typed `PUT /construction-checks/:id` and `PUT /construction-actions/:id` routes with project-scoped RBAC, request validation and audit. Pending RFI-backed checks and technical NCR/legacy issue backfills remain read-only.
- Kept action source/work item/evidence/impact/status immutable; safe metadata changes synchronize linked issue/rework title, description, priority/severity, assignee and due date in one transaction.
- Added migration V63 with `construction.check.edit` and `construction.action.edit`, restored both permissions in Golden seed resets and granted them to Admin, PM, QA/QC, Supervisor and Construction Team.
- Added focused editability regression coverage and extended Golden validation for permission/grant counts; updated central/backend/frontend/construction/quality mappings for the new files, routes, helpers and invariants.
- Ran `cd backend && make fmt`, `make test`, `make migrate-check`, `make seed-fixtures-check`, `make seed-validate`, `cd frontend && npm run build` and `git diff --check`: pass; Vite reported only the existing large-chunk advisory.
- Built/recreated only the backend container; the custom migration runner reported `applied=1 skipped=62`, and Golden validation returned `GOLDEN_SAMPLE_VALIDATION_PASSED`.
- Authenticated API smoke confirmed the workspace remains 1 template/4 checks/1 pending/1 completed action/0 active actions, pending manual check edit returns HTTP 200, completed action edit is rejected with `VALIDATION_ERROR`, and the Construction Team receives both new edit permissions.
- Browser verification was skipped per repository rule because the user did not explicitly request it.

## Simplified Construction Check And Action Workspace

Status: complete

Mục đích chính:
- Giúp người dùng tại công trường ghi nhận kết quả kiểm tra và xử lý một việc chưa đạt bằng ít thao tác nhất, không phải hiểu hay vận hành trực tiếp ITP, RFI, NCR, rework hoặc replanning.

Will be done:
- Thay bề mặt chính của `Thi công` bằng ba tab `Nhật ký`, `Kiểm tra`, `Cần xử lý`; bỏ dashboard kỹ thuật và các form ITP/RFI/NCR/rework/replanning khỏi luồng người dùng hiện trường.
- Thêm contract dữ liệu đơn giản cho mẫu kiểm tra, lần kiểm tra, việc cần xử lý và lịch sử xử lý; giữ liên kết kỹ thuật với dữ liệu cũ để bảo toàn audit và báo cáo.
- Cho phép hoàn tất kiểm tra bằng `Đạt/Không đạt + ảnh + ghi chú`; kết quả không đạt tự tạo một việc cần xử lý và chỉ hỏi người phụ trách, hạn xử lý, ảnh hưởng.
- Cho phép việc cần xử lý đi qua ba trạng thái `Cần xử lý -> Chờ xác nhận -> Đã xong`, gồm ảnh khắc phục, xác nhận đạt/không đạt và lịch sử thử lại.
- Đồng bộ cờ ảnh hưởng với issue/rework/replanning kỹ thuật phía sau và bổ sung action đang mở vào điều kiện sẵn sàng bàn giao.
- Thêm sample data cho project `30000000-0000-0000-0000-000000000020` với kiểm tra đạt, kiểm tra không đạt đã khắc phục, lịch sử hoàn tất và một kiểm tra đang chờ; không tạo blocker mở trên công trình đã bàn giao.
- Cập nhật central/backend/frontend/construction/quality mappings; chạy migration check, backend tests, seed validation, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the full simplified review package and requested implementation plus sample data for project `30000000-0000-0000-0000-000000000020`.
- Loaded the mandatory central/backend/frontend/construction/quality mappings plus repository architecture, backend, frontend, impact-analysis and database workflow instructions.
- Confirmed the existing operations page directly exposes technical Quality and Rework workspaces, while legacy project quality/rework routes redirect into those tabs.
- Confirmed legacy quality, issue, rework and replanning records can remain as technical compatibility links behind one new field-facing model without changing historical rows.
- Confirmed the target Golden Real project is already handed over, so sample data must contain no open simplified action blocker.
- Added migration V62 with canonical simplified check templates, checks, action items/events, `construction.*` permissions, technical-history backfill and open-action handover blocking.
- Added protected workspace/template/check/action APIs with typed validation, same-project/work-item evidence ownership, active-project assignee validation, audit events and atomic compatibility issue/rework/optional replanning synchronization.
- Replaced the operations quality/issues embeds and technical status cards with `Nhật ký`, `Kiểm tra`, `Cần xử lý`; all field create/result/resolution/verification interactions now use compact shared dialogs and camera/file evidence.
- Kept legacy global quality/rework consumers and technical API routes available, while project compatibility routes now redirect to `tab=checks|actions`.
- Added `golden_project_construction.sql` to the seed sequence and loaded project `30000000-0000-0000-0000-000000000020` with one active template, four checks, one resolved action and three action events; the only pending sample is a check, not an action blocker.
- Updated seed validation for the simplified dataset, `construction.*` role grants, open-action handover invariant, Golden Live untouched state and both terminal rework status spellings used by current code.
- Applied V62 through the custom migration runner in Docker, loaded only the dedicated construction seed, rebuilt/recreated only backend and confirmed the runner reports 62 migrations.
- Ran the Golden validator: `GOLDEN_SAMPLE_VALIDATION_PASSED`; authenticated API smoke returned 1 template, 4 checks, 1 pending check, 1 action, 0 open actions and 7 participants.
- Updated central/backend/frontend/construction/quality mappings for V62 ownership, field terminology, compatibility links and handover impact.
- Ran `cd backend && make test`, `make migrate-check`, `cd frontend && npm run build`, focused legacy-label searches and `git diff --check`: pass; Vite reported only the existing large-chunk advisory.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Contract Workspace Vietnamese Redesign

Status: complete

Mục đích chính:
- Giúp người quản lý nhanh chóng chọn đúng hợp đồng, nhận ra tình trạng và việc cần xử lý, rồi đi sâu vào phiên bản, mốc thanh toán, phụ lục hoặc khoản giữ lại bằng giao diện hoàn toàn tiếng Việt.

Will be done:
- Giữ bố cục danh sách–chi tiết trên màn hình lớn; bổ sung tìm kiếm, lọc chiều công nợ/trạng thái và luồng danh sách–chi tiết rõ ràng trên màn hình nhỏ.
- Đưa tình trạng, giá trị và một việc cần xử lý lên đầu chi tiết; chuyển bốn bảng dài thành các thẻ nội dung có số lượng.
- Việt hóa toàn bộ nhãn hệ thống của trang, loại hợp đồng, mốc thanh toán, công nợ, trạng thái, biểu mẫu và phản hồi; giữ nguyên mã, tên riêng và dữ liệu người dùng nhập.
- Dùng bảng dày trên màn hình rất rộng và thẻ dữ liệu trên màn hình hẹp để loại bỏ phụ thuộc vào bảng rộng 900px.
- Giữ nguyên API, quyền và các cổng nghiệp vụ phiên bản/mốc/phụ lục/giữ lại; cập nhật central/frontend/commercial/accounting mappings.
- Chạy `cd frontend && npm run build`, tìm chuỗi giao diện chưa Việt hóa và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Loaded the mandatory central/frontend/commercial/accounting mappings and inspected the overlapping dirty worktree before editing.
- Confirmed the existing contract list/detail APIs, permission gates and financial transitions already support a frontend-only redesign.
- Rebuilt the project contract workspace with a searchable/filterable master list, selected-contract summary, local Vietnamese contract/type/direction labels and mobile list-to-detail navigation.
- Added one permission-aware next-action band ordered through version review/approval/signing, contract activation, milestone eligibility/obligation, variation lifecycle and retention release without changing server transitions.
- Replaced four stacked 900px-minimum tables with count-bearing tabs, very-wide desktop tables and compact responsive record cards for versions, milestones, variations and retentions.
- Removed raw obligation ID fragments from the presentation, localized gross/net/hash/AP-AR concepts, formatted dates with the Vietnamese locale and translated the remaining shared `review`/`incorporated` status labels.
- Kept shared dialogs, inline form validation, dismissible mutation feedback, all current permission checks and the existing contract/AP/AR endpoints; no backend, migration, seed or response-contract change was required.
- Updated central/frontend/commercial/accounting mappings with the new action-first contract workspace ownership and preserved financial/commercial invariants.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran focused user-facing English-label searches and `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Contract Detail Project Permission Allowlist Fix

Status: complete

Mục đích chính:
- Khôi phục workspace chi tiết Hợp đồng cho người có quyền bằng cách cho phép helper phân quyền theo bản ghi phân giải `contracts.project_id`, không thay đổi UI, API contract hoặc nghiệp vụ AP/AR.

Will be done:
- Bổ sung `contracts` vào allowlist tên bảng an toàn mà `RecordProjectID` sử dụng.
- Thêm regression test khóa cả trường hợp hợp lệ và tên bảng có nội dung injection.
- Cập nhật backend function map và root-cause triage map cho lỗi `500 Bảng không hợp lệ`.
- Chạy format, focused backend tests, authenticated API smoke test và kiểm tra quyền; không chạy browser verification.

Done:
- User approved the diagnosed backend-only fix with `OK triển khai`.
- Reproduced `GET /api/contracts/65000000-0000-0000-0000-000000000201` returning HTTP 500 `INTERNAL_ERROR: Bảng không hợp lệ`, while the project contract list returned HTTP 200 and every contract-detail SQL query passed directly.
- Confirmed `GetContract` calls `requireRecordProjectPermission(..., "contracts", ...)`, `RecordProjectID` rejects names absent from `safeTable`, and the allowlist omitted `contracts`.
- Added `contracts` to the safe table allowlist and added a source-level regression test that preserves rejection of an unsafe table name.
- Updated the backend function map with the `contracts`/`RecordProjectID` dependency and added a root-cause recipe for HTTP 500 `Bảng không hợp lệ`; central/frontend/domain maps did not require changes because routes, response shapes, UI and commercial/finance invariants are unchanged.
- Ran the focused `TestSafeTableAllowsContractsForProjectRecordPermission`: pass.
- Ran `GOCACHE="$PWD/.gocache" go test ./...` under `backend`: pass.
- Built the backend image and recreated only the backend container; no migration or seed changed.
- Smoke-tested the authenticated Admin contract detail: HTTP 200 with 2 versions, 3 milestones, 1 variation and 2 retentions; the project contract list remains HTTP 200 with 3 records.
- Smoke-tested Warehouse without `contract.view`: HTTP 403 `FORBIDDEN`, confirming project RBAC remains enforced.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Members Role Label SQL Correction

Status: complete

Mục đích chính:
- Khôi phục workspace Thành viên cho người có quyền quản lý bằng cách sửa lỗi PostgreSQL `DISTINCT + ORDER BY` mà không đổi UI, permission hoặc contract API.

Will be done:
- Bọc tập nhãn distinct trong subquery và sắp xếp case-insensitive ở outer query.
- Thêm DB-backed regression coverage gọi `ProjectMemberOptions` để khóa lỗi SQLSTATE 42P10.
- Cập nhật backend mapping; chạy truy vấn pass/fail gốc, backend tests, build/recreate backend Docker và `git diff --check`.
- Không thay frontend, migration, seed hoặc chạy browser verification.

Done:
- User approved the diagnosed fix with `OK triển khai sửa SQL thành viên`.
- Reproduced the exact SQLSTATE 42P10 against PostgreSQL Docker and confirmed the subquery-wrapped candidate returns 13 distinct role labels.
- Wrapped the distinct role-label projection in an inner query and applied `ORDER BY lower(label)` only in the outer query; response shape and ordering semantics remain unchanged.
- Extended `TestProjectMemberLifecycleAndProjectManagerProtection` to call `ProjectMemberOptions`, require non-empty unique role labels and confirm the project operational role remains present.
- Updated the backend function map with the distinct/outer-ordering ownership rule; no central/frontend/domain map change was required because routes, response contract, UI, permissions and business invariants are unchanged.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran the DB-backed `TestProjectMemberLifecycleAndProjectManagerProtection` inside the Consflow Docker network: pass.
- Built the production backend image and recreated only the backend container; V60 remained applied and no migration changed.
- Smoke-tested the authenticated Admin member API: success, 12 items, 12 active, 13 role labels and `can_manage=true`.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Workflow Execution Workspace Redesign

Status: complete

Mục đích chính:
- Giúp người điều phối nhận ra ngay giai đoạn hiện tại, tiến độ thực thi, việc ưu tiên tiếp theo và mở đúng đầu việc; giữ điều phối node là hành động phụ có kiểm soát.

Will be done:
- Dùng chung một workflow execution workspace cho route `/projects/:id/pipeline` và tab `Quy trình` trong tổng quan công trình.
- Đưa current node, tiến độ bước loại trừ nhánh bỏ qua, count cần xử lý/chờ duyệt/bị chặn và một CTA `Mở việc tiếp theo` lên đầu.
- Dùng stage rail 280px trên desktop, selector trên mobile, bảng công việc desktop và card mobile với bộ lọc trạng thái/tìm kiếm.
- Chuyển request approval, advance và block vào khu vực `Thao tác giai đoạn`; chuyển `Tạo lại quy trình` vào menu phụ với dialog xác nhận.
- Giữ nguyên API, permission và runtime gates; không thay backend, migration, status key hoặc work-item detail behavior.
- Cập nhật central/frontend/workflow mappings; chạy `cd frontend && npm run build` và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Loaded the mandatory central/frontend/workflow mappings and inspected the overlapping dirty worktree plus the active Planning plan before editing.
- Confirmed the route `/projects/:id/pipeline` still renders the legacy `PipelineRuntimeSection`, while `ProjectDetailPage` already contains the approved action-first workflow presentation over `/projects/:id/operations`.
- Exported one `ProjectWorkflowWorkspace` and reused it in both the project Workflow tab and `/projects/:id/pipeline`, preserving the existing work-item detail navigation and backend runtime gates.
- Replaced the 12 equal-weight node cards on `/pipeline` with current-node/next-work summary, executable step progress, attention counts, 280px desktop stage rail, responsive selector, filtered desktop work table and mobile work cards.
- Moved request approval, advance and block into `Thao tác giai đoạn`; added required-reason block dialog and moved recreate into `Quản lý quy trình` with a confirmation dialog.
- Added project-operations loading/error states and invalidated the shared operations query after runtime actions so summary, stages and work rows refresh together.
- Updated central/frontend/workflow mappings for the shared workspace ownership, `/operations` presentation source and secondary node-control contract.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Planning Action-first Workspace

Status: complete

Mục đích chính:
- Giúp người dùng mở trang `Kế hoạch` và nhận ra ngay giai đoạn hiện tại, việc nào phải xử lý tiếp theo, cùng hàng đợi công việc đúng trạng thái runtime mà không phải quét toàn bộ biểu mẫu tương lai.

Will be done:
- Chuyển `Kế hoạch` từ inventory 51 biểu mẫu sang hàng đợi 52 đầu việc, mặc định ưu tiên blocker -> chờ duyệt/rà soát -> việc đang mở theo thứ tự workflow.
- Tách rõ `Cần xử lý`, `Chờ duyệt`, `Sắp tới`, `Hoàn tất` và `Tất cả`; giữ trạng thái hồ sơ, mức bắt buộc và minh chứng như metadata riêng.
- Đưa một việc tiếp theo, trạng thái node hiện tại, tiến độ bước thực hiện và các count cần chú ý lên đầu màn hình.
- Giữ node transition, phê duyệt, chặn node và audit đầy đủ trong route `Quy trình`; `Kế hoạch` chỉ liên kết sang đó.
- Trong `Kế hoạch`, chỉ hiển thị CTA khởi động khi chưa có run. Việc tạo lại run đang hoạt động thuộc route `Quy trình`, nằm trong menu quản lý và bắt buộc qua dialog xác nhận theo review package mới nhất đã được duyệt.
- Giữ nguyên API, route, permission và work-item detail gates; cập nhật central/frontend/project-planning/workflow mappings.
- Chạy `cd frontend && npm run build` và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Loaded the mandatory central/frontend/project-planning/workflow mappings, confirmed `PlanningPage.tsx` ownership and inspected the overlapping dirty worktree before editing.
- Confirmed the current `needsSubmit` classification ignores future `pending` work-item state, while the existing pipeline-run response already supplies all 52 ordered steps/work items and 51 form instances needed for a frontend-only redesign.
- Rebuilt `Kế hoạch` as an action-first queue over all ordered runtime steps, with default actionable, waiting approval/review, future pending, completed/history and all-work views.
- Added one next-work CTA ordered blocker -> waiting approval/review -> other open work, plus current-stage progress, actionable/waiting/blocked counts and submitted-form coverage.
- Kept workflow-form status, requirement and evidence as supporting metadata; added dense desktop rows, responsive mobile cards, search/type/status/requirement filters and loading/error/empty/filter-empty states.
- Kept planning rows navigation-only and routed node transition/approval/block operations to `Quy trình`; the active Planning header no longer presents `Khởi động / tạo lại quy trình` as its primary action.
- Preserved work-item return navigation, permission-gated detail behavior, API routes and frontend domain contracts; no backend, migration or seed change was required.
- Integrated the concurrent `/projects/:id/pipeline` execution-workspace redesign without restoring the removed legacy runtime section.
- Updated central/frontend/project-planning/workflow mappings with the queue classification and navigation-only invariants.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran focused `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Members Governance Workspace

Status: complete

Mục đích chính:
- Giúp thành viên công trình xem đúng danh bạ đang hoạt động; giúp người có quyền quản lý thêm, chỉnh vai trò/quyền và ngừng tham gia mà vẫn giữ lịch sử truy cập.

Will be done:
- Tách danh sách thành viên khỏi payload Operations thành API typed, permission-scoped và có capability quản lý rõ ràng.
- Cho thành viên đang hoạt động xem danh bạ active; chỉ người có `project.member.manage` được xem lịch sử inactive và thực hiện thay đổi.
- Thêm permission `project.member.manage`, unique membership theo công trình/nhân sự và luồng thêm/cập nhật/ngừng tham gia dạng transaction đồng bộ `project_members` với `user_roles` project scope.
- Chặn ngừng tham gia PM hiện tại, dùng soft-deactivate, ghi audit cho mọi thay đổi và không hiển thị `assigned_nodes` chưa có runtime semantics.
- Xây `ProjectMembersPanel` với tìm kiếm, bộ lọc, bảng/card responsive, dialog dùng chung, loading/empty/error/filter-empty và toast dismissible.
- Đồng bộ tab/sidebar theo visibility/capability backend; cập nhật central/backend/frontend/project-planning/admin-governance mappings.
- Chạy backend format/tests, migration check, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Loaded the mandatory central/backend/frontend/project-planning/admin-governance mappings and inspected the dirty worktree before editing.
- Confirmed current members are embedded in `/projects/:id/operations`, only Admin/Director receive them, the navigation still exposes the empty tab to project members, inactive rows are not filtered, and project-scoped `user_roles` are managed separately from `project_members`.
- Added typed protected member routes for list/add/update/soft-deactivate with `project.view` directory access and server-computed `project.member.manage` capabilities.
- Added migration V60 with member notes/updated timestamps, one-active-membership integrity, non-destructive duplicate normalization and the new permission granted to Admin/Director/PM.
- Added transactional membership persistence that synchronizes project-scoped `user_roles`, preserves global roles, requires effective `project.view`, excludes Admin/Director from assignable project roles, audits every change and blocks removal of the current PM.
- Removed members from the Operations payload/hidden-section contract and kept the legacy project detail member list active-only.
- Replaced the flat four-column table with `ProjectMembersPanel`: summary, search/department/role/status filters, dense desktop table, mobile cards, detail/add/edit/deactivate dialogs, inline validation, loading/error/empty/filter-empty states and dismissible feedback.
- Kept operational `role_ids`, project roles and global roles visually/contractually separate; left unused `assigned_nodes` out of the UI.
- Added frontend domain types plus focused normalization and DB-backed lifecycle/PM-protection tests; updated central/backend/frontend/project-planning/admin-governance mappings.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran `make migrate-check`: pass with 60 migrations.
- Built the backend Docker image and applied V60 through `/app/consflow-migrate`: one migration applied, 59 skipped.
- Ran `make seed-validate`: `GOLDEN_SAMPLE_VALIDATION_PASSED` for GOLDEN-LIVE, GOLDEN-NO-DESIGN and GOLDEN-REAL.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Audit Investigation Workspace

Status: complete

Mục đích chính:
- Giúp người quản trị nhanh chóng xác định ai đã làm gì, vào lúc nào, trên đối tượng nào và dữ liệu chi tiết của thay đổi là gì.

Will be done:
- Chuyển trang Audit theo công trình và toàn hệ thống thành workspace điều tra với tìm kiếm, bộ lọc server-side, phân trang, chế độ danh sách/timeline và dialog chi tiết chỉ đọc.
- Thu gọn tab Audit trong Tổng quan công trình thành preview năm sự kiện gần nhất, không lặp Timeline và bảng song song.
- Giữ nguyên `audit.view`, `requirePermission`, `requireProjectPermission`, cách ghi `audit_logs` và phạm vi nhìn thấy theo công trình.
- Thêm contract Audit có kiểu dữ liệu rõ ràng và response phân trang; trang toàn hệ thống hiển thị thông tin công trình nhưng vẫn áp `ProjectAccessFilter`.
- Cập nhật central/backend/frontend/admin-governance mappings; chạy backend format/tests, frontend build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu trong lượt này.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Loaded the mandatory central/backend/frontend/admin-governance mappings and completed the cross-contract impact matrix.
- Confirmed the only frontend consumers of `/audit-logs` and `/projects/:id/audit-logs` are `AuditPage` and `ProjectAuditPanel`.
- Confirmed no migration, seed, permission, audit-write, report or export change is required for the approved scope.
- Added typed `AuditLogFilter` and paginated Audit response with server-side project/event/target/actor/date/search filters, stable newest-first ordering and permission-scoped filter options.
- Preserved global `ProjectAccessFilter`, project `requireProjectPermission`, route `audit.view` guards and all existing `AddAudit` write behavior.
- Rebuilt `/audit` and `/projects/:id/audit` as a dense investigation workspace with global project context, list/timeline toggle, responsive cards, read-only metadata detail, refresh, empty/error states and server pagination.
- Replaced the project Audit tab's duplicated timeline/table layout with a five-event preview and removed the misleading preview-length badge.
- Updated Planning's audit timeline to consume `AuditLogPage.items` after the contract impact scan found it as a third endpoint consumer.
- Updated central/backend/frontend/admin-governance/project-planning mappings for the new contract and UI ownership.
- Added unit coverage for Audit filter normalization and a DB-backed query test that runs when `CONSFLOW_TEST_DATABASE_URL` is available.
- Verified the generated count/list/filter-option SQL with read-only queries inside the Consflow PostgreSQL container.
- Ran `cd backend && make test`, `cd frontend && npm run build`, and `git diff --check` successfully; Vite reported only the existing large-chunk advisory.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Overview Vietnamese Redesign

Status: complete

Mục đích chính:
- Giúp người điều hành trong khoảng 10 giây nhận ra công trình đang ở đâu, việc nào phải xử lý tiếp theo và rủi ro nào cần can thiệp.

Will be done:
- Đưa một công việc cần làm tiếp theo và giai đoạn hiện tại lên đầu tab `Tổng quan`.
- Tách cảnh báo cần chú ý khỏi thông tin tham khảo; rút danh sách ưu tiên còn năm việc và bổ sung bố cục thẻ trên màn hình nhỏ.
- Việt hóa toàn bộ nhãn hệ thống trong tab, hành động và cây nghiệp vụ của không gian công trình; giữ nguyên mã, tên riêng và dữ liệu do người dùng nhập.
- Giữ nguyên API `/projects/:id/operations`, quyền, trạng thái và thứ tự ưu tiên công việc.
- Cập nhật frontend/project/workflow mapping; chạy frontend build, kiểm tra chuỗi tiếng Anh và `git diff --check`.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Confirmed the redesign is presentation-only and the existing project operations response already supplies project summary, health counts, workflow timeline and complete scoped work items.
- Loaded the mandatory central/frontend/project-planning/workflow mappings and inspected the overlapping dirty worktree before editing.
- Rebuilt `Tổng quan` around one prioritized next action, current runtime stage, a distinct attention strip, five-item priority preview and a separate project-information panel.
- Added compact mobile work cards while preserving the dense desktop table and existing work-item detail navigation.
- Localized project tab, action, empty/error, workflow and project-tree system labels into Vietnamese; preserved route keys, API status keys, project codes, names and user-entered work-item text.
- Updated central/frontend/project-planning/workflow mappings for the new overview hierarchy and localization contract.
- Ran `cd frontend && npm run build` successfully; Vite reported only its existing large-chunk advisory.
- Ran focused `git diff --check` successfully and confirmed the remaining English search hits are internal identifiers, route/status keys or type names rather than user-facing labels.
- Browser verification was skipped per repository rule because it was not explicitly requested in this turn.

## Project Activity Emotion Button Contrast

Status: complete

Mục đích chính:
- Làm nút thêm emotion luôn được nhận biết là hành động tương tác trên cả bubble xanh và nền sáng mà không cạnh tranh với reaction đã chọn.

Will be done:
- Thêm nền trắng, viền, shadow nhẹ và icon xanh đậm cho nút `SmilePlus`.
- Thêm state hover, focus-visible, open và pending/disabled rõ ràng; giữ nguyên picker, emoji và API.
- Cập nhật frontend mapping; chạy frontend build và `git diff --check`.

Done:
- User approved the focused review package and Mockup as Text with `OK triển khai nút emotion`.
- Confirmed the root cause is the transparent `text-slate-500` icon button rendered on the own-message `bg-blue-600` bubble; reaction state and API remain healthy.
- Replaced the transparent trigger with a bordered `bg-white/95` surface, blue icon and light shadow so it remains visible on both blue and light bubbles.
- Added distinct hover, focus-visible and open styling, increased the icon to 16px and exposed `aria-expanded`.
- Disabled the trigger while a reaction mutation is pending; the existing picker, emoji set, optimistic mutation and reaction pills remain unchanged.
- Updated the frontend UI map for the shared interactive reaction trigger. Central/domain maps did not need updates because route, API contract, persistence, permission and activity invariants are unchanged.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Evidence Workspace Redesign

Status: complete

Mục đích chính:
- Biến tab `Hồ sơ` thành trung tâm tra cứu và rà soát minh chứng công trình dạng danh sách dày, có dữ liệu đầy đủ, phân trang/lọc nguồn thật và hành động đúng quyền.

Will be done:
- Thêm API project evidence riêng với tìm kiếm, filter loại/trạng thái/đầu việc/người gửi/thời gian, cursor pagination, summary theo đúng project scope và `can_review` theo từng bản ghi.
- Giữ hồ sơ của work item `cancelled` ngoài danh sách vận hành nhưng cho phép xem lịch sử `rejected` trong tab Hồ sơ.
- Thay gallery PDF có preview lớn bằng project evidence workspace: KPI, filter dày, danh sách desktop, card compact mobile, loading/empty/error/filter-empty và dialog chi tiết dùng shared primitives.
- Giữ upload trong work-item detail; tab Hồ sơ chỉ tra cứu, mở và rà soát, không gộp `documents` hoặc workflow PDF trong lượt này.
- Cập nhật central/backend/frontend/project-planning/workflow mappings; chạy backend format/tests, frontend build và `git diff --check`.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Confirmed current `/projects/:id/operations` exposes only 50 latest non-rejected evidence rows, while the project UI filters locally and presents a rejected filter that cannot return data.
- Confirmed PostgreSQL direct-SQL repository ownership, existing `evidence_records` indexes/status model and backend `CanDecideWorkItem` enforcement; no migration is planned unless focused query validation proves an index is required.
- Loaded mandatory central/backend/frontend/project-planning/workflow mappings and preserved the existing dirty worktree without reverting overlapping Activity/Workflow changes.
- Added protected `GET /projects/:id/evidence` with validated search/type/status/work-item/sender/visibility/date filters, stable waiting-first cursor pagination, filtered totals, source-backed summary and scoped filter options.
- Included rejected evidence as review history while keeping evidence from cancelled work items out of the project workspace; internal project users can see internal/project/client-visible evidence in their work scope, while client profiles remain restricted to client-visible evidence.
- Added per-row `can_review` derived from the existing `CanDecideWorkItem` gate; the accept/reject endpoints remain the authoritative backend enforcement.
- Added `ProjectEvidencePanel.tsx` with KPI filters, dense search/filter toolbar, compact desktop/mobile evidence rows, loading/error/empty/filter-empty states and cursor `Tải thêm`; upload remains in work-item detail.
- Extended shared `EvidenceGallery` with externally controlled filters, a project list context, stage/source metadata and permission-aware review controls while preserving the existing work-item evidence list/gallery.
- Changed the project Hồ sơ tab badge to use the full permission-scoped evidence summary instead of the capped `latest_evidence` slice.
- Updated upload/review invalidation so project evidence queries refresh after work-item evidence changes.
- Added unit coverage for evidence filter/cursor normalization and PostgreSQL integration coverage for waiting-first order, cursor continuation, summary parity, rejected history, cancelled-work-item exclusion and review capability.
- Updated central/backend/frontend/project-planning/workflow mappings for the new route, contract, repository functions, UI owner and evidence-history invariants.
- No migration or seed change was required because existing `evidence_records` status/visibility fields and project/work-item indexes support the new read contract.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran focused PostgreSQL integration test `TestProjectEvidencePageScopeHistoryAndReviewCapability` against an isolated temporary PostgreSQL 17 database: pass; the temporary server was stopped afterward.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Chat Edit UX Correction

Status: complete

Mục đích chính:
- Làm cho chế độ chỉnh sửa tin nhắn chỉ phát request khi nội dung thực sự thay đổi, phản hồi validation gọn, không che workspace và giữ các điều khiển chat rõ ràng, nhất quán.

Will be done:
- Thêm dirty-state đã trim; khóa `Lưu chỉnh sửa` khi chưa thay đổi hoặc đang pending và không phát request/toast cho no-op.
- Phân loại validation edit thành warning; deduplicate toast giống nhau toàn hệ thống và giới hạn ba toast khác nhau.
- Đóng menu tin nhắn sau action, Escape hoặc click ngoài; giữ một menu duy nhất tại một thời điểm trong bubble.
- Tăng tương phản metadata của bubble người dùng và thu gọn banner edit về thời gian/phạm vi thay vì lặp tên công trình dài.
- Thêm `decisions` nguồn thật vào project activity summary và hiển thị count nhất quán trên tab `Quyết định`.
- Cập nhật central/backend/frontend/project-planning mappings; chạy backend format/tests, focused integration test, frontend build và `git diff --check`.

Done:
- User approved all eight diagnosed corrections and the Mockup as Text with `OK triển khai gói sửa UX edit`.
- Confirmed the winning frontend causes: no dirty comparison, generic error mapping, random-ID toast stacking, uncontrolled native `details`, fixed slate metadata on blue bubble, repeated long target context and a conditional that hides the decision badge.
- Confirmed backend correctly rejects unchanged content and current project activity summary does not yet expose a dedicated decision count.
- Added trimmed dirty-state comparison in the chat composer; unchanged edits show `Chưa có thay đổi`, keep the save button disabled and never call the API or create a toast.
- Kept pending edit submissions disabled and mapped backend `VALIDATION_ERROR` responses to a warning toast while preserving unexpected failures as error toasts.
- Updated the global toast provider to ignore identical active variant/title/description notifications and keep at most three distinct visible toasts.
- Replaced the native message-action `details` with a controlled popover that closes on edit/withdraw action, Escape and outside click.
- Added own-blue-bubble contrast tokens for visibility, target, compact time, edited state and work-item link metadata.
- Compacted the edit banner to message time plus scope/visibility; removed the repeated long project target label.
- Added source-backed `decisions` to `ProjectActivitySummary`, frontend domain/default state and the `Quyết định` badge; removed the conditional that hid that badge.
- Extended the Pha 2B integration test to compare the returned decision summary against the project decision row count.
- Updated central/backend/frontend/project-planning maps for dirty edit behavior, controlled menu, toast deduplication and decision summary ownership.
- No migration or seed change was required because this correction changes response derivation and frontend interaction only.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `make restart`: pass; backend skipped all 59 already-applied migrations and listened on port 8080.
- Ran Docker-network integration tests `TestActivityReplyAndReactionScope` and `TestActivityMessageEditAndWithdrawScope`: pass, including decision summary parity.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Chat Edit And Withdraw Phase 2B

Status: complete

Mục đích chính:
- Cho phép tác giả sửa hoặc thu hồi comment trong thời hạn ngắn mà vẫn giữ nguyên chuỗi trả lời, lịch sử phiên bản, audit và ranh giới giữa trao đổi với activity nghiệp vụ.

Will be done:
- Chỉ cho tác giả chỉnh sửa hoặc thu hồi activity loại `comment` trong 15 phút; kiểm tra quyền và thời hạn ở backend.
- Chỉnh sửa chỉ thay đổi nội dung; scope, visibility, work item, mentions, attachments và liên kết reply giữ nguyên.
- Thu hồi dùng tombstone, không xóa vật lý; giữ replies, ẩn reactions và chặn reply/reaction mới.
- Thêm migration V59 cho metadata edit/withdraw và `activity_message_revisions` append-only ledger.
- Loại nội dung đã thu hồi khỏi unread, mentions, attention và search nhưng vẫn giữ activity trong timeline/count lịch sử.
- Thêm menu `Chỉnh sửa`/`Thu hồi`, composer edit mode, shared confirmation dialog và trạng thái tombstone trong Project Activity chat.
- Giữ typed operational activities bất biến; không thêm manager override hoặc workflow transition.
- Cập nhật central/backend/frontend/project-planning/workflow/admin mappings; chạy backend format/tests, migration check, frontend build và `git diff --check`.

Done:
- User approved the Phase 2B review package with `OK triển khai Pha 2B`.
- Re-confirmed backend and UI currently support reply/reaction but have no edit, withdraw or message revision contract.
- Re-confirmed `Dialog` is the shared modal primitive and the project chat composer is the correct edit surface.
- Added migration `V59__activity_message_edit_withdraw.sql` with edit/withdraw actor timestamps, monotonic revision number and append-only `activity_message_revisions` ledger.
- Added protected `PUT /activities/:id/message` and `POST /activities/:id/withdraw`; both reuse activity visibility, require exact author ownership, allow only `comment` and enforce the 15-minute server window.
- Kept scope, visibility, target, work item, mentions, attachments and reply parent immutable during edit; typed operational activities remain immutable.
- Made withdrawal transactional: preserve a tombstone and replies, clear current comment, deactivate reactions, close unread states and append revision plus audit without changing workflow state.
- Blocked replies and reactions to withdrawn messages; conversation responses expose withdrawn parent previews, server-computed capabilities and no active reactions for tombstones.
- Excluded withdrawn messages from My Work attention and project unread/mention counts while retaining them in project history, totals, pagination and reply counts.
- Added the own-message `…` menu, sticky composer edit mode, shared withdrawal confirmation dialog, edited label and responsive tombstone rendering; work-item history and reply quotes render tombstones read-only.
- Added focused integration coverage for author/type/time-window gates, revision/audit persistence, tombstone behavior, attention cleanup, reply preservation and reply/reaction rejection.
- Updated central/backend/frontend/project-planning/workflow-orchestration/admin-governance maps for V59, routes, tables, UI ownership and communication-only invariants.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `make migrate-check`: pass with 59 migrations.
- Ran `make restart`: pass; backend auto-applied V59 and started on port 8080.
- Verified PostgreSQL Compose reports 59 applied migrations, the new columns and `activity_message_revisions` table.
- Ran Docker-network integration tests `TestActivityReplyAndReactionScope` and `TestActivityMessageEditAndWithdrawScope`: pass.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Chat Reply And Reactions Phase 2A

Status: complete

Mục đích chính:
- Cho phép thành viên phản hồi trực tiếp một trao đổi và thể hiện phản ứng nhanh mà không làm mất tính truy vết, visibility, quyền theo công trình hoặc ranh giới giữa trao đổi và trạng thái nghiệp vụ.

Will be done:
- Thêm reply dạng quote trong luồng chat phẳng; reply luôn là `comment` và kế thừa project/target/work item/visibility của tin gốc.
- Tự động tạo mention/unread cho tác giả tin gốc khi người khác reply.
- Thêm reaction idempotent `👍`, `👀`, `🙏`, `🎉`; reaction không thay đổi read, resolve, work-item status hoặc activity summary.
- Thêm migration `V58` cho `parent_activity_id` và `activity_reactions` active-state ledger; không sửa migration đã áp dụng.
- Đồng bộ API response, Project Activity chat, Work Item activity timeline và My Work attention context.
- Giữ edit/delete ngoài Pha 2A; giữ activity audit cho reply và không tạo audit/system event cho reaction.
- Cập nhật central/backend/frontend/project-planning/workflow/admin mappings; chạy backend format/tests, migration check, frontend build và `git diff --check`.

Done:
- User approved the Phase 2A review package, inheritance rules and reaction set with `Đồng ý`.
- Confirmed current persistence has no parent reference or reaction state and the existing activity table/query contract is shared by project chat, work-item detail and My Work attention.
- Added migration `V58__activity_replies_reactions.sql` with nullable self-reference `parent_activity_id`, parent lookup index and active-state `activity_reactions` ledger for `👍`, `👀`, `🙏`, `🎉`.
- Added protected `POST /activities/:id/reactions`; reaction upserts reuse `ActivityVisibleToUser`, reject system events and publish existing activity SSE invalidation without audit/system-event noise.
- Added project reply normalization: backend forces `comment`, inherits the parent project/target/work item/visibility, rejects hidden/system/cross-project parents and auto-mentions the parent actor.
- Extended activity query contracts with `reply_to`, `reply_count` and reaction summaries across project activity, work-item detail and My Work attention.
- Added chat reply mode with parent quote, inherited/locked context, reply counts, optimistic reaction toggles and shared read-only reply/reaction metadata in operational activity surfaces.
- Added focused integration coverage for inheritance, cross-project rejection, auto-mention/unread, reaction idempotency/whitelist, system-event rejection and response conversation fields.
- Updated central/backend/frontend/project-planning/workflow-orchestration/admin-governance maps for V58, routes, tables, UI ownership and communication-only invariants.
- Ran `make restart`: pass; backend auto-applied one migration and skipped 57, then listened successfully on port 8080.
- Verified PostgreSQL Compose reports 58 applied migrations and smoke-tested project activity, My Work attention and work-item detail API contracts: all returned success.
- Ran the focused PostgreSQL Compose integration test `TestActivityReplyAndReactionScope`: pass.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran `make migrate-check`: pass with 58 migrations.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Chat-like Discussion Phase 1

Status: complete

Mục đích chính:
- Giúp thành viên công trình trao đổi theo nhịp hội thoại tự nhiên trong tab `Thảo luận`, đồng thời giữ `Tất cả` và `Nhật ký hệ thống` ở dạng activity feed phục vụ vận hành và truy vết.

Will be done:
- Chỉ chuyển tab `Thảo luận` sang luồng chat theo thứ tự cũ đến mới; tin của người hiện tại nằm bên phải, người khác nằm bên trái.
- Hiển thị câu hỏi, quyết định, điểm chặn, nhắc việc và tệp dưới dạng bubble có nhận diện loại; giữ hành động đã đọc/đã xử lý và liên kết đầu việc.
- Đặt composer chat cố định cuối luồng, tái sử dụng validation/payload hiện có và không tự thu gọn sau khi gửi.
- Tải tin cũ ở đầu luồng, giữ vị trí cuộn, tự cuộn khi người dùng đang ở cuối và báo có tin mới khi họ đang đọc nội dung cũ.
- Giữ nguyên API, RBAC, visibility, SSE, flat-message model và activity feed hiện có; chưa làm reply, reaction, edit hoặc delete trong Pha 1.
- Cập nhật frontend/central/project-planning mappings; chạy `cd frontend && npm run build` và `git diff --check`; không chạy browser vì người dùng không yêu cầu.

Done:
- User approved the Phase 1 chat-like implementation with `OK triển khai chat-like Pha 1.`
- Confirmed the existing activity contract already provides actor, typed activity, visibility, mentions, attachments, unread/resolution metadata and realtime invalidation required by this frontend-only phase.
- Changed only the `Thảo luận` tab to a chronological chat surface with current-user messages aligned right, other participants aligned left, grouped authors and typed bubbles for operational messages.
- Added top loading for older messages with scroll anchoring, initial/latest auto-scroll and a `Có tin nhắn mới` affordance when the reader is away from the bottom.
- Added a persistent compact chat composer that shares the existing payload, validation, visibility, target, mention, attachment, owner and due-date logic; it remains open after successful send.
- Kept `Tất cả`, `Quyết định` and `Nhật ký hệ thống` on the existing descending activity-card feed and retained read/resolve/work-item actions in chat bubbles.
- Kept backend routes, database schema, RBAC, SSE and flat-message persistence unchanged; reply, reaction, edit and delete remain deferred.
- Updated central routing, frontend UI and project-planning domain maps with the chat/feed split and Phase 1 boundary.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Sample Visibility Correction

Status: complete

Mục đích chính:
- Khi mở `Trao đổi`, người dùng nhìn thấy toàn bộ activity hiện có trước; thảo luận và nhật ký hệ thống là các bộ lọc chủ động, không làm sample project trông như thiếu dữ liệu.

Will be done:
- Đổi filter mặc định từ `Thảo luận` sang `Tất cả`.
- Đổi badge tab `Trao đổi` từ attention count sang tổng activity nhìn thấy được; attention count tiếp tục nằm trong panel `Cần chú ý`.
- Khi filter `Thảo luận` rỗng nhưng có system activity, hiển thị CTA mở `Nhật ký hệ thống`.
- Giữ nguyên backend rule ẩn activity thuộc work item `cancelled` trong lượt sửa này.
- Cập nhật frontend/project activity mappings và chạy `cd frontend && npm run build`, `git diff --check`; browser verification bỏ qua vì người dùng không yêu cầu.

Done:
- Reproduced against live `GOLDEN-REAL-001`: database has 52 activity rows, 0 discussion and 52 system rows; API exposes 48 system rows after excluding four cancelled-work-item rows.
- User approved the focused diagnosis with `OK triển khai theo chuẩn đoán này`.
- Changed project activity default from `Thảo luận` to `Tất cả` so seeded system history is visible immediately.
- Changed the project-tab badge to the source-backed total visible activity count; attention remains in `Cần chú ý`.
- Added a `Mở Nhật ký hệ thống` CTA when the discussion filter is empty but system history exists.
- Updated frontend, central and project-planning maps with the corrected default/count semantics.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Operational Redesign

Status: complete

Mục đích chính:
- Biến tab `Trao đổi` thành trung tâm phối hợp công việc của công trình: ưu tiên nội dung cần phản hồi, gửi đúng người/đúng đối tượng và tách thảo luận khỏi nhật ký hệ thống.

Will be done:
- Thu gọn composer mặc định; chỉ hiện trường phụ theo loại ghi chú/câu hỏi/quyết định/điểm chặn/nhắc việc/tệp.
- Thay nhập UUID bằng picker thành viên, vai trò, đầu việc và đối tượng nghiệp vụ được backend trả theo project scope/RBAC.
- Mở rộng project activity API với tìm kiếm, filter, cursor pagination và summary count nguồn thật.
- Thêm hành động đã trả lời/đã xử lý/hoàn tất nhắc việc cùng mark-all-read, lưu trạng thái trong `metadata_json` và giữ activity history append-only.
- Tách thảo luận khỏi nhật ký hệ thống; thêm panel `Cần chú ý`, trạng thái loading/empty/error/filter-empty và responsive mobile.
- Đổi badge tab từ số bản ghi latest bị giới hạn sang tổng activity nhìn thấy được; số cần chú ý nằm trong panel riêng.
- Giữ nguyên visibility `internal/project/client`, project/work-item permission gates, SSE invalidation và auditability.
- Không triển khai thread reply, reaction hoặc external notification trong pha này; không cần migration schema.
- Cập nhật central/backend/frontend/project-planning/workflow-orchestration mappings trong cùng lượt.
- Chạy backend format/tests, frontend build và `git diff --check`; browser verification bỏ qua vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Confirmed current activity persistence already supports typed messages, visibility, mentions, unread states, attachments, arbitrary metadata and realtime SSE.
- Confirmed existing project activity query only supports scope filters with a hard limit and the tab badge currently reflects the latest-activity slice rather than a total.
- Confirmed schema migration is unnecessary while threaded replies remain deferred.
- Added cursor-paginated project activity feed with server search/filter for kind, attention queue, scope, activity type, actor and visibility.
- Added source-backed project activity summary, permission-scoped member/role/target/attachment options and a total-visible-activity tab badge; attention counts remain inside the attention panel.
- Added mark-all-read and authorized question/blocker/reminder resolution; resolution metadata remains on the original activity while a system event and audit record preserve append-only history.
- Rebuilt the project activity UI with a collapsed type-aware composer, no UUID inputs, discussion/decision/system tabs, desktop attention panel, mobile attention chips, filters, grouped activity cards, loading/filter-empty/error states and cursor loading.
- Reused project activity options in work-item detail so the shared composer retains mention and owner pickers without technical IDs.
- Updated central/backend/frontend/project-planning/workflow-orchestration maps for the new routes, contracts, UI surface and resolution invariant.
- Ran `GOCACHE="$PWD/.gocache" go test ./...` under `backend`: pass; DB-backed tests remain environment-gated by `CONSFLOW_TEST_DATABASE_URL`.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Work Tab Operational Queue Redesign

Status: complete

Mục đích chính:
- Giúp PM/Admin nhìn thấy ngay công việc cần xử lý, công việc đang chờ hoặc bị chặn, người/vai trò phụ trách và mở đúng đầu việc trong phạm vi một công trình.

Will be done:
- Thay `Department Surfaces` bằng hàng đợi công việc theo mức độ hành động, mặc định `Cần xử lý`.
- Tách rõ `Chờ duyệt`, `Bị chặn`, `Chưa đến lượt`, `Hoàn tất`, `Đã bỏ qua` và vẫn giữ `Tất cả` để truy vết.
- Bổ sung tìm kiếm, lọc giai đoạn, người/vai trò phụ trách, trạng thái và sắp xếp ưu tiên/workflow/hạn.
- Dùng bảng dày trên desktop và card gọn trên mobile; giữ một CTA `Mở` và điều hướng project-scoped hiện hữu.
- Dùng toàn bộ `work_items` từ `/projects/:id/operations`, không dùng danh sách con tối đa 10 item của `department_surfaces` làm nguồn lọc.
- Giữ nguyên backend contract, RBAC, transition, approval, evidence, workflow form và lịch sử cancelled read-only.
- Cập nhật central/frontend/workflow mappings trong cùng lượt thay đổi.
- Chạy `cd frontend && npm run build` và `git diff --check`; browser verification bỏ qua vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Confirmed the existing operations response already contains the status, node, assignee, role, due date, form and evidence fields required by the redesign.
- Confirmed `department_surfaces.work_items` is capped at 10 records per department and cannot be used as the complete filtered list.
- Confirmed no backend API or shared frontend type contract change is required.
- Added `ProjectWorkTab.tsx` and replaced the system-facing `Department Surfaces` layout with an operational project work queue.
- Added actionable/waiting/blocked/future/completed/skipped/all quick filters, accent-insensitive search, stage/owner-role/status filters and priority/workflow/due sorting.
- Added a 20-row dense desktop table, responsive mobile cards and project-scoped `Mở` navigation without adding transition controls to the list.
- Presenter now prioritizes direct assignee, then required roles, then department fallback; `Chưa phân công` appears only when those sources are absent.
- Updated central, frontend UI and workflow orchestration maps with the new Work-tab ownership and complete-list invariant.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Workflow Execution Workspace Redesign

Status: complete

Mục đích chính:
- Giúp người điều hành biết ngay workflow đang ở giai đoạn nào, việc nào cần xử lý tiếp theo và mở đúng đầu việc với ít thao tác nhất.

Will be done:
- Thay layout Workflow tab hai cột gần cân bằng bằng summary điều hành, stage rail 280px và vùng công việc chiếm phần lớn không gian.
- Hiển thị tiến độ toàn workflow, giai đoạn hiện tại, số việc cần xử lý/chờ duyệt/bị chặn và CTA duy nhất `Mở việc tiếp theo` theo thứ tự blocker -> chờ duyệt -> việc sẵn sàng.
- Đổi danh sách work item trong node sang bảng/row dày dữ liệu ở desktop và card gọn ở mobile.
- Mặc định filter `Cần xử lý`; giữ `Tất cả` để truy vết lịch sử completed/skipped.
- Bổ sung stage selector responsive, loading/empty/filter-empty/skipped state và giữ cancelled branch ở chế độ chỉ đọc.
- Không đổi backend API, work-item transition, approval, workflow form, evidence hay orchestration invariant.
- Cập nhật frontend UI map và workflow orchestration domain map trong cùng lượt thay đổi.
- Chạy `cd frontend && npm run build` và `git diff --check`; browser verification tiếp tục bỏ qua vì người dùng không yêu cầu.

Done:
- User approved the review package and generated visual mockup with `OK, triển khai.`
- Confirmed owner surface is `frontend/src/modules/projects/ProjectDetailPage.tsx` under `/projects/:id`.
- Confirmed the existing `/projects/:id/operations` response already provides timeline, work-item status, owner, due date, form and evidence fields required by the redesign.
- Confirmed no backend or shared frontend type contract change is required.
- Replaced the balanced timeline/work-card layout with a workflow execution summary, 280px desktop stage rail and responsive stage selector.
- Scoped summary counts and next-work selection to the current runtime node so future pending work does not inflate the actionable count.
- Added next-work priority blocker -> waiting approval/review -> other current-node open work, with one primary CTA inside the Workflow tab.
- Replaced node work cards with a dense desktop work table and compact mobile cards; retained owner, due date, form requirement, evidence count, status and blocker reason.
- Changed active-node default filter to `Cần xử lý`; skipped/cancelled nodes automatically open `Đã bỏ qua` read-only history; `Tất cả` remains available.
- Updated central, frontend UI and workflow orchestration maps for the new execution workspace behavior.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Pipeline Builder Back Navigation Target

Status: complete

Mục đích chính:
- Make the Pipeline Builder `[← Quay lại]` control return to the `Công trình` list as a workflow navigation action, not as a browser-history back action.

Will be done:
- Change Pipeline Builder back handling from `navigate(-1)` history behavior to a fixed `/projects` navigation target.
- Keep the visible button label `Quay lại`.
- Change the button tooltip to `Về trang Công trình`.
- Update the frontend UI map to record that Pipeline Builder sidebar back navigation targets `/projects`.
- Keep backend APIs, template publish/clone/edit behavior, canvas/diagram interactions, workflow gates, and runtime copies unchanged.
- Run `cd frontend && npm run build` and `git diff --check`; browser verification remains skipped unless explicitly requested.

Done:
- User approved the review package with `OK triển khai`.
- Re-confirmed current code uses `window.history.state?.idx` and `navigate(-1)`, which behaves like browser back.
- Re-confirmed `/projects` is the guarded project list route and current expected target.
- Changed Pipeline Builder back handling to fixed `/projects` navigation.
- Changed the button tooltip to `Về trang Công trình`.
- Updated `frontend-ui-utility-map.md` to record that Pipeline Builder sidebar back navigation targets `/projects`.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Official Workflow Text Correction — Trao đổi

Status: complete

Mục đích chính:
- Sửa nhãn `Trảo đổi` thành `Trao đổi` ở mọi bề mặt dữ liệu workflow đang hiển thị cho người dùng, không thay đổi trạng thái hay lịch sử nghiệp vụ.

Will be done:
- Thêm migration V57 sửa ba Step template và các snapshot pipeline-run/work-item/workflow-form instance tương ứng.
- Đồng bộ ba nhãn seed để lần seed sau không ghi đè lại bản sửa.
- Bổ sung Golden validator chống tái phát, cập nhật mapping và chạy migration/check truy vấn cục bộ; không chạy `make seed` vì lệnh reset dữ liệu local.

Done:
- User approved the review package with `Approve`.
- Confirmed the local database has 3 affected template steps, 9 run steps, 9 work items and 6 workflow-form instances.
- Confirmed V40 is already applied but `seed.sql` runs afterward and upserts the three incorrect labels.
- Added V57 to correct the three labels in template, pipeline-run, work-item and workflow-form snapshots without changing statuses, answers, approvals, audit records or PDF artifacts.
- Corrected the three seed labels and added Golden validator coverage that rejects any remaining `Trảo đổi` label in those four data surfaces.
- Applied V57 through the backend Docker custom migration runner: one migration applied and 56 skipped.
- Verified locally: template/run/work-item/form-instance typo counts are all zero; all three configured Step labels now start with `Trao đổi`.
- Validation passed: `make migrate-check`, `make seed-validate` (`GOLDEN_SAMPLE_VALIDATION_PASSED`) and `git diff --check`. `make seed` was intentionally skipped because it resets local data.

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

## Procurement And Warehouse Operational Workspace Redesign

Status: complete

Mục đích chính:
- Làm rõ chuỗi BOQ -> yêu cầu mua -> đặt hàng -> nghiệm thu đầu vào -> phiếu nhập kho -> xuất công trình -> xử lý vật tư dư, để người dùng luôn thấy đúng trạng thái và một hành động tiếp theo có thể thực hiện.

Will be done:
- Thiết kế lại Mua sắm thành workspace ưu tiên việc cần xử lý, có bộ lọc nghiệp vụ, tiến độ nghiệm thu/nhập kho, CTA chính theo trạng thái và action visibility khớp RBAC.
- Thiết kế lại Kho thành bốn bề mặt: tồn theo kho, phiếu kho, tồn công trình/vật tư dư và danh mục vật tư.
- Bổ sung tồn theo từng kho, ledger/phân trang phiếu kho, chứng từ nhiều dòng và liên kết rõ với nghiệm thu đầu vào mà không tự động tạo stock movement.
- Hoàn thiện điều chuyển hai đầu với kho nguồn/kho đích trong một transaction; giữ kiểm tra không âm tồn và site balance/surplus hiện hữu.
- Tách ý nghĩa audit đối soát nghiệm thu mua sắm khỏi event nhập kho thật; giữ lịch sử cũ không bị viết lại.
- Cập nhật report query, frontend/backend contracts, integration tests và central/detail/domain mappings liên quan.
- Chạy backend format/tests, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the full review package and Mockup as Text with `Duyệt, triển khai theo review package này`.
- Loaded the mandatory central/backend/frontend mappings, commercial procurement, materials warehouse, quality acceptance, reporting, impact-analysis and database workflows.
- Confirmed the PostgreSQL/custom SQL migration boundary, existing V60 maximum, current procurement acceptance-derived receive invariant and overlapping dirty worktree that must be preserved.
- Added migration V61 with posted multi-line stock documents/items, movement document linkage and target warehouse linkage for two-ended transfers; applied it through the custom migration runner in Docker (`applied=1 skipped=60`).
- Added source-backed per-warehouse positions, movement ledger, stock document list/detail/create and project warehouse-receipt-option endpoints with project visibility and warehouse permission gates.
- Refactored stock posting through one transaction helper; transfer locks source/target in stable order, subtracts the source and adds the target, while issue/return continue updating site balances and surplus status.
- Linked explicit receive movements to incoming-acceptance items, capped receipt quantity by remaining accepted quantity and preserved the invariant that submitting acceptance alone creates no stock movement.
- Split procurement acceptance reconciliation audit events from real warehouse-receipt events and extended procurement list/detail data with accepted-vs-stocked quantities plus latest acceptance/work-item linkage.
- Rebuilt Mua sắm as a permission-aware action-first queue with truthful acceptance/stock progress, dense desktop table, responsive cards, operational filters, one primary next action and enhanced detail dialog.
- Rebuilt Kho as four tabs for per-warehouse stock, posted documents, project site/surplus and material catalog; added a shared responsive multi-line document dialog for receive/issue/transfer/return and acceptance-source prefill.
- Updated frontend contracts/status mappings plus central/backend/frontend/commercial/materials/quality/reporting mappings for the new routes, tables, audit semantics and UI ownership.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass after updating the procurement event regression expectation.
- Ran the DB-backed `TestStockDocumentTransferMovesBothWarehouseBalancesAtomically` in the Consflow Docker network: pass.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Preserved Golden Sample compatibility by recognizing historical explicit receipts linked to their source BOQ items; runtime smoke now reports both received procurement requests with accepted quantity equal to stocked quantity and no duplicate receipt options.
- Rebuilt and recreated the local backend/admin containers, confirmed V61 remains applied, and smoke-tested health plus project-filtered procurement, per-warehouse positions, stock documents and receipt options successfully.
- Ran `git diff --check`: pass; user-facing `Pending / Partial / Handled` labels are no longer present in the redesigned pages.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Construction And Acceptance Focused Workspace

Status: complete

Mục đích chính:
- Gom bốn bề mặt Thi công, Quản lý chất lượng, Nghiệm thu và Khắc phục sai lỗi thành hai không gian dễ hiểu: theo dõi thực thi hằng ngày tại `Thi công` và lập/kết luận biên bản tại `Nghiệm thu`.

Will be done:
- Rút gọn cây nghiệp vụ `Thi công & Chất lượng` còn `Thi công` và `Nghiệm thu`; giữ route cũ `/quality` và `/rework` bằng redirect tương thích.
- Chuyển `Thi công` thành status cards và ba tab `Nhật ký & tiến độ`, `Checklist chất lượng`, `Vấn đề & khắc phục` với timeline báo cáo gồm đã làm, đang làm, sẽ làm và vấn đề.
- Tái sử dụng các lifecycle ITP/RFI/NCR, issue, rework và replanning hiện hữu trong đúng tab, không gộp bảng dữ liệu hoặc thay đổi transition backend.
- Chuyển `Nghiệm thu` thành master-detail, checklist rõ ràng, một CTA tiếp theo theo trạng thái và một dialog kết luận Đạt/Đạt có điều kiện/Không đạt.
- Tách editor checklist nghiệm thu dùng chung cho project workspace và work-item outgoing acceptance.
- Cập nhật central/frontend/construction/quality/workflow mappings; chạy frontend build, backend test phù hợp và `git diff --check`.

Done:
- User approved the complete review package with `Approve`.
- Confirmed Release 1 can preserve the current API tables and lifecycle: failed RFI still creates NCR plus linked rework atomically; failed acceptance still creates an issue and exposes the explicit idempotent create-rework action.
- Confirmed the implementation overlaps a dirty worktree; existing user changes will be preserved and target-file diffs reviewed before patching.
- Reduced the project business tree to `Thi công` and `Nghiệm thu`; `/projects/:id/quality` and `/projects/:id/rework` now preserve old bookmarks through redirects to the matching construction tab.
- Rebuilt `OperationsPage` around four status cards and the approved three tabs. The daily timeline reads workflow-form structured answers and legacy summary rows, keeps report blockers separate from official issues, and captures quick reports through distinct Đã/Đang/Sẽ làm/Vấn đề fields without a backend migration.
- Embedded `QualityPage` as field checks plus advanced ITP configuration and as the NCR remediation section; embedded `ReworkPage` retains separate rework/replanning queries and RBAC-gated create/approve actions.
- Rebuilt `AcceptancePage` as a responsive master-detail workspace with status cards, progress steps, explicit checklist/evidence table, one state-driven CTA, and one radio-based conclusion dialog with required comment.
- Added shared `AcceptanceEditor.tsx` for checklist rows, evidence/required-action validation, draft payload mapping and submit gates; both the project acceptance workspace and `OutgoingAcceptancePanel` now use it.
- Kept the existing backend/domain lifecycle unchanged: acceptance fail creates its issue first, `Mở yêu cầu khắc phục` calls the existing idempotent rework action, and RFI/NCR transitions remain owned by quality APIs.
- Updated central/frontend/construction/quality/workflow mappings for the consolidated presentation routes, shared editor ownership, structured/legacy timeline mapping, permissions and compatibility redirects. No backend-function map or migration update was required because API contracts, handlers, repository logic and schema were unchanged.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `cd backend && go test ./...`: pass.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.

## Construction Daily Report Create/Detail Contract Alignment

Status: complete

Mục đích chính:
- Đảm bảo báo cáo được tạo tại `Thi công` lưu đúng cấu trúc mà form chi tiết trình bày, đồng thời minh chứng luôn có trạng thái rõ ràng và giữ nguyên permission/work-item evidence semantics.

Will be done:
- Thay form tạo generic bằng dialog chuyên biệt cho ngày/thời tiết/tiến độ, Đã làm, Đang làm, Sẽ làm, blocker, ghi chú, nhân công, vật tư, đầu việc nguồn và minh chứng.
- Cho phép chọn minh chứng hiện hữu hoặc tải tệp mới sau khi chọn đầu việc nguồn; tái sử dụng work-item evidence APIs và không tạo project-level upload mới.
- Mở rộng typed create contract của `/projects/:id/daily-reports` để lưu `work_item_id`, `structured_json`, `photos` và `primary_evidence_id`, với kiểm tra project/work-item/evidence/status.
- Giữ workflow-form bridge idempotent hiện hữu, legacy report fallback, audit và schema V53; không thêm migration.
- Luôn hiển thị trạng thái `Minh chứng · 0` trong chi tiết và đồng bộ CTA frontend với quyền `operation.report` của backend.
- Cập nhật backend/frontend/central/construction/workflow mappings và focused regression tests.
- Chạy backend format/tests, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved all three review points with `Approve`.
- Confirmed root cause: the generic create form and repository whitelist persist only legacy summary fields, while the detail reads the richer V53/workflow-backed structure and hides evidence when no IDs exist.
- Confirmed evidence upload remains work-item scoped (`evidence_records.work_item_id NOT NULL`, `CanActOnWorkItem`), so the approved create flow must select a source work item before uploading or linking evidence.
- Completed cross-feature impact check: no workflow-form lifecycle, approval, stock, quality, acceptance, report export or migration behavior needs to change.
- Added typed `ConstructionDailyReportCreateRequest` handling and expanded `dailyReportColumns()` to persist the existing V53 `work_item_id`, `structured_json` and `primary_evidence_id` columns together with legacy summaries and evidence IDs.
- Added backend validation for report date/progress/work groups/resources, same-project and permission-visible source work item, same-work-item non-rejected evidence, client visibility and a maximum of 50 linked evidence records; audit metadata now records source and evidence count.
- Replaced the generic create form with `ConstructionReportFormDialog.tsx`; its hierarchy matches the detail dialog and supports structured Đã/Đang/Sẽ làm, blockers, neutral notes, crew, materials, source work item, existing evidence selection and immediate work-item evidence upload.
- Updated the shared report view model so manual structured reports are not labeled as workflow reports, and neutral notes are excluded from the red timeline problem card when canonical blockers exist.
- The detail dialog now always shows `Minh chứng · N`; historical reports without linked IDs display the explicit `Minh chứng · 0` empty state instead of hiding the section. The create CTA now requires `operation.report`, matching backend enforcement.
- Added focused backend regression coverage for structured/source/evidence persistence and mismatched work-item evidence rejection. No migration was added because V53 already owns all required columns; workflow-form bridge behavior remains unchanged and legacy summary rows remain readable.
- Updated central/backend/frontend/root-cause/construction/workflow mapping references for the typed manual-report contract, component ownership, RBAC, evidence invariants and workflow boundary.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Browser verification was skipped per repository rule because the user did not request it.

## Construction Report Evidence Capture And Manual Edit

Status: complete

Mục đích chính:
- Cho phép người dùng đính kèm minh chứng bằng tệp hoặc camera ngay trong tạo mới, xem chi tiết và chỉnh sửa báo cáo thi công thủ công, với một luồng nhất quán và truy vết về đầu việc nguồn.

Will be done:
- Luôn hiển thị `Chọn tệp` và `Mở camera`; tệp/ảnh được giữ trong hàng đợi cục bộ trước khi báo cáo có đầu việc nguồn.
- Dùng camera thật qua `navigator.mediaDevices.getUserMedia`, có đổi camera, chụp lại, dùng ảnh và fallback sang file input có `capture` khi trình duyệt không cấp camera.
- Thêm cập nhật báo cáo thủ công và upload minh chứng theo báo cáo; khi có tệp chờ tải, đầu việc nguồn là bắt buộc.
- Cho phép `Sửa báo cáo` và `Thêm minh chứng` từ form chi tiết của báo cáo thủ công; báo cáo do workflow-form sinh ra tiếp tục chỉ đọc và phải chỉnh tại đầu việc nguồn.
- Giữ work-item evidence permission, giới hạn 50 minh chứng, audit và schema V53; không thêm migration, không thay lifecycle workflow/quality/acceptance/stock/finance.
- Cập nhật central/backend/frontend/root-cause/construction/workflow mappings; chạy backend format/tests, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Approve`.
- Confirmed create currently hides all upload controls until a source work item is selected, the camera icon is decorative, detail is read-only, and no daily-report update/report-evidence endpoint exists.
- Confirmed existing evidence persistence remains work-item scoped and report rows already have `work_item_id`, `photos`, `structured_json` and `primary_evidence_id`; no database migration is required.
- Added permission-scoped `PUT /daily-reports/:id` and `POST /daily-reports/:id/evidence/upload`; create/update share one normalizer, preserve project/reporter identity and legacy resource summaries, enforce same-project source/evidence plus the 50-record cap, and emit update/evidence-link audit events.
- Kept report uploads on the existing work-item evidence storage/permission path. The report-scoped upload attaches the resulting evidence ID to `photos`, assigns the source/primary evidence when absent, and rejects direct update/upload for workflow-form-backed rows.
- Added `ConstructionEvidenceCapture.tsx` with always-visible multi-file selection, local pre-save queue, image previews, editable names/captions, real `getUserMedia` camera, facing-mode switch, capture/retry/use flow and a mobile/system-camera `capture="environment"` fallback.
- Reused the capture component in create, edit and the evidence-only quick-add dialog. Manual report detail now exposes `Sửa báo cáo` and `Thêm minh chứng`; workflow-backed detail remains read-only and points to its source work item.
- Added a report edit model that preserves structured rows and legacy summaries, and kept explicit partial-upload feedback so a saved report is not duplicated when a later file upload fails.
- Added focused DB-backed regression coverage for manual update, report/work-item evidence linking, audit events, source mismatch and workflow read-only gates. The focused test passed against the running Docker PostgreSQL instance via its LAN-bound port.
- Updated central/backend/frontend/root-cause/construction/workflow mappings. No migration or other domain mapping was required because schema V53 and workflow/quality/acceptance/stock/finance lifecycles were unchanged.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass (DB-backed tests skip without `CONSFLOW_TEST_DATABASE_URL`).
- Ran the focused DB-backed daily-report regression test with `CONSFLOW_TEST_DATABASE_URL`: pass.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.

## Construction Daily Report Detail Dialog

Status: complete

Mục đích chính:
- Giúp người dùng hiểu trong 5–10 giây tình hình một ngày thi công: tiến độ, đã/đang/sẽ làm, blocker, ghi chú hiện trường, nguồn lực, minh chứng và nguồn báo cáo.

Will be done:
- Thay definition-list chung bằng dialog chi tiết báo cáo chuyên biệt, rộng trên desktop và một cột trên mobile.
- Dùng chung một view-model cho timeline và dialog; tách blocker có cấu trúc khỏi ghi chú `issues_summary` tự do để không tô đỏ câu như `Không có sự cố an toàn`.
- Hiển thị nhân công, vật tư, minh chứng và liên kết đầu việc nguồn khi dữ liệu hiện hữu; giữ dialog chỉ đọc.
- Tái sử dụng `/projects/:id/evidence`, `/evidence/:id/file` và project operations cache; không thêm migration hoặc thay đổi lifecycle/API ghi báo cáo.
- Cập nhật frontend/construction mappings, chạy frontend build, backend test phù hợp và `git diff --check`; không chạy browser verification nếu người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Approve`.
- Confirmed the current report detail is the generic `RecordDetailDialog` definition list, while the report row already carries structured answers, source IDs and evidence IDs.
- Confirmed project evidence metadata/file routes and project operations work-item metadata already exist, so the approved detail can remain a frontend/read-model change.
- Added `ConstructionReportDetailDialog.tsx` as a responsive 1080px read-only workspace with date/weather/source context, progress bar, four work cards, resources, neutral field notes, linked evidence and source work-item navigation.
- Added `constructionReportViewModel.ts` and reused it from the timeline and detail dialog so structured and legacy daily reports share one parser; structured blocked items/blockers are separated from free-text `issues_summary` in the detail.
- Reused the permission-scoped project evidence read, protected evidence gallery/file behavior, project operations cache and existing work-item return navigation; no API contract, backend handler, lifecycle, migration or database mapping changed.
- Added the shared `Dialog` `report` width and updated central/frontend/construction mapping files for the new component ownership, read-model invariant and evidence/source behavior. No backend-function map update was required because backend behavior and contracts are unchanged.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `cd backend && go test ./...`: pass.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.

## Construction Daily Report Related Work Wording

Status: complete

Mục đích chính:
- Thay thuật ngữ kỹ thuật `Đầu việc nguồn` bằng wording người dùng dễ hiểu `Công việc liên quan` trong toàn bộ luồng Báo cáo thi công, đồng thời giữ nguyên liên kết `work_item_id`, evidence, permission và API.

Will be done:
- Đổi wording trong form tạo/sửa, dialog thêm minh chứng, chi tiết báo cáo và cảnh báo tải tệp của module `operations`.
- Đồng bộ thông báo validation backend của riêng Báo cáo thi công để lỗi trả về không lộ lại wording cũ.
- Giữ nguyên module Nghiệm thu, payload, route, trạng thái, persistence và workflow-form gates.
- Cập nhật central/frontend/construction mappings với thuật ngữ UI mới.
- Chạy tìm kiếm chuỗi, frontend build, backend tests và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Duyệt, làm đi`.
- Loaded the mandatory central/frontend/construction mappings and confirmed the change is wording-only over the existing source-work-item/evidence contract.
- Confirmed the target files overlap a dirty worktree; patched only the approved user-facing strings and preserved unrelated changes.
- Replaced `Đầu việc nguồn` and adjacent technical wording with `Công việc liên quan`, `Công việc`, `Chọn công việc` and `Không chọn công việc` across create/edit, evidence capture, evidence-only add and report detail states.
- Updated the daily-report backend validation messages to use the same terminology while preserving `work_item_id`, permission checks, evidence ownership, the 50-record cap and workflow-form read-only gates.
- Left the separate Acceptance wording unchanged as approved; no route, request/response type, migration, seed or domain lifecycle changed.
- Updated central, frontend and construction-domain mappings with the user-facing terminology and unchanged internal contract.
- Confirmed the Operations module has no remaining old source-work-item wording; the one remaining `Đầu việc liên kết` backend string belongs to a separate out-of-scope flow.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `cd backend && make test`: pass.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.
