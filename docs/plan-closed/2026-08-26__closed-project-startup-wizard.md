# [Closed] Wizard khởi tạo công trình 3 bước

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED. Closed by user request on 2026-08-26.

## Mục tiêu

Thay luồng tạo công trình thủ công bằng wizard `Thông tin → Chọn nhân sự → Thiết lập`, bảo đảm công trình mới có nhân sự hợp lệ và PM đủ quyền trước khi khởi chạy pipeline.

## Will be done

- Bổ sung API bulk membership nguyên tử, giới hạn số lượng, kiểm tra trùng nhân sự và giữ audit cho từng thành viên.
- Mở rộng metadata vai trò để frontend xác định chính xác người đủ điều kiện PM.
- Tạo wizard dùng chung cho trang Công trình và Bàn làm việc; bước nhân sự có tìm kiếm không dấu, checkbox và cấu hình vai trò.
- Khóa thành viên đang hoạt động trong wizard; giữ luồng vô hiệu hóa tại màn Thành viên.
- Giữ nguyên luồng import công trình hàng loạt.
- Cập nhật tests và các mapping bị ảnh hưởng; chạy backend tests và frontend build.
- Cho phép quay lại các bước đã hoàn tất; khi resume từ Bàn làm việc phải tải đúng dữ liệu Công trình, giữ draft nhân sự và chỉ cho sửa khi có `project.update`.

## Done

- Đã xác nhận root cause từ code: dialog Thiết lập chỉ lấy thành viên đang hoạt động, trong khi luồng tạo công trình chưa tạo membership.
- Review package, Mockup as Text và 5 điểm phạm vi đã được duyệt bằng `OK triển khai theo 5 điểm trên`.
- Đã bổ sung `POST /projects/:id/members/bulk`: 1–50 Staff duy nhất, normalize trước khi ghi, lưu membership/role/audit trong một transaction và chỉ invalidation sau commit.
- Đã mở rộng metadata global/project role với các grant `project.view`, `project.member.manage`, `pipeline.run`.
- Đã thay dialog cũ bằng `ProjectStartupWizard` dùng chung cho Công trình và Bàn làm việc, có progress 3 bước, search không dấu, checkbox, cấu hình vai trò, resume và khóa membership đã lưu.
- Đã giữ nguyên luồng import hàng loạt và chỉ thay luồng tạo thủ công.
- Đã cập nhật frontend/backend/domain mapping và regression tests.
- Đã chạy frontend build, frontend tests, toàn bộ Go tests và nhóm Project Member DB-backed trên PostgreSQL Docker cục bộ; tất cả pass ở lần kiểm tra ban đầu.
- Review package bổ sung về điều hướng lùi đã được duyệt bằng `OK triển khai`.
- Đã bổ sung điều hướng từ `Chọn nhân sự → Thông tin` và `Thiết lập → Chọn nhân sự`, đồng thời cho bấm bước đã hoàn tất trên progress.
- Wizard resume tải lại bản ghi Công trình và dùng chung cấu hình form/danh mục với trang Công trình; draft nhân sự không bị xóa khi đổi bước.
- Startup readiness đã trả capability `can_update_project`; backend `PUT /projects/:id` tiếp tục là lớp thực thi quyền chính thức.
- Source hiện tại đã pass frontend build, 283/283 frontend tests, bundle budget, toàn bộ Go tests và focused startup integration trên PostgreSQL Docker cục bộ; browser/UAT chưa chạy theo giới hạn xác minh của repo.
