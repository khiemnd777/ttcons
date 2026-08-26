# [Closed] Count `Làm ngay` theo phạm vi Bàn làm việc

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED. Closed by user request on 2026-08-26.

## Mục tiêu

Backend duy trì count `Làm ngay` cho năm scope tương thích; Bàn làm việc hiển thị bốn phạm vi `Tôi phụ trách`, `Phòng ban`, `Đang theo dõi` và `Tất cả được phép xem`, đồng thời giữ số liệu khớp với các dòng hành động thực tế.

## Will be done

- Thêm actor-scoped API summary trả đồng thời count của cả năm phạm vi và hỗ trợ lọc theo Công trình.
- Đếm Work Item theo predicate `need_action` hiện hành; cộng workflow action sẵn sàng và project startup cần thiết lập chỉ vào `Tôi phụ trách` và `Tất cả được phép xem`.
- Hiển thị count bằng chip trên bốn lựa chọn của `ScopeRail` chỉ khi tổng `Làm ngay` lớn hơn `0`; loading, lỗi và số `0` không tạo placeholder chip.
- Bổ sung manual refresh, realtime invalidation, backend/frontend regression tests và cập nhật mapping Consflow.
- Chạy Go format/test, frontend tests và frontend build; không chạy browser/UAT nếu không được yêu cầu riêng.

## Done

- Review package, Mockup as Text và năm điểm phạm vi đã được duyệt bằng `OK triển khai`.
- Đã xác nhận từ code rằng danh sách `Làm ngay` kết hợp Work Item, workflow action sẵn sàng và project startup; API hiện tại chỉ có summary cho phạm vi đang chọn.
- Đã thêm `GET /me/work-items/scope-summary?project_id`: một truy vấn đếm Work Item theo năm phạm vi và một lần đánh giá workflow-ready toàn phạm vi; response giữ breakdown `work_items`, `workflow_actions`, `project_startups` và `total`.
- Đã giới hạn nguồn workflow/startup vào `Tôi phụ trách` và `Tất cả được phép xem`; backend `mine`, `department`, `watching` chỉ nhận Work Item `need_action` theo đúng queue hiện hành. Scope `mine` không còn là lựa chọn UI sau review đơn giản hóa.
- Đã sửa presentation theo review bổ sung: desktop/mobile chỉ hiển thị chip count khi tổng `Làm ngay` lớn hơn `0`; loading, lỗi và số `0` đều không tạo chip. Manual refresh, project scoping và realtime invalidation giữ nguyên.
- Đã cập nhật backend/frontend/project-planning/workflow-orchestration mapping và regression tests; không có migration hoặc thay đổi schema.
- Đã chạy toàn bộ Go package tests, 284/284 frontend tests, frontend production build và bundle budget; hai test đối chiếu scope/workflow với PostgreSQL Docker cục bộ đều pass. Browser/UAT không chạy theo giới hạn xác minh của repo.
