# [Closed] Mở rộng Image Viewer cho minh chứng

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED. Closed by user request on 2026-08-26.

## Will be done

- Chuẩn hóa shared evidence viewer để tải protected ảnh/PDF/Office từ `EvidenceRecord` đầy đủ hoặc tham chiếu tối thiểu `id + name`.
- Giữ nguyên quyền tải `/evidence/:id/file`, storage/audit/status và mọi transition nghiệp vụ.
- Áp dụng viewer tại Thi công (kiểm tra, cần xử lý), Nghiệm thu/Bàn giao, đối soát và các form chọn minh chứng Vật tư/Work Item theo review package đã duyệt.
- Bổ sung accessibility cho dialog lồng nhau, trạng thái loading/error/retry và cleanup object URL.
- Cập nhật regression tests, central/frontend/domain mappings và chạy frontend validation.

## Done

- Đã duyệt review package và Mockup as Text.
- Đã xác nhận shared owner: `EvidenceViewerDialog`, `EvidenceRecordViewerDialog`, `EvidenceGallery`.
- Đã xác nhận các consumer cũ còn mở raw tab hoặc chỉ hiện tên trong Operations, Acceptance, Handover và Work Item domain panels.
- Đã thêm `EvidenceReferenceViewer` cho tham chiếu ID/name, nhận dạng MIME/filename, URL ngoài an toàn, retry và lifecycle object URL.
- Đã chuyển Operations check/action/timeline, Acceptance detail/editor, Handover linked evidence, Material snapshot, Incoming acceptance, Surplus handling và hai panel reconciliation sang viewer tại chỗ.
- Đã bổ sung focus trap, Escape top-dialog-only và restore focus trong shared viewer.
- Đã cập nhật central/frontend và các domain mapping liên quan cho shared viewer cùng các consumer đã chuyển đổi.
- TECHNICALLY VERIFIED trên working tree hiện tại: `npm run build`, `npm test` (278/278), `npm run check:bundle-budget`, `git diff --check` đều đạt.

## Preserved contracts

- Giữ nguyên backend endpoint `/evidence/:id/file`, auth/RBAC, storage, audit, evidence ID trong payload và mọi state transition nghiệp vụ.
- Không có thay đổi backend, database, schema hoặc migration.
- Các đường mở raw còn lại chỉ phục vụ tệp gốc/định dạng không hỗ trợ hoặc workflow artifact ngoài phạm vi plan.

## Skipped / remaining acceptance

- Browser/UAT không được yêu cầu trong lượt triển khai nên chưa chạy; trạng thái UX là `NOT UX ACCEPTED`.
