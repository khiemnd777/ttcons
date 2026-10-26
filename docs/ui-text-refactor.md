# Dọn text giao diện ngoài Pipeline và Workflow Form

## Phạm vi đã duyệt — 2026-10-26

Mục đích chính: ưu tiên dữ liệu và hành động, giảm tagline/chỉ dẫn lặp trên trang, dialog, rail và bảng. Rail là cột phụ/thanh bên trong màn hình. Người dùng đã duyệt proposal, triển khai ba đợt, sau đó giao browser/UAT desktop/mobile và TechNote Level4; chưa giao commit, push hoặc deploy.

Quy tắc: bỏ copy tĩnh lặp; rút câu khi giữ đủ nghĩa; mở giải thích bổ sung khi cần; giữ labels, trạng thái, giới hạn nhập, điều kiện nguồn, cảnh báo và nhãn CTA nghiệp vụ. Không sửa mô tả do người dùng nhập, dữ liệu lịch sử, nguồn/revision, quyền, payload, query, API hay database.

## Ba đợt implementation

| Đợt | Đã thực hiện | Giữ lại có chủ đích |
|---|---|---|
| 1 — Trang/bảng | Bỏ tagline tại các page callsite; bỏ eyebrow/mô tả Báo cáo; bỏ “Tự tổng hợp” trong Mua sắm; bỏ mô tả folder điều hướng; dùng chevron mở folder; bỏ subtitle metric lặp | MyWork tên/role, Thông báo counts, Vật tư chế độ chỉ xem, Kế hoạch tên công trình/mẫu; tiền, mô tả dòng thật, trạng thái và toàn bộ CTA nghiệp vụ |
| 2 — Dialog/rail | Bỏ hướng dẫn nhập lặp ở Profile/Staff/Acceptance/Operations; rút import/export; dọn các section báo cáo, rail Bản gửi/Chuỗi liên quan; icon hóa làm mới Dashboard/Audit và mở trao đổi trong rail việc cá nhân | Cảnh báo bảo mật/xóa, required/inline error, formats/max size, khóa nguồn, bản chụp khác nguồn hiện tại, actor/time/revision/comment; mô tả tiêu chí thật ở desktop/mobile/detail |
| 3 — Tích hợp | Review độc lập bởi ba agent; sửa cap tiêu đề Báo cáo; bổ sung tiêu chí chọn loại yêu cầu trong dialog Rework; tests/build/bundle budget và so sánh vùng loại trừ | Browser/UAT chưa chạy; không tuyên bố UX đã nghiệm thu hoặc tốc độ thao tác đã được đo |

Các nhóm đã rà: Dashboard; Công trình/Tiến trình/Tổng quan; MyWork/Thảo luận; Thông báo; hồ sơ chuẩn bị/checklist/báo cáo kế hoạch/biên bản; BOQ/Báo giá/Mua sắm/Hợp đồng; Kho/Vật tư; Thi công/QA/QC/Sửa lỗi/Nghiệm thu/Bàn giao; Hồ sơ bàn giao kế toán; Đối tác/Nhân sự/Vai trò/Phân quyền; Audit/Event logs; Thống kê/Báo cáo; Profile/Auth/Recovery/Cấu hình/Hồ sơ nguồn. Bề mặt chỉ có dữ liệu hoặc cảnh báo cần thiết được giữ nguyên; đây không phải mục tiêu xóa mọi đoạn mô tả.

## Component và entrypoint

- `ContextHelp` mới là disclosure mở tại chỗ, không phải hover-only tooltip. Nút có tên, expanded/controls; mở bằng nút native (chuột/chạm/bàn phím), đóng bằng nút, Escape, pointer ngoài hoặc focus rời vùng. Không tạo overlay để tránh clipping. Dùng tại Reports (nguyên văn cách tính/phạm vi từng báo cáo), Profile avatar (cơ chế lưu), AccountingHandoffPage (ranh giới MISA), Rework (giải thích hai loại yêu cầu). Dialog Rework vẫn hiện tiêu chí chọn loại trước submit.
- `IconAction` mới dành cho thao tác phụ, có `type=button`, accessible name, native title, disabled/focus state và vùng bấm 44px. Dùng tại Dashboard/Audit làm mới và DiscussionWorkRail mở trao đổi đúng item. Native title chưa chứng minh trợ giúp trên touch; không có custom tooltip touch.
- Tiếp tục tái sử dụng PageHeader, DataTable/RowActions, Dialog, SimpleForm/field owner, RecordDetailDialog/RecordSection, feedback chuẩn, EvidenceUploader và EvidenceRecordViewerDialog. Không đổi defaults của các component này hoặc global CSS. AuthCard chỉ phục vụ auth được thêm description optional; các SectionHeading/MetricCard cục bộ cũng hỗ trợ bỏ mô tả trống.
- Không đổi trải nghiệm xem file, source endpoint, current/historical file/revision hoặc upload timing. Các evidence composition giữ nguyên source.

