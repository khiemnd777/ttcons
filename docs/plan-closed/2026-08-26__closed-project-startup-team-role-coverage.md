# [Closed] Hiển thị đội ngũ và độ phủ vai trò khi thiết lập công trình

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED. Closed by user request on 2026-08-26.

## Mục tiêu

Giúp người thiết lập nhìn thấy đầy đủ thành viên đã tham gia Công trình, vai trò vận hành của từng người và mức độ phủ vai trò của toàn bộ Pipeline trước khi khởi động, thay vì chỉ nhìn thấy lựa chọn PM và người làm bước đầu.

## Will be done

- Mở rộng `GET /projects/:id/startup-readiness` bằng projection additive về độ phủ vai trò của Pipeline đã chọn; không biến độ phủ toàn Pipeline thành điều kiện khởi động mới.
- Chỉ tính thành viên Công trình đang hoạt động, có Staff/User App đang hoạt động; giữ nguyên role label snapshot và quyền hiện hành.
- Bước `Thiết lập` hiển thị toàn bộ đội ngũ, vai trò vận hành, trạng thái tài khoản, PM hiện tại và danh sách vai trò Pipeline đã có người/chưa có người.
- Giữ riêng khối chỉ định PM, người thực hiện bước đầu, blockers và CTA bắt đầu quy trình.
- Không thay đổi notification, audit, membership mutation, template/runtime snapshot hoặc Pipeline start gate.
- Cập nhật backend/frontend contract tests, mapping liên quan; chạy format, focused/full tests và frontend build. Browser/UAT không chạy nếu không được yêu cầu riêng.
- Sửa regression khi vai trò chưa có người bị API tuần tự hóa `members: null`: backend phải trả `[]`, frontend chuẩn hóa phòng vệ để bước `Thiết lập` không làm sập route.

## Done

- Review package tập trung vào đội ngũ và độ phủ vai trò đã được duyệt bằng `OK triển khai`.
- Đã xác nhận từ code và dữ liệu cục bộ rằng membership được lưu đầy đủ; thiếu sót nằm ở presentation của `SetupStep`, còn readiness chỉ trả coverage của bước đầu.
- Đã chốt giữ nguyên notification: người tạo/người thiết lập hiện hành nhận thông báo; không broadcast cho các vai trò vận hành khác.
- `GET /projects/:id/startup-readiness` đã trả additive `role_coverage` từ toàn bộ Node/Step của đúng Pipeline template đã chọn, kèm số Node/Bước, thành viên App hoạt động phù hợp và trạng thái covered; không thêm blocker mới.
- Bước `Thiết lập` đã hiển thị hai khối responsive: toàn bộ đội ngũ đã lưu với vai trò vận hành/trạng thái tài khoản/PM, và độ phủ vai trò toàn Pipeline với người phù hợp hoặc trạng thái còn thiếu.
- Khối chỉ định PM, người thực hiện bước đầu, blockers và CTA bắt đầu quy trình giữ nguyên trách nhiệm; notification, audit, membership mutation và template/runtime không thay đổi.
- Đã cập nhật backend/frontend/project-planning/workflow-orchestration mapping và focused regression tests; không cần migration hoặc cập nhật admin-governance map vì quyền/RBAC không đổi.
- Working tree hiện tại đã pass toàn bộ Go packages, 285/285 frontend tests, production build, bundle budget và DB-backed startup integration trên PostgreSQL Docker cục bộ. Backend image đã rebuild/recreate; startup log báo `applied=0 skipped=130`, `/api/health` và `/api/ready` đều trả OK. Browser/UAT không chạy theo giới hạn xác minh của repo, nên UI vẫn ở trạng thái `NOT UX ACCEPTED`.
- Review package regression `members: null` đã được duyệt; chẩn đoán runtime xác nhận bulk save trả `201`, readiness trả `200`, còn frontend lỗi khi dereference `role.members.length` cho vai trò chưa có người.
- Backend đã khởi tạo `role_coverage[].members` thành non-nil slice để JSON luôn trả mảng; frontend đồng thời dùng `Array.isArray` để chuẩn hóa response cũ trước khi đọc hoặc render danh sách.
- Regression test fail-before/pass-after đã khóa cả JSON `members: []` và frontend guard. Working tree sau sửa pass toàn bộ Go packages, 285/285 frontend tests, production build và focused startup integration trên một PostgreSQL cô lập đã được xóa sau kiểm tra.
- Backend local đã rebuild/recreate thành công; migration log `applied=0 skipped=130`, health/readiness đều OK. API runtime của đúng `PRJ-65131` trả `200` và cả 8 vai trò đều có `members` dạng array, gồm `[]` cho 5 vai trò còn thiếu. Browser/UAT chưa chạy nên UI vẫn là `NOT UX ACCEPTED`.
