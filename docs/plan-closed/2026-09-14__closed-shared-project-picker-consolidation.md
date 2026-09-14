# [Closed] Shared project-picker consolidation

Status: IMPLEMENTED; TECHNICALLY VERIFIED; NOT UX ACCEPTED

Mục đích chính:
- Chuẩn hóa mọi điểm chọn công trình còn lại về shared async picker, giữ đúng phạm vi quyền và không để tracker hướng agent quay lại component cũ.

Done:
- Đã có phê duyệt triển khai theo review package.
- Giữ endpoint permission-specific đã có của Procurement; không ghi đè thay đổi trước đó.
- Đã thêm `GET /projects/options` với `project.view`, project scope, tìm kiếm, cursor pagination và selected-ID có ủy quyền.
- Đã chuyển chín consumer còn lại sang shared async picker và xóa component cũ cùng tracker liên quan.
- Đã cập nhật central/frontend/backend mappings và bổ sung regression coverage frontend/backend.
- Đã pass `go test ./...`, frontend picker regressions, `npm run build` và `git diff --check`.

Scope limitation:
- Browser/UAT không chạy vì người dùng chưa yêu cầu quyền browser trong lượt triển khai; trạng thái UX là `NOT UX ACCEPTED`.
- Full frontend suite còn fail ở `projectEvidenceColumns.test.ts`: test đòi header `Giai đoạn` trong khi source hiện dùng `Nguồn / Giai đoạn`; Evidence Gallery không thuộc phạm vi các file đã sửa.
