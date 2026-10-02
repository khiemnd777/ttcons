# Phó Quản Lý — Deputy Manager

Role key: `deputy_manager`. Phó Quản Lý theo dõi tiến độ công ty và công việc của Giám sát. `pm` vẫn là Quản lý dự án (Project Manager).

| Phạm vi | Phó Quản Lý |
| --- | --- |
| Công trình thực tế, hiện tại và mới | Tự động có phạm vi theo dõi toàn công ty khi được gán vai trò toàn hệ thống |
| Công việc Giám sát | Xem, lọc và trao đổi; không nhận quyền thực hiện, sửa phân công hoặc phê duyệt chỉ nhờ vai trò này |
| Báo cáo Giám sát | Đồng nhận với người nhận hiện tại, xem lịch sử/minh chứng, yêu cầu bổ sung và xác nhận đã xem |
| Xác nhận đã xem | Riêng từng người và từng lần gửi; thao tác lặp không tạo lịch sử trùng |
| Yêu cầu bổ sung | Có lý do; không bị một xác nhận đã xem khác ghi đè; gửi lại phải có nội dung nguồn thay đổi |
| Tài chính và quản trị | Không tự cấp quyền Kế toán, PM, Admin hoặc Giám đốc |

Bàn làm việc có hai tab Công việc Giám sát và Báo cáo Giám sát. Các danh sách lọc tại API và tải từng trang. Chi tiết dùng các dialog và trình xem minh chứng hiện có. Trao đổi mở trang trao đổi công trình; người chỉ theo dõi toàn công ty dùng chế độ bình luận, không sửa trạng thái hay minh chứng công việc.

Migration V146 thêm vai trò, marker quyền, phân loại báo cáo và xác nhận đã xem. Báo cáo cũ được phân loại bằng vai trò chuẩn hiện có tại thời điểm migration; đây không phải bằng chứng về chức danh lịch sử. Nội dung, sự kiện và minh chứng phiên bản cũ không bị viết lại. V147–V148 bổ sung invalidation cache khi đổi quyền hoặc phạm vi dữ liệu mẫu. Dùng custom migration runner hiện có; không sửa checksum migration đã áp dụng.

Không tự chuyển tài khoản bằng tên người. Việc đổi Cường sang Phó Quản Lý và giữ Nguyệt ở PM cần kiểm kê đúng user/staff ID, quyền bổ sung, phân công đang chạy và môi trường đích, rồi thực hiện theo quyền thao tác được giao riêng. Vai trò trong file import sử dụng `deputy_manager`; trường chức danh không thay thế role code.

Xác nhận đã xem không phải duyệt chất lượng, hoàn tất Đầu việc hoặc điều kiện mới để chuyển Giai đoạn. Công trình mẫu không thuộc phạm vi theo dõi toàn công ty này.
