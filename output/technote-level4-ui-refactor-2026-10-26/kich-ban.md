# TechNote Level 4 — Kiểm tra giao diện

Lời đọc tổng hợp: Linh vi_VN. Ảnh từ browser local, dữ liệu giả định.

## 1. Kết quả kiểm tra giao diện

Mục đích là kiểm tra giao diện đã rút gọn trên máy tính và điện thoại giả lập, rồi đối chiếu các vùng được giữ nguyên.

Trong một trăm hai mươi tám ca, một trăm hai mươi hai ca đạt tiêu chí; bốn ca mobile tràn ngang và hai ca thông báo bị chặn bởi lỗi API local.

Mục tiêu dọn text toàn phạm vi còn điểm mở, nên chưa kết luận nghiệm thu toàn bộ.

## 2. Desktop: trang, bảng và dialog

Trên desktop, tiêu đề trang gọn hơn; dữ liệu, trạng thái và nhãn hành động nghiệp vụ vẫn còn.

Các dialog đã được mở và đóng bằng dữ liệu thử có sẵn. Import nhân sự được kiểm tra bằng tài khoản Developer đã có quyền.

Các ca này không lưu, duyệt, xóa hoặc thực hiện giao dịch nghiệp vụ.

## 3. Mobile: phát hiện tràn ngang

Bốn ca kiểm tra trên mobile bị tràn ngang, tại ba bề mặt: Audit, Nhật ký sự kiện và Thảo luận.

Độ rộng nội dung lần lượt là tám trăm hai mươi, tám trăm mười bốn và năm trăm bốn mươi pixel, vượt viewport cấu hình ba trăm chín mươi pixel.

Cần đo theo viewport cấu hình vì Chrome có thể tự giãn khung bố cục. Đây là giả lập, chưa kiểm tra trên điện thoại vật lý.

## 4. Trợ giúp: mở, đóng và focus

Trợ giúp đã được kiểm tra tại ảnh đại diện, thống kê, hồ sơ kế toán và sửa lỗi trên cả hai viewport.

Nút mở được bằng bàn phím hoặc chạm. Escape đóng trợ giúp, trả focus về nút và giữ dialog cha mở.

Click ngoài và chuyển focus ra ngoài cũng đóng nội dung trợ giúp. Giới hạn nhập và cảnh báo cần thiết vẫn hiển thị.

## 5. Pipeline và Workflow Form giữ nguyên

Mười bốn đối chiếu browser cho cùng nội dung nhìn thấy với baseline trước refactor.

Các mẫu gồm Pipeline Builder, thư viện Form, quy trình công trình, tab quy trình và các thẻ hoặc Form nhúng.

So sánh source xác nhận một trăm mười hai tệp và mười hai hàm được bảo vệ không đổi. Ảnh có thể khác trạng thái hover hoặc focus; không tuyên bố mọi pixel giống nhau.

## 6. Còn copy tĩnh cần dọn tiếp

Còn tagline ở thanh trên cùng, mô tả lặp trong hai thẻ Profile và câu hướng dẫn ở đầu hai dialog báo cáo.

Đề xuất bỏ hoặc rút gọn phần lặp, chuyển giải thích bổ sung sang trợ giúp. Thanh dùng chung cần áp riêng ngoài vùng Pipeline và Form.

Các điểm này mới được ghi nhận trong UAT; chưa sửa thêm source ứng dụng.

## 7. Dialog thông báo: API local trả 500

Danh mục thông báo local trả mã HTTP năm trăm trên cả desktop và mobile.

Dialog hiển thị chưa có nhóm thông báo, nên ca tải danh mục chưa đạt. Backend vẫn là image local có sẵn.

Nguyên nhân gốc chưa xác định. Cần chẩn đoán riêng; chưa quy kết lỗi này cho refactor giao diện.

## 8. Điểm chốt và bước tiếp theo

Đã có bằng chứng browser local và đối chiếu vùng loại trừ. Còn tràn ngang mobile, copy cần dọn và lỗi danh mục thông báo cần xử lý.

Đề xuất duyệt phạm vi sửa các điểm mở, sau đó kiểm tra lại và mời người dùng đại diện nghiệm thu.

Chưa có điện thoại vật lý, chưa đo thời gian thao tác và chưa kiểm tra mọi biến thể Form. Không commit, push hoặc deploy trong lượt này.
