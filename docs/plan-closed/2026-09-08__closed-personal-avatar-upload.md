# [Closed] Upload ảnh đại diện cá nhân — 2026-09-08

## [Closed] Upload ảnh đại diện cá nhân

- Mục tiêu: cho người dùng đã xác thực tự tải/thay ảnh đại diện PNG/JPG/WebP, hiển thị tại Hồ sơ cá nhân và khu vực tài khoản trên AppShell mà không công khai object storage.
- Đã làm: đã duyệt review package/mockup; thêm migration V137, endpoint actor-owned, chuẩn hóa PNG + lưu protected storage, audit, component avatar dùng blob xác thực, dialog upload, hợp đồng `User` và kiểm thử tập trung. `go test ./...` backend, `npm run build` frontend, kiểm thử trình bày Profile (4/4), `make migrate-check` và `make migrate` đều đã đạt; V137 đã áp dụng vào PostgreSQL local lúc 2026-09-08 15:19 ICT.
- Trạng thái đóng: TECHNICALLY VERIFIED theo các kiểm tra ghi nhận; NOT UX ACCEPTED vì chưa chạy browser/UAT trong lượt này theo quy ước repository. Backend local cần được nạp lại mã mới trước khi kiểm thử endpoint trực tiếp.
- Phạm vi: `users` avatar metadata, `/auth/avatar/upload`, `/auth/avatar`, Profile/AppShell và mapping Admin Governance. Không thay đổi Staff, RBAC, phiên đăng nhập hay khả năng xóa avatar.
