# Phân loại cấu hình Consflow

## Cấu hình lõi giữ trong `.env`

Các field này cần tồn tại trước khi backend có thể kết nối database, tải cấu hình runtime hoặc dựng các security boundary. UI chỉ hiển thị trạng thái/giá trị an toàn và không cho sửa.

- Runtime bootstrap: `APP_ENV`, `PORT`, `FRONTEND_ORIGIN`, `MIGRATIONS_DIR`.
- Kết nối/root secret: `DATABASE_URL`, `REDIS_URL`, `REDIS_CACHE_URL`, `JWT_SECRET`, `SETTINGS_ENCRYPTION_KEY`; từng field hỗ trợ biến `<FIELD>_FILE` thay cho giá trị trực tiếp.
- Migration: `AUTO_MIGRATE`, `MIGRATION_LOCK_TIMEOUT`, `MIGRATION_TIMEOUT`.
- Storage/deployment: `STORAGE_PROVIDER`, `STORAGE_PATH`, `STORAGE_OPERATION_TIMEOUT`, `STORAGE_MIGRATION_TIMEOUT`, `STORAGE_S3_BUCKET`, `STORAGE_S3_REGION`, `STORAGE_S3_ENDPOINT`, `STORAGE_S3_USE_PATH_STYLE`, `STORAGE_S3_ALLOW_INSECURE_ENDPOINT`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`.
- Network/security anchor: `TRUSTED_PROXY_CIDRS`, `AUTH_REFRESH_COOKIE_SECURE`.
- Biến chỉ dành cho Docker/host như bind address, port mapping, resource limit và đường dẫn secret file không thuộc `system_settings`.

## Cấu hình do Database UI quản lý

Sau migration và lần bootstrap đầu tiên, database là nguồn có thẩm quyền. Các biến môi trường cũ chỉ được dùng để import khi row chưa tồn tại; chúng không ghi đè row đã có.

- Nghiệp vụ (hiệu lực ngay): `client_payment_min_percent`, `surplus_handling_options`.
- Xác thực/khôi phục (cần restart): `AUTH_ACCESS_TOKEN_MINUTES`, `AUTH_REFRESH_TOKEN_HOURS`, `AUTH_REFRESH_COOKIE_SAME_SITE`, `AUTH_RECOVERY_ENABLED`, `AUTH_RECOVERY_TOKEN_TTL`, `AUTH_RECOVERY_VERIFY_TTL`.
- SMTP (cần restart): `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_ADDRESS`, `SMTP_FROM_NAME`, `SMTP_TLS_MODE`, `SMTP_TIMEOUT`.
- HTTP/upload (cần restart): `MAX_UPLOAD_BYTES`, `HTTP_READ_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`, `SHUTDOWN_TIMEOUT`, `READINESS_TIMEOUT`.
- Database runtime pool/timeout (cần restart): `DB_STATEMENT_TIMEOUT`, `DB_LOCK_TIMEOUT`, `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME`, `DB_CONN_MAX_IDLE_TIME`.
- Redis/cache (cần restart): `REDIS_OPERATION_TIMEOUT`, `PROJECT_EXPLORER_CACHE_MODE`, `PROJECT_EXPLORER_CACHE_TIMEOUT`, `PROJECT_EXPLORER_CACHE_INDEX_TTL`, `PROJECT_EXPLORER_CACHE_ROW_TTL`, `PROJECT_EXPLORER_CACHE_LOCK_TTL`, `PROJECT_EXPLORER_CACHE_MAX_VALUE_BYTES`.
- Rate limit (cần restart): `RATE_LIMIT_REDIS_MAX_IN_FLIGHT`, `RATE_LIMIT_ENABLED`, `RATE_LIMIT_MODE`, `RATE_LIMIT_LOCAL_MAX_KEYS` và các bộ `RATE_LIMIT_{IP|ACTOR}_{AUTH|GENERAL|UPLOAD|EXPORT|SSE}_{LIMIT|BURST|WINDOW}`.
- Concurrency/SSE (cần restart): `UPLOAD_CONCURRENCY`, `EXPORT_CONCURRENCY`, `REPORT_CONCURRENCY`, `SSE_MAX_CONNECTIONS`, `SSE_MAX_CONNECTIONS_PER_IP`, `SSE_MAX_CONNECTIONS_PER_ACTOR`, `SSE_VISIBILITY_TIMEOUT`, `SSE_MAX_CONNECTION_LIFETIME`, `SSE_BROKER_CHANNEL`.

`SMTP_PASSWORD` được mã hóa AES-GCM trước khi lưu; API chỉ trả trạng thái đã/chưa cấu hình. Root key `SETTINGS_ENCRYPTION_KEY` không được lưu trong database.

## Phân quyền

- `developer`: xem/sửa Thiết lập hệ thống và quản lý con dấu.
- `admin`: xem Thiết lập hệ thống ở chế độ chỉ đọc và quản lý con dấu.
- Vai trò khác: không được cấp `settings.view`, vì vậy không thấy menu/trang Cấu hình.

## Sao lưu và khôi phục JSON

- Admin và Developer có thể tải `GET /settings/backup`; chỉ Developer có `settings.update` để gọi preview/restore.
- Bản sao có format `consflow-system-settings`, `format_version=1`, thời điểm/môi trường nguồn, registry fingerprint và toàn bộ setting database/UI không nhạy cảm.
- File không chứa field lõi `.env`, con dấu, user/role assignment, dữ liệu công trình/nghiệp vụ, giá trị secret hoặc ciphertext. `SMTP_PASSWORD` chỉ xuất trạng thái đã/chưa cấu hình.
- Tên file theo mẫu `consflow-settings-YYYYMMDD-HHmmss.json`; upload chỉ nhận `.json` tối đa 256 KB.
- Preview kiểm tra format/version, key trùng, core/secret key, value type/range, cấu hình liên-field và chênh lệch registry/môi trường mà không ghi database.
- Field lạ được bỏ qua với cảnh báo; field không có trong file và secret hiện tại được giữ nguyên; field hiện hữu nhưng sai giá trị chặn toàn bộ restore.
- Restore gửi lại cùng file hash và expected row versions. Mọi field thay đổi được ghi trong một transaction, dùng `source=backup_restore`, audit từng field và một summary event; stale preview hoặc một lỗi bất kỳ không ghi một phần.
- Hệ thống mới phải cấu hình `.env`/Docker secrets, chạy migration và khởi động backend trước khi đăng nhập để restore. Sau đó Developer nhập lại `SMTP_PASSWORD` nếu cần và restart backend cho các setting có `requires_restart=true`.
