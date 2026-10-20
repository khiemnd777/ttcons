# Checklist release production — 2026-10-20

## Trạng thái đã kiểm chứng

| Hạng mục | Trạng thái | Bằng chứng |
| --- | --- | --- |
| Release code | PASS | `v2026-10-20.10` tại commit `3a09d16`; GitHub Actions run `39365390955` đạt CI, build image, deploy và public health. |
| Migration | PASS | Production có 158 bản ghi migration; deploy áp dụng V146–V158. Read-only inventory run `39368171516`. |
| Dọn database demo | PASS | Giao dịch run `39363771944` dọn Project/Run/Work Item; run `39368069174` dọn 20 User/Staff ngoài nhóm bảo vệ cùng session, log, notification revision, lịch sử import và cache demo. Read-only inventory run `39368171516` xác nhận trạng thái sau ghi. |
| Backup phục hồi | PASS trong phạm vi thử nghiệm | Archive V145 và archive V158 đều được khôi phục vào database tạm rồi xóa database tạm. Đây chưa phải diễn tập phục hồi toàn bộ ứng dụng. |
| Object S3 demo hiện hành | PASS trong phạm vi key hiện hành | Run `39369157932` xóa đúng 23 key từ archive và xác nhận từng key không còn đọc được; public health đạt sau restart. Chưa kiểm tra lịch sử phiên bản S3 nếu bucket bật versioning. |
| Workflow dọn demo một lần | Đã tắt trên `main` | Các workflow xóa Project, identity và object S3 được gỡ khỏi nhánh chính sau khi chạy thành công; run và tag lịch sử còn làm bằng chứng. Workflow inventory chỉ đọc và workflow phục hồi container vẫn được giữ trong giai đoạn import. |
| Admin và Developer | PASS về dữ liệu/quyền | Còn 2 Admin, 1 Developer đang active và có Staff active. Grant `developer:staff.import`, `developer:sample_data.create`, `developer:workflow_definition.import`, `admin:workflow_definition.import` đều có. Đăng nhập thực tế chưa được kiểm chứng. |
| Form/role cho Pipeline v10 local | PASS về catalog tham chiếu | 14 Form reference đúng version đều đã publish; 8 role name đều có thể gán. Read-only run `39368500697`. |
| Permission cho Pipeline v10 local | BLOCKED | Bốn key `accounting.budget_view`, `project_finance.cost_create`, `project_finance.request_create`, `project_finance.view` đã bị loại bỏ ở cả local V158 và production V158, nhưng bản Pipeline v10 đang publish tại local còn tham chiếu. File v10 thực tế của người vận hành chưa được xem trước. |

## Trình tự còn lại

1. **[Anh] Kiểm tra đăng nhập:** dùng tài khoản Admin và Developer hiện có để đăng nhập production. Không gửi mật khẩu cho agent. Nếu một vai trò không đăng nhập được, dừng import và xử lý quyền truy cập trước.
2. **[Anh] Import nhân sự:** đăng nhập bằng Developer, mở màn hình Nhân sự, chạy preview của tệp Excel/CSV. Chỉ execute khi `invalid_count=0`, kiểm tra số dòng/vai trò rồi xác nhận. Tệp import này dành cho nhân sự nghiệp vụ; không dùng nó để tạo Admin hoặc Developer. Sau import, gửi kết quả số tài khoản đã tạo, số role assignment và lỗi nếu có.
3. **[Anh và tôi] Kiểm tra file Workflow Pipeline v10:** mở import ở Pipeline Builder, chạy preview trên đúng file sẽ nhập. Gửi danh sách blocker và warning, hoặc cung cấp file v10 để tôi kiểm tra cục bộ. Không execute khi còn blocker. Bốn permission kế toán cũ trong bản local là dấu hiệu phải kiểm tra, không phải chỉ dẫn tái tạo permission đã nghỉ dùng.
4. **[Anh] Import và publish:** khi preview không có blocker, execute import. Thao tác import chỉ tạo bản nháp. Nếu package tạo Form draft thì kiểm tra và publish các Form đó trước; sau đó publish Pipeline revision mục tiêu. Xác nhận production có published v10 và không thay đổi các revision cũ.
5. **[Tôi] Xác minh sau import:** kiểm tra số User/Staff mới, role coverage, bản Form/Pipeline đã publish, migration vẫn V158, không có Project/Run/Work Item demo và các quyền bắt buộc còn hiệu lực. Không xuất danh sách nhân sự hoặc thông tin đăng nhập ra log.
6. **[Tôi] Tạo sample data theo nhân sự mới:** chỉ bắt đầu khi Staff mới và Pipeline v10 đã được xác minh. Generator hiện tại yêu cầu tài khoản mẫu riêng (`is_sample`) và sẽ không tự dùng Staff mới. Cần xác định mapping Staff theo vai trò Pipeline, xử lý vai trò thiếu/trùng và kiểm thử trước khi tạo Project mẫu. Không chạy generator hiện tại để thay thế bước này.

## Điều kiện dừng

- Nếu Admin hoặc Developer không đăng nhập được, dừng các bước import.
- Nếu preview nhân sự có dòng sai hoặc vai trò không hợp lệ, sửa tệp trước khi execute.
- Nếu preview v10 báo permission/Form/role thiếu hoặc draft conflict, giải quyết đúng blocker trước khi execute. Không khôi phục permission đã nghỉ dùng chỉ để làm preview xanh.
- Nếu publish v10 không thành công, không tạo sample data trên Pipeline v9.

Trạng thái tiến hành và run IDs đầy đủ được lưu tại `docs/current-states.md` cho đến khi kế hoạch hoàn tất.
