# [Closed] Trao đổi mở thẳng workspace xử lý Đầu việc

Status at closure: IMPLEMENTED · TECHNICALLY VERIFIED for target scope · NOT UX ACCEPTED

Known limitations at closure:

- Browser/UAT không chạy vì người dùng chưa yêu cầu xác minh bằng trình duyệt; giao diện vẫn là `NOT UX ACCEPTED`.
- Full frontend suite còn một lỗi baseline ngoài phạm vi tại `projectEvidenceColumns.test.ts`: `HEAD` đã dùng nhãn `Nguồn / Giai đoạn` trong `EvidenceGallery.tsx`, trong khi test cũ vẫn đòi `Giai đoạn`.

Done:

- Đã xác định owner UI và phạm vi thay đổi qua module memory, frontend workflow map và workflow orchestration domain map.
- Đã xác nhận compact-to-full toggle chỉ thuộc `DiscussionWorkDialog`; các dialog nghiệp vụ chuyên biệt không thuộc phạm vi loại bỏ.
- `DiscussionWorkDialog` mở thẳng `WorkItemDetailWorkspace` cho Đầu việc không thuộc mua sắm; đã bỏ nhánh compact và các nhãn `Xem toàn bộ & lịch sử` / `Quay lại hồ sơ`.
- `ProcurementDialog` vẫn là owner của yêu cầu mua; đường dẫn sang điều kiện Đầu việc và nút `Quay lại yêu cầu mua` được giữ riêng.
- Workspace đầy đủ truyền dirty/busy từ form, minh chứng và thiết kế về guard đóng/bỏ thay đổi/`beforeunload`; tab hỗ trợ không rời editor đang bẩn hoặc đang lưu trong entrypoint Trao đổi.
- Đã cập nhật central map, frontend UI map, root-cause triage map và workflow orchestration domain map.

Validation:

- PASS: `cd frontend && npm run build` trên working-tree source đã triển khai, gồm TypeScript no-emit và Vite production build.
- PASS: `cd frontend && npm run check:bundle-budget` — 121 chunks, không vượt ngưỡng raw/gzip.
- PASS: `discussionWork.test.ts` + `workItemFocusLayout.test.ts` — 18/18.
- FAIL ngoài phạm vi thay đổi: full `npm test` còn một assertion cũ trong `projectEvidenceColumns.test.ts`; `EvidenceGallery.tsx` và test này không được sửa trong plan.
- SKIPPED: browser/UAT; trạng thái UX là `NOT UX ACCEPTED`.
