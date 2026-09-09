# [Closed] Thống nhất Revision thiết kế vào Hồ sơ/Minh chứng

## Objective

Giữ Revision thiết kế là gói nghiệp vụ có phiên bản, đồng thời đưa toàn bộ ảnh, PDF và tài liệu của gói vào sổ chuẩn `evidence_records` thay vì vận hành một hệ file độc lập.

## Will Be Done

- Thêm liên kết `design_release_items` từ revision thiết kế tới bản ghi chuẩn `evidence_records`, kèm migration/backfill giữ nguyên blob và lịch sử tài liệu cũ.
- Chuyển upload/phát hành/bàn giao revision sang vòng đời Evidence: nháp, chờ rà soát, đã chấp nhận.
- Đưa hồ sơ thiết kế vào Hồ sơ công trình với ngữ cảnh revision, bộ lọc/tìm kiếm và quyền truy cập đúng nguồn.
- Dùng `EvidenceUploader` và `EvidenceRecordViewerDialog` tại các điểm vào thiết kế mới; giữ viewer tài liệu cũ cho snapshot lịch sử.
- Bổ sung kiểm thử backend/frontend, cập nhật module/domain maps và chạy kiểm tra không dùng trình duyệt.

## Done

- Đã xác nhận nguyên nhân: upload thiết kế ghi vào `documents`, trong khi Hồ sơ công trình chỉ đọc `evidence_records`.
- Đã chốt mô hình đích và Mockup as Text; người dùng xác nhận triển khai.
- Đã thêm V140-V142: `design_release_items`, backfill giữ nguyên storage key/checksum và không sao chép blob, chặn write legacy vào `documents`, bảo toàn cascade owner nhưng vẫn khóa item của revision đã phát hành.
- Upload thiết kế mới tạo Evidence `draft`; phát hành chuyển sang `submitted`; PM bàn giao chuyển sang `accepted`; generic accept/reject theo từng file bị chặn.
- Hồ sơ công trình đọc chung Evidence và hiển thị ngữ cảnh revision. Draft và superseded-unhanded không nằm trong hàng chờ; current released-unhanded nằm trong hàng chờ; handed-off nằm trong đã chấp nhận.
- `DesignReleasePanel` dùng `EvidenceUploader`, `EvidenceDraftViewerDialog` và `EvidenceReferenceViewButton`; snapshot mới dùng Evidence, snapshot document cũ tiếp tục dùng adapter được bảo vệ.
- Đã cập nhật central/backend/frontend/root-cause và ba domain map liên quan.
- Database sạch đã áp đủ 142 migration; full backend `go test -p 1 ./... -count=1` đã qua. Frontend test trọng tâm 20/20 và `npm run build` đã qua.
- Database local đã migrate/check đủ V142. UAT-R01 có một Evidence `accepted`, storage key/checksum khớp tài liệu cũ. Backend image đã rebuild; `/api/health` và `/api/ready` trả 200.
- Không chạy browser/UAT theo quy định repository; trạng thái UI/UX là `NOT UX ACCEPTED`.
- Đóng theo yêu cầu của người dùng ngày 2026-09-09.
