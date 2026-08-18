# [Closed] Đổi tên “Tiến độ & Phụ thuộc” thành “Quy trình”

## Objective

Đổi đồng bộ nhãn người dùng của workspace `/projects/:id/pipeline` thành `Quy trình` mà không thay đổi route, API, quyền, trạng thái hoặc logic điều phối workflow.

## Will Be Done

- Đổi menu, tiêu đề trang, thẻ tổng hợp, tab tổng quan, liên kết và câu hướng dẫn liên quan thành `Quy trình`.
- Giữ nguyên `/pipeline`, API, quyền và logic điều phối workflow.
- Cập nhật mapping frontend, project-planning và workflow-orchestration liên quan.
- Kiểm tra chuỗi cũ còn sót và chạy frontend build.

## Done

- Người dùng đã duyệt tên mới, review package và Mockup as Text.
- Đã đổi menu, tiêu đề trang, thẻ tổng hợp, tab tổng quan, liên kết và câu hướng dẫn liên quan thành `Quy trình`.
- Đã giữ nguyên `/pipeline`, API, quyền và logic điều phối workflow.
- Đã cập nhật module keyword map, frontend UI utility map và domain map của project planning/workflow orchestration.
- `cd frontend && npm run build` hoàn tất thành công trên working tree hiện tại ngày 2026-08-18.
- Không chạy browser/UAT theo quy định repository; trạng thái UI/UX là `NOT UX ACCEPTED`.
- Đóng theo yêu cầu của người dùng ngày 2026-08-18.
