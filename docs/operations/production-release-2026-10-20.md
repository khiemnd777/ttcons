# Checklist release production — 2026-10-20

## Trạng thái đã kiểm chứng

| Hạng mục | Trạng thái | Bằng chứng |
| --- | --- | --- |
| Release code | PASS | Runtime gốc `v2026-10-20.10` tại commit `3a09d16` (run `39365390955`). Bản sửa preview `v2026-10-20.17` tại commit `71e8fe0` (run `39384533088`) đạt CI, build image, deploy và public health 204/200. |
| Migration | PASS | Production có 158 bản ghi migration; deploy áp dụng V146–V158. Read-only inventory run `39368171516`. |
| Dọn database demo | PASS | Giao dịch run `39363771944` dọn Project/Run/Work Item; run `39368069174` dọn 20 User/Staff ngoài nhóm bảo vệ cùng session, log, notification revision, lịch sử import và cache demo. Read-only inventory run `39368171516` xác nhận trạng thái sau ghi. |
| Backup phục hồi | PASS trong phạm vi thử nghiệm | Archive V145 và archive V158 đều được khôi phục vào database tạm rồi xóa database tạm. Đây chưa phải diễn tập phục hồi toàn bộ ứng dụng. |
| Object S3 demo hiện hành | PASS trong phạm vi key hiện hành | Run `39369157932` xóa đúng 23 key từ archive và xác nhận từng key không còn đọc được; public health đạt sau restart. Chưa kiểm tra lịch sử phiên bản S3 nếu bucket bật versioning. |
| Workflow dọn demo một lần | Đã tắt trên `main` | Các workflow xóa Project, identity và object S3 được gỡ khỏi nhánh chính sau khi chạy thành công; run và tag lịch sử còn làm bằng chứng. Workflow inventory chỉ đọc và workflow phục hồi container vẫn được giữ trong giai đoạn import. |
| Admin và Developer | PASS về dữ liệu/quyền | Còn 2 Admin, 1 Developer đang active và có Staff active. Grant `developer:staff.import`, `developer:sample_data.create`, `developer:workflow_definition.import`, `admin:workflow_definition.import` đều có. Đăng nhập thực tế chưa được kiểm chứng. |
| Form/role cho Pipeline v10 local | PASS về catalog tham chiếu | 14 Form reference đúng version đều đã publish; 8 role name đều có thể gán. Read-only run `39368500697`. |
| Permission cho Pipeline v10 local | PASS với gói tương thích đã duyệt | Bản export nguyên gốc có 16 blocker do sáu lần dùng bốn key đã nghỉ và thông báo phụ thuộc. Gói được duyệt chỉ bỏ sáu tham chiếu đã nghỉ và nhắm đúng family; preview đạt 0 blocker/0 warning. |
| Pipeline v10 production | PASS về import và publish | Đã tạo draft `cea8f5f1-97d0-420a-acbe-8bf4c87ce9a3` rồi backend publish thành công. UI production hiển thị v10 với 15 Giai đoạn là `Đã phát hành`; v9 vẫn còn. Chưa chạy dự án mẫu để kiểm tra runtime. |
| Nút Kiểm tra trên Pipeline builder | Cần xử lý riêng | Sau import, client báo 14 tham chiếu Form không có trong catalog UI, trong khi backend cho phép publish vì các Form version đã publish/active. Đây là sai lệch giữa tập ID của UI catalog và điều kiện publish backend; chưa có bản sửa UI được duyệt. |

## Trình tự còn lại

1. **[Anh] Kiểm tra đăng nhập:** dùng tài khoản Admin và Developer hiện có để đăng nhập production. Không gửi mật khẩu cho agent. Nếu một vai trò không đăng nhập được, dừng import và xử lý quyền truy cập trước.
2. **[Anh] Import nhân sự:** đăng nhập bằng Developer, mở màn hình Nhân sự, chạy preview của tệp Excel/CSV. Chỉ execute khi `invalid_count=0`, kiểm tra số dòng/vai trò rồi xác nhận. Tệp import này dành cho nhân sự nghiệp vụ; không dùng nó để tạo Admin hoặc Developer. Sau import, gửi kết quả số tài khoản đã tạo, số role assignment và lỗi nếu có.
3. **[Tôi] Kiểm tra file Workflow Pipeline v10 — đã làm:** đã export từ local và preview trên production. Anh đã duyệt gói tương thích bỏ sáu tham chiếu permission đã nghỉ; preview đạt 0 blocker/0 warning.
4. **[Tôi] Import và publish — đã làm:** đã tạo draft revision trong đúng family và publish v10. Gói Pipeline-only không tạo Form draft. UI production xác nhận published v10, published v9 vẫn còn.
5. **[Tôi] Xác minh sau import:** kiểm tra số User/Staff mới, role coverage, bản Form/Pipeline đã publish, migration vẫn V158, không có Project/Run/Work Item demo và các quyền bắt buộc còn hiệu lực. Không xuất danh sách nhân sự hoặc thông tin đăng nhập ra log.
6. **[Tôi] Tạo sample data theo nhân sự mới:** chỉ bắt đầu khi Staff mới và Pipeline v10 đã được xác minh. Generator hiện tại yêu cầu tài khoản mẫu riêng (`is_sample`) và sẽ không tự dùng Staff mới. Cần xác định mapping Staff theo vai trò Pipeline, xử lý vai trò thiếu/trùng và kiểm thử trước khi tạo Project mẫu. Không chạy generator hiện tại để thay thế bước này.

## Điều kiện dừng

- Nếu Admin hoặc Developer không đăng nhập được, dừng các bước import.
- Nếu preview nhân sự có dòng sai hoặc vai trò không hợp lệ, sửa tệp trước khi execute.
- Nếu chuẩn bị import revision tiếp theo và preview báo permission/Form/role thiếu hoặc draft conflict, giải quyết đúng blocker trước khi execute. Không khôi phục permission đã nghỉ dùng chỉ để làm preview xanh.
- Nếu sau import nhân sự chưa đủ role cho Pipeline v10, không tạo sample data cho đến khi mapping Staff được xác minh.

Trạng thái tiến hành và run IDs đầy đủ được lưu tại `docs/current-states.md` cho đến khi kế hoạch hoàn tất.
