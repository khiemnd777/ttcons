# [Closed] Chặn index tt-dnh.com — 10-11-2026

**Mục tiêu/được duyệt:** Thêm noindex, kiểm tra và triển khai production; giữ crawler đọc được noindex, không Disallow toàn site.

**Đã làm:** Xác minh production thiếu noindex; /robots.txt trả SPA HTML. Thêm header Caddy và meta HTML vào source. Production đang chạy v2026-10-26.1, khác main 37c3310 và có thay đổi local không liên quan; không phát hành toàn bộ main.

**Kiểm chứng 10-11-2026 lúc 13:54 Việt Nam:** `npm run build` PASS trên working tree base 37c3310 có thay đổi local hiện hữu; bundle local không được deploy. `caddy validate` trên cấu hình patch lấy từ container production PASS; `docker compose config --quiet` PASS. Đã gắn hai file read-only từ shared/noindex-2026-11-10 vào đúng image admin digest bcb767384513c6a2d0c53557e77fd462a732ee3f0ab32aa370eeb85ad79cdb13 (v2026-10-26.1), chỉ recreate admin. curl kiểm TLS với User-Agent Googlebot: /, /login, /robots.txt, /api/health, /api/ready, /brand-logo.svg đều HTTP200 và header noindex; / và /login có meta noindex. JS production vẫn index-Si_HA7RC.js. Admin/backend healthy; ID container backend không đổi; không chạy migration hay ghi database. Bản gốc và Compose backup đã lưu theo runbook. Python urllib trên máy local thiếu CA issuer nên không dùng làm bằng chứng; curl kiểm TLS thành công, không tắt TLS verification.

**Bàn giao/còn giới hạn:** Source, mapping và runbook được tách riêng trên nhánh `bugfix/block-search-indexing` để commit/push theo yêu cầu 10-11. Release tag kế tiếp phải chứa hai thay đổi source và kiểm lại noindex để thay thế hotfix. Search Console Removals cần quyền tài khoản riêng; chưa gửi yêu cầu và chưa xác nhận Google đã gỡ kết quả. Browser/UAT NOT_RUN / NOT UX ACCEPTED; không thay đổi giao diện hiển thị.

**Đóng theo yêu cầu:** 10-11-2026. Đã commit `3f7253345f73598108ee124f895c24ec344f6f88`, push nhánh `bugfix/block-search-indexing`, tạo PR https://github.com/khiemnd777/ttcons/pull/6. Việc đóng plan không xác nhận PR đã merge hoặc Google đã gỡ kết quả. Các giới hạn kiểm chứng và việc Search Console còn lại được giữ nguyên ở trên. Kiến thức vận hành bền vững đã có tại runbook và mapping, không phụ thuộc hồ sơ đóng này.