## Ranh giới Pipeline/Form được bảo vệ

Toàn bộ `pipeline-builder/**`, `workflow-forms/**`, `workflow-definitions/**`, `WorkflowForm*`, workflow signature components và `DiscussionWorkflowActionCard` giữ nguyên. `PlanningPage`, `ConstructionHandoffPanel`, `ProjectStartupWizard`, `WorkItemDetailDialog`, `WorkItemPrimarySurface`, `DiscussionWorkUpdateCard` giữ nguyên toàn file.

Trong ProjectDetailPage, giữ nguyên tám hàm Pipeline: ProjectWorkflowWorkspace, WorkflowEmptyState, WorkflowExecutionSummary, WorkflowSummaryMetric, WorkflowStageRail, WorkflowWorkPanel, WorkflowWorkTableRow, WorkflowWorkMobileCard. Chỉ dọn header tổng quan ngoài các hàm này.

Trong MyWorkPage, giữ nguyên WorkflowActionCard, NextQueuePanel, ProjectStartupActionCard và ProjectStartupNextPanel. Chỉ dọn header, copy scope/attention ngoài các hàm này.

Shared components có từ baseline và shared module defaults (trừ header của RecordSourcePage) được so sánh nguyên văn với baseline. Không dùng CSS ẩn text toàn cục hoặc thay các default để tác động ngầm vào Pipeline/Form.

## Kiểm chứng giai đoạn refactor đầu — lịch sử

Baseline: `fba39cdcf72a2d6714d5eac3d15eba8cdf4014c9`, nhánh main, working tree có thay đổi chưa commit. Các tài liệu/output đã có trước task được giữ nguyên. Môi trường: local macOS, Node v23.11.0, dependencies hiện có; không Docker restart, DB, migration hoặc production.

- `cd frontend && npm test`: PASS 617/617, gồm sáu test mới về help/icon/rail/revision; có controlled hooks/event targets và React server rendering, không browser.
- `cd frontend && npm run build`: PASS TypeScript và Vite. Build đầu phát hiện description bắt buộc ở hai helper cục bộ; đã sửa và build lại bản source cuối.
- `cd frontend && npm run check:bundle-budget`: PASS 133 JavaScript chunks; giữ gate raw 500kB/gzip250KiB và code-splitting hiện có.
- `git diff --check`: PASS.
- `node docs/acceptance-evidence/ui-text-refactor-2026-10-26/verify-boundary.mjs`: PASS 112 file + 12 hàm embedded nguyên văn so với baseline. Đây là receipt cho task này, không khóa thay đổi hợp lệ của tương lai.
- Bốn source presentation test cũ được cập nhật theo copy/layout đã duyệt; assertions về quyền, query, nguồn dữ liệu và optional fields vẫn giữ. Không bỏ test hoặc nới quyền để làm xanh.

Receipt có fingerprint source/tests tại `docs/acceptance-evidence/ui-text-refactor-2026-10-26/checks.json`. Kết quả tests/build chỉ áp dụng cho fingerprint đó. Không có lỗi kiểm tra kỹ thuật còn mở tại thời điểm ghi receipt.

**Trạng thái source:** IMPLEMENTED / TECHNICALLY VERIFIED local. **Browser/UAT đã chạy, NOT UX ACCEPTED toàn phạm vi.**128 ca:122 PASS/4 FAIL/2 BLOCKED. Bốn ca mobile tràn ngang tại Audit/EventLogs/Thảo luận đã đối chiếu baseline cùng lỗi; catalog thông báo local HTTP500, còn copy tĩnh cần dọn.14 đối chiếu nội dung Pipeline/Form/nhúng khớp. Giọng/video tiếng Việt và ghi chú tại `output/technote-level4-ui-refactor-2026-10-26/`; receipt `docs/acceptance-evidence/ui-text-refactor-2026-10-26/browser-summary.json`. Chưa đo tốc độ thao tác/người dùng thực/thiết bị vật lý, chưa thử luồng ghi nghiệp vụ. Chưa commit, push hoặc deploy.

## Khắc phục sau UAT — package đã duyệt ngày 26-10

Mục đích chính: khôi phục catalog thông báo, xử lý ba trang mobile và dọn đúng danh sách copy còn lại. Người dùng đã duyệt toàn bộ review package/Mockup as Text và browser/UAT local; chưa giao commit/push/deploy.

