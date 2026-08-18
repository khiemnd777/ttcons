# [Closed] All Current States

Closed: 2026-08-19
Reason: User requested closing all current state entries.

## [Closed] UX chọn nguồn công việc cho nghiệm thu đầu ra

### Will be done

- Tự gắn nguồn khi trang Nghiệm thu được mở với `?task=<workItemId>` hợp lệ của đúng công trình.
- Bỏ dropdown toàn bộ `Công việc liên quan` khỏi phần thông tin mặc định.
- Khi cần tải ảnh/tệp mới mà chưa có nguồn, mở dialog tìm kiếm ưu tiên công việc đang xử lý/sẵn sàng và thu gọn các công việc khác.
- Giữ luồng chọn minh chứng đã có ở cấp công trình không bắt buộc nguồn công việc.
- Giữ nguyên API, payload, RBAC, giới hạn minh chứng và kiểm tra cùng công việc.
- Bổ sung kiểm thử hồi quy frontend, cập nhật mapping và chạy build.

### Done

- Review package và Mockup as Text đã được người dùng duyệt ngày 2026-08-19.
- Impact analysis xác nhận phạm vi triển khai thuần frontend; không cần migration hoặc API mới.
- Đã thêm dialog tìm kiếm/nhóm nguồn công việc và bỏ dropdown dài khỏi thông tin biên bản.
- Đã nối `?task=` với nguồn nghiệm thu khi backend xác định workspace `acceptance`.
- Đã chặn thao tác thêm ảnh/tệp tại đúng điểm cần nguồn và tiếp tục hành động ngay sau khi chọn.
- Đã bổ sung kiểm thử hồi quy cho phân nhóm, tìm kiếm không dấu và hợp đồng tích hợp UI.
- Đã cập nhật central/frontend/root-cause/domain mapping cho hành vi chọn nguồn mới.
- Đã chạy toàn bộ 161 kiểm thử frontend, production build và bundle-budget gate trên working tree hiện tại; tất cả đều đạt.
- Chưa chạy Browser/UAT theo quy định mặc định của repository; trạng thái UX vẫn là `NOT UX ACCEPTED` cho tới khi có xác nhận kiểm thử giao diện.
