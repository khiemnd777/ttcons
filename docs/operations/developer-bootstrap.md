# Thiết lập Developer qua OTP

Màn hình bootstrap xuất hiện khi không có tài khoản Developer hợp lệ ở phạm vi toàn hệ thống. Trong thời gian này, API ứng dụng bị khóa; health/readiness và các API bootstrap vẫn hoạt động. Email nhận OTP và email đăng nhập là `khiemnd777@gmail.com`. Nếu email đã thuộc tài khoản hiện có, bootstrap dừng; không tự đổi mật khẩu hoặc gán role.

## Chuẩn bị SMTP

Người vận hành cần kiểm tra chính tài khoản Zoho Mail `khiem@knasoftware.com` để xác định SMTP host theo loại tài khoản và data center. Zoho nêu cổng 587 với STARTTLS hoặc 465 với TLS. Cấu hình `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_FROM_ADDRESS=khiem@knasoftware.com`, `SMTP_TLS_MODE` và SMTP password qua cơ chế secret của môi trường; dùng application-specific password khi tài khoản Zoho yêu cầu. Không lưu secret trong Git hoặc gửi qua chat. `SETTINGS_ENCRYPTION_KEY` phải ổn định và được bảo vệ: backend dùng khóa dẫn xuất để kiểm tra OTP.

Backend dùng SMTP đã lưu trong system settings khi bất kỳ trường host, người gửi, tài khoản hoặc mật khẩu đã được cấu hình. Nếu cả bốn còn trống, bootstrap dùng cấu hình môi trường tại thời điểm khởi động. Cấu hình database chưa đầy đủ không tự chuyển sang môi trường. Thay đổi SMTP cần restart. Không cần bật `AUTH_RECOVERY_ENABLED` để bootstrap gửi OTP. Người vận hành cần xác minh thông số và kết nối Zoho trên môi trường thực; cấu hình ví dụ không chứng minh email đã được gửi hoặc nhận.

### Khung Zoho cho Docker local

File `docker-compose.zoho.local.yml` là override tùy chọn cho `docker-compose.yml`. Mặc định dùng `smtp.zoho.com:465` với TLS, tài khoản và địa chỉ gửi `khiem@knasoftware.com`. Đã kiểm tra xác thực thành công với tài khoản này trên local; host có thể khác theo loại tài khoản và data center. Override không cấu hình production và không tự gửi email.

Khi người vận hành đã có application-specific password, đặt giá trị vào một file riêng ngoài repository, chỉ cho tài khoản hệ điều hành chạy container đọc. Đặt đường dẫn file đó vào biến `ZOHO_SMTP_PASSWORD_FILE` trong cấu hình local được bảo vệ. Không đưa mật khẩu vào `.env`, lệnh shell, Git hoặc chat. Nếu chưa cấp file, override dùng `/dev/null` và bootstrap báo dịch vụ email chưa sẵn sàng. Container backend chạy theo `LOCAL_UID:LOCAL_GID`; file secret phải đọc được bởi UID đó.

Khi dùng `.env` mặc định và `.env.zoho.local` tồn tại, Makefile tự chọn override cho `make up`, `make restart`, `make down` và các lệnh Compose cùng nhóm. Khi dùng `ENV_FILE` khác hoặc cung cấp `COMPOSE` riêng, cần chọn override tường minh. Trước khi restart stack có dữ liệu, kiểm tra môi trường và ảnh hưởng: nếu database chưa có Developer hợp lệ, backend mới sẽ khóa API cho tới khi bootstrap thành công. Kiểm tra cấu hình bằng `docker compose --env-file .env --env-file .env.zoho.local -f docker-compose.yml -f docker-compose.zoho.local.yml config --quiet`; không in toàn bộ cấu hình vì các service khác có thể chứa secret.

Gửi thất bại, cấu hình không hợp lệ hoặc thiếu khóa làm bootstrap dừng. OTP dùng một lần, hết hạn sau 10 phút, tối đa 5 lần nhập sai và tối thiểu 60 giây giữa các lần gửi. Sau khi gửi thư, mã chỉ được chấp nhận khi backend đã xác nhận trạng thái lưu thành công. Khi tạo tài khoản thành công, đăng nhập bằng mật khẩu vừa đặt.

Nếu Developer cuối cùng mất điều kiện hợp lệ, hệ thống khóa lại. Nếu email bootstrap đã tồn tại, cần xử lý sự cố theo quy trình được phê duyệt riêng; không xóa dữ liệu hoặc đặt lại tài khoản chỉ để vượt bootstrap.