- Catalog: `NotificationCatalog` chuyển PostgreSQL TEXT[] sang JSON để giải mã bằng pgx/database/sql, giữ nguyên API/DTO và cờ enabled/required. Không migration hoặc sửa dữ liệu catalog.
- Dialog mới tách tại `notifications/NotificationPreferencesDialog.tsx`, tiếp tục dùng shared `Dialog`, `LoadingSpinner`, `ToastProvider`. Loading/error/empty của hai query được phân biệt; retry đúng nguồn; không lưu rỗng/chưa thay đổi; required groups khóa. Chỉ ghi nhóm tùy chọn đã đổi. Lưu một phần thất bại không báo thành công, refetch baseline và giữ thay đổi chưa lưu. Đóng dialog reset cả draft và lỗi mutation.
- Đo browser và DOM experiment xác nhận project select rộng787/781px làm Audit/EventLogs tràn820/814px tại viewport390; min/max width tại consumer đưa root về390. Discussion implicit auto grid/rail làm root540; explicit minmax(0,1fr)/min-width host đưa về390. Source chỉ sửa filter local và ActivityThread host, không đụng nội dung card Pipeline.
- Bỏ hai subtitle Profile và hai intro report hero. AppShell chỉ giữ tagline trên Workflow Form bằng matcher tương đương router, gồm dấu slash cuối và hoa/thường. Dữ liệu, source/revision và cảnh báo nghiệp vụ vẫn giữ.
- Review độc lập đã xử lý hai finding: lỗi lưu cũ khi mở lại dialog; ngoại lệ header Workflow Form trên URL alias.

**Bằng chứng hiện tại:** `docs/acceptance-evidence/ui-text-refactor-2026-10-26/fix-summary.json`. 56 ca hợp lệ PASS,0 FAIL,0 BLOCKED: desktop1440×1000 và mobile360/390/430×844 có touch emulation. 14 đối chiếu browser giữ cùng nội dung; Form nhúng so sánh trọn vùng WorkflowFormPanel, ghi riêng thay đổi AppShell bên ngoài đã duyệt. Không tuyên bố mọi pixel giống nhau. Guard112 file+12 vùng source nguyên văn baseline tiếp tục PASS.

Frontend622/622 tests PASS; build TypeScript/Vite PASS, bundle133chunks PASS. Test catalog PostgreSQL thật và hai unit test notification filter/template PASS; full backend suite NOT_RUN. Source/tests SHA256 `f111335a9f1653f738fc581bd30e042efcb5637aed1221c954e7622873e01fb9`; thời gian build/test ở receipt. Backend image `sha256:c6e6f71d6e58d53b36593f512f01dad4b8906d8f6caf5732f91e53ac000c6cf3`; local guide DB160 migrations trước/sau, AUTO_MIGRATE=false. Regression DB riêng `consflow_ui_fix_test_9fb9d24ace`; không chạy migration lên guide/production.

Catalog local thật200 với83events,21bật,6nhóm hiện tại đều bắt buộc. Luồng lưu nhóm tùy chọn, lỗi tải/empty/loading/lưu một phần được kiểm tra qua HTTP fixtures có ghi rõ; không thêm nhóm hoặc thay dữ liệu catalog/prefs thật để làm UAT đạt. Không coi các ca đó là end-to-end lưu tùy chọn qua backend thật.

Lượt browser đồng thời chạm auth rate-limit bị loại khỏi kết luận. Recheck chạy tuần tự, giãn baseline load; không hạ bảo vệ. Ba ca Event Logs dùng locator công trình cố định không tồn tại trong facet được thay bằng recheck chọn option API thật. Receipt thô vẫn giữ; summary dùng kết quả hợp lệ mới nhất theo từng ca.

**Trạng thái:** IMPLEMENTED / TECHNICALLY VERIFIED LOCAL. Chưa có nghiệm thu của người dùng đại diện hoặc điện thoại vật lý; chưa đo tốc độ, camera, mọi Form/validation hoặc luồng ghi nghiệp vụ. Không commit/push/deploy. Video trước khắc phục là lịch sử; bản TechNote4 sau khắc phục ở `output/technote-level4-ui-refactor-2026-10-26/sau-khac-phuc/`.

## Bước tiếp theo

**Nghiệm thu local chạy lại ngày26-10 theo yêu cầu người dùng:** hai bộ chạy tuần tự trên cùng fingerprint source/tests và build không đổi.56 ca PASS,0 FAIL/BLOCKED/CHECK_REQUIRED/NOT_RUN trong registry,0 JavaScript pageerror;14 đối chiếu Pipeline/Form và guard112file+12vùng PASS. Preview HTML khớp artifact build; backend image và guide DB160migrations không đổi. Receipt mới tại `docs/acceptance-evidence/ui-text-refactor-2026-10-26/acceptance-local-20261026/summary.json`. Đây là LOCAL_UAT_PASS_AGENT; các giới hạn fixtures/thiết bị thật/người dùng đại diện vẫn giữ. Không sửa source hoặc chạy lại build/tests vì fingerprint đã đối chiếu không đổi; mapping không cần cập nhật trong lượt nghiệm thu này.

1. Người dùng đại diện nghiệm thu bản local và xác nhận phạm vi còn cần thử trên thiết bị thật.
2. Sau review source và nghiệm thu phù hợp, người dùng có thể giao commit; push/deploy cần phạm vi riêng.
